package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/HieronimusMorgan/common-golang/requestctx"
	rdb "github.com/redis/go-redis/v9"
	"time"
)

// FencedLock embeds the ownership-checked lock. Fence must be checked monotonically by the protected resource. Redis failover/restore can roll counters back: use a durable SQL sequence for correctness-critical fencing.
type FencedLock struct {
	*Lock
	Fence int64
}

var fenceScript = rdb.NewScript(`if redis.call('EXISTS',KEYS[1])==1 then return 0 end; redis.call('INCR',KEYS[2]); local token=redis.call('GET',KEYS[2]); redis.call('SET',KEYS[1],ARGV[1],'PX',ARGV[2]); return token`)

func (c *Client) TryFencedLock(ctx context.Context, key string, ttl time.Duration) (*FencedLock, bool, error) {
	if key == "" || len(key) > 2048 || ttl < time.Millisecond || ttl > 24*time.Hour {
		return nil, false, errors.New("invalid fencing options")
	}
	id, err := requestctx.NewID()
	if err != nil {
		return nil, false, err
	}
	h := sha256.Sum256([]byte(key))
	base := c.key("fence:{" + hex.EncodeToString(h[:]) + "}")
	k := base + ":lock"
	n, err := fenceScript.Run(ctx, c.raw, []string{k, base + ":sequence"}, id, ttl.Milliseconds()).Int64()
	if err != nil || n == 0 {
		return nil, false, err
	}
	return &FencedLock{&Lock{c, k, id}, n}, true, nil
}
