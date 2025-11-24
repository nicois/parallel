package parallel

import (
	"context"
	"errors"
	"os"
	"sync"
	"syscall"
	"time"
)

func Run(ctx context.Context, stats *Stats, interruptChannel <-chan os.Signal, opts Opts, commands <-chan RenderedCommand) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	// Show the current status, every 10ish seconds
	go func() {
		_ = SleepInLockstep(ctx, 10*time.Second)
		ticker := time.NewTicker(10 * time.Second)
		var lastShown time.Time
	loop:
		for {
			select {
			case <-ctx.Done():
				break loop
			default:
			}
			if stats.ClearDirty() || time.Since(lastShown) >= time.Minute {
				logger.Info(stats.String())
				lastShown = time.Now()
			}
			select {
			case <-ctx.Done():
				break loop
			case <-ticker.C:
			}
		}
		ticker.Stop()
		_ = SleepInLockstep(context.Background(), time.Second)
		ticker = time.NewTicker(time.Second)
		for {
			if stats.ClearDirty() || time.Since(lastShown) >= time.Minute {
				logger.Info(stats.String())
				lastShown = time.Now()
			}
			<-ticker.C
		}
	}()

	signallers := make([]chan os.Signal, 0, opts.Concurrency)

	// spawn the workers
	wg := &sync.WaitGroup{}
	for range opts.Concurrency {
		signaller := make(chan os.Signal, 2)
		signallers = append(signallers, signaller)
		wg.Add(1)
		go func() {
			defer wg.Done()
			Worker(ctx, opts, signaller, cancel, commands, stats)
		}()
	}

	// Provide user feedback when starting the exit process, but waiting for running jobs
	go func() {
		select {
		case <-interruptChannel:
			logger.Warn("received cancellation signal. Waiting for current jobs to finish before exiting. Hit CTRL-C again to exit sooner")
			if stats.ClearDirty() {
				logger.Info(stats.String())
			}
			stats.Total.Add(-1 * stats.Queued.Swap(0))
			stats.SetDirty()
			cancel(errors.New("user-initiated shutdown"))
		case <-ctx.Done():
			return
		}

		<-interruptChannel
		for _, signaller := range signallers {
			select {
			case signaller <- syscall.SIGTERM:
			default:
			}
		}
		logger.Warn("second CTRL-C received. Sending SIGTERM to running jobs. Hit CTRL-C again to use SIGKILL instead")

		<-interruptChannel
		for _, signaller := range signallers {
			select {
			case signaller <- syscall.SIGKILL:
			default:
			}
		}
		logger.Warn("third CTRL-C received. Sending SIGKILL to running jobs. Hit CTRL-C again to kill all subprocesses too")

		<-interruptChannel
		for _, signaller := range signallers {
			select {
			case signaller <- syscall.SIGQUIT:
			default:
			}
			close(signaller)
		}
		logger.Warn("fourth CTRL-C received. Sending SIGKILL to running jobs and their subprocesses")
	}()

	wg.Wait()
	return context.Cause(ctx)
}
