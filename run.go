package parallel

import (
	"context"
	"errors"
	"sync"
	"time"
)

func Run(ctx context.Context, stats *Stats, opts Opts, commands []RenderedCommand) error {
	if len(commands) == 0 {
		return errors.New("no commands were provided")
	}
	wg := &sync.WaitGroup{}
	ch := make(chan RenderedCommand)
	wg.Go(func() {
		defer close(ch)
		for _, command := range commands {
			select {
			case <-ctx.Done():
				return
			case ch <- command:
			}
			stats.Submitted.Add(1)
			stats.SetDirty()
		}
	})
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		for {
			<-ticker.C
			if stats.ClearDirty() {
				logger.Info(stats.String())
			}
		}
	}()

	for range opts.Concurrency {
		wg.Go(func() {
			Worker(ctx, opts, ch, stats)
		})
	}
	wg.Wait()
	return nil
}
