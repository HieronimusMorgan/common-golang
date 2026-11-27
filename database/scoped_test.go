package database

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
)

type stateError string

func (s stateError) Error() string    { return string(s) }
func (s stateError) SQLState() string { return string(s) }
func TestScopedFailClosed(t *testing.T) {
	if err := WithinScopedTx(context.Background(), nil, nil, func(context.Context, *sql.Tx) error { t.Fatal("callback invoked"); return nil }); err == nil {
		t.Fatal("missing scope accepted")
	}
	if !RetryableTransactionError(fmt.Errorf("wrapped: %w", stateError("40001"))) || RetryableTransactionError(stateError("08006")) {
		t.Fatal("retry classification")
	}
}
