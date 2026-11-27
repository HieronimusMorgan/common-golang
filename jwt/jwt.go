// Package jwt issues and validates access tokens with pinned signing algorithms.
package jwt

import (
	"crypto/rsa"
	"errors"
	"strings"
	"time"

	"github.com/HieronimusMorgan/common-golang/requestctx"
	"github.com/HieronimusMorgan/common-golang/scope"
	jwtlib "github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid access token")

type Config struct {
	Issuer   string
	Audience string
	TTL      time.Duration
	Leeway   time.Duration
}
type Claims struct {
	TenantID    string   `json:"tenant_id,omitempty"`
	BranchIDs   []string `json:"branch_ids,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	TokenUse    string   `json:"token_use"`
	jwtlib.RegisteredClaims
}

func (c Claims) Identity() scope.Identity {
	return scope.Identity{Subject: c.Subject, TenantID: c.TenantID, BranchIDs: c.BranchIDs, Permissions: c.Permissions}
}

type Manager struct {
	cfg       Config
	method    jwtlib.SigningMethod
	signKey   any
	verifyKey any
}

func validate(c Config) error {
	if strings.TrimSpace(c.Issuer) == "" || strings.TrimSpace(c.Audience) == "" || c.TTL < time.Second || c.Leeway < 0 || c.Leeway > time.Minute {
		return errors.New("issuer, audience, TTL >= 1s and leeway 0-60s required")
	}
	return nil
}
func NewHS256(c Config, key []byte) (*Manager, error) {
	if e := validate(c); e != nil {
		return nil, e
	}
	if len(key) < 32 {
		return nil, errors.New("HS256 key must contain at least 32 random bytes")
	}
	k := append([]byte(nil), key...)
	return &Manager{c, jwtlib.SigningMethodHS256, k, k}, nil
}
func NewRS256(c Config, public *rsa.PublicKey, private *rsa.PrivateKey) (*Manager, error) {
	if e := validate(c); e != nil {
		return nil, e
	}
	if public == nil || public.N == nil || public.N.BitLen() < 2048 || public.E < 3 {
		return nil, errors.New("RSA public key must be at least 2048 bits")
	}
	var sign any
	if private != nil {
		if e := private.Validate(); e != nil {
			return nil, e
		}
		if !private.PublicKey.Equal(public) {
			return nil, errors.New("RSA key pair mismatch")
		}
		sign = private
	}
	return &Manager{c, jwtlib.SigningMethodRS256, sign, public}, nil
}
func (m *Manager) Issue(i scope.Identity) (string, error) { return m.issue(i, "") }
func (m *Manager) issue(i scope.Identity, kid string) (string, error) {
	if m.signKey == nil {
		return "", errors.New("manager is verification-only")
	}
	if strings.TrimSpace(i.Subject) == "" {
		return "", errors.New("subject is required")
	}
	id, e := requestctx.NewID()
	if e != nil {
		return "", e
	}
	now := time.Now().UTC()
	c := Claims{TenantID: i.TenantID, BranchIDs: i.BranchIDs, Permissions: i.Permissions, TokenUse: "access", RegisteredClaims: jwtlib.RegisteredClaims{Issuer: m.cfg.Issuer, Subject: i.Subject, Audience: jwtlib.ClaimStrings{m.cfg.Audience}, ExpiresAt: jwtlib.NewNumericDate(now.Add(m.cfg.TTL)), NotBefore: jwtlib.NewNumericDate(now), IssuedAt: jwtlib.NewNumericDate(now), ID: id}}
	t := jwtlib.NewWithClaims(m.method, c)
	if kid != "" {
		t.Header["kid"] = kid
	}
	return t.SignedString(m.signKey)
}
func (m *Manager) Verify(raw string) (*Claims, error) { return m.verify(raw, "") }
func (m *Manager) verify(raw, kid string) (*Claims, error) {
	if len(raw) == 0 || len(raw) > 16384 {
		return nil, ErrInvalidToken
	}
	c := new(Claims)
	t, e := jwtlib.ParseWithClaims(raw, c, func(t *jwtlib.Token) (any, error) {
		if t.Method.Alg() != m.method.Alg() || (kid != "" && t.Header["kid"] != kid) {
			return nil, ErrInvalidToken
		}
		return m.verifyKey, nil
	}, jwtlib.WithValidMethods([]string{m.method.Alg()}), jwtlib.WithIssuer(m.cfg.Issuer), jwtlib.WithAudience(m.cfg.Audience), jwtlib.WithExpirationRequired(), jwtlib.WithIssuedAt(), jwtlib.WithLeeway(m.cfg.Leeway))
	if e != nil || t == nil || !t.Valid || strings.TrimSpace(c.Subject) == "" || c.TokenUse != "access" || c.IssuedAt == nil {
		return nil, ErrInvalidToken
	}
	return c, nil
}
