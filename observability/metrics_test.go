package observability

import (
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestMetrics(t *testing.T) {
	r, e := New("asset_update")
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); done := r.Start("asset_update"); done(errors.New("secret")); done(nil) }()
	}
	wg.Wait()
	r.Start("tenant_secret")(nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, nil)
	s := w.Body.String()
	for _, x := range []string{`common_operation_calls_total{operation="asset_update"} 100`, `common_operation_failures_total{operation="asset_update"} 100`, `common_operation_active{operation="asset_update"} 0`} {
		if !strings.Contains(s, x) {
			t.Fatal(s)
		}
	}
	if strings.Contains(s, "secret") {
		t.Fatal(s)
	}
}
