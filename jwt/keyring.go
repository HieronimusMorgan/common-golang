package jwt

import (
	"errors"
	"sync"

	"github.com/HieronimusMorgan/common-golang/requestctx"
	"github.com/HieronimusMorgan/common-golang/scope"
	jwtlib "github.com/golang-jwt/jwt/v5"
)

// KeyRing supports overlapping verification keys. Rotate replaces the complete immutable snapshot atomically. All keys must have identical claim policy and algorithm. Publish new public keys before switching signers; retire old keys after their tokens expire.
type KeyRing struct {
	mu     sync.RWMutex
	active string
	keys   map[string]*Manager
	cfg    Config
	alg    string
}

func NewKeyRing(active string, keys map[string]*Manager) (*KeyRing, error) {
	r := &KeyRing{}
	if e := r.Rotate(active, keys); e != nil {
		return nil, e
	}
	return r, nil
}
func (r *KeyRing) Rotate(active string, keys map[string]*Manager) error {
	if len(keys) < 1 || len(keys) > 64 || !requestctx.ValidID(active) || keys[active] == nil {
		return errors.New("invalid key ring")
	}
	m := keys[active]
	if m.method == nil {
		return errors.New("invalid active manager")
	}
	copyKeys := make(map[string]*Manager, len(keys))
	for id, k := range keys {
		if !requestctx.ValidID(id) || k == nil || k.method == nil || k.cfg != m.cfg || k.method.Alg() != m.method.Alg() {
			return errors.New("inconsistent key ring policy")
		}
		copyKeys[id] = k
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.keys != nil && (r.cfg != m.cfg || r.alg != m.method.Alg()) {
		return errors.New("cannot change key ring policy during rotation")
	}
	r.active, r.keys, r.cfg, r.alg = active, copyKeys, m.cfg, m.method.Alg()
	return nil
}
func (r *KeyRing) Issue(i scope.Identity) (string, error) {
	r.mu.RLock()
	id, m := r.active, r.keys[r.active]
	r.mu.RUnlock()
	if m == nil {
		return "", errors.New("key ring not configured")
	}
	return m.issue(i, id)
}
func (r *KeyRing) Verify(raw string) (*Claims, error) {
	if len(raw) == 0 || len(raw) > 16384 {
		return nil, ErrInvalidToken
	}
	t, _, e := jwtlib.NewParser().ParseUnverified(raw, new(Claims))
	if e != nil || t == nil {
		return nil, ErrInvalidToken
	}
	id, ok := t.Header["kid"].(string)
	if !ok || !requestctx.ValidID(id) {
		return nil, ErrInvalidToken
	}
	r.mu.RLock()
	m := r.keys[id]
	r.mu.RUnlock()
	if m == nil {
		return nil, ErrInvalidToken
	}
	return m.verify(raw, id)
}
