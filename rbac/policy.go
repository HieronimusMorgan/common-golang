// Package rbac provides deny-by-default role and resource authorization.
package rbac

import (
	"context"
	"errors"
	"github.com/HieronimusMorgan/common-golang/scope"
)

type Resource struct{ Tenant, Branch, Owner string }
type Policy interface {
	Allow(context.Context, scope.Identity, string, Resource) (bool, error)
}
type PolicyFunc func(context.Context, scope.Identity, string, Resource) (bool, error)

func (f PolicyFunc) Allow(c context.Context, i scope.Identity, a string, r Resource) (bool, error) {
	return f(c, i, a, r)
}

var ErrDenied = errors.New("access denied")

// Require enforces scope even for custom policies. Cross-branch commands must authorize both branches separately.
func Require(ctx context.Context, action string, r Resource, p Policy) error {
	i, ok := scope.From(ctx)
	if !ok || i.Subject == "" || i.TenantID == "" || r.Tenant != i.TenantID || r.Branch == "" || r.Branch != scope.Branch(ctx) || action == "" || p == nil {
		return ErrDenied
	}
	allowed, err := p.Allow(ctx, i, action, r)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrDenied
	}
	return nil
}
func Permissions(ownerOnly bool) Policy {
	return PolicyFunc(func(_ context.Context, i scope.Identity, a string, r Resource) (bool, error) {
		return i.HasPermission(a) && (!ownerOnly || r.Owner == i.Subject), nil
	})
}

// ResolveRoles expands only server-assigned roles. Unknown roles add no permissions.
func ResolveRoles(roles []string, catalog map[string][]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range roles {
		for _, p := range catalog[r] {
			if p != "" && !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	return out
}
