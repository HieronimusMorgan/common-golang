package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/HieronimusMorgan/common-golang/requestctx"
)

func TestJSON(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Request-ID") != "abc" {
			t.Error("missing correlation")
		}
		switch r.URL.Path {
		case "/large":
			_, _ = w.Write([]byte("12345678901234567890"))
		case "/redirect":
			http.Redirect(w, r, "/", 302)
		default:
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer s.Close()
	c := New(time.Second, 16)
	defer c.CloseIdleConnections()
	ctx := requestctx.WithID(context.Background(), "abc")
	var dst map[string]bool
	if e := c.JSON(ctx, "GET", s.URL, nil, nil, &dst); e != nil || !dst["ok"] {
		t.Fatal(dst, e)
	}
	if e := c.JSON(ctx, "GET", s.URL+"/large", nil, nil, nil); e == nil {
		t.Fatal("unbounded read")
	}
	if e := c.JSON(ctx, "GET", s.URL+"/redirect", nil, nil, nil); e == nil {
		t.Fatal("redirect followed")
	}
}
