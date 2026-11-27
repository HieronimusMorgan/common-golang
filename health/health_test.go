package health

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReadiness(t *testing.T) {
	h := Readiness(time.Second, Dependency{"db", func(context.Context) error { return errors.New("secret") }})
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 503 || strings.Contains(w.Body.String(), "secret") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	Liveness(w, httptest.NewRequest("GET", "/livez", nil))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
}

func TestStuckAndPanic(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	h := Readiness(20*time.Millisecond, Dependency{"stuck", func(context.Context) error { <-release; return nil }}, Dependency{"ok", func(context.Context) error { return nil }}, Dependency{"panic", func(context.Context) error { panic("secret") }})
	for i := 0; i < 3; i++ {
		start := time.Now()
		w := httptest.NewRecorder()
		h(w, httptest.NewRequest("GET", "/ready", nil))
		if time.Since(start) > time.Second || w.Code != 503 || strings.Contains(w.Body.String(), "secret") || !strings.Contains(w.Body.String(), `"ok":"ok"`) {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
