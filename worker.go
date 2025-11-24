package parallel

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"
)

var UserCancelled = errors.New("user-cancelled session")

type PreparationOpts struct {
	CSV            bool      `long:"csv" description:"interpret STDIN as a CSV"`
	JsonLine       bool      `long:"json-line" description:"interpret STDIN as JSON objects, one per line"`
	SkipSuccesses  bool      `long:"skip-successes" description:"skip jobs which have already been run successfully"`
	SkipFailures   bool      `long:"skip-failures" description:"skip jobs which have already been run unsuccessfully"`
	DebouncePeriod *Duration `long:"debounce" description:"re-run jobs outside the debounce period, even if they would normally be skipped"`
	DeferReruns    bool      `long:"defer-reruns" description:"give priority to jobs which have not previously been run"`
}
type ExecutionOpts struct {
	Concurrency   int64     `long:"concurrency" description:"run this many jobs in parallel" default:"10"`
	Input         *string   `long:"input" description:"send the input string (plus newline) forever as STDIN to each job"`
	Timeout       *Duration `long:"timeout" description:"cancel each job after this much time"`
	AbortOnError  bool      `long:"abort-on-error" description:"stop running (as though CTRL-C were pressed) if a job fails"`
	DryRun        bool      `long:"dry-run" description:"simulate what would be run"`
	HideSuccesses bool      `long:"hide-successes" description:"do not display a message each time a job succeeds"`
	HideFailures  bool      `long:"hide-failures" description:"do not display a message each time a job fails"`
}
type DebuggingOpts struct {
	Debug bool `long:"debug"`
}
type Opts struct {
	PreparationOpts `group:"preparation"`
	ExecutionOpts   `group:"execution"`
	DebuggingOpts
}

var (
	CacheDir   = filepath.Join(Must(os.UserHomeDir()), ".cache", "parallel")
	SuccessDir = filepath.Join(Must(os.UserHomeDir()), ".cache", "parallel", "success")
	FailureDir = filepath.Join(Must(os.UserHomeDir()), ".cache", "parallel", "failure")
)

func init() {
	Must0(os.MkdirAll(SuccessDir, 0700))
	Must0(os.MkdirAll(FailureDir, 0700))
}

func SuccessMarker(cmd RenderedCommand) string {
	h := sha256.New()
	for _, arg := range cmd.command {
		h.Write([]byte(arg))
		h.Write([]byte("\t"))
	}
	if cmd.input != "" {
		h.Write([]byte(cmd.input))
	}
	return filepath.Join(SuccessDir, fmt.Sprintf("parallel-marker-%x", h.Sum(nil)))
}

func FailureMarker(cmd RenderedCommand) string {
	h := sha256.New()
	for _, arg := range cmd.command {
		h.Write([]byte(arg))
		h.Write([]byte("\t"))
	}
	if cmd.input != "" {
		h.Write([]byte(cmd.input))
	}
	return filepath.Join(FailureDir, fmt.Sprintf("parallel-marker-%x", h.Sum(nil)))
}

type Stats struct {
	Queued     atomic.Int64
	Skipped    atomic.Int64
	InProgress atomic.Int64
	Succeeded  atomic.Int64
	Failed     atomic.Int64
	Aborted    atomic.Int64

	dirty atomic.Bool
	Total atomic.Int64

	since time.Time
	rate  *rate
}

func (s *Stats) AddSucceeded() {
	s.Succeeded.Add(1)
	s.rate.Increment(1)
	s.SetDirty()
}

func (s *Stats) AddAborted() {
	s.Aborted.Add(1)
	s.rate.Increment(1)
	s.SetDirty()
}

func (s *Stats) AddFailed() {
	s.Failed.Add(1)
	s.rate.Increment(1)
	s.SetDirty()
}

func NewStats() *Stats {
	result := Stats{rate: NewRate(100), since: time.Now()}
	result.rate.Insert(0)
	return &result
}

func (s *Stats) IsDirty() bool {
	return s.dirty.Load()
}

func (s *Stats) SetDirty() {
	s.dirty.Store(true)
}

func (s *Stats) ClearDirty() bool {
	return s.dirty.Swap(false)
}

