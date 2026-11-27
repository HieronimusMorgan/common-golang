// Package idempotency serializes duplicate commands in a PostgreSQL transaction.
package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"strings"

	"github.com/HieronimusMorgan/common-golang/database"
	"github.com/HieronimusMorgan/common-golang/scope"
)

var ErrConflict = errors.New("idempotency key reused for different request")

type Key struct{ Tenant, Branch, Actor, Operation, Request string }

func FromContext(ctx context.Context, operation, key string) (Key, error) {
	i, ok := scope.From(ctx)
	k := Key{i.TenantID, scope.Branch(ctx), i.Subject, operation, key}
	if !ok {
		return Key{}, errors.New("identity required")
	}
	return k, k.Validate()
}
func (k Key) Validate() error {
	for _, v := range []string{k.Tenant, k.Branch, k.Actor, k.Operation, k.Request} {
		if strings.TrimSpace(v) == "" || len(v) > 256 || strings.ContainsRune(v, 0) {
			return errors.New("invalid idempotency scope or key")
		}
	}
	return nil
}
func Fingerprint(request []byte) [32]byte { return sha256.Sum256(request) }

type Result struct {
	Response []byte
	Replayed bool
}

// Execute atomically reserves key, runs fn with tx, stores response and commits. fn must do all effects in tx (including outbox), not call external systems. Fingerprint must cover the entire semantic command. Authorize every request before calling.
func Execute(ctx context.Context, db *sql.DB, k Key, fingerprint [32]byte, fn func(context.Context, *sql.Tx) ([]byte, error)) (result Result, err error) {
	return execute(ctx, db, k, fingerprint, fn, false)
}

// ExecuteScoped derives the dedupe scope and binds the business transaction before any SQL. Install RLS separately.
func ExecuteScoped(ctx context.Context, db *sql.DB, operation, key string, fingerprint [32]byte, fn func(context.Context, *sql.Tx) ([]byte, error)) (Result, error) {
	k, err := FromContext(ctx, operation, key)
	if err != nil {
		return Result{}, err
	}
	return execute(ctx, db, k, fingerprint, fn, true)
}
func execute(ctx context.Context, db *sql.DB, k Key, fingerprint [32]byte, fn func(context.Context, *sql.Tx) ([]byte, error), scoped bool) (result Result, err error) {
	if err = k.Validate(); err != nil {
		return result, err
	}
	if fn == nil {
		return result, errors.New("callback required")
	}
	args := []any{k.Tenant, k.Branch, k.Actor, k.Operation, k.Request}
	err = database.WithinTx(ctx, db, &sql.TxOptions{Isolation: sql.LevelReadCommitted}, func(ctx context.Context, tx *sql.Tx) error {
		if scoped {
			if e := database.BindScope(ctx, tx); e != nil {
				return e
			}
		}
		r, e := tx.ExecContext(ctx, `INSERT INTO common_idempotency(tenant_id,branch_id,actor_id,operation,request_key,fingerprint) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, append(args, fingerprint[:])...)
		if e != nil {
			return e
		}
		n, e := r.RowsAffected()
		if e != nil {
			return e
		}
		if n == 0 {
			var stored []byte
			var completed bool
			e = tx.QueryRowContext(ctx, `SELECT fingerprint,response,response IS NOT NULL FROM common_idempotency WHERE tenant_id=$1 AND branch_id=$2 AND actor_id=$3 AND operation=$4 AND request_key=$5 FOR UPDATE`, args...).Scan(&stored, &result.Response, &completed)
			if e != nil {
				return e
			}
			if !bytes.Equal(stored, fingerprint[:]) {
				return ErrConflict
			}
			if !completed {
				return errors.New("incomplete idempotency record")
			}
			result.Replayed = true
			return nil
		}
		result.Response, e = fn(ctx, tx)
		if e != nil {
			return e
		}
		if len(result.Response) > 1<<20 {
			return errors.New("idempotency response too large")
		}
		if result.Response == nil {
			result.Response = []byte{}
		}
		_, e = tx.ExecContext(ctx, `UPDATE common_idempotency SET response=$6 WHERE tenant_id=$1 AND branch_id=$2 AND actor_id=$3 AND operation=$4 AND request_key=$5`, append(args, result.Response)...)
		return e
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}
