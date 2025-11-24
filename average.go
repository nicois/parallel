package parallel

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/exp/constraints"
)

type Sample[T constraints.Float] struct {
	value     T
	timestamp time.Time
}

type rate[T constraints.Float] struct {
	samples      []Sample[T]
	capacity     int
	oldest, next int
	mutex        *sync.RWMutex
}

func NewRate[T constraints.Float](maxSamples int) *rate[T] {
	result := &rate[T]{
		samples:  make([]Sample[T], maxSamples),
		capacity: maxSamples,
		mutex:    new(sync.RWMutex),
	}
	return result
}

func (l *rate[T]) Increment(v T) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	newest := l.next - 1
	if newest < 0 {
		newest = l.capacity - 1
	}

	l.samples[l.next] = Sample[T]{value: l.samples[newest].value + v, timestamp: time.Now()}
	if l.oldest == (l.next+1)%l.capacity {
		l.oldest = (l.oldest + 1) % l.capacity
	}
	l.next = (l.next + 1) % l.capacity
}

func (l *rate[T]) Insert(v T) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.samples[l.next] = Sample[T]{value: v, timestamp: time.Now()}
	if l.oldest == (l.next+1)%l.capacity {
		l.oldest = (l.oldest + 1) % l.capacity
	}
	l.next = (l.next + 1) % l.capacity
}

// ETA returns the expected amount of time before reaching the target value
func (l *rate[T]) ETA(minimumSamples int, target T) (time.Time, error) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	if minimumSamples < 2 {
		return time.Time{}, errors.New("minimum samples must be at least 2")
	}
	if l.oldest == l.next {
		return time.Time{}, errors.New("no sample data")
	}
	nSamples := (l.next - l.oldest + l.capacity) % l.capacity
	if nSamples < minimumSamples {
		return time.Time{}, fmt.Errorf("only have %v of the required %v samples", nSamples, minimumSamples)
	}
	newest := l.next - 1
	if newest < 0 {
		newest += l.capacity
	}
	change := l.samples[newest].value - l.samples[l.oldest].value
	period := l.samples[newest].timestamp.Sub(l.samples[l.oldest].timestamp)
	if change == 0 {
		return time.Time{}, errors.New("no difference in sample values")
	}

	if target == l.samples[newest].value {
		return l.samples[newest].timestamp, nil
	}
	if (change > 0) != (target > l.samples[newest].value) {
		return time.Time{}, fmt.Errorf("change is %v at %v but target is %v", change, l.samples[newest].value, target)
	}
	scale := (target - l.samples[newest].value) / (l.samples[newest].value - l.samples[l.oldest].value)
	d := time.Duration(T(period) * scale)
	return l.samples[newest].timestamp.Add(d), nil
}
