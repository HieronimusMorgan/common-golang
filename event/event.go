// Package event defines versioned, transport-neutral event envelopes.
package event

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/HieronimusMorgan/common-golang/requestctx"
	"github.com/HieronimusMorgan/common-golang/scope"
	"strings"
	"time"
)

type Envelope struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Version       int             `json:"version"`
	Source        string          `json:"source"`
	TenantID      string          `json:"tenant_id"`
	BranchID      string          `json:"branch_id"`
	AggregateID   string          `json:"aggregate_id"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	CausationID   string          `json:"causation_id,omitempty"`
	OccurredAt    time.Time       `json:"occurred_at"`
	Payload       json.RawMessage `json:"payload"`
}

func (e Envelope) Validate() error {
	if !requestctx.ValidID(e.ID) || e.Version < 1 || e.OccurredAt.IsZero() || len(e.Payload) > 1<<20 || !json.Valid(e.Payload) {
		return errors.New("invalid event envelope")
	}
	for _, v := range []string{e.Type, e.Source, e.TenantID, e.BranchID, e.AggregateID} {
		if strings.TrimSpace(v) == "" || len(v) > 256 || strings.ContainsRune(v, 0) {
			return errors.New("invalid event field")
		}
	}
	for _, v := range []string{e.CorrelationID, e.CausationID} {
		if v != "" && !requestctx.ValidID(v) {
			return errors.New("invalid event correlation")
		}
	}
	return nil
}
func New(ctx context.Context, typ string, version int, source, aggregate string, payload any) (Envelope, error) {
	i, ok := scope.From(ctx)
	if !ok {
		return Envelope{}, errors.New("identity required")
	}
	id, err := requestctx.NewID()
	if err != nil {
		return Envelope{}, err
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	e := Envelope{ID: id, Type: typ, Version: version, Source: source, TenantID: i.TenantID, BranchID: scope.Branch(ctx), AggregateID: aggregate, CorrelationID: requestctx.ID(ctx), OccurredAt: time.Now().UTC(), Payload: b}
	return e, e.Validate()
}
