// Package redis wraps go-redis with explicit lifecycle and bounded cache operations.
package redis

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/HieronimusMorgan/common-golang/requestctx"
	rdb "github.com/redis/go-redis/v9"
)

var ErrMiss = errors.New("cache miss")
var ErrLockLost = errors.New("lock ownership lost")

type Config struct {
	Addr     string
	Username string
	Password string
	DB       int
	Prefix   string
	TLS      *tls.Config
	Timeout  time.Duration
	PoolSize int
}
type Client struct {
	raw    *rdb.Client
	prefix string
}

func Open(ctx context.Context, c Config) (*Client, error) {
	if strings.TrimSpace(c.Addr) == "" || strings.TrimSpace(c.Prefix) == "" || c.DB < 0 || c.PoolSize < 0 || c.Timeout < 0 {
		return nil, errors.New("invalid Redis config")
	}
	if c.Timeout == 0 {
		c.Timeout = 3 * time.Second
	}
	if c.TLS != nil {
		c.TLS = c.TLS.Clone()
	}
	raw := rdb.NewClient(&rdb.Options{Addr: c.Addr, Username: c.Username, Password: c.Password, DB: c.DB, TLSConfig: c.TLS, DialTimeout: c.Timeout, ReadTimeout: c.Timeout, WriteTimeout: c.Timeout, PoolTimeout: c.Timeout, PoolSize: c.PoolSize, ContextTimeoutEnabled: true})
	probe, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	if e := raw.Ping(probe).Err(); e != nil {
		_ = raw.Close()
		return nil, e
	}
	return &Client{raw: raw, prefix: c.Prefix + ":"}, nil
}
func (c *Client) Raw() *rdb.Client               { return c.raw }
func (c *Client) Ping(ctx context.Context) error { return c.raw.Ping(ctx).Err() }
func (c *Client) Close() error                   { return c.raw.Close() }
func (c *Client) key(k string) string            { return c.prefix + k }
func (c *Client) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	if key == "" || ttl < time.Millisecond {
		return errors.New("key and TTL >= 1ms required")
	}
	b, e := json.Marshal(value)
	if e != nil {
		return e
	}
	return c.raw.Set(ctx, c.key("cache:"+key), b, ttl).Err()
}
func (c *Client) GetJSON(ctx context.Context, key string, dst any) error {
	if key == "" {
		return errors.New("key required")
	}
	b, e := c.raw.Get(ctx, c.key("cache:"+key)).Bytes()
	if errors.Is(e, rdb.Nil) {
		return ErrMiss
	}
	if e != nil {
		return e
	}
	return json.Unmarshal(b, dst)
}
func (c *Client) Delete(ctx context.Context, key string) error {
	if key == "" {
		return errors.New("key required")
	}
	return c.raw.Del(ctx, c.key("cache:"+key)).Err()
}

type Lock struct {
	client *Client
	key    string
	token  string
}

func (c *Client) TryLock(ctx context.Context, key string, ttl time.Duration) (*Lock, bool, error) {
	if key == "" || ttl < time.Millisecond {
		return nil, false, errors.New("key and TTL >= 1ms required")
	}
	token, e := requestctx.NewID()
	if e != nil {
		return nil, false, e
	}
	k := c.key("lock:" + key)
	ok, e := c.raw.SetNX(ctx, k, token, ttl).Result()
	if e != nil || !ok {
		return nil, false, e
	}
	return &Lock{c, k, token}, true, nil
}

var release = rdb.NewScript(`if redis.call('GET', KEYS[1]) == ARGV[1] then return redis.call('DEL', KEYS[1]) else return 0 end`)
var renew = rdb.NewScript(`if redis.call('GET', KEYS[1]) == ARGV[1] then return redis.call('PEXPIRE', KEYS[1], ARGV[2]) else return 0 end`)

func (l *Lock) Release(ctx context.Context) error {
	n, e := release.Run(ctx, l.client.raw, []string{l.key}, l.token).Int()
	if e != nil {
		return e
	}
	if n != 1 {
		return ErrLockLost
	}
	return nil
}
func (l *Lock) Renew(ctx context.Context, ttl time.Duration) error {
	if ttl < time.Millisecond {
		return errors.New("TTL >= 1ms required")
	}
	n, e := renew.Run(ctx, l.client.raw, []string{l.key}, l.token, ttl.Milliseconds()).Int()
	if e != nil {
		return e
	}
	if n != 1 {
		return ErrLockLost
	}
	return nil
}
