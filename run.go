package parallel

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

func Run(ctx context.Context, stats *Stats, opts Opts, commands <-chan RenderedCommand) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	// Show the current status, every 10ish seconds
	go func() {
		_ = SleepInLockstep(ctx, 10*time.Second)
		ticker := time.NewTicker(10 * time.Second)
		var lastShown time.Time
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if stats.ClearDirty() || time.Since(lastShown) >= time.Minute {
				logger.Info(stats.String())
				lastShown = time.Now()
			}
			<-ticker.C
		}
	}()

	// Provide user feedback when starting the exit process, but waiting for running jobs
	if opts.GracefulExit {
		go func() {
			<-ctx.Done()
			if err := context.Cause(ctx); err != ctx.Err() {
				logger.Info("received cancellation signal. Waiting for current jobs to finish before exiting", slog.Any("error", err))
				if stats.ClearDirty() {
					logger.Info(stats.String())
				}
			}
		}()
	}

	// spawn the workers
	wg := &sync.WaitGroup{}
	for range opts.Concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Worker(ctx, opts, cancel, commands, stats)
		}()
	}

	wg.Wait()
	return context.Cause(ctx)
}
