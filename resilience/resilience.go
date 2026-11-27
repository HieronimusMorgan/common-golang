// Package resilience supplies bounded retry and concurrency-safe circuit breaking.
package resilience

import (
	"context"
	"errors"
	"math/rand/v2"
	"sync"
	"time"
)

// Retry executes only operations safe to repeat. retryable must classify transient errors.
func Retry(ctx context.Context, attempts int, base, maxDelay time.Duration, retryable func(error) bool, fn func(context.Context) error) error {
	if attempts < 1 || attempts > 20 || base <= 0 || maxDelay < base || retryable == nil || fn == nil {
		return errors.New("invalid retry policy")
	}
	delay := base
	for i := 0; i < attempts; i++ {
		if e := ctx.Err(); e != nil {
			return e
		}
		e := fn(ctx)
		if e == nil {
			return nil
		}
		if i == attempts-1 || !retryable(e) {
			return e
		}
		wait := time.Duration(rand.Int64N(int64(delay)))
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
		if delay > maxDelay/2 {
			delay = maxDelay
		} else {
			delay *= 2
		}
	}
	return nil
}

var ErrOpen = errors.New("circuit open")

type Breaker struct {
	mu         sync.Mutex
	threshold  int
	cooldown   time.Duration
	failures   int
	until      time.Time
	probe      bool
	generation uint64
}

func NewBreaker(threshold int, cooldown time.Duration) (*Breaker, error) {
	if threshold < 1 || cooldown <= 0 {
		return nil, errors.New("invalid breaker policy")
	}
	return &Breaker{threshold: threshold, cooldown: cooldown}, nil
}

// Do permits one half-open probe. A panic counts as failure and propagates. classify should return true for dependency failures.
func (b *Breaker) Do(ctx context.Context, classify func(error) bool, fn func(context.Context) error) (err error) {
	if fn == nil || classify == nil {
		return errors.New("callbacks required")
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	half := !b.until.IsZero()
	if half && (time.Now().Before(b.until) || b.probe) {
		b.mu.Unlock()
		return ErrOpen
	}
	if half {
		b.probe = true
	}
	gen := b.generation
	b.mu.Unlock()
	failed := true
	defer func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if gen != b.generation {
			return
		}
		if half {
			b.probe = false
			if failed {
				b.until = time.Now().Add(b.cooldown)
			} else {
				b.until = time.Time{}
				b.failures = 0
			}
			b.generation++
			return
		}
		if failed {
			b.failures++
			if b.failures >= b.threshold {
				b.until = time.Now().Add(b.cooldown)
				b.generation++
			}
		} else {
			b.failures = 0
		}
	}()
	err = fn(ctx)
	failed = err != nil && classify(err)
	return err
}
