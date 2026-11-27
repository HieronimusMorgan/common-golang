package audit

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestChange(t *testing.T) {
	e := Event{ID: "id", Tenant: "t", Branch: "b", Actor: "a", Action: "update", ResourceType: "asset", ResourceID: "1"}
	c, err := Snapshot(map[string]any{"state": "old", "secret": "password"}, map[string]any{"state": "new"}, []string{"state"})
	if err != nil || len(c.Fields) != 1 || c.Before["secret"] != nil {
		t.Fatal(c, err)
	}
	key := bytes.Repeat([]byte{1}, 32)
	mac, err := Seal(e, c, key)
	if err != nil || !Verify(e, c, key, mac) {
		t.Fatal(err)
	}
	c.After["state"] = json.RawMessage(`"changed"`)
	if Verify(e, c, key, mac) {
		t.Fatal("tamper accepted")
	}
	if _, err := Snapshot(map[string]any{"nested": map[string]any{"secret": "x"}}, nil, []string{"nested"}); err == nil {
		t.Fatal("nested accepted")
	}
}
