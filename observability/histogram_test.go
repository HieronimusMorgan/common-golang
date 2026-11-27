package observability

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHistogram(t *testing.T) {
	h, e := NewHistogram("latency_seconds", []float64{.1, 1})
	if e != nil {
		t.Fatal(e)
	}
	h.Observe(50 * time.Millisecond)
	h.Observe(500 * time.Millisecond)
	h.Observe(2 * time.Second)
	r := httptest.NewRecorder()
	h.ServeHTTP(r, nil)
	for _, want := range []string{`latency_seconds_bucket{le="0.1"} 1`, `latency_seconds_bucket{le="1"} 2`, `latency_seconds_count 3`} {
		if !strings.Contains(r.Body.String(), want) {
			t.Fatal(r.Body.String())
		}
	}
}
