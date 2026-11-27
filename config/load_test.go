package config

import (
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	type C struct {
		Port    int           `env:"PORT" default:"8080"`
		Timeout time.Duration `env:"TIMEOUT"`
		Secret  string        `env:"SECRET" required:"true"`
	}
	m := map[string]string{"SECRET": "x", "TIMEOUT": "3s"}
	lookup := func(k string) (string, bool) { v, ok := m[k]; return v, ok }
	c, e := Load(C{}, lookup, nil)
	if e != nil || c.Port != 8080 || c.Timeout != 3*time.Second {
		t.Fatal(c, e)
	}
	m["PORT"] = "bad"
	if _, e = Load(C{}, lookup, nil); e == nil {
		t.Fatal("invalid port accepted")
	}
	var s Snapshot[C]
	if e = s.Store(c, nil); e != nil {
		t.Fatal(e)
	}
	if got, ok := s.Get(); !ok || got != c {
		t.Fatal(got)
	}
}
