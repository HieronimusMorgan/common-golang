package validation

import (
	"strings"
	"testing"
)

func TestDecode(t *testing.T) {
	type input struct {
		Name   string `json:"name"`
		Nested struct {
			K int `json:"k"`
		} `json:"nested"`
	}
	for _, s := range []string{`null`, `[]`, `{"name":"a","Name":"b"}`, `{"nested":{"K":1,"K":2}}`, `{"unknown":1}`, `{"name":"a"}{}`, `{"name":`, "{\"name\":\"\xff\"}"} {
		var v input
		if DecodeJSON(strings.NewReader(s), 1024, &v) == nil {
			t.Errorf("accepted %q", s)
		}
	}
	var v input
	if e := DecodeJSON(strings.NewReader(`{"name":"asset"}`), 1024, &v); e != nil || v.Name != "asset" {
		t.Fatal(v, e)
	}
	if DecodeJSON(strings.NewReader(`{"name":"asset"}`), 4, &v) == nil {
		t.Fatal("limit")
	}
	var m map[string]any
	if DecodeJSON(strings.NewReader(`{}`), 20, &m) == nil {
		t.Fatal("map accepted")
	}
}
func TestErrors(t *testing.T) {
	var e Errors
	e.Require("name", " ")
	e.MaxBytes("code", "long", 2)
	if e.Err() == nil || len(e) != 2 {
		t.Fatal(e)
	}
}
