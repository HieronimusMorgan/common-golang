package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	rdb "github.com/redis/go-redis/v9"
)

type RateDecision struct {
	Allowed    bool
	Remaining  int64
	ResetAfter time.Duration
}

// Atomic fixed window starting on first request. Denied requests do not increment or extend TTL.
var rateScript = rdb.NewScript(`local count=tonumber(redis.call('GET',KEYS[1]) or '0'); local ttl=redis.call('PTTL',KEYS[1]); if ttl<0 then count=0; redis.call('SET',KEYS[1],0,'PX',ARGV[2]); end; local allowed=0; if count<tonumber(ARGV[1]) then count=redis.call('INCR',KEYS[1]); allowed=1; end; return {allowed,math.max(0,tonumber(ARGV[1])-count),redis.call('PTTL',KEYS[1])}`)

// AllowRate uses a bounded hashed policy/subject key. Callers must include tenant and branch in subject where applicable. Errors are not an allow decision; choose a fail policy explicitly.
func (c *Client) AllowRate(ctx context.Context, policy, subject string, limit int64, window time.Duration) (RateDecision, error) {
	if strings.ContainsRune(policy, 0) || strings.ContainsRune(subject, 0) || policy == "" || len(policy) > 128 || subject == "" || len(subject) > 2048 || limit < 1 || limit > 1_000_000_000 || window < time.Millisecond || window > 24*time.Hour {
		return RateDecision{}, errors.New("invalid rate limit policy")
	}
	sum := sha256.Sum256([]byte(policy + "\x00" + subject))
	key := c.key("rate:" + hex.EncodeToString(sum[:]))
	v, e := rateScript.Run(ctx, c.raw, []string{key}, limit, window.Milliseconds()).Slice()
	if e != nil {
		return RateDecision{}, e
	}
	if len(v) != 3 {
		return RateDecision{}, errors.New("unexpected rate limit response")
	}
	a, ok := v[0].(int64)
	r, ok2 := v[1].(int64)
	t, ok3 := v[2].(int64)
	if !ok || !ok2 || !ok3 || t < 0 {
		return RateDecision{}, errors.New("invalid rate limit response")
	}
	return RateDecision{a == 1, r, time.Duration(t) * time.Millisecond}, nil
}
