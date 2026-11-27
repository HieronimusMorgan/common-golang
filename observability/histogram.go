package observability

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"
)

type Histogram struct {
	mu     sync.Mutex
	bounds []float64
	counts []uint64
	total  uint64
	sum    float64
	name   string
}

func NewHistogram(name string, bounds []float64) (*Histogram, error) {
	if !valid.MatchString(name) || len(bounds) == 0 || len(bounds) > 64 {
		return nil, errors.New("invalid histogram")
	}
	b := append([]float64(nil), bounds...)
	if !sort.Float64sAreSorted(b) {
		return nil, errors.New("bounds must increase")
	}
	for i, v := range b {
		if v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) || (i > 0 && v == b[i-1]) {
			return nil, errors.New("invalid histogram bounds")
		}
	}
	return &Histogram{name: name, bounds: b, counts: make([]uint64, len(b))}, nil
}
func (h *Histogram) Observe(d time.Duration) {
	if d < 0 {
		return
	}
	v := d.Seconds()
	h.mu.Lock()
	defer h.mu.Unlock()
	h.total++
	h.sum += v
	for i, b := range h.bounds {
		if v <= b {
			h.counts[i]++
		}
	}
}
func (h *Histogram) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	h.mu.Lock()
	counts := append([]uint64(nil), h.counts...)
	n, sum := h.total, h.sum
	h.mu.Unlock()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# TYPE %s histogram\n", h.name)
	for i, b := range h.bounds {
		fmt.Fprintf(w, "%s_bucket{le=\"%g\"} %d\n", h.name, b, counts[i])
	}
	fmt.Fprintf(w, "%s_bucket{le=\"+Inf\"} %d\n%s_count %d\n%s_sum %g\n", h.name, n, h.name, n, h.name, sum)
}
