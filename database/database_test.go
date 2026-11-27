package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync/atomic"
	"testing"
)

type stub struct{ commits, rollbacks atomic.Int32 }
type conn struct{ s *stub }
type tx struct{ s *stub }

func (s *stub) Open(string) (driver.Conn, error)   { return conn{s}, nil }
func (c conn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unsupported") }
func (c conn) Close() error                        { return nil }
func (c conn) Begin() (driver.Tx, error)           { return tx{c.s}, nil }
func (c conn) Ping(context.Context) error          { return nil }
func (t tx) Commit() error                         { t.s.commits.Add(1); return nil }
func (t tx) Rollback() error                       { t.s.rollbacks.Add(1); return nil }

var d = &stub{}

func init() { sql.Register("common_test_stub", d) }
func TestTransaction(t *testing.T) {
	db, e := Open(context.Background(), "common_test_stub", "", Config{})
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = WithinTx(context.Background(), db, nil, func(context.Context, *sql.Tx) error { return nil }); e != nil {
		t.Fatal(e)
	}
	want := errors.New("fail")
	if e = WithinTx(context.Background(), db, nil, func(context.Context, *sql.Tx) error { return want }); !errors.Is(e, want) {
		t.Fatal(e)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic swallowed")
			}
		}()
		_ = WithinTx(context.Background(), db, nil, func(context.Context, *sql.Tx) error { panic("test") })
	}()
	if d.commits.Load() != 1 || d.rollbacks.Load() != 2 {
		t.Fatal(d.commits.Load(), d.rollbacks.Load())
	}
}
