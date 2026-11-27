package redis

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/HieronimusMorgan/common-golang/requestctx"
)

func TestIntegration(t *testing.T) {
	if os.Getenv("COMMON_INTEGRATION") != "1" {
		t.Skip("requires Redis")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	id, e := requestctx.NewID()
	if e != nil {
		t.Fatal(e)
	}
	addr := os.Getenv("COMMON_REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	c, e := Open(ctx, Config{Addr: addr, Prefix: "test:" + id})
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	defer c.Delete(ctx, "a")
	if e = c.SetJSON(ctx, "a", 42, time.Minute); e != nil {
		t.Fatal(e)
	}
	var n int
	if e = c.GetJSON(ctx, "a", &n); e != nil || n != 42 {
		t.Fatal(n, e)
	}
	if e = c.GetJSON(ctx, "missing", &n); !errors.Is(e, ErrMiss) {
		t.Fatal(e)
	}
	lock, ok, e := c.TryLock(ctx, "asset", time.Minute)
	if e != nil || !ok {
		t.Fatal(ok, e)
	}
	if _, ok, e = c.TryLock(ctx, "asset", time.Minute); e != nil || ok {
		t.Fatal(ok, e)
	}
	if e = lock.Release(ctx); e != nil {
		t.Fatal(e)
	}
	other, ok, e := c.TryLock(ctx, "asset", time.Minute)
	if e != nil || !ok {
		t.Fatal(ok, e)
	}
	defer other.Release(ctx)
	if e = lock.Release(ctx); !errors.Is(e, ErrLockLost) {
		t.Fatal(e)
	}
	if e = other.Renew(ctx, time.Minute); e != nil {
		t.Fatal(e)
	}
}

func TestRateLimitIntegration(t *testing.T) {
	if os.Getenv("COMMON_INTEGRATION") != "1" {
		t.Skip("requires Redis")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	id, _ := requestctx.NewID()
	addr := os.Getenv("COMMON_REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	c, e := Open(ctx, Config{Addr: addr, Prefix: "rate-test:" + id})
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	for i := 0; i < 3; i++ {
		r, e := c.AllowRate(ctx, "asset_write", "tenant:branch:actor", 2, time.Minute)
		if e != nil || r.Allowed != (i < 2) || r.Remaining != int64(max(0, 1-i)) {
			t.Fatal(i, r, e)
		}
	}
	r, e := c.AllowRate(ctx, "asset_write", "other", 2, time.Minute)
	if e != nil || !r.Allowed {
		t.Fatal(r, e)
	}
}

func TestV3RedisIntegration(t *testing.T) {
	if os.Getenv("COMMON_INTEGRATION") != "1" {
		t.Skip("requires Redis")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	id, e := requestctx.NewID()
	if e != nil {
		t.Fatal(e)
	}
	addr := os.Getenv("COMMON_REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	c, e := Open(ctx, Config{Addr: addr, Prefix: "v3-test:" + id})
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	for i := 0; i < 3; i++ {
		d, e := c.AllowTokenBucket(ctx, "policy-v1", "subject", 2, 1, time.Minute)
		if e != nil || d.Allowed != (i < 2) {
			t.Fatal(d, e)
		}
	}
	a, ok, e := c.TryFencedLock(ctx, "asset", time.Minute)
	if e != nil || !ok {
		t.Fatal(ok, e)
	}
	if _, ok, e = c.TryFencedLock(ctx, "asset", time.Minute); e != nil || ok {
		t.Fatal(ok, e)
	}
	if e = a.Release(ctx); e != nil {
		t.Fatal(e)
	}
	b, ok, e := c.TryFencedLock(ctx, "asset", time.Minute)
	if e != nil || !ok || b.Fence <= a.Fence {
		t.Fatal(b, ok, e)
	}
	defer b.Release(ctx)
	if e = a.Release(ctx); !errors.Is(e, ErrLockLost) {
		t.Fatal(e)
	}
	if e = c.Revoke(ctx, "issuer", "jti", time.Now().Add(time.Minute)); e != nil {
		t.Fatal(e)
	}
	rev, e := c.Revoked(ctx, "issuer", "jti")
	if e != nil || !rev {
		t.Fatal(rev, e)
	}
	rev, e = c.Revoked(ctx, "other", "jti")
	if e != nil || rev {
		t.Fatal(rev, e)
	}
}
