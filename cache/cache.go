// Package cache provides typed cache-aside with process-local duplicate load suppression.
package cache

import (
	"context"
	"errors"
	"math/rand/v2"
	"sync"
	"time"
)

type Store interface {
	GetJSON(context.Context, string, any) error
	SetJSON(context.Context, string, any, time.Duration) error
}
type call[T any] struct {
	done chan struct{}
	v    T
	err  error
}
type Loader[T any] struct {
	store   Store
	miss    error
	mu      sync.Mutex
	calls   map[string]*call[T]
	timeout time.Duration
	max     int
}

func New[T any](s Store, miss error, timeout time.Duration, maxInflight int) (*Loader[T], error) {
	if s == nil || miss == nil || timeout <= 0 || maxInflight < 1 {
		return nil, errors.New("invalid cache loader")
	}
	return &Loader[T]{store: s, miss: miss, timeout: timeout, max: maxInflight, calls: map[string]*call[T]{}}, nil
}

// GetOrLoad returns cache errors (no implicit fail-open). The first caller supplies the shared load context. Values should be immutable.
func (l *Loader[T]) GetOrLoad(ctx context.Context, key string, ttl time.Duration, load func(context.Context) (T, error)) (v T, err error) {
	if key == "" || ttl < time.Millisecond || ttl > 365*24*time.Hour || load == nil {
		return v, errors.New("invalid cache load")
	}
	if err = l.store.GetJSON(ctx, key, &v); err == nil {
		return v, nil
	} else if !errors.Is(err, l.miss) {
		return v, err
	}
	l.mu.Lock()
	if c := l.calls[key]; c != nil {
		l.mu.Unlock()
		select {
		case <-ctx.Done():
			return v, ctx.Err()
		case <-c.done:
			return c.v, c.err
		}
	}
	if len(l.calls) >= l.max {
		l.mu.Unlock()
		return v, errors.New("cache load capacity exceeded")
	}
	c := &call[T]{done: make(chan struct{})}
	l.calls[key] = c
	l.mu.Unlock()
	defer func() {
		if p := recover(); p != nil {
			c.err = errors.New("cache loader panicked")
			l.mu.Lock()
			delete(l.calls, key)
			close(c.done)
			l.mu.Unlock()
			panic(p)
		}
		c.v, c.err = v, err
		l.mu.Lock()
		delete(l.calls, key)
		close(c.done)
		l.mu.Unlock()
	}()
	run, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()
	if err = l.store.GetJSON(run, key, &v); err == nil {
		return v, nil
	} else if !errors.Is(err, l.miss) {
		return v, err
	}
	v, err = load(run)
	if err != nil {
		return v, err
	}
	return v, l.store.SetJSON(run, key, v, Jitter(ttl))
}

// Jitter subtracts up to 10% to spread expirations without exceeding the requested TTL.
func Jitter(ttl time.Duration) time.Duration {
	if ttl < 10*time.Millisecond {
		return ttl
	}
	return ttl - time.Duration(rand.Int64N(int64(ttl/10)+1))
}
