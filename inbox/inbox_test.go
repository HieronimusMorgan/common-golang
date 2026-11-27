package inbox

import (
	"context"
	"database/sql"
	"github.com/HieronimusMorgan/common-golang/event"
	"testing"
)

func TestInvalidEvent(t *testing.T) {
	h := func(context.Context, *sql.Tx, event.Envelope) error {
		t.Fatal("invalid event invoked handler")
		return nil
	}
	if _, e := Process(context.Background(), nil, "consumer", event.Envelope{}, h); e == nil {
		t.Fatal("invalid event accepted")
	}
}
