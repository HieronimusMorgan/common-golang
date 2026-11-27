// Package lifecycle coordinates graceful HTTP shutdown. Close dependencies after Serve returns.
package lifecycle

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

// Serve takes ownership of the listener. It returns only after serving stops and
// the HTTP shutdown deadline expires or active requests finish.
func Serve(ctx context.Context, s *http.Server, l net.Listener, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	done := make(chan error, 1)
	go func() { done <- s.Serve(l) }()
	select {
	case e := <-done:
		if errors.Is(e, http.ErrServerClosed) {
			return nil
		}
		return e
	case <-ctx.Done():
		stop, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		e := s.Shutdown(stop)
		if e != nil {
			e = errors.Join(e, s.Close())
		}
		serveErr := <-done
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		return errors.Join(e, serveErr)
	}
}
func HTTPServer(addr string, h http.Handler) *http.Server {
	return &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
}
