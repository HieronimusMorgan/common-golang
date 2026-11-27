package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/HieronimusMorgan/common-golang/requestctx"
)

func TestJSON(t *testing.T) {
	var b bytes.Buffer
	l, e := New(Config{Service: "assets", JSON: true, Writer: &b})
	if e != nil {
		t.Fatal(e)
	}
	WithContext(requestctx.WithID(context.Background(), "abc"), l).Info("test", "token", "secret-value")
	var m map[string]any
	if e = json.Unmarshal(b.Bytes(), &m); e != nil || m["token"] != "[REDACTED]" || m["request_id"] != "abc" || m["service"] != "assets" {
		t.Fatal(b.String(), e)
	}
}
