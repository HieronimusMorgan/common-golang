// Package observability provides bounded-cardinality, Prometheus-compatible process metrics.
package observability

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"sync/atomic"
	"time"
)

var valid = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,63}$`)

type operation struct {
	calls, failures, nanos atomic.Uint64
	active                 atomic.Int64
}
type Registry struct {
	ops   map[string]*operation
	names []string
}

// New requires static operation names, never request IDs, URLs or tenant names.
func New(names ...string) (*Registry, error) {
	if len(names) == 0 || len(names) > 256 {
		return nil, fmt.Errorf("operation count must be 1-256")
	}
	r := &Registry{ops: map[string]*operation{}}
	for _, n := range names {
		if !valid.MatchString(n) || r.ops[n] != nil {
			return nil, fmt.Errorf("invalid or duplicate operation")
		}
		r.ops[n] = &operation{}
		r.names = append(r.names, n)
	}
	sort.Strings(r.names)
	return r, nil
}

// Start returns an idempotent completion callback. Unknown names are ignored rather than creating unbounded labels.
func (r *Registry) Start(name string) func(error) {
	o := r.ops[name]
	if o == nil {
		return func(error) {}
	}
	o.active.Add(1)
	start := time.Now()
	var done atomic.Bool
	return func(err error) {
		if !done.CompareAndSwap(false, true) {
			return
		}
		o.active.Add(-1)
		o.calls.Add(1)
		if err != nil {
			o.failures.Add(1)
		}
		o.nanos.Add(uint64(time.Since(start)))
	}
}
func (r *Registry) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprint(w, "# TYPE common_operation_calls_total counter\n# TYPE common_operation_failures_total counter\n# TYPE common_operation_duration_seconds_total counter\n# TYPE common_operation_active gauge\n")
	for _, n := range r.names {
		o := r.ops[n]
		fmt.Fprintf(w, "common_operation_calls_total{operation=%q} %d\ncommon_operation_failures_total{operation=%q} %d\ncommon_operation_duration_seconds_total{operation=%q} %.9f\ncommon_operation_active{operation=%q} %d\n", n, o.calls.Load(), n, o.failures.Load(), n, float64(o.nanos.Load())/1e9, n, o.active.Load())
	}
}
