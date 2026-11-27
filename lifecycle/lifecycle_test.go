package lifecycle

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestShutdown(t *testing.T) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := HTTPServer(l.Addr().String(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, s, l, time.Second) }()
	c := &http.Client{Timeout: time.Second}
	r, e := c.Get("http://" + l.Addr().String())
	if e != nil {
		t.Fatal(e)
	}
	r.Body.Close()
	cancel()
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown timed out")
	}
}
