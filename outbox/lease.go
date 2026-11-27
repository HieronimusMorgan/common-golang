package outbox

import (
	"context"
	"database/sql"
	"errors"
	"github.com/HieronimusMorgan/common-golang/database"
	"github.com/HieronimusMorgan/common-golang/requestctx"
	"time"
)

var ErrClaimLost = errors.New("outbox claim lost")

type Claim struct {
	Event    Event
	Token    string
	Attempts int
}

// ClaimBatch commits short database claims before publication. Never mix legacy RunOne workers with leased workers until all are upgraded.
func ClaimBatch(ctx context.Context, db *sql.DB, limit int, lease time.Duration, maxAttempts int) ([]Claim, error) {
	if limit < 1 || limit > 100 || lease < time.Second || lease > time.Hour || maxAttempts < 1 {
		return nil, errors.New("invalid claim options")
	}
	token, err := requestctx.NewID()
	if err != nil {
		return nil, err
	}
	var out []Claim
	err = database.WithinTx(ctx, db, nil, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE common_outbox SET dead_at=clock_timestamp(),claim_token=NULL,claim_until=NULL WHERE published_at IS NULL AND dead_at IS NULL AND attempts >= $1 AND (claim_until IS NULL OR claim_until<=clock_timestamp())`, maxAttempts); err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx, `WITH picked AS (SELECT id FROM common_outbox WHERE published_at IS NULL AND dead_at IS NULL AND available_at<=clock_timestamp() AND (claim_until IS NULL OR claim_until<=clock_timestamp()) AND attempts<$4 ORDER BY available_at,created_at,id LIMIT $1 FOR UPDATE SKIP LOCKED) UPDATE common_outbox o SET claim_token=$2,claim_until=clock_timestamp()+($3::bigint*interval '1 millisecond'),attempts=o.attempts+1 FROM picked WHERE o.id=picked.id RETURNING o.id,o.subject,o.payload,o.attempts`, limit, token, lease.Milliseconds(), maxAttempts)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			c := Claim{Token: token}
			if err = rows.Scan(&c.Event.ID, &c.Event.Subject, &c.Event.Payload, &c.Attempts); err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Complete uses a compare-and-set ownership guard. An expired claim cannot mark a newer claim complete.
func Complete(ctx context.Context, db *sql.DB, c Claim, publishErr error, retry time.Duration, maxAttempts int) error {
	if db == nil || !requestctx.ValidID(c.Token) || !requestctx.ValidID(c.Event.ID) || retry < time.Millisecond || retry > 24*time.Hour || maxAttempts < 1 {
		return errors.New("invalid completion")
	}
	r, err := db.ExecContext(ctx, `UPDATE common_outbox SET published_at=CASE WHEN $3 THEN clock_timestamp() ELSE NULL END,dead_at=CASE WHEN NOT $3 AND attempts >= $4 THEN clock_timestamp() ELSE NULL END,available_at=clock_timestamp()+($5::bigint*interval '1 millisecond'),claim_token=NULL,claim_until=NULL WHERE id=$1 AND claim_token=$2 AND claim_until>clock_timestamp() AND published_at IS NULL AND dead_at IS NULL`, c.Event.ID, c.Token, publishErr == nil, maxAttempts, retry.Milliseconds())
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrClaimLost
	}
	return nil
}

// Redrive is an explicit administrative operation; consumers must retain dedupe records through replay windows.
func Redrive(ctx context.Context, db *sql.DB, id string) error {
	if db == nil || !requestctx.ValidID(id) {
		return errors.New("invalid redrive")
	}
	r, err := db.ExecContext(ctx, `UPDATE common_outbox SET dead_at=NULL,attempts=0,available_at=clock_timestamp(),claim_token=NULL,claim_until=NULL WHERE id=$1 AND dead_at IS NOT NULL AND published_at IS NULL`, id)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return sql.ErrNoRows
	}
	return nil
}
