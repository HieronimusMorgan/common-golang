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

func (c *Client) revocationKey(issuer, id string) (string, error) {
	if issuer == "" || id == "" || len(issuer) > 2048 || len(id) > 256 || strings.ContainsRune(issuer, 0) || strings.ContainsRune(id, 0) {
		return "", errors.New("invalid revocation key")
	}
	h := sha256.Sum256([]byte(issuer + "\x00" + id))
	return c.key("revoked:" + hex.EncodeToString(h[:])), nil
}

// Revoke marks a verified token ID revoked until its full validity window (expiry plus leeway). Use a durable no-eviction store; Redis loss would otherwise forget revocations.
func (c *Client) Revoke(ctx context.Context, issuer, id string, validUntil time.Time) error {
	k, e := c.revocationKey(issuer, id)
	if e != nil {
		return e
	}
	ttl := time.Until(validUntil)
	if ttl < time.Millisecond || ttl > 365*24*time.Hour {
		return errors.New("invalid revocation lifetime")
	}
	return revokeScript.Run(ctx, c.raw, []string{k}, ttl.Milliseconds()).Err()
}

// Revoked implements auth.Revocations. Propagate errors and fail closed; Redis persistence/eviction policy matters.
func (c *Client) Revoked(ctx context.Context, issuer, id string) (bool, error) {
	k, e := c.revocationKey(issuer, id)
	if e != nil {
		return false, e
	}
	n, e := c.raw.Exists(ctx, k).Result()
	return n > 0, e
}

var revokeScript = rdb.NewScript(`local ttl=redis.call('PTTL',KEYS[1]); if ttl == -1 or ttl >= tonumber(ARGV[1]) then return 1 end; redis.call('SET',KEYS[1],'1','PX',ARGV[1]); return 1`)
