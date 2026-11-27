package scope

import (
	"context"
	"testing"
)

func TestBranch(t *testing.T) {
	i := Identity{Subject: "u", TenantID: "t", BranchIDs: []string{"b1"}, Permissions: []string{"asset.read"}}
	ctx := WithIdentity(context.Background(), i)
	i.BranchIDs[0] = "b2"
	if _, e := SelectBranch(ctx, "b2"); e == nil {
		t.Fatal("unauthorized")
	}
	ctx, e := SelectBranch(ctx, "b1")
	if e != nil || Branch(ctx) != "b1" {
		t.Fatal(e)
	}
	copy, _ := From(ctx)
	copy.BranchIDs[0] = "b2"
	if Branch(ctx) != "b1" {
		t.Fatal("mutable context")
	}
	ctx = WithIdentity(ctx, Identity{Subject: "new", TenantID: "new", BranchIDs: []string{"b1"}})
	if Branch(ctx) != "" {
		t.Fatal("scope carried across principals")
	}
}
