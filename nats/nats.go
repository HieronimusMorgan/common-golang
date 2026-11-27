// Package nats provides Core NATS and JetStream transport primitives.
package nats

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/HieronimusMorgan/common-golang/requestctx"
	nc "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type Config struct {
	URL             string
	Name            string
	Username        string
	Password        string
	Token           string
	CredentialsFile string
	TLS             *tls.Config
	Timeout         time.Duration
	Logger          *slog.Logger
}
type Client struct{ conn *nc.Conn }

func Open(c Config) (*Client, error) {
	if strings.TrimSpace(c.URL) == "" || c.Timeout < 0 {
		return nil, errors.New("invalid NATS config")
	}
	if c.Timeout == 0 {
		c.Timeout = 5 * time.Second
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	auth := 0
	if c.Username != "" || c.Password != "" {
		auth++
	}
	if c.Token != "" {
		auth++
	}
	if c.CredentialsFile != "" {
		auth++
	}
	if auth > 1 {
		return nil, errors.New("configure only one NATS authentication method")
	}
	opts := []nc.Option{nc.Name(c.Name), nc.Timeout(c.Timeout), nc.MaxReconnects(-1), nc.ReconnectWait(time.Second), nc.DrainTimeout(15 * time.Second), nc.DisconnectErrHandler(func(_ *nc.Conn, _ error) { c.Logger.Warn("NATS disconnected") }), nc.ReconnectHandler(func(_ *nc.Conn) { c.Logger.Info("NATS reconnected") }), nc.ErrorHandler(func(_ *nc.Conn, _ *nc.Subscription, _ error) { c.Logger.Error("NATS asynchronous error") })}
	if c.Username != "" || c.Password != "" {
		opts = append(opts, nc.UserInfo(c.Username, c.Password))
	}
	if c.Token != "" {
		opts = append(opts, nc.Token(c.Token))
	}
	if c.CredentialsFile != "" {
		opts = append(opts, nc.UserCredentials(c.CredentialsFile))
	}
	if c.TLS != nil {
		opts = append(opts, nc.Secure(c.TLS.Clone()))
	}
	conn, e := nc.Connect(c.URL, opts...)
	if e != nil {
		return nil, e
	}
	return &Client{conn}, nil
}
func (c *Client) Raw() *nc.Conn                           { return c.conn }
func (c *Client) JetStream() (jetstream.JetStream, error) { return jetstream.New(c.conn) }
func (c *Client) Ping(ctx context.Context) error {
	if !c.conn.IsConnected() {
		return errors.New("NATS is not connected")
	}
	probe, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.conn.FlushWithContext(probe)
}
func (c *Client) Close()       { c.conn.Close() }
func (c *Client) Drain() error { return c.conn.Drain() }
func message(ctx context.Context, subject string, value any) (*nc.Msg, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if strings.TrimSpace(subject) == "" {
		return nil, errors.New("subject required")
	}
	b, e := json.Marshal(value)
	if e != nil {
		return nil, e
	}
	m := nc.NewMsg(subject)
	m.Data = b
	m.Header.Set("Content-Type", "application/json")
	if id := requestctx.ID(ctx); id != "" {
		m.Header.Set("X-Request-ID", id)
	}
	return m, nil
}

// PublishJSON uses Core NATS: successful publish does not guarantee delivery.
func (c *Client) PublishJSON(ctx context.Context, subject string, value any) error {
	m, e := message(ctx, subject, value)
	if e != nil {
		return e
	}
	return c.conn.PublishMsg(m)
}
func (c *Client) RequestJSON(ctx context.Context, subject string, value, dst any) error {
	m, e := message(ctx, subject, value)
	if e != nil {
		return e
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	r, e := c.conn.RequestMsgWithContext(bounded, m)
	if e != nil {
		return e
	}
	return json.Unmarshal(r.Data, dst)
}

// PublishDurable waits for a JetStream server acknowledgement. Reuse eventID on retries.
func (c *Client) PublishDurable(ctx context.Context, subject, eventID string, value any) (*jetstream.PubAck, error) {
	if strings.TrimSpace(eventID) == "" {
		return nil, errors.New("event ID required")
	}
	m, e := message(ctx, subject, value)
	if e != nil {
		return nil, e
	}
	js, e := c.JetStream()
	if e != nil {
		return nil, e
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return js.PublishMsg(bounded, m, jetstream.WithMsgID(eventID))
}
