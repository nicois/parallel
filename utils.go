package parallel

import (
	"context"
	"time"
)

// Must panics if it is given a non-nil error.
// Otherwise, it returns the first argument
func Must[T any](result T, err error) T {
	if err != nil {
		panic(err)
	}
	return result
}

// Must0 panics if it is given a non-nil error.
func Must0(err error) {
	if err != nil {
		panic(err)
	}
}

func SleepInLockstep(ctx context.Context, duration time.Duration) error {
	targetTime := time.Now().Round(duration)
	if !time.Now().Before(targetTime) {
		targetTime = targetTime.Add(duration)
	}
	return SleepUntil(ctx, targetTime)
}

func SleepUntil(ctx context.Context, t time.Time) error {
	duration := time.Until(t)
	if duration < time.Millisecond {
		return nil
	}
	return Sleep(ctx, duration)
}

func Sleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
