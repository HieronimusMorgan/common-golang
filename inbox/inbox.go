// Package inbox deduplicates transactional consumer effects. Broker ack must follow a successful commit.
package inbox

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/HieronimusMorgan/common-golang/database"
	"github.com/HieronimusMorgan/common-golang/event"
	"github.com/HieronimusMorgan/common-golang/requestctx"
)

var ErrConflict = errors.New("event ID reused with different content")

type Handler func(context.Context, *sql.Tx, event.Envelope) error

// Process returns true for a duplicate. The handler may only perform transactional effects using tx. Event authenticity is the caller's responsibility.
func Process(ctx context.Context, db *sql.DB, consumer string, e event.Envelope, h Handler) (bool, error) {
	if !requestctx.ValidID(consumer) || h == nil {
		return false, errors.New("consumer and handler required")
	}
	if err := e.Validate(); err != nil {
		return false, err
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return false, err
	}
	hash := sha256.Sum256(raw)
	duplicate := false
	err = database.WithinTx(ctx, db, &sql.TxOptions{Isolation: sql.LevelReadCommitted}, func(ctx context.Context, tx *sql.Tx) error {
		r, err := tx.ExecContext(ctx, `INSERT INTO common_inbox(consumer,source,event_id,fingerprint) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, consumer, e.Source, e.ID, hash[:])
		if err != nil {
			return err
		}
		n, err := r.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			var old []byte
			if err = tx.QueryRowContext(ctx, `SELECT fingerprint FROM common_inbox WHERE consumer=$1 AND source=$2 AND event_id=$3`, consumer, e.Source, e.ID).Scan(&old); err != nil {
				return err
			}
			if !bytes.Equal(old, hash[:]) {
				return ErrConflict
			}
			duplicate = true
			return nil
		}
		return h(ctx, tx, e)
	})
	return duplicate && err == nil, err
}
