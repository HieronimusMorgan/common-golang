//go:build postgres_integration

package database_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/HieronimusMorgan/common-golang/database"
	"github.com/HieronimusMorgan/common-golang/event"
	"github.com/HieronimusMorgan/common-golang/inbox"
	"github.com/HieronimusMorgan/common-golang/outbox"
	"github.com/HieronimusMorgan/common-golang/requestctx"
	"github.com/HieronimusMorgan/common-golang/scope"
	_ "github.com/jackc/pgx/v5/stdlib"
	"os"
	"testing"
	"time"
)

func TestV3Postgres(t *testing.T) {
	if os.Getenv("COMMON_POSTGRES_INTEGRATION") != "1" {
		t.Skip("requires disposable PostgreSQL")
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
	id, e := requestctx.NewID()
	if e != nil {
		t.Fatal(e)
	}
	schema := "v3_test_" + id
	if _, e = db.ExecContext(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := db.ExecContext(c, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	sd, e := scopedDSN(dsn, schema)
	if e != nil {
		t.Fatal(e)
	}
	sdb, e := database.Open(ctx, "pgx", sd, database.Config{})
	if e != nil {
		t.Fatal(e)
	}
	defer sdb.Close()
	for _, f := range []string{"001_common.sql", "002_v3.sql"} {
		raw, e := os.ReadFile("../migrations/" + f)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = sdb.ExecContext(ctx, string(raw)); e != nil {
			t.Fatal(e)
		}
	}
	ctx = scope.WithIdentity(ctx, scope.Identity{Subject: "actor", TenantID: "tenant", BranchIDs: []string{"branch"}})
	ctx, _ = scope.SelectBranch(ctx, "branch")
	e = database.WithinScopedTx(ctx, sdb, nil, func(ctx context.Context, tx *sql.Tx) error {
		var v string
		if err := tx.QueryRowContext(ctx, `SELECT current_setting('app.tenant_id')`).Scan(&v); err != nil {
			return err
		}
		if v != "tenant" {
			t.Fatal(v)
		}
		return outbox.Enqueue(ctx, tx, outbox.Event{ID: "event-1", Subject: "asset.updated", Payload: json.RawMessage(`{"x":1}`)})
	})
	if e != nil {
		t.Fatal(e)
	}
	claims, e := outbox.ClaimBatch(ctx, sdb, 1, time.Minute, 3)
	if e != nil || len(claims) != 1 {
		t.Fatal(claims, e)
	}
	next, e := outbox.ClaimBatch(ctx, sdb, 1, time.Minute, 3)
	if e != nil || len(next) != 0 {
		t.Fatal(next, e)
	}
	if e = outbox.Complete(ctx, sdb, claims[0], nil, time.Second, 3); e != nil {
		t.Fatal(e)
	}
	ev, e := event.New(ctx, "asset.updated", 1, "asset", "1", 1)
	if e != nil {
		t.Fatal(e)
	}
	n := 0
	h := func(context.Context, *sql.Tx, event.Envelope) error { n++; return nil }
	for i := 0; i < 2; i++ {
		dup, e := inbox.Process(ctx, sdb, "asset-reader", ev, h)
		if e != nil || dup != (i == 1) {
			t.Fatal(dup, e)
		}
	}
	if n != 1 {
		t.Fatal(n)
	}
}
