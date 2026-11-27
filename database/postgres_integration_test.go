//go:build postgres_integration

package database_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HieronimusMorgan/common-golang/audit"
	"github.com/HieronimusMorgan/common-golang/database"
	"github.com/HieronimusMorgan/common-golang/idempotency"
	"github.com/HieronimusMorgan/common-golang/outbox"
	"github.com/HieronimusMorgan/common-golang/requestctx"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Only use an isolated, disposable test database. RunOne deliberately scans the outbox globally.
func TestPostgresIntegration(t *testing.T) {
	if os.Getenv("COMMON_POSTGRES_INTEGRATION") != "1" {
		t.Skip("requires isolated PostgreSQL test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dsn := os.Getenv("COMMON_POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgres://common:common-local-only@127.0.0.1:5432/common_test?sslmode=disable"
	}
	db, e := database.Open(ctx, "pgx", dsn, database.Config{})
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	// Create a separate schema, keeping any unrelated database records untouched.
	id, e := requestctx.NewID()
	if e != nil {
		t.Fatal(e)
	}
	schema := "common_test_" + id
	if _, e = db.ExecContext(ctx, `CREATE SCHEMA `+schema); e != nil {
		t.Fatal(e)
	}
	defer func() {
		clean, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		if _, e := db.ExecContext(clean, `DROP SCHEMA `+schema+` CASCADE`); e != nil {
			t.Error(e)
		}
	}()
	// pgx supports search_path as a startup parameter. Restrict DSN format to URL for this fixture.
	scoped, e := scopedDSN(dsn, schema)
	if e != nil {
		t.Fatal(e)
	}
	sdb, e := database.Open(ctx, "pgx", scoped, database.Config{})
	if e != nil {
		t.Fatal(e)
	}
	defer sdb.Close()
	ddl, e := os.ReadFile("../migrations/001_common.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = sdb.ExecContext(ctx, string(ddl)); e != nil {
		t.Fatal(e)
	}
	if _, e = sdb.ExecContext(ctx, `CREATE TABLE asset_fixture(id text PRIMARY KEY)`); e != nil {
		t.Fatal(e)
	}
	key := idempotency.Key{Tenant: "tenant", Branch: "branch", Actor: "actor", Operation: "asset.create", Request: "request"}
	finger := idempotency.Fingerprint([]byte("asset"))
	var calls atomic.Int32
	fn := func(ctx context.Context, tx *sql.Tx) ([]byte, error) {
		calls.Add(1)
		if _, e := tx.ExecContext(ctx, `INSERT INTO asset_fixture(id) VALUES('asset')`); e != nil {
			return nil, e
		}
		if e := audit.Append(ctx, tx, audit.Event{ID: "audit-1", Tenant: "tenant", Branch: "branch", Actor: "actor", Action: "create", ResourceType: "asset", ResourceID: "asset"}); e != nil {
			return nil, e
		}
		if e := outbox.Enqueue(ctx, tx, outbox.Event{ID: "event-1", Subject: "asset.created", Payload: json.RawMessage(`{"id":"asset"}`)}); e != nil {
			return nil, e
		}
		return []byte(`{"id":"asset"}`), nil
	}
	var wg sync.WaitGroup
	results := make(chan idempotency.Result, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := idempotency.Execute(ctx, sdb, key, finger, fn)
			results <- r
			errs <- e
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	fresh := 0
	for r := range results {
		if !r.Replayed {
			fresh++
		}
		if string(r.Response) != `{"id":"asset"}` {
			t.Fatal(r)
		}
	}
	if fresh != 1 || calls.Load() != 1 {
		t.Fatal(fresh, calls.Load())
	}
	if _, e = idempotency.Execute(ctx, sdb, key, idempotency.Fingerprint([]byte("different")), fn); !errors.Is(e, idempotency.ErrConflict) {
		t.Fatal(e)
	}
	failed := key
	failed.Request = "rollback"
	boom := errors.New("rollback")
	if _, e = idempotency.Execute(ctx, sdb, failed, finger, func(ctx context.Context, tx *sql.Tx) ([]byte, error) {
		_, e := tx.ExecContext(ctx, `INSERT INTO asset_fixture(id) VALUES('rollback')`)
		if e != nil {
			return nil, e
		}
		return nil, boom
	}); !errors.Is(e, boom) {
		t.Fatal(e)
	}
	var count int
	if e = sdb.QueryRowContext(ctx, `SELECT count(*) FROM asset_fixture`).Scan(&count); e != nil || count != 1 {
		t.Fatal(count, e)
	}
	if e = sdb.QueryRowContext(ctx, `SELECT count(*) FROM common_audit`).Scan(&count); e != nil || count != 1 {
		t.Fatal(count, e)
	}
	worker := outbox.Worker{DB: sdb, Publish: func(context.Context, outbox.Event) error { return boom }, RetryDelay: time.Millisecond, MaxAttempts: 2}
	r, e := worker.RunOne(ctx)
	if e != nil || r.ID != "event-1" || r.Published || r.Dead || r.PublishError == nil {
		t.Fatal(r, e)
	}
	time.Sleep(10 * time.Millisecond)
	worker.Publish = func(_ context.Context, e outbox.Event) error {
		if e.ID != "event-1" || !json.Valid(e.Payload) {
			return errors.New("wrong event")
		}
		return nil
	}
	r, e = worker.RunOne(ctx)
	if e != nil || !r.Published {
		t.Fatal(r, e)
	}
	r, e = worker.RunOne(ctx)
	if e != nil || r.ID != "" {
		t.Fatal(r, e)
	}
	// A permanently failing event is recorded as dead rather than retried forever.
	if e = database.WithinTx(ctx, sdb, nil, func(ctx context.Context, tx *sql.Tx) error {
		return outbox.Enqueue(ctx, tx, outbox.Event{ID: "event-2", Subject: "asset.created", Payload: json.RawMessage(`{}`)})
	}); e != nil {
		t.Fatal(e)
	}
	worker.MaxAttempts = 1
	worker.Publish = func(context.Context, outbox.Event) error { return boom }
	r, e = worker.RunOne(ctx)
	if e != nil || !r.Dead {
		t.Fatal(r, e)
	}
}

func scopedDSN(dsn, schema string) (string, error) {
	u, e := url.Parse(dsn)
	if e != nil {
		return "", e
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return "", errors.New("integration DSN must be a PostgreSQL URL")
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
