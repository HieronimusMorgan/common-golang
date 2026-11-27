package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var miss = errors.New("miss")

type mem struct {
	mu  sync.Mutex
	v   int
	set bool
}

func (m *mem) GetJSON(_ context.Context, _ string, d any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.set {
		return miss
	}
	*d.(*int) = m.v
	return nil
}
func (m *mem) SetJSON(_ context.Context, _ string, v any, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.v = v.(int)
	m.set = true
	return nil
}
func TestConcurrent(t *testing.T) {
	l, e := New[int](&mem{}, miss, time.Second, 10)
	if e != nil {
		t.Fatal(e)
	}
	var count atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, e := l.GetOrLoad(context.Background(), "x", time.Second, func(context.Context) (int, error) { count.Add(1); time.Sleep(5 * time.Millisecond); return 42, nil })
			if e != nil || v != 42 {
				t.Error(v, e)
			}
		}()
	}
	wg.Wait()
	if count.Load() != 1 {
		t.Fatal(count.Load())
	}
}
