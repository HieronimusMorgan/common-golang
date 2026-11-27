package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"time"

	"github.com/HieronimusMorgan/common-golang/scope"
	jwtlib "github.com/golang-jwt/jwt/v5"
)

func TestJWT(t *testing.T) {
	cfg := Config{Issuer: "auth", Audience: "assets", TTL: time.Minute}
	k := []byte(strings.Repeat("a", 32))
	m, e := NewHS256(cfg, k)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := m.Issue(scope.Identity{Subject: "u", TenantID: "t", BranchIDs: []string{"b"}})
	if e != nil {
		t.Fatal(e)
	}
	c, e := m.Verify(raw)
	if e != nil || !c.Identity().AllowsBranch("b") {
		t.Fatal(e)
	}
	other, _ := NewHS256(Config{Issuer: "auth", Audience: "other", TTL: time.Minute}, k)
	if _, e = other.Verify(raw); e == nil {
		t.Fatal("wrong audience accepted")
	}
	if _, e = m.Verify(raw + "x"); e == nil {
		t.Fatal("invalid signature accepted")
	}
	expired := Claims{TokenUse: "access", RegisteredClaims: jwtlib.RegisteredClaims{Issuer: "auth", Audience: jwtlib.ClaimStrings{"assets"}, Subject: "u", IssuedAt: jwtlib.NewNumericDate(time.Now().Add(-2 * time.Hour)), ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(-time.Hour))}}
	token, _ := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, expired).SignedString(k)
	if _, e = m.Verify(token); e == nil {
		t.Fatal("expired token accepted")
	}
	token, _ = jwtlib.NewWithClaims(jwtlib.SigningMethodHS384, expired).SignedString(k)
	if _, e = m.Verify(token); e == nil {
		t.Fatal("wrong algorithm accepted")
	}
}
func TestRS256(t *testing.T) {
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	cfg := Config{Issuer: "auth", Audience: "assets", TTL: time.Minute}
	sign, e := NewRS256(cfg, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	verify, e := NewRS256(cfg, &key.PublicKey, nil)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := sign.Issue(scope.Identity{Subject: "u"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = verify.Verify(raw); e != nil {
		t.Fatal(e)
	}
	if _, e = verify.Issue(scope.Identity{Subject: "u"}); e == nil {
		t.Fatal("verification-only manager issued token")
	}
}

func TestRotation(t *testing.T) {
	cfg := Config{Issuer: "auth", Audience: "assets", TTL: time.Minute}
	a, _ := NewHS256(cfg, []byte(strings.Repeat("a", 32)))
	b, _ := NewHS256(cfg, []byte(strings.Repeat("b", 32)))
	ring, e := NewKeyRing("old", map[string]*Manager{"old": a})
	if e != nil {
		t.Fatal(e)
	}
	old, e := ring.Issue(scope.Identity{Subject: "u"})
	if e != nil {
		t.Fatal(e)
	}
	if e = ring.Rotate("new", map[string]*Manager{"old": a, "new": b}); e != nil {
		t.Fatal(e)
	}
	fresh, e := ring.Issue(scope.Identity{Subject: "u"})
	if e != nil {
		t.Fatal(e)
	}
	for _, token := range []string{old, fresh} {
		if _, e = ring.Verify(token); e != nil {
			t.Fatal(e)
		}
	}
	if e = ring.Rotate("new", map[string]*Manager{"new": b}); e != nil {
		t.Fatal(e)
	}
	if _, e = ring.Verify(old); e == nil {
		t.Fatal("retired key accepted")
	}
	legacy, _ := b.Issue(scope.Identity{Subject: "u"})
	if _, e = ring.Verify(legacy); e == nil {
		t.Fatal("missing kid accepted")
	}
}
