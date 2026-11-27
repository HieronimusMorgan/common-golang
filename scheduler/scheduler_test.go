package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	n := 0
	err := Run(ctx, Config{Name: "job", Interval: time.Hour, Timeout: time.Second, Immediate: true}, func(context.Context) error { n++; cancel(); return nil })
	if !errors.Is(err, context.Canceled) || n != 1 {
		t.Fatal(err, n)
	}
}
