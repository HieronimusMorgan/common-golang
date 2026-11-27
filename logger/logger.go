// Package logger builds structured, dependency-free service loggers.
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/HieronimusMorgan/common-golang/requestctx"
)

type Config struct {
	Service     string
	Environment string
	Level       string
	JSON        bool
	Writer      io.Writer
}

func New(c Config) (*slog.Logger, error) {
	var level slog.Level
	if c.Level != "" {
		if e := level.UnmarshalText([]byte(c.Level)); e != nil {
			return nil, e
		}
	}
	w := c.Writer
	if w == nil {
		w = os.Stdout
	}
	opts := &slog.HandlerOptions{Level: level, ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
		switch strings.ToLower(a.Key) {
		case "password", "secret", "authorization", "token", "access_token", "refresh_token", "api_key", "cookie", "set-cookie":
			return slog.String(a.Key, "[REDACTED]")
		}
		return a
	}}
	var h slog.Handler
	if c.JSON {
		h = slog.NewJSONHandler(w, opts)
	} else {
		h = slog.NewTextHandler(w, opts)
	}
	return slog.New(h).With("service", c.Service, "environment", c.Environment), nil
}
func WithContext(ctx context.Context, l *slog.Logger) *slog.Logger {
	if l == nil {
		l = slog.Default()
	}
	if id := requestctx.ID(ctx); id != "" {
		return l.With("request_id", id)
	}
	return l
}
