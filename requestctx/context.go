// Package requestctx carries transport-neutral request correlation IDs.
package requestctx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"regexp"
)

type key struct{}

var valid = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func ValidID(id string) bool { return valid.MatchString(id) }
func NewID() (string, error) {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", e
	}
	return hex.EncodeToString(b[:]), nil
}
func WithID(ctx context.Context, id string) context.Context {
	if !ValidID(id) {
		return ctx
	}
	return context.WithValue(ctx, key{}, id)
}
func ID(ctx context.Context) string { v, _ := ctx.Value(key{}).(string); return v }
