package idempotency

import (
	"context"
	"testing"

	"github.com/HieronimusMorgan/common-golang/scope"
)

func TestKey(t *testing.T) {
	ctx := scope.WithIdentity(context.Background(), scope.Identity{Subject: "u", TenantID: "t", BranchIDs: []string{"b"}})
	if _, e := FromContext(ctx, "create", "key"); e == nil {
		t.Fatal("no branch")
	}
	ctx, e := scope.SelectBranch(ctx, "b")
	if e != nil {
		t.Fatal(e)
	}
	k, e := FromContext(ctx, "create", "key")
	if e != nil || k.Branch != "b" || k.Tenant != "t" {
		t.Fatal(k, e)
	}
	if Fingerprint([]byte("a")) == Fingerprint([]byte("b")) {
		t.Fatal("hash")
	}
}
