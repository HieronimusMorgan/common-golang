package rbac

import (
	"context"
	"errors"
	"github.com/HieronimusMorgan/common-golang/scope"
	"testing"
)

func TestScope(t *testing.T) {
	ctx := scope.WithIdentity(context.Background(), scope.Identity{Subject: "a", TenantID: "t", BranchIDs: []string{"b"}, Permissions: []string{"read"}})
	ctx, _ = scope.SelectBranch(ctx, "b")
	if err := Require(ctx, "read", Resource{"t", "b", "a"}, Permissions(true)); err != nil {
		t.Fatal(err)
	}
	for _, r := range []Resource{{"other", "b", "a"}, {"t", "other", "a"}, {"t", "b", "other"}} {
		if !errors.Is(Require(ctx, "read", r, Permissions(true)), ErrDenied) {
			t.Fatal("scope bypass", r)
		}
	}
}
