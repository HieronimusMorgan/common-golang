// Package storage defines scoped object storage and bounded upload staging.
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/HieronimusMorgan/common-golang/requestctx"
	"github.com/HieronimusMorgan/common-golang/scope"
	"io"
	"mime"
	"net/http"
	"os"
	"time"
)

type Object struct {
	Key, ContentType, SHA256 string
	Size                     int64
}
type Provider interface {
	Put(context.Context, Object, io.Reader) error
	Get(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
	PresignGet(context.Context, string, time.Duration) (string, error)
}
type Client struct {
	Provider     Provider
	MaxBytes     int64
	AllowedTypes []string
}

func key(ctx context.Context, id string) (string, error) {
	if !requestctx.ValidID(id) {
		return "", errors.New("invalid object ID")
	}
	prefix, err := scope.Key(ctx, "object", "branch", false)
	if err != nil {
		return "", err
	}
	return prefix + "/" + id, nil
}

// Upload stages the bounded body in a private temporary file, sniffs content and verifies an optional SHA256 checksum. Caller authorizes resources before invoking.
func (c Client) Upload(ctx context.Context, id string, body io.Reader, expectedSHA256 string) (Object, error) {
	if c.Provider == nil || body == nil || c.MaxBytes < 1 || c.MaxBytes > 1<<30 || len(c.AllowedTypes) == 0 {
		return Object{}, errors.New("invalid storage config")
	}
	k, err := key(ctx, id)
	if err != nil {
		return Object{}, err
	}
	f, err := os.CreateTemp("", "common-upload-*")
	if err != nil {
		return Object{}, err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(&contextReader{ctx, body}, c.MaxBytes+1))
	if err != nil {
		return Object{}, err
	}
	if n < 1 || n > c.MaxBytes {
		return Object{}, errors.New("invalid upload size")
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if expectedSHA256 != "" && expectedSHA256 != sum {
		return Object{}, errors.New("checksum mismatch")
	}
	if _, err = f.Seek(0, 0); err != nil {
		return Object{}, err
	}
	buf := make([]byte, 512)
	m, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return Object{}, err
	}
	typ, _, _ := mime.ParseMediaType(http.DetectContentType(buf[:m]))
	allowed := false
	for _, a := range c.AllowedTypes {
		if a == typ {
			allowed = true
		}
	}
	if !allowed {
		return Object{}, errors.New("content type not allowed")
	}
	if _, err = f.Seek(0, 0); err != nil {
		return Object{}, err
	}
	o := Object{k, typ, sum, n}
	return o, c.Provider.Put(ctx, o, f)
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
func (c Client) Download(ctx context.Context, id string) (io.ReadCloser, error) {
	k, err := key(ctx, id)
	if err != nil {
		return nil, err
	}
	if c.Provider == nil {
		return nil, errors.New("provider required")
	}
	return c.Provider.Get(ctx, k)
}
func (c Client) Delete(ctx context.Context, id string) error {
	k, err := key(ctx, id)
	if err != nil {
		return err
	}
	if c.Provider == nil {
		return errors.New("provider required")
	}
	return c.Provider.Delete(ctx, k)
}
func (c Client) PresignGet(ctx context.Context, id string, ttl time.Duration) (string, error) {
	k, err := key(ctx, id)
	if err != nil {
		return "", err
	}
	if c.Provider == nil || ttl < time.Second || ttl > time.Hour {
		return "", errors.New("invalid presign options")
	}
	return c.Provider.PresignGet(ctx, k, ttl)
}
