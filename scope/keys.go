package scope

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// Key hashes an unambiguous scoped tuple. Include actor when caching user-specific data.
func Key(ctx context.Context, namespace, resource string, perActor bool) (string, error) {
	i, ok := From(ctx)
	b := Branch(ctx)
	if !ok || i.TenantID == "" || i.Subject == "" || b == "" || namespace == "" || resource == "" {
		return "", errors.New("scope and key required")
	}
	a := ""
	if perActor {
		a = i.Subject
	}
	raw, _ := json.Marshal([]string{i.TenantID, b, a, namespace, resource})
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:]), nil
}
