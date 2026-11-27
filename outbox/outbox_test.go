package outbox

import (
	"encoding/json"
	"testing"
)

func TestValidate(t *testing.T) {
	e := Event{"a", "asset.created", json.RawMessage(`{"id":1}`)}
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"asset.*", "asset..created", "asset.>", "asset. bad", "asset.\u00a0bad", ".asset"} {
		e.Subject = s
		if e.Validate() == nil {
			t.Fatal(s)
		}
	}
}
