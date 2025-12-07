package main

import (
	"bufio"
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/btree"
	"github.com/jessevdk/go-flags"
	"github.com/lmittmann/tint"
	"github.com/nicois/parallel"
)

var logger *slog.Logger

type UnsortedCommand struct {
	command   parallel.RenderedCommand
	timestamp time.Time
	index     uint64
}

func main() {
	// collect command-line options
	var opts parallel.Opts
	commandLine, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(1)
	}

	// set up the logger
	var handler slog.Handler
	handlerOptions := tint.Options{}
	if opts.Debug {
		handlerOptions.Level = slog.LevelDebug
		handlerOptions.AddSource = true
	} else {
		handlerOptions.Level = slog.LevelInfo
	}
	handler = tint.NewHandler(os.Stdout, &handlerOptions)
	logger = slog.New(handler)
	parallel.SetLogger(logger)

	// listen for signals
	// to support escalation, do not simply use NotifyContext
	interruptChannel := make(chan os.Signal, 2)
	ctx, cancelCause := context.WithCancelCause(context.Background())
	defer cancelCause(nil)
	signal.Notify(interruptChannel, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	// provide stub commands if required
	if len(commandLine) == 0 {
		if opts.CSV || opts.JsonLine {
			commandLine = []string{"echo", "foo is {{.foo}}, bar is {{.bar}}"}
		} else {
			commandLine = []string{"echo", "value is {{.value}}"}
		}
		logger.Info("no command was provided, so just echoing the input", slog.Any("commandline", commandLine))
	}

	// prepare for processing STDIN
	reader := bufio.NewReader(os.Stdin)
	var generator parallel.Generator

	if opts.JsonLine {
		generator = parallel.JsonLineGenerator
	} else if opts.CSV {
		generator = parallel.CsvGenerator
	} else {
		generator = parallel.SimpleLineGenerator
	}
	templ, err := parallel.ParseCommandline(commandLine)
	var input *template.Template
	if inputString := opts.Input; inputString != nil {
		if t, err := template.New("Input").Parse(*inputString); err == nil {
			input = t
		} else {
			logger.Error("cannot parse the input template", slog.Any("error", err))
			os.Exit(1)
		}
	}
	if err != nil {
		logger.Error("Fatal error while parsing the commandline", slog.Any("error", err))
		os.Exit(1)
	}

	// initialise the stats collector
	stats := parallel.NewStats()

	// this channel is where we insert jobs we want to do,
	presortedCommands := make(chan UnsortedCommand, 10)

	// this channel provides the workers with the highest priority next job
	postSortedCommands := make(chan parallel.RenderedCommand)

	// ingest STDIN, generating commands and updating stats
	go func() {
		defer close(presortedCommands)
		var index uint64
		for args := range generator(ctx, cancelCause, reader) {
			var mostRecentlyLastRun time.Time
			renderedCommand, err := parallel.Render(templ, input, args)
			if err != nil {
				logger.Info("could not render", slog.Any("error", err))
				stats.AddFailed(0)
				continue
			}
			marker := parallel.SuccessMarker(renderedCommand)
			if stat, err := os.Stat(marker); err == nil {
				if stat.ModTime().After(mostRecentlyLastRun) {
					mostRecentlyLastRun = stat.ModTime()
				}
				if opts.SkipSuccesses {
					// skip jobs previously run successfully, unless outside of the debounce period
					if period := time.Since(stat.ModTime()); opts.DebouncePeriod != nil && period > time.Duration(*opts.DebouncePeriod) {
						logger.Debug("already successfully executed, but outside the debounce period", slog.Any("command", renderedCommand))
					} else {
						logger.Debug("already successfully executed", "command", renderedCommand, slog.String("cached combined output file", marker))
						stats.Skipped.Add(1)
						continue
					}
				}
			}
			marker = parallel.FailureMarker(renderedCommand)
			if stat, err := os.Stat(marker); err == nil {
				if stat.ModTime().After(mostRecentlyLastRun) {
					mostRecentlyLastRun = stat.ModTime()
				}
				if opts.SkipFailures {
					// skip jobs previously run unsuccessfully, unless outside of the debounce period
					if period := time.Since(stat.ModTime()); opts.DebouncePeriod != nil && period > time.Duration(*opts.DebouncePeriod) {
						logger.Debug("already unsuccessfully executed, but outside the debounce period", slog.Any("command", renderedCommand))
					} else {
						logger.Debug("already unsuccessfully executed", "command", renderedCommand, slog.String("cached combined output file", marker))
						stats.Skipped.Add(1)
						continue
					}
				}
			}
			if !opts.DeferReruns {
				// treat all times as Zero, meaning that the original sequence is preserved in the btree, via the index
				mostRecentlyLastRun = time.Time{}
			}
			index = index + 1
			select {
			case <-ctx.Done():
				return
			case presortedCommands <- UnsortedCommand{command: renderedCommand, timestamp: mostRecentlyLastRun, index: index}:
				logger.Debug("inserted unsorted command", slog.Any("command", renderedCommand))
			}
			stats.Total.Add(1)
			stats.AddQueued()
		}
	}()

	go sorter(ctx, presortedCommands, postSortedCommands)

	// call the main entrypoint, now everything is in place
	err = parallel.Run(ctx, stats, interruptChannel, opts, postSortedCommands)

	// provide a summary before exiting
	logger.Info(stats.String())

	// show exit reasons
	if err != nil {
		if err != parallel.UserCancelled {
			logger.Error(fmt.Sprintf("%v", err))
		}
		os.Exit(1)
	}
	select {
	case <-ctx.Done():
		if err := context.Cause(ctx); err != nil {
			if err != parallel.UserCancelled {
				logger.Error(fmt.Sprintf("! %v", err))
			}
			os.Exit(1)
		}
		if err := ctx.Err(); err != nil {
			logger.Error(fmt.Sprintf("- %v", err))
			os.Exit(1)
		}
		logger.Debug("exiting")
	default:
	}
}

