package worker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestPool(t *testing.T) {
	jobs := make(chan Job, 21)
	var done, failed atomic.Int32
	for i := 0; i < 20; i++ {
		jobs <- func(context.Context) error { done.Add(1); return nil }
	}
	jobs <- func(context.Context) error { panic("x") }
	close(jobs)
	p := Pool{Concurrency: 4, Timeout: time.Second, OnError: func(error) { failed.Add(1) }}
	if e := p.Run(context.Background(), jobs); e != nil || done.Load() != 20 || failed.Load() != 1 {
		t.Fatal(e, done.Load(), failed.Load())
	}
}
