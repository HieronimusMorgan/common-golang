package featureflag

import (
	"context"
	"github.com/HieronimusMorgan/common-golang/scope"
	"testing"
)

func TestPrecedence(t *testing.T) {
	f := Flag{Default: Rule{true, 10000}, Tenants: map[string]Rule{"t": {false, 0}}, Branches: map[string]map[string]Rule{"t": {"b": {true, 10000}}}}
	e, err := New(map[string]Flag{"x": f})
	if err != nil {
		t.Fatal(err)
	}
	ctx := scope.WithIdentity(context.Background(), scope.Identity{Subject: "a", TenantID: "t", BranchIDs: []string{"b", "c"}})
	b, _ := scope.SelectBranch(ctx, "b")
	c, _ := scope.SelectBranch(ctx, "c")
	if !e.Enabled(b, "x") || e.Enabled(c, "x") || e.Enabled(ctx, "x") || e.Enabled(b, "unknown") {
		t.Fatal("bad precedence")
	}
	f.Branches["t"]["b"] = Rule{}
	if !e.Enabled(b, "x") {
		t.Fatal("mutable input")
	}
}
