package audit

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
)

// Change stores only explicitly allowlisted top-level fields. Never allow nested secrets; only scalar values accepted.
type Change struct {
	Before map[string]json.RawMessage `json:"before"`
	After  map[string]json.RawMessage `json:"after"`
	Fields []string                   `json:"changed_fields"`
}

func Snapshot(before, after map[string]any, allow []string) (Change, error) {
	c := Change{map[string]json.RawMessage{}, map[string]json.RawMessage{}, nil}
	seen := map[string]bool{}
	for _, k := range allow {
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		for _, pair := range []struct {
			src map[string]any
			dst map[string]json.RawMessage
		}{{before, c.Before}, {after, c.After}} {
			if v, ok := pair.src[k]; ok {
				b, err := json.Marshal(v)
				if err != nil || len(b) > 4096 || len(b) == 0 || b[0] == '{' || b[0] == '[' {
					return Change{}, errors.New("audit values must be bounded scalars")
				}
				pair.dst[k] = b
			}
		}
		if !bytes.Equal(c.Before[k], c.After[k]) {
			c.Fields = append(c.Fields, k)
		}
	}
	sort.Strings(c.Fields)
	return c, nil
}

// Seal provides per-record tamper evidence (not deletion detection). Keep HMAC keys outside the database.
func Seal(e Event, c Change, key []byte) ([]byte, error) {
	if len(key) < 32 {
		return nil, errors.New("audit MAC key must contain at least 32 random bytes")
	}
	if err := e.Validate(); err != nil {
		return nil, err
	}
	b, err := json.Marshal(struct {
		Event  Event
		Change Change
	}{e, c})
	if err != nil {
		return nil, err
	}
	if len(b) > 64<<10 {
		return nil, errors.New("audit change too large")
	}
	h := hmac.New(sha256.New, key)
	h.Write([]byte("common-audit-change-v1\x00"))
	h.Write(b)
	return h.Sum(nil), nil
}
func Verify(e Event, c Change, key, mac []byte) bool {
	expected, err := Seal(e, c, key)
	return err == nil && hmac.Equal(expected, mac)
}
func AppendChange(ctx context.Context, tx *sql.Tx, e Event, c Change, keyID string, key []byte) error {
	if tx == nil || keyID == "" || len(keyID) > 128 {
		return errors.New("transaction and key ID required")
	}
	mac, err := Seal(e, c, key)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO common_audit(id,tenant_id,branch_id,actor_id,request_id,action,resource_type,resource_id,change_data,change_mac,mac_key_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, e.ID, e.Tenant, e.Branch, e.Actor, e.RequestID, e.Action, e.ResourceType, e.ResourceID, string(raw), mac, keyID)
	return err
}
