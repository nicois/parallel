package parallel

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"
)

type PreparationOpts struct {
	CSV      bool `long:"csv" description:"interpret STDIN as a CSV"`
	JsonLine bool `long:"json-line" description:"interpret STDIN as JSON objects, one per line"`
	Shuffle  bool `long:"shuffle" description:"run the jobs in a random order"`
}
type ExecutionOpts struct {
	Concurrency    int64     `long:"concurrency" description:"number of jobs to run in parallel" default:"10"`
	DebouncePeriod *Duration `long:"debounce" description:"also re-run successful jobs unless within the debounce period"`
	GracefulExit   bool      `long:"graceful-exit" description:"wait for current jobs to finish before exiting due to an interrupt"`
	Input          *string   `long:"input" description:"send the input string (plus newline) forever as STDIN to each job"`
	Timeout        *Duration `long:"timeout" description:"maximum time a job may run for before being cancelled"`
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
	for _, arg := range cmd {
		h.Write([]byte(arg))
		h.Write([]byte("\t"))
	}
	return filepath.Join(SuccessDir, fmt.Sprintf("parallel-marker-%x", h.Sum(nil)))
}

func FailureMarker(cmd RenderedCommand) string {
	h := sha256.New()
	for _, arg := range cmd {
		h.Write([]byte(arg))
		h.Write([]byte("\t"))
	}
	return filepath.Join(FailureDir, fmt.Sprintf("parallel-marker-%x", h.Sum(nil)))
}

type Stats struct {
	Submitted  atomic.Int64
	Skipped    atomic.Int64
	InProgress atomic.Int64
	Succeeded  atomic.Int64
	Failed     atomic.Int64

	dirty atomic.Bool

	Total int64
	rate  *rate[float64]
}

func (s *Stats) AddSucceeded() {
	s.Succeeded.Add(1)
	s.rate.Increment(1)
	s.SetDirty()
}

func (s *Stats) AddFailed() {
	s.Failed.Add(1)
	s.rate.Increment(1)
	s.SetDirty()
}

func NewStats() *Stats {
	result := Stats{rate: NewRate[float64](100)}
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
	etaString := "?"
	if s.rate != nil {
		eta, err := s.rate.ETA(4, float64(s.Total))
		if err == nil {
			etaString = time.Until(eta).Round(time.Second).String()
			// } else { etaString = fmt.Sprintf("%v", err)
		}
	}
	return fmt.Sprintf("Submitted: %v; Skipped: %v; In progress: %v; Succeeded: %v; Failed: %v; Total: %v; Estimated time remaining: %v",
		s.Submitted.Load(),
		s.Skipped.Load(),
		s.InProgress.Load(),
		s.Succeeded.Load(),
		s.Failed.Load(),
		s.Total,
		etaString)
}

func Worker(ctx context.Context, opts Opts, ch <-chan RenderedCommand, stats *Stats) {
	var ok bool
	var command RenderedCommand
	var stdin io.Reader
	if opts.Input != nil {
		stdin = Yes{Line: []byte(fmt.Sprintf("%v\n", *opts.Input))}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case command, ok = <-ch:
			if !ok {
				// channel is closed
				return
			}
		}
		subCtx := ctx
		var cancel context.CancelFunc
		if opts.GracefulExit {
			// shield the command from context cancellation
			subCtx = context.Background()
		}
		if opts.Timeout != nil {
			subCtx, cancel = context.WithTimeout(subCtx, time.Duration(*opts.Timeout))
		}
		cmd := exec.CommandContext(subCtx, command[0], command[1:]...)

		// launch as new process group so that signals (ex: SIGINT) are not sent also the the child process
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		cmd.Stdin = stdin
		stats.InProgress.Add(1)
		stats.SetDirty()
		if output, err := cmd.CombinedOutput(); err == nil {
			stats.Succeeded.Add(1)
			stats.rate.Increment(1)
			stats.InProgress.Add(-1)
			stats.SetDirty()
			logger.Info("Success", slog.Any("command", command), slog.String("combined output", string(output)))
			if err = os.WriteFile(SuccessMarker(command), []byte(output), 0644); err != nil {
				logger.Error("could not mark command as successful", slog.Any("error", err))
			}
		} else {
			if subCtx.Err() != nil {
				logger.Warn("job was aborted due to context cancellation", slog.Any("command", command))
			}
			stats.Failed.Add(1)
			stats.rate.Increment(1)
			stats.InProgress.Add(-1)
			stats.SetDirty()
			logger.Info("Failure", slog.Any("command", command), slog.String("combined output", string(output)), slog.Any("error", err))
			if err = os.WriteFile(FailureMarker(command), []byte(output), 0644); err != nil {
				logger.Error("could not mark command as failed", slog.Any("error", err))
			}
		}
		if cancel != nil {
			cancel()
		}
	}
}
