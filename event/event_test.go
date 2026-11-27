package event

import (
	"context"
	"github.com/HieronimusMorgan/common-golang/scope"
	"testing"
)

func TestNew(t *testing.T) {
	ctx := scope.WithIdentity(context.Background(), scope.Identity{Subject: "a", TenantID: "t", BranchIDs: []string{"b"}})
	if _, err := New(ctx, "created", 1, "asset", "asset-1", 1); err == nil {
		t.Fatal("accepted missing branch")
	}
	ctx, _ = scope.SelectBranch(ctx, "b")
	e, err := New(ctx, "created", 1, "asset", "asset-1", 1)
	if err != nil || e.BranchID != "b" || e.TenantID != "t" {
		t.Fatal(e, err)
	}
	e.Version = 0
	if e.Validate() == nil {
		t.Fatal("accepted invalid version")
	}
}
