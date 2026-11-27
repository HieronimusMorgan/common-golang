package nats

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/HieronimusMorgan/common-golang/logger"
	"github.com/HieronimusMorgan/common-golang/requestctx"
	"github.com/nats-io/nats.go/jetstream"
)

// Handler must commit its durable effects before returning nil and must be idempotent.
// Do not call Ack/Nak/Term from Handler; the wrapper owns acknowledgement.
type Handler func(context.Context, jetstream.Msg) error

type ConsumerOptions struct {
	HandlerTimeout time.Duration
	RetryDelay     time.Duration
	Logger         *slog.Logger
}

// Consume attaches to an existing explicit-ack consumer. The caller owns its
// lifecycle (Drain/Stop and wait on Closed). Configure AckWait > HandlerTimeout
// + acknowledgement time; MaxDeliver and dead-letter handling are service policies.
func Consume(ctx context.Context, consumer jetstream.Consumer, opts ConsumerOptions, handler Handler) (jetstream.ConsumeContext, error) {
	if consumer == nil || handler == nil || opts.HandlerTimeout < 0 || opts.RetryDelay < 0 {
		return nil, errors.New("invalid consumer configuration")
	}
	if opts.HandlerTimeout == 0 {
		opts.HandlerTimeout = 15 * time.Second
	}
	if opts.RetryDelay == 0 {
		opts.RetryDelay = 5 * time.Second
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	probe, cancelProbe := context.WithTimeout(ctx, 3*time.Second)
	defer cancelProbe()
	info, err := consumer.Info(probe)
	if err != nil {
		return nil, err
	}
	if info.Config.AckPolicy != jetstream.AckExplicitPolicy {
		return nil, errors.New("explicit acknowledgement policy required")
	}
	return consumer.Consume(func(msg jetstream.Msg) {
		msgCtx := requestctx.WithID(ctx, msg.Headers().Get("X-Request-ID"))
		l := logger.WithContext(msgCtx, opts.Logger)
		run, cancel := context.WithTimeout(msgCtx, opts.HandlerTimeout)
		err := run.Err()
		if err == nil {
			err = callHandler(run, handler, msg)
		}
		if err == nil {
			err = run.Err()
		}
		cancel()
		if err != nil {
			l.Warn("JetStream handler failed; scheduling retry")
			if e := msg.NakWithDelay(opts.RetryDelay); e != nil {
				l.Warn("JetStream negative acknowledgement failed")
			}
			return
		}
		ack, cancelAck := context.WithTimeout(context.WithoutCancel(msgCtx), 3*time.Second)
		defer cancelAck()
		if e := msg.DoubleAck(ack); e != nil {
			l.Warn("JetStream acknowledgement not confirmed; message may be redelivered")
		}
	}, jetstream.ConsumeErrHandler(func(_ jetstream.ConsumeContext, _ error) {
		logger.WithContext(ctx, opts.Logger).Warn("JetStream consumer error")
	}))
}

func callHandler(ctx context.Context, h Handler, m jetstream.Msg) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("handler panic")
		}
	}()
	return h(ctx, m)
}
