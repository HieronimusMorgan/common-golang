package nats

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/HieronimusMorgan/common-golang/event"
	"github.com/HieronimusMorgan/common-golang/inbox"
	"github.com/HieronimusMorgan/common-golang/validation"
	"github.com/nats-io/nats.go/jetstream"
)

// InboxHandler adapts a transactional consumer to Consume. Authorize the event source/tenant before processing; use a trusted broker with subject ACLs.
func InboxHandler(db *sql.DB, consumer string, authorize func(context.Context, event.Envelope) error, h inbox.Handler) Handler {
	return func(ctx context.Context, m jetstream.Msg) error {
		if authorize == nil || h == nil {
			return errors.New("event authorization and handler required")
		}
		var e event.Envelope
		if err := validation.DecodeJSON(bytes.NewReader(m.Data()), (1<<20)+16384, &e); err != nil {
			return err
		}
		if err := e.Validate(); err != nil {
			return err
		}
		if err := authorize(ctx, e); err != nil {
			return err
		}
		_, err := inbox.Process(ctx, db, consumer, e, h)
		return err
	}
}

// EnvelopePayload validates before handing raw bytes to a transport/outbox.
func EnvelopePayload(e event.Envelope) (json.RawMessage, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(e)
}
