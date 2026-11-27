package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	rdb "github.com/redis/go-redis/v9"
	"strings"
	"time"
)

// Redis server time avoids host clock skew. Units are tokens and milliseconds; key expires once the bucket could refill completely.
var bucketScript = rdb.NewScript(`local t=redis.call('TIME'); local now=t[1]*1000+math.floor(t[2]/1000); local cap=tonumber(ARGV[1]); local rate=tonumber(ARGV[2]); local state=redis.call('HMGET',KEYS[1],'tokens','time'); local last=tonumber(state[2]) or now; now=math.max(now,last); local tokens=math.min(cap,(tonumber(state[1]) or cap)+(now-last)*rate); local allowed=0; if tokens>=1 then tokens=tokens-1; allowed=1; end; redis.call('HSET',KEYS[1],'tokens',tostring(tokens),'time',tostring(now)); redis.call('PEXPIRE',KEYS[1],math.ceil(cap/rate)+1); return {allowed,math.floor(tokens),math.ceil(math.max(0,1-tokens)/rate)}`)

// AllowTokenBucket consumes one token. ResetAfter is time until one token is available, not full refill. Version the policy name when changing limits.
func (c *Client) AllowTokenBucket(ctx context.Context, policy, subject string, capacity, refill int64, period time.Duration) (RateDecision, error) {
	if policy == "" || subject == "" || len(policy) > 128 || len(subject) > 2048 || strings.ContainsRune(policy, 0) || strings.ContainsRune(subject, 0) || capacity < 1 || capacity > 1e6 || refill < 1 || refill > 1e6 || period < time.Millisecond || period > 24*time.Hour || float64(capacity)*float64(period.Milliseconds())/float64(refill) > float64((7*24*time.Hour).Milliseconds()) {
		return RateDecision{}, errors.New("invalid token bucket")
	}
	h := sha256.Sum256([]byte(policy + "\x00" + subject))
	v, err := bucketScript.Run(ctx, c.raw, []string{c.key("bucket:" + hex.EncodeToString(h[:]))}, capacity, float64(refill)/float64(period.Milliseconds())).Slice()
	if err != nil {
		return RateDecision{}, err
	}
	if len(v) != 3 {
		return RateDecision{}, errors.New("invalid bucket reply")
	}
	a, ok := v[0].(int64)
	r, ok2 := v[1].(int64)
	d, ok3 := v[2].(int64)
	if !ok || !ok2 || !ok3 || d < 0 {
		return RateDecision{}, errors.New("invalid bucket reply")
	}
	return RateDecision{a == 1, r, time.Duration(d) * time.Millisecond}, nil
}
