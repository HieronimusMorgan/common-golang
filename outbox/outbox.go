// Package outbox implements a PostgreSQL transactional outbox. Delivery is at least once.
package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"

	"github.com/HieronimusMorgan/common-golang/database"
	"github.com/HieronimusMorgan/common-golang/requestctx"
)

type Event struct {
	ID, Subject string
	Payload     json.RawMessage
}

func (e Event) Validate() error {
	if !requestctx.ValidID(e.ID) || len(e.Subject) > 255 || e.Subject == "" || strings.ContainsAny(e.Subject, "*>") || strings.ContainsFunc(e.Subject, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) || strings.HasPrefix(e.Subject, ".") || strings.HasSuffix(e.Subject, ".") || strings.Contains(e.Subject, "..") || len(e.Payload) > 1<<20 || !json.Valid(e.Payload) {
		return errors.New("invalid outbox event")
	}
	return nil
}

// Enqueue must receive the same transaction that writes business state.
func Enqueue(ctx context.Context, tx *sql.Tx, e Event) error {
	if tx == nil {
		return errors.New("transaction required")
	}
	if err := e.Validate(); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO common_outbox(id,subject,payload) VALUES($1,$2,$3::jsonb)`, e.ID, e.Subject, string(e.Payload))
	return err
}

// Publisher must await a durable broker acknowledgement and use event ID as deduplication ID.
type Publisher func(context.Context, Event) error
type Worker struct {
	DB                  *sql.DB
	Publish             Publisher
	Timeout, RetryDelay time.Duration
	MaxAttempts         int
}
type Result struct {
	ID              string
	Published, Dead bool
	PublishError    error
}

// RunOne locks one row, publishes with a bounded context and commits its outcome. A crash after publish may redeliver. Concurrent workers skip locked rows. Publisher must honor context.
func (w Worker) RunOne(ctx context.Context) (result Result, err error) {
	if w.DB == nil || w.Publish == nil || w.Timeout < 0 || w.RetryDelay < 0 || w.MaxAttempts < 0 {
		return result, errors.New("invalid outbox worker")
	}
	if w.Timeout == 0 {
		w.Timeout = 5 * time.Second
	}
	if w.RetryDelay == 0 {
		w.RetryDelay = 10 * time.Second
	}
	if w.MaxAttempts == 0 {
		w.MaxAttempts = 10
	}
	run, cancel := context.WithTimeout(ctx, w.Timeout+5*time.Second)
	defer cancel()
	err = database.WithinTx(run, w.DB, &sql.TxOptions{Isolation: sql.LevelReadCommitted}, func(ctx context.Context, tx *sql.Tx) error {
		var e Event
		var attempts int
		err := tx.QueryRowContext(ctx, `SELECT id,subject,payload,attempts FROM common_outbox WHERE published_at IS NULL AND dead_at IS NULL AND available_at <= now() ORDER BY available_at,created_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&e.ID, &e.Subject, &e.Payload, &attempts)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		result.ID = e.ID
		p, cancel := context.WithTimeout(ctx, w.Timeout)
		defer cancel()
		result.PublishError = publishSafely(p, w.Publish, e)
		if result.PublishError == nil {
			result.PublishError = p.Err()
		}
		if result.PublishError == nil {
			_, err = tx.ExecContext(ctx, `UPDATE common_outbox SET published_at=clock_timestamp(), attempts=attempts+1 WHERE id=$1`, e.ID)
			result.Published = err == nil
			return err
		}
		if attempts+1 >= w.MaxAttempts {
			_, err = tx.ExecContext(ctx, `UPDATE common_outbox SET dead_at=clock_timestamp(), attempts=attempts+1 WHERE id=$1`, e.ID)
			result.Dead = err == nil
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE common_outbox SET attempts=attempts+1, available_at=clock_timestamp()+($2::bigint * interval '1 millisecond') WHERE id=$1`, e.ID, max(int64(1), w.RetryDelay.Milliseconds()))
		return err
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func publishSafely(ctx context.Context, fn Publisher, e Event) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("outbox publisher panicked")
		}
	}()
	return fn(ctx, e)
}
