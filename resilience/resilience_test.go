package resilience

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRetry(t *testing.T) {
	n := 0
	e := Retry(context.Background(), 3, time.Nanosecond, time.Nanosecond, func(error) bool { return true }, func(context.Context) error {
		n++
		if n < 3 {
			return errors.New("temporary")
		}
		return nil
	})
	if e != nil || n != 3 {
		t.Fatal(n, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e = Retry(ctx, 3, time.Second, time.Second, func(error) bool { return true }, func(context.Context) error { t.Fatal("executed"); return nil }); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestBreaker(t *testing.T) {
	b, _ := NewBreaker(1, time.Millisecond)
	bad := errors.New("fail")
	class := func(error) bool { return true }
	ctx := context.Background()
	_ = b.Do(ctx, class, func(context.Context) error { return bad })
	if !errors.Is(b.Do(ctx, class, func(context.Context) error { t.Fatal("executed"); return nil }), ErrOpen) {
		t.Fatal("not open")
	}
	time.Sleep(5 * time.Millisecond)
	started := make(chan struct{})
	release := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = b.Do(ctx, class, func(context.Context) error { close(started); <-release; return nil })
	}()
	<-started
	if !errors.Is(b.Do(ctx, class, func(context.Context) error { return nil }), ErrOpen) {
		t.Fatal("second probe")
	}
	close(release)
	wg.Wait()
	if e := b.Do(ctx, class, func(context.Context) error { return nil }); e != nil {
		t.Fatal(e)
	}
}
