package jwt

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/HieronimusMorgan/common-golang/auth"
	"github.com/HieronimusMorgan/common-golang/requestctx"
	jwtlib "github.com/golang-jwt/jwt/v5"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// JWKSVerifier fetches RS256 public keys from a fixed trusted HTTPS endpoint. No token-supplied URLs or redirects. Refresh errors fail closed after expiry.
type JWKSVerifier struct {
	cfg         Config
	endpoint    string
	client      *http.Client
	ttl         time.Duration
	mu          sync.RWMutex
	keys        map[string]*Manager
	expires     time.Time
	refresh     chan struct{}
	revocations auth.Revocations
}

func NewJWKS(c Config, endpoint string, ttl time.Duration, transport http.RoundTripper, rev auth.Revocations) (*JWKSVerifier, error) {
	if err := validate(c); err != nil {
		return nil, err
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || ttl < time.Second || ttl > time.Hour {
		return nil, errors.New("invalid JWKS configuration")
	}
	return &JWKSVerifier{cfg: c, endpoint: endpoint, ttl: ttl, client: &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, revocations: rev, refresh: make(chan struct{}, 1)}, nil
}
func (v *JWKSVerifier) snapshot() (map[string]*Manager, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.keys, time.Now().Before(v.expires)
}
func (v *JWKSVerifier) load(ctx context.Context) error {
	select {
	case v.refresh <- struct{}{}:
		defer func() { <-v.refresh }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if _, fresh := v.snapshot(); fresh {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New("JWKS HTTP failure")
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 {
		return errors.New("invalid JWKS body")
	}
	var set struct {
		Keys []struct{ Kid, Kty, Use, Alg, N, E string }
	}
	if json.Unmarshal(raw, &set) != nil || len(set.Keys) > 64 {
		return errors.New("invalid JWKS")
	}
	keys := map[string]*Manager{}
	for _, k := range set.Keys {
		if k.Kty != "RSA" || (k.Use != "" && k.Use != "sig") || (k.Alg != "" && k.Alg != "RS256") {
			continue
		}
		if !requestctx.ValidID(k.Kid) || keys[k.Kid] != nil {
			return errors.New("invalid JWKS key ID")
		}
		n, e := base64.RawURLEncoding.DecodeString(k.N)
		exp, e2 := base64.RawURLEncoding.DecodeString(k.E)
		if e != nil || e2 != nil || len(n) > 1024 || len(exp) > 4 {
			return errors.New("invalid RSA JWK")
		}
		x := new(big.Int).SetBytes(exp)
		if !x.IsInt64() || x.Int64() > 2147483647 || x.Int64()%2 == 0 {
			return errors.New("invalid exponent")
		}
		m, err := NewRS256(v.cfg, &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(x.Int64())}, nil)
		if err != nil {
			return err
		}
		keys[k.Kid] = m
	}
	if len(keys) == 0 {
		return errors.New("no valid keys")
	}
	v.mu.Lock()
	v.keys = keys
	v.expires = time.Now().Add(v.ttl)
	v.mu.Unlock()
	return nil
}

// VerifyContext checks revocation after signature validation. Unknown kids fail until cache expiry; publish new keys before use.
func (v *JWKSVerifier) VerifyContext(ctx context.Context, raw string) (*Claims, error) {
	if len(raw) == 0 || len(raw) > 16384 {
		return nil, ErrInvalidToken
	}
	t, _, err := jwtlib.NewParser().ParseUnverified(raw, new(Claims))
	if err != nil || t == nil || t.Method == nil || t.Method.Alg() != "RS256" {
		return nil, ErrInvalidToken
	}
	kid, ok := t.Header["kid"].(string)
	if !ok || !requestctx.ValidID(kid) {
		return nil, ErrInvalidToken
	}
	keys, fresh := v.snapshot()
	if !fresh {
		if err = v.load(ctx); err != nil {
			return nil, err
		}
		keys, _ = v.snapshot()
	}
	m := keys[kid]
	if m == nil {
		return nil, ErrInvalidToken
	}
	c, err := m.verify(raw, kid)
	if err != nil {
		return nil, err
	}
	if v.revocations != nil {
		if c.ID == "" {
			return nil, ErrInvalidToken
		}
		revoked, err := v.revocations.Revoked(ctx, c.Issuer, c.ID)
		if err != nil {
			return nil, err
		}
		if revoked {
			return nil, ErrInvalidToken
		}
	}
	return c, nil
}
func (v *JWKSVerifier) Verify(raw string) (*Claims, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	return v.VerifyContext(ctx, raw)
}