func lessUnsortedCommand(a, b UnsortedCommand) bool {
	if a.timestamp.Equal(b.timestamp) {
		return a.index < b.index
	}
	return a.timestamp.Before(b.timestamp)
}

func sorter(ctx context.Context, presortedCommands <-chan UnsortedCommand, postSortedCommands chan<- parallel.RenderedCommand) {
	// hold a sorted representation of the commands
	tree := btree.NewG(2, lessUnsortedCommand)

	// ensure reading and writing don't interfere with each other
	mutex := new(sync.RWMutex)

	// notify at least one item is in the btree
	youHaveMail := make(chan struct{})

	// insert new items into the btree
	go func() {
		var minitime time.Time
		defer close(youHaveMail)
		for {
			select {
			case <-ctx.Done():
				return
			case uc, ok := <-presortedCommands:
				if !ok {
					// channel is closed; nothing more is incoming
					return
				}
				if uc.timestamp.IsZero() {
					// similate a very old time, but not identical to other very old times
					minitime = minitime.Add(time.Nanosecond)
					uc.timestamp = minitime
				}
				// insert the command into the btree
				mutex.Lock()

				_, replaced := tree.ReplaceOrInsert(uc)
				logger.Debug("inserted into BTREE", slog.Any("command", uc))
				mutex.Unlock()
				if replaced {
					panic("this should not happen")
				}
				// notify anyone who cares that new data is available
				select {
				case youHaveMail <- struct{}{}:
				default:
				}
			}
		}
	}()

	// postSortedCommands is the channel which yields the jobs
	// which are ready to run as soon as a worker is available
	defer close(postSortedCommands)

	// sleep a teeny tiny amount to allow the BTREE a chance to get populated
	// with higher-priority jobs. Otherwise, the first jobs to be inserted will
	// immediately be retrieved, even if they are lower priority than jobs
	// inserted a ms later
	_ = parallel.Sleep(ctx, 100*time.Millisecond)

	for {
		var finalIteration bool

		// wait for at least one item to be in the btree
		select {
		case <-ctx.Done():
			return
		case _, ok := <-youHaveMail:
			if !ok {
				finalIteration = true
			}
		}
		// keep sending the oldest known item until the tree
		// is empty or the context is cancelled
		for {
			mutex.Lock()
			uc, found := tree.DeleteMin()
			mutex.Unlock()
			if !found {
				if finalIteration {
					return
				}
				break
			}
			select {
			case <-ctx.Done():
				return

				// this is a zero-length channel and will
				// mostly be blocked as all workers will be busy.
			case postSortedCommands <- uc.command:
				logger.Debug("inserted into queue", slog.Any("command", uc))
			}
		}
	}
}
