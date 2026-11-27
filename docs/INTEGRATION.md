# Integration cookbook

The following snippets assume imports of the named packages and error handling in a composition root that returns errors. Use aliases `commonredis`, `commonnats`, `commonjwt` when importing the corresponding common packages alongside vendor packages. For full HTTP setup see `examples/api`.

## Configuration and logging

```go
addr, err := config.Required("REDIS_ADDR")
if err != nil { return err }
timeout, err := config.Duration("REDIS_TIMEOUT", 3*time.Second)
if err != nil { return err }
l, err := logger.New(logger.Config{Service: "asset-service", Level: "INFO", JSON: true})
if err != nil { return err }
```

The library reads the process environment. Load `.env` externally (shell, Compose, Kubernetes secrets), not implicitly in package init. The logger uses `logger.WithContext(ctx, l).Info("asset read", "asset_id", id)` for request correlation. Do not log confidential data or log user strings as arbitrary attribute keys.

## Redis cache and lease lock

```go
cache, err := commonredis.Open(ctx, commonredis.Config{
    Addr: addr, Prefix: "asset-service:v1", Timeout: timeout,
})
if err != nil { return err }
defer cache.Close()
```

For a scoped cache key, use a collision-safe encoding for tenant, branch and asset identifiers, e.g. a JSON-encoded tuple encoded with base64.RawURLEncoding. `SetJSON(ctx, key, value, 5*time.Minute)` requires a positive TTL; `GetJSON(ctx, key, &dst)` returns `redis.ErrMiss` for missing data. Invalidate after committed writes. Use `Raw()` for vendor-specific operations when needed (raw keys bypass library namespaces).

```go
lock, acquired, err := cache.TryLock(ctx, key, 30*time.Second)
if err != nil { return err }
if !acquired { return apperror.New(apperror.Conflict, "Asset is busy") }
defer func() {
    releaseCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()
    if err := lock.Release(releaseCtx); err != nil { l.Warn("lease release failed") }
}()
```

`Renew` and `Release` fail with `ErrLockLost` if the caller no longer owns the key. Critical data changes still require database integrity guarantees.

## JWT

In the authentication service, issue with `commonjwt.NewRS256(cfg, &private.PublicKey, private)` and `Issue(scope.Identity{...})`. Store the private key securely; resource services use the matching public key only:

```go
verifier, err := commonjwt.NewRS256(commonjwt.Config{
    Issuer: "authentication-service", Audience: "asset-service",
    TTL: 15*time.Minute, Leeway: 5*time.Second,
}, publicKey, nil)
if err != nil { return err }
```

`TTL` is used for issuance, not a maximum lifetime enforced against external tokens. Verify issuer/audience with your real auth contract. `middleware.Authenticate(verifier)` populates scope only after signature and claim verification. Attach `RequireBranch()` and `RequirePermission("asset.read")` where required. Access tokens only; refresh and revocation remain authentication-service concerns. `TTL` must be >= 1s even for verifier-only configuration.

## NATS and JetStream

```go
broker, err := commonnats.Open(commonnats.Config{
    URL: "nats://127.0.0.1:4222", Name: "asset-service", Logger: l,
})
if err != nil { return err }
defer broker.Close()
js, err := broker.JetStream()
if err != nil { return err }
```

Provision streams and consumers through deployment configuration. This is an illustrative local provisioning example (do not blindly overwrite production streams):

```go
stream, err := js.CreateStream(ctx, jetstream.StreamConfig{
    Name: "ASSET_EVENTS", Subjects: []string{"asset.events.>"},
    Storage: jetstream.FileStorage, Duplicates: 2*time.Minute,
    MaxAge: 7*24*time.Hour, MaxBytes: 1<<30,
})
if err != nil { return err }
consumer, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
    Durable: "asset-worker", AckPolicy: jetstream.AckExplicitPolicy,
    FilterSubject: "asset.events.>", AckWait: 45*time.Second, MaxDeliver: 5,
})
if err != nil { return err }
```

Publish only after obtaining a persistent event ID (normally via an outbox):

```go
ack, err := broker.PublishDurable(ctx, "asset.events.updated", eventID, eventPayload)
if err != nil { return err }
l.Info("event persisted", "stream", ack.Stream, "sequence", ack.Sequence)
```

Consume with your idempotent handler (`process` is a service-owned function accepting context and payload):

```go
cc, err := commonnats.Consume(ctx, consumer, commonnats.ConsumerOptions{
    HandlerTimeout: 20*time.Second, RetryDelay: 5*time.Second, Logger: l,
}, func(ctx context.Context, msg jetstream.Msg) error {
    return process(ctx, msg.Data()) // commit business changes and inbox atomically
})
if err != nil { return err }
// At shutdown, call cc.Drain() then wait on cc.Closed() with an outer deadline.
// On deadline call cc.Stop(); then close the NATS connection.
```

The consumer wrapper checks explicit ack policy, calls DoubleAck only on handler success, and schedules retry on failure. Cancellation is cooperative; the caller owns consume lifecycle. Durable publishing deduplicates only within configured server limits. Use a service-owned DLQ workflow/advisory monitoring, not an assumed automatic transfer. NATS headers including request IDs are not security claims. Validate event tenant/branch and publisher permissions at consumer boundaries.

## Service shutdown and health

Initialize clients at startup; fail fast for mandatory dependencies. Register `health.Liveness` separately from `health.Readiness`. Example:

```go
ready := health.Readiness(2*time.Second,
    health.Dependency{Name: "redis", Check: cache.Ping},
    health.Dependency{Name: "nats", Check: broker.Ping},
)
r.GET("/readyz", gin.WrapF(ready))
```

Use a signal-aware root context for `lifecycle.Serve`; stop HTTP traffic and worker consumption before closing Redis/NATS and other dependencies. Health readiness check functions must respect context deadlines. Keep separate worker shutdown deadlines and do not close dependencies while handlers still need them.

## V2 extensions

See [V2 integration guide](V2_GUIDE.md) for PostgreSQL transaction helpers, outbox, idempotency, audit, JWT rotation, rate limiting, validation, resilience and metrics.
