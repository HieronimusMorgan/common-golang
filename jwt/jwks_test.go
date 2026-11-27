package jwt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"github.com/HieronimusMorgan/common-golang/scope"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestJWKS(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var count atomic.Int32
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		enc := base64.RawURLEncoding.EncodeToString
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{"kid": "k1", "kty": "RSA", "use": "sig", "alg": "RS256", "n": enc(key.N.Bytes()), "e": enc(big.NewInt(int64(key.E)).Bytes())}}})
	}))
	defer s.Close()
	cfg := Config{Issuer: "issuer", Audience: "asset", TTL: time.Minute}
	m, err := NewRS256(cfg, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := m.issue(scope.Identity{Subject: "actor"}, "k1")
	if err != nil {
		t.Fatal(err)
	}
	v, err := NewJWKS(cfg, s.URL, time.Minute, s.Client().Transport, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		c, err := v.VerifyContext(context.Background(), raw)
		if err != nil || c.Subject != "actor" {
			t.Fatal(c, err)
		}
	}
	if count.Load() != 1 {
		t.Fatal(count.Load())
	}
	other, err := m.issue(scope.Identity{Subject: "actor"}, "unknown")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = v.Verify(other); err == nil {
		t.Fatal("unknown kid accepted")
	}
	if count.Load() != 1 {
		t.Fatal("unknown kid triggered fetch")
	}
}
