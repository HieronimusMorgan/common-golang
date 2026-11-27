package scope

import (
	"context"
	"testing"
)

func TestKeyIsolation(t *testing.T) {
	c := func(tenant, branch, actor string) context.Context {
		ctx := WithIdentity(context.Background(), Identity{Subject: actor, TenantID: tenant, BranchIDs: []string{branch}})
		ctx, _ = SelectBranch(ctx, branch)
		return ctx
	}
	a, e := Key(c("t", "b", "a"), "cache", "asset", true)
	if e != nil {
		t.Fatal(e)
	}
	for _, ctx := range []context.Context{c("u", "b", "a"), c("t", "c", "a"), c("t", "b", "z")} {
		b, e := Key(ctx, "cache", "asset", true)
		if e != nil || a == b {
			t.Fatal("collision")
		}
	}
}
