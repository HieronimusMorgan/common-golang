// Package audit records business actions in the same PostgreSQL transaction as the mutation.
package audit

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/HieronimusMorgan/common-golang/requestctx"
	"github.com/HieronimusMorgan/common-golang/scope"
)

type Event struct{ ID, Tenant, Branch, Actor, RequestID, Action, ResourceType, ResourceID string }

func New(ctx context.Context, action, resourceType, resourceID string) (Event, error) {
	i, ok := scope.From(ctx)
	if !ok {
		return Event{}, errors.New("identity required")
	}
	id, err := requestctx.NewID()
	if err != nil {
		return Event{}, err
	}
	e := Event{id, i.TenantID, scope.Branch(ctx), i.Subject, requestctx.ID(ctx), action, resourceType, resourceID}
	return e, e.Validate()
}
func (e Event) Validate() error {
	if !requestctx.ValidID(e.ID) || (e.RequestID != "" && !requestctx.ValidID(e.RequestID)) {
		return errors.New("invalid audit correlation")
	}
	for _, s := range []string{e.Tenant, e.Branch, e.Actor, e.Action, e.ResourceType, e.ResourceID} {
		if strings.TrimSpace(s) == "" || len(s) > 256 || strings.ContainsRune(s, 0) {
			return errors.New("invalid audit field")
		}
	}
	return nil
}
func Append(ctx context.Context, tx *sql.Tx, e Event) error {
	if tx == nil {
		return errors.New("transaction required")
	}
	if err := e.Validate(); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO common_audit(id,tenant_id,branch_id,actor_id,request_id,action,resource_type,resource_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, e.ID, e.Tenant, e.Branch, e.Actor, e.RequestID, e.Action, e.ResourceType, e.ResourceID)
	return err
}
