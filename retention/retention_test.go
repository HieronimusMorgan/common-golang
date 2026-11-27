package retention

import (
	"context"
	"testing"
	"time"
)

func TestRejectUnsafe(t *testing.T) {
	if _, e := PurgeBatch(context.Background(), nil, Kind("audit"), time.Now().Add(-time.Hour), 1); e == nil {
		t.Fatal("unsafe deletion accepted")
	}
}
