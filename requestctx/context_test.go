package requestctx

import (
	"context"
	"testing"
)

func TestIDs(t *testing.T) {
	a, e := NewID()
	if e != nil || !ValidID(a) {
		t.Fatal(a, e)
	}
	b, e := NewID()
	if e != nil || a == b {
		t.Fatal(b, e)
	}
	ctx := WithID(context.Background(), a)
	if ID(WithID(ctx, "bad\nvalue")) != a {
		t.Fatal("invalid ID replaced trusted value")
	}
}
