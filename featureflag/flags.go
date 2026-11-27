// Package featureflag evaluates immutable scoped rollout rules. Flags never grant permissions.
package featureflag

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"github.com/HieronimusMorgan/common-golang/scope"
)

type Rule struct {
	Enabled     bool
	BasisPoints int
}
type Flag struct {
	Default  Rule
	Tenants  map[string]Rule
	Branches map[string]map[string]Rule
}
type Evaluator struct{ flags map[string]Flag }

func New(flags map[string]Flag) (*Evaluator, error) {
	e := &Evaluator{flags: map[string]Flag{}}
	valid := func(r Rule) bool { return r.BasisPoints >= 0 && r.BasisPoints <= 10000 }
	for name, f := range flags {
		if name == "" || !valid(f.Default) {
			return nil, errors.New("invalid flag")
		}
		c := Flag{Default: f.Default, Tenants: map[string]Rule{}, Branches: map[string]map[string]Rule{}}
		for t, r := range f.Tenants {
			if t == "" || !valid(r) {
				return nil, errors.New("invalid tenant rule")
			}
			c.Tenants[t] = r
		}
		for t, bs := range f.Branches {
			if t == "" {
				return nil, errors.New("invalid tenant")
			}
			c.Branches[t] = map[string]Rule{}
			for b, r := range bs {
				if b == "" || !valid(r) {
					return nil, errors.New("invalid branch rule")
				}
				c.Branches[t][b] = r
			}
		}
		e.flags[name] = c
	}
	return e, nil
}
func (e *Evaluator) Enabled(ctx context.Context, name string) bool {
	f, ok := e.flags[name]
	i, ok2 := scope.From(ctx)
	b := scope.Branch(ctx)
	if !ok || !ok2 || i.Subject == "" || i.TenantID == "" || b == "" {
		return false
	}
	r := f.Default
	if v, ok := f.Tenants[i.TenantID]; ok {
		r = v
	}
	if v, ok := f.Branches[i.TenantID][b]; ok {
		r = v
	}
	if !r.Enabled {
		return false
	}
	raw, _ := json.Marshal([]string{name, i.TenantID, b, i.Subject})
	h := sha256.Sum256(raw)
	return binary.BigEndian.Uint64(h[:8])%10000 < uint64(r.BasisPoints)
}
