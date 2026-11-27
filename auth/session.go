// Package auth provides session primitives; persistence and rotation are authentication-service responsibilities.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

type Revocations interface {
	Revoked(ctx context.Context, issuer, tokenID string) (bool, error)
}

// NewRefreshToken returns an opaque 256-bit token and its storage digest. Never store or log the raw value.
func NewRefreshToken() (string, [32]byte, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", [32]byte{}, err
	}
	raw := base64.RawURLEncoding.EncodeToString(b[:])
	return raw, sha256.Sum256([]byte(raw)), nil
}
func RefreshDigest(raw string) ([32]byte, error) {
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(b) != 32 || base64.RawURLEncoding.EncodeToString(b) != raw {
		return [32]byte{}, errors.New("invalid refresh token")
	}
	return sha256.Sum256([]byte(raw)), nil
}
