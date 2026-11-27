// Package scheduler runs non-overlapping interval jobs. Use an external durable lease for replica coordination.
package scheduler

import (
	"context"
	"errors"
	"time"
)

type Lease interface {
	RunIfOwner(context.Context, string, func(context.Context) error) (bool, error)
}
type Config struct {
	Name              string
	Interval, Timeout time.Duration
	Immediate         bool
	Lease             Lease
	OnError           func(error)
}

func Run(ctx context.Context, c Config, job func(context.Context) error) error {
	if c.Name == "" || c.Interval <= 0 || c.Timeout <= 0 || job == nil {
		return errors.New("invalid schedule")
	}
	timer := time.NewTimer(c.Interval)
	defer timer.Stop()
	run := func() {
		r, cancel := context.WithTimeout(ctx, c.Timeout)
		defer cancel()
		var err error
		if c.Lease != nil {
			_, err = c.Lease.RunIfOwner(r, c.Name, job)
		} else {
			err = job(r)
		}
		if err == nil {
			err = r.Err()
		}
		if err != nil && c.OnError != nil {
			c.OnError(err)
		}
	}
	if c.Immediate && ctx.Err() == nil {
		run()
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(c.Interval)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			if ctx.Err() != nil {
				return ctx.Err()
			}
			run()
			timer.Reset(c.Interval)
		}
	}
}
