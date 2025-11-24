package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/lmittmann/tint"
	"github.com/nicois/parallel"
)

var logger *slog.Logger

func main() {
	var opts parallel.Opts
	commandLine, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(1)
	}
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
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()
	ctx, cancelCause := context.WithCancelCause(ctx)
	defer cancelCause(nil)

	if len(commandLine) == 0 {
		if opts.CSV || opts.JsonLine {
			commandLine = []string{"echo", "foo is {{.foo}}, bar is {{.bar}}"}
		} else {
			commandLine = []string{"echo", "value is {{.value}}"}
		}
		logger.Info("no command was provided, so just echoing the input", slog.Any("commandline", commandLine))
	}
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
	if err != nil {
		logger.Error("Fatal error while parsing the commandline", slog.Any("error", err))
		os.Exit(1)
	}

	stats := parallel.NewStats()
	commands := make([]parallel.RenderedCommand, 0, 1000)

	for args := range generator(ctx, cancelCause, reader) {
		renderedCommand, err := parallel.Render(templ, args)
		if err != nil {
			logger.Info("could not render", slog.Any("error", err))
			stats.AddFailed()
			continue
		}
		marker := parallel.SuccessMarker(renderedCommand)
		if stat, err := os.Stat(marker); err == nil {
			if period := time.Since(stat.ModTime()); opts.DebouncePeriod != nil && period > time.Duration(*opts.DebouncePeriod) {
				logger.Debug("already successfully executed, but outside the debounce period", slog.Any("command", renderedCommand))
			} else {
				logger.Debug("already successfully executed", "command", renderedCommand, slog.String("cached combined output file", marker))
				stats.Skipped.Add(1)
				continue
			}
		}
		commands = append(commands, renderedCommand)
		stats.Total++
	}

	if opts.Shuffle {
		rand.Shuffle(len(commands), func(i, j int) {
			commands[i], commands[j] = commands[j], commands[i]
		})
	}

	err = parallel.Run(ctx, stats, opts, commands)
	if err != nil {
		logger.Error("Fatal error", slog.Any("error", err))
		os.Exit(1)
	}
	logger.Info(stats.String())
	select {
	case <-ctx.Done():
		if err := context.Cause(ctx); err != nil {
			logger.Error(fmt.Sprintf("%v", err))
			os.Exit(1)
		}
	default:
	}
}
