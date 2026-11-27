package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/HieronimusMorgan/common-golang/scope"
)

// WithinScopedTx sets transaction-local PostgreSQL scope. Install RLS policies and use a non-owner, non-BYPASSRLS role; this helper alone is not isolation.
func WithinScopedTx(ctx context.Context, db *sql.DB, opts *sql.TxOptions, fn func(context.Context, *sql.Tx) error) error {
	i, ok := scope.From(ctx)
	b := scope.Branch(ctx)
	if !ok || i.TenantID == "" || i.Subject == "" || b == "" || fn == nil {
		return errors.New("authorized tenant and branch required")
	}
	return WithinTx(ctx, db, opts, func(ctx context.Context, tx *sql.Tx) error {
		if err := BindScope(ctx, tx); err != nil {
			return err
		}
		return fn(ctx, tx)
	})
}

// RetryableTransactionError identifies definitive PostgreSQL transaction aborts, not ambiguous commit/network errors.
func RetryableTransactionError(err error) bool {
	var e interface{ SQLState() string }
	return errors.As(err, &e) && (e.SQLState() == "40001" || e.SQLState() == "40P01")
}

// BindScope binds an already-open transaction to the authenticated scope. Only call before any scoped query; do not switch identities inside the transaction.
func BindScope(ctx context.Context, tx *sql.Tx) error {
	i, ok := scope.From(ctx)
	b := scope.Branch(ctx)
	if tx == nil || !ok || i.TenantID == "" || i.Subject == "" || b == "" {
		return errors.New("authorized scope and transaction required")
	}
	_, err := tx.ExecContext(ctx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.branch_id',$2,true),set_config('app.actor_id',$3,true)`, i.TenantID, b, i.Subject)
	return err
}
