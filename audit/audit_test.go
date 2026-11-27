package audit

import (
	"context"
	"testing"

	"github.com/HieronimusMorgan/common-golang/requestctx"
	"github.com/HieronimusMorgan/common-golang/scope"
)

func TestAudit(t *testing.T) {
	ctx := scope.WithIdentity(context.Background(), scope.Identity{Subject: "u", TenantID: "t", BranchIDs: []string{"b"}})
	if _, e := New(ctx, "update", "asset", "123"); e == nil {
		t.Fatal("missing branch")
	}
	ctx, _ = scope.SelectBranch(ctx, "b")
	ctx = requestctx.WithID(ctx, "req-1")
	e, err := New(ctx, "update", "asset", "123")
	if err != nil || e.Actor != "u" || e.RequestID != "req-1" || e.Branch != "b" {
		t.Fatal(e, err)
	}
}
