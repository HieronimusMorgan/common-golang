// Package httpclient provides a reusable HTTP client with bounded response sizes.
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/HieronimusMorgan/common-golang/requestctx"
)

type Client struct {
	raw      *http.Client
	maxBytes int64
}
type StatusError struct{ StatusCode int }

func (e *StatusError) Error() string { return fmt.Sprintf("upstream HTTP status %d", e.StatusCode) }
func New(timeout time.Duration, maxBytes int64) *Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if maxBytes <= 0 || maxBytes > 64<<20 {
		maxBytes = 4 << 20
	}
	t := &http.Transport{Proxy: http.ProxyFromEnvironment, MaxIdleConns: 100, MaxIdleConnsPerHost: 10, IdleConnTimeout: 90 * time.Second, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: timeout, ForceAttemptHTTP2: true}
	return &Client{raw: &http.Client{Timeout: timeout, Transport: t, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}, maxBytes: maxBytes}
}
func (c *Client) CloseIdleConnections() { c.raw.CloseIdleConnections() }

// JSON requires a trusted, application-configured URL; never pass arbitrary user URLs.
func (c *Client) JSON(ctx context.Context, method, target string, headers http.Header, body, dst any) error {
	u, e := url.Parse(target)
	if e != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("invalid upstream URL")
	}
	var reader io.Reader
	if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			return e
		}
		reader = bytes.NewReader(b)
	}
	req, e := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if e != nil {
		return e
	}
	if headers != nil {
		req.Header = headers.Clone()
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if id := requestctx.ID(ctx); id != "" {
		req.Header.Set("X-Request-ID", id)
	}
	r, e := c.raw.Do(req)
	if e != nil {
		return e
	}
	defer r.Body.Close()
	if r.StatusCode < 200 || r.StatusCode >= 300 {
		return &StatusError{r.StatusCode}
	}
	b, e := io.ReadAll(io.LimitReader(r.Body, c.maxBytes+1))
	if e != nil {
		return e
	}
	if int64(len(b)) > c.maxBytes {
		return errors.New("upstream response exceeds limit")
	}
	if dst == nil || r.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.Unmarshal(b, dst)
}
