package storage

import (
	"context"
	"github.com/HieronimusMorgan/common-golang/scope"
	"io"
	"strings"
	"testing"
	"time"
)

type fake struct {
	key  string
	body string
}

func (f *fake) Put(_ context.Context, o Object, r io.Reader) error {
	f.key = o.Key
	b, e := io.ReadAll(r)
	f.body = string(b)
	return e
}
func (f *fake) Get(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(f.body)), nil
}
func (f *fake) Delete(context.Context, string) error                              { return nil }
func (f *fake) PresignGet(context.Context, string, time.Duration) (string, error) { return "test", nil }
func TestUpload(t *testing.T) {
	f := new(fake)
	c := Client{f, 8, []string{"text/plain"}}
	ctx := scope.WithIdentity(context.Background(), scope.Identity{Subject: "a", TenantID: "t", BranchIDs: []string{"b"}})
	ctx, _ = scope.SelectBranch(ctx, "b")
	o, e := c.Upload(ctx, "file", strings.NewReader("hello"), "")
	if e != nil || o.Size != 5 || f.body != "hello" || len(o.SHA256) != 64 {
		t.Fatal(o, e)
	}
	for _, s := range []string{"123456789", ""} {
		if _, e = c.Upload(ctx, "file", strings.NewReader(s), ""); e == nil {
			t.Fatal("invalid size accepted")
		}
	}
	if _, e = c.Upload(ctx, "file", strings.NewReader("hello"), "bad"); e == nil {
		t.Fatal("bad checksum accepted")
	}
	if _, e = c.Upload(ctx, "../bad", strings.NewReader("hello"), ""); e == nil {
		t.Fatal("traversal accepted")
	}
}
