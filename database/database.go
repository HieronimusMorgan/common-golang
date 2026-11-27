// Package database provides database/sql lifecycle and transaction helpers. Drivers are registered by consumers.
package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Config struct {
	MaxOpen, MaxIdle                      int
	MaxLifetime, MaxIdleTime, PingTimeout time.Duration
}

func Open(ctx context.Context, driver, dsn string, c Config) (*sql.DB, error) {
	if driver == "" || c.MaxOpen < 0 || c.MaxIdle < 0 || (c.MaxOpen > 0 && c.MaxIdle > c.MaxOpen) || c.MaxLifetime < 0 || c.MaxIdleTime < 0 || c.PingTimeout < 0 {
		return nil, errors.New("invalid database config")
	}
	if c.MaxOpen == 0 {
		c.MaxOpen = 20
	}
	if c.MaxIdle == 0 {
		c.MaxIdle = min(5, c.MaxOpen)
	}
	if c.MaxIdle > c.MaxOpen {
		return nil, errors.New("invalid pool size")
	}
	if c.PingTimeout == 0 {
		c.PingTimeout = 3 * time.Second
	}
	if c.MaxLifetime == 0 {
		c.MaxLifetime = 30 * time.Minute
	}
	if c.MaxIdleTime == 0 {
		c.MaxIdleTime = 5 * time.Minute
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(c.MaxOpen)
	db.SetMaxIdleConns(c.MaxIdle)
	db.SetConnMaxLifetime(c.MaxLifetime)
	db.SetConnMaxIdleTime(c.MaxIdleTime)
	probe, cancel := context.WithTimeout(ctx, c.PingTimeout)
	defer cancel()
	if err = db.PingContext(probe); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// WithinTx commits only after fn succeeds. fn must use tx exclusively and must not commit or roll it back. Panic propagates after rollback.
func WithinTx(ctx context.Context, db *sql.DB, opts *sql.TxOptions, fn func(context.Context, *sql.Tx) error) (err error) {
	if db == nil || fn == nil {
		return errors.New("database and callback required")
	}
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	defer func() {
		e := tx.Rollback()
		if e != nil && !errors.Is(e, sql.ErrTxDone) {
			err = errors.Join(err, e)
		}
	}()
	if err = fn(ctx, tx); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return tx.Commit()
}
