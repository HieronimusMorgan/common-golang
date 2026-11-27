// Package worker executes bounded concurrent tasks. Durable job ownership belongs to the source.
package worker

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Job func(context.Context) error
type Pool struct {
	Concurrency int
	Timeout     time.Duration
	OnError     func(error)
}

// Run stops accepting work on cancellation, waits for active callbacks, and returns. Jobs and OnError must honor cancellation / return promptly. No goroutine is forcibly killed.
func (p Pool) Run(ctx context.Context, jobs <-chan Job) error {
	if p.Concurrency < 1 || p.Concurrency > 1024 || p.Timeout <= 0 || jobs == nil {
		return errors.New("invalid worker pool")
	}
	var wg sync.WaitGroup
	var report sync.Mutex
	for i := 0; i < p.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case j, ok := <-jobs:
					if !ok || ctx.Err() != nil {
						return
					}
					run, cancel := context.WithTimeout(ctx, p.Timeout)
					err := execute(run, j)
					if err == nil {
						err = run.Err()
					}
					cancel()
					if err != nil && p.OnError != nil {
						report.Lock()
						p.OnError(err)
						report.Unlock()
					}
				}
			}
		}()
	}
	wg.Wait()
	return ctx.Err()
}
func execute(ctx context.Context, j Job) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("job panicked")
		}
	}()
	if j == nil {
		return errors.New("nil job")
	}
	return j(ctx)
}