func (s *Stats) String() string {
	etaString := ""
	if s.InProgress.Load() > 0 && s.rate != nil {
		eta, err := s.rate.ETA(4, float64(s.Total.Load()))
		if err == nil {
			etaString = time.Until(eta).Round(time.Second).String()
		}
	}
	if etaString == "" {
		return fmt.Sprintf("Queued: %v; Skipped: %v; In progress: %v; Succeeded: %v; Failed: %v; Aborted: %v; Total: %v; Elapsed time: %v",
			s.Queued.Load(),
			s.Skipped.Load(),
			s.InProgress.Load(),
			s.Succeeded.Load(),
			s.Failed.Load(),
			s.Aborted.Load(),
			s.Total.Load(),
			time.Since(s.since).Round(time.Second))
	} else {
		return fmt.Sprintf("Queued: %v; Skipped: %v; In progress: %v; Succeeded: %v; Failed: %v; Aborted: %v; Total: %v; Estimated time remaining: %v",
			s.Queued.Load(),
			s.Skipped.Load(),
			s.InProgress.Load(),
			s.Succeeded.Load(),
			s.Failed.Load(),
			s.Aborted.Load(),
			s.Total.Load(),
			etaString)
	}
}

func Worker(ctx context.Context, opts Opts, signaller <-chan os.Signal, cancel context.CancelCauseFunc, ch <-chan RenderedCommand, stats *Stats) {
	var ok bool
	var command RenderedCommand
	var cmd *exec.Cmd
	go func() {
		for sig := range signaller {
			if cmd != nil {
				if process := cmd.Process; process != nil {
					var err error
					if sig == syscall.SIGKILL {
						logger.Debug("sent kill signal", slog.Any("signal", sig), slog.Any("process", command), slog.Any("error", err))
						err = process.Kill()
					} else if sig == syscall.SIGQUIT {
						logger.Debug("sent kill signal to all subprocesses too", slog.Any("signal", sig), slog.Any("process", command), slog.Any("error", err))
						syscall.Kill(-process.Pid, syscall.SIGKILL)
					} else {
						err = process.Signal(sig)
						logger.Debug("sent signal", slog.Any("signal", sig), slog.Any("process", command), slog.Any("error", err))
					}
				}
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		select {
		case <-ctx.Done():
			return
		case command, ok = <-ch:
			if !ok {
				// channel is closed
				return
			}
		}
		logger.Debug("about to execute", slog.Any("command", command))
		subCtx := ctx
		var subCancel context.CancelFunc
		subCtx = context.Background()
		if opts.Timeout != nil {
			subCtx, subCancel = context.WithTimeout(subCtx, time.Duration(*opts.Timeout))
		}
		cmd = exec.CommandContext(subCtx, command.command[0], command.command[1:]...)

		// launch as new process group so that signals (ex: SIGINT) are not sent also the the child process
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		if command.input != "" {
			cmd.Stdin = Yes{Line: []byte(fmt.Sprintf("%v\n", command.input))}
		}

		stats.Queued.Add(-1)
		stats.InProgress.Add(1)
		stats.SetDirty()
		var err error
		var output []byte
		if opts.DryRun {
			_ = Sleep(ctx, time.Second)
			output = []byte("(dry run)")
		} else {
			output, err = cmd.CombinedOutput()
		}
		cmd = nil
		if err == nil {
			stats.Succeeded.Add(1)
			stats.rate.Increment(1)
			stats.InProgress.Add(-1)
			stats.SetDirty()
			if !opts.HideSuccesses {
				logger.Info("Success", slog.Any("command", command), slog.String("combined output", string(output)))
			}
			if !opts.DryRun {
				if err = os.WriteFile(SuccessMarker(command), []byte(output), 0644); err != nil {
					logger.Error("could not mark command as successful", slog.Any("error", err))
				}
			}
		} else {
			// the job has failed - but is it because we chose to cancel before it was done,
			// or because the job actually failed? Remember that a timeout counts as a real failure
			realFailure := subCtx.Err() == nil || errors.Is(subCtx.Err(), context.DeadlineExceeded)
			if realFailure {
				stats.AddFailed()
			} else {
				logger.Warn("job was aborted due to context cancellation", slog.Any("command", command))
				stats.AddAborted()
			}
			stats.InProgress.Add(-1)
			if !opts.HideFailures {
				logger.Warn("Failure", slog.Any("command", command), slog.String("combined output", string(output)), slog.Any("error", err))
			}
			// store the fact this failed (unless it was due to context cancellation)
			if !opts.DryRun && realFailure {
				if err = os.WriteFile(FailureMarker(command), []byte(output), 0644); err != nil {
					logger.Error("could not mark command as failed", slog.Any("error", err))
				}
			}
			if cancel != nil && opts.AbortOnError {
				cancel(errors.New("nonzero exit code"))
			}
		}
		if subCancel != nil {
			subCancel()
		}
	}
}
