// Package retention exposes bounded administrative cleanup. Cutoffs must exceed business replay windows.
package retention

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Kind string

const (
	PublishedOutbox Kind = "published_outbox"
	Inbox           Kind = "inbox"
	Idempotency     Kind = "idempotency"
)

// PurgeBatch never deletes audit or pending/dead outbox rows. Caller supplies a reviewed cutoff; no automatic cleanup.
func PurgeBatch(ctx context.Context, db *sql.DB, kind Kind, before time.Time, limit int) (int64, error) {
	if db == nil || before.IsZero() || !before.Before(time.Now()) || limit < 1 || limit > 10000 {
		return 0, errors.New("invalid retention request")
	}
	var q string
	switch kind {
	case PublishedOutbox:
		q = `DELETE FROM common_outbox WHERE id IN (SELECT id FROM common_outbox WHERE published_at<$1 ORDER BY published_at LIMIT $2 FOR UPDATE SKIP LOCKED)`
	case Inbox:
		q = `DELETE FROM common_inbox WHERE (consumer,source,event_id) IN (SELECT consumer,source,event_id FROM common_inbox WHERE processed_at<$1 ORDER BY processed_at LIMIT $2 FOR UPDATE SKIP LOCKED)`
	case Idempotency:
		q = `DELETE FROM common_idempotency WHERE (tenant_id,branch_id,actor_id,operation,request_key) IN (SELECT tenant_id,branch_id,actor_id,operation,request_key FROM common_idempotency WHERE created_at<$1 AND response IS NOT NULL ORDER BY created_at LIMIT $2 FOR UPDATE SKIP LOCKED)`
	default:
		return 0, errors.New("unsupported retention kind")
	}
	r, err := db.ExecContext(ctx, q, before, limit)
	if err != nil {
		return 0, err
	}
	return r.RowsAffected()
}
