// Package scope stores an already-authorized identity and branch selection.
package scope

import (
	"context"
	"errors"
	"strings"
)

type Identity struct {
	Subject     string
	TenantID    string
	BranchIDs   []string
	Permissions []string
}
type identityKey struct{}
type branchKey struct{}

func copyIdentity(i Identity) Identity {
	i.BranchIDs = append([]string(nil), i.BranchIDs...)
	i.Permissions = append([]string(nil), i.Permissions...)
	return i
}
func WithIdentity(ctx context.Context, i Identity) context.Context {
	return context.WithValue(context.WithValue(ctx, branchKey{}, ""), identityKey{}, copyIdentity(i))
}
func From(ctx context.Context) (Identity, bool) {
	i, ok := ctx.Value(identityKey{}).(Identity)
	return copyIdentity(i), ok
}
func (i Identity) AllowsBranch(branch string) bool {
	if strings.TrimSpace(branch) == "" {
		return false
	}
	for _, b := range i.BranchIDs {
		if b == branch {
			return true
		}
	}
	return false
}
func (i Identity) HasPermission(p string) bool {
	if p == "" {
		return false
	}
	for _, v := range i.Permissions {
		if v == p {
			return true
		}
	}
	return false
}
func SelectBranch(ctx context.Context, branch string) (context.Context, error) {
	i, ok := From(ctx)
	if !ok || i.Subject == "" || i.TenantID == "" || !i.AllowsBranch(branch) {
		return ctx, errors.New("branch access denied")
	}
	return context.WithValue(ctx, branchKey{}, branch), nil
}
func Branch(ctx context.Context) string {
	b, _ := ctx.Value(branchKey{}).(string)
	i, ok := From(ctx)
	if !ok || !i.AllowsBranch(b) {
		return ""
	}
	return b
}
