package nats

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/HieronimusMorgan/common-golang/requestctx"
	"github.com/nats-io/nats.go/jetstream"
)

func TestIntegration(t *testing.T) {
	if os.Getenv("COMMON_INTEGRATION") != "1" {
		t.Skip("requires NATS JetStream")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	id, e := requestctx.NewID()
	if e != nil {
		t.Fatal(e)
	}
	url := os.Getenv("COMMON_NATS_URL")
	if url == "" {
		url = "nats://127.0.0.1:4222"
	}
	c, e := Open(Config{URL: url})
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	if e = c.Ping(ctx); e != nil {
		t.Fatal(e)
	}
	js, e := c.JetStream()
	if e != nil {
		t.Fatal(e)
	}
	name := "TEST_" + id
	subject := "test." + id
	stream, e := js.CreateStream(ctx, jetstream.StreamConfig{Name: name, Subjects: []string{subject}, Storage: jetstream.MemoryStorage, Duplicates: time.Minute})
	if e != nil {
		t.Fatal(e)
	}
	defer js.DeleteStream(ctx, name)
	a, e := c.PublishDurable(ctx, subject, "event-1", map[string]string{"asset": "a"})
	if e != nil || a.Stream != name {
		t.Fatal(a, e)
	}
	a, e = c.PublishDurable(ctx, subject, "event-1", map[string]string{"asset": "a"})
	if e != nil || !a.Duplicate {
		t.Fatal(a, e)
	}
	consumer, e := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{Durable: "worker", AckPolicy: jetstream.AckExplicitPolicy, FilterSubject: subject})
	if e != nil {
		t.Fatal(e)
	}
	msg, e := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	if e != nil {
		t.Fatal(e)
	}
	if e = msg.DoubleAck(ctx); e != nil {
		t.Fatal(e)
	}
}

func TestConsumerRetryIntegration(t *testing.T) {
	if os.Getenv("COMMON_INTEGRATION") != "1" {
		t.Skip("requires NATS JetStream")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	id, e := requestctx.NewID()
	if e != nil {
		t.Fatal(e)
	}
	url := os.Getenv("COMMON_NATS_URL")
	if url == "" {
		url = "nats://127.0.0.1:4222"
	}
	c, e := Open(Config{URL: url})
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	js, e := c.JetStream()
	if e != nil {
		t.Fatal(e)
	}
	name, subject := "TEST_"+id, "test."+id
	s, e := js.CreateStream(ctx, jetstream.StreamConfig{Name: name, Subjects: []string{subject}, Storage: jetstream.MemoryStorage})
	if e != nil {
		t.Fatal(e)
	}
	defer js.DeleteStream(ctx, name)
	con, e := s.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{Durable: "retry", AckPolicy: jetstream.AckExplicitPolicy, AckWait: 10 * time.Second, MaxDeliver: 3})
	if e != nil {
		t.Fatal(e)
	}
	delivered := make(chan struct{}, 1)
	attempts := 0 // Consume invokes this handler serially.
	cc, e := Consume(ctx, con, ConsumerOptions{HandlerTimeout: time.Second, RetryDelay: 100 * time.Millisecond}, func(_ context.Context, _ jetstream.Msg) error {
		attempts++
		if attempts == 1 {
			return context.DeadlineExceeded
		}
		select {
		case delivered <- struct{}{}:
		default:
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		cc.Stop()
		select {
		case <-cc.Closed():
		case <-time.After(5 * time.Second):
			t.Error("consumer did not stop")
		}
	}()
	if _, e = c.PublishDurable(ctx, subject, "retry-event", 42); e != nil {
		t.Fatal(e)
	}
	select {
	case <-delivered:
	case <-ctx.Done():
		t.Fatal("no retry delivery")
	}
}
