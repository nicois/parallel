package parallel

import (
	"sync"
	"time"
)

type etc struct {
	successes []time.Duration
	failures  []time.Duration
	mutex     *sync.RWMutex
}

func (e *etc) Estimate(stats *Stats) time.Duration {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	// assume concurrency is the same as the number in-progress.
	// This is only incorrect when the queue is empty, when it doesn't matter
	if len(e.failures)+len(e.successes) == 0 {
		return time.Duration(0)
	}
	pSuccess := float64(len(e.successes)) / float64(len(e.successes)+len(e.failures))
	var meanSuccess time.Duration
	var meanFailure time.Duration
	var maxSuccess time.Duration
	var maxFailure time.Duration
	for _, success := range e.successes {
		if success > maxSuccess {
			maxSuccess = success
		}
		meanSuccess = meanSuccess + success
	}
	for _, failure := range e.failures {
		if failure > maxFailure {
			maxFailure = failure
		}
		meanFailure = meanFailure + failure
	}
	// convert from total duration to average duration
	meanSuccess = meanSuccess / time.Duration(len(e.successes))
	if len(e.failures) > 0 {
		meanFailure = meanFailure / time.Duration(len(e.failures))
	}

	// weighted mean job duration:
	wDurationSeconds := (meanSuccess*time.Duration(pSuccess) + meanFailure*time.Duration(1-pSuccess)).Seconds()
	// fudge the weighted duration if we have fewer samples than WIP. as we are biased towards the jobs which take less time
	if wip := stats.InProgress.Load(); int(wip) > len(e.successes)+len(e.failures) {
		wDurationSeconds *= float64(wip) / float64(len(e.successes)+len(e.failures))
	}

	// weighted max time
	wMaxDuration := time.Duration((maxSuccess.Seconds()*pSuccess + maxFailure.Seconds()*(1-pSuccess)) * float64(time.Second))
	// var qet time.Duration
	if stats.queueEmptyTime.IsZero() {
		// estimate queue empty time: number of queued items * weighted job run time

		qet := time.Duration(wDurationSeconds * float64(stats.Queued.Load()) / float64(stats.InProgress.Load()) * float64(time.Second))
		return qet + wMaxDuration
	}
	return wMaxDuration - time.Since(stats.queueEmptyTime)
}

func NewEtc() *etc {
	return &etc{successes: make([]time.Duration, 0, 100), failures: make([]time.Duration, 0, 100), mutex: new(sync.RWMutex)}
}

func (e *etc) AddSuccess(d time.Duration) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.successes = append(e.successes, d)
}

func (e *etc) AddFailure(d time.Duration) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.successes = append(e.failures, d)
}
