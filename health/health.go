// Package health provides standard-library liveness and readiness handlers.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Check func(context.Context) error
type Dependency struct {
	Name  string
	Check Check
}

func Liveness(w http.ResponseWriter, _ *http.Request) { write(w, 200, map[string]any{"status": "ok"}) }
func Readiness(timeout time.Duration, deps ...Dependency) http.HandlerFunc {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	deps = append([]Dependency(nil), deps...)
	seen := map[string]bool{}
	for _, d := range deps {
		if d.Name == "" || d.Check == nil || seen[d.Name] {
			panic("invalid readiness dependency")
		}
		seen[d.Name] = true
	}
	// One slot per dependency across requests limits stuck probes to one goroutine each.
	slots := make([]chan struct{}, len(deps))
	for i := range slots {
		slots[i] = make(chan struct{}, 1)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		results := map[string]string{}
		status := 200
		type outcome struct {
			name string
			ok   bool
		}
		done := make(chan outcome, len(deps))
		for i, d := range deps {
			results[d.Name] = "unavailable"
			select {
			case slots[i] <- struct{}{}:
				go func(d Dependency, slot chan struct{}) {
					ok := false
					defer func() { _ = recover(); <-slot; done <- outcome{d.Name, ok} }()
					ok = d.Check(ctx) == nil && ctx.Err() == nil
				}(d, slots[i])
			default:
				done <- outcome{d.Name, false}
			}
		}
		for i := 0; i < len(deps); i++ {
			select {
			case r := <-done:
				if r.ok {
					results[r.name] = "ok"
				}
			case <-ctx.Done():
				i = len(deps)
			}
		}
		for _, v := range results {
			if v != "ok" {
				status = 503
			}
		}

		s := "ok"
		if status != 200 {
			s = "unavailable"
		}
		write(w, status, map[string]any{"status": s, "dependencies": results})
	}
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
