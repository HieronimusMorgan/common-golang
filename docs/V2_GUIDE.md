# V2 integration guide

## Atomic command processing

Use PostgreSQL 13+ (test fixture targets 16). Apply `migrations/001_common.sql` through the service's migration tool; the library does not apply schema automatically. In the consuming service register a database/sql driver (e.g. blank import `github.com/jackc/pgx/v5/stdlib`), then call `database.Open(ctx, "pgx", dsn, database.Config{})`. Configure statement/lock timeouts, pool size and TLS. Do not log the DSN. The test-only pgx import is build-tagged.

For mutating endpoints, authenticate and check current permissions before idempotency lookup. Use `idempotency.FromContext` with the operation and client key; fingerprint the whole command including path arguments and relevant query parameters. Use a canonical application representation where semantically equivalent inputs should match. Scope all queries by authenticated tenant/branch; the helpers do not enforce SQL row-level policy. Cross-branch transfers require separate authorization on both branches.

```go
key, err := idempotency.FromContext(ctx, "asset.transfer", clientKey)
if err != nil { return err }
result, err := idempotency.Execute(ctx, db, key, idempotency.Fingerprint(commandBytes),
    func(ctx context.Context, tx *sql.Tx) ([]byte, error) {
        // updateAsset must use tx, enforce tenant/source/destination authorization and row constraints.
        if err := updateAsset(ctx, tx, cmd); err != nil { return nil, err }
        ev, err := audit.New(ctx, "transfer", "asset", cmd.AssetID)
        if err != nil { return nil, err }
        if err = audit.Append(ctx, tx, ev); err != nil { return nil, err }
        eventID, err := requestctx.NewID()
        if err != nil { return nil, err }
        if err = outbox.Enqueue(ctx, tx, outbox.Event{ID: eventID, Subject: "asset.transferred", Payload: commandBytes}); err != nil { return nil, err }
        return []byte(`{"status":"transferred"}`), nil
    })
```

Illustrative snippet: service-owned identifiers/types are placeholders. Do not publish directly, send email, or perform other non-transactional effects inside the callback. Retry ambiguous commits with the same key. Map `ErrConflict` to HTTP 409. Response is opaque bytes (up to 1 MiB); if status/headers are needed, include them in a service-controlled response envelope. Keep records for at least the agreed retry horizon; deleting a record permits replay. Do not store secrets in response bytes. The SQL helpers use caller-owned DB pools; use a deadline for every request.

## Outbox worker

```go
worker := outbox.Worker{DB: db, Publish: func(ctx context.Context, e outbox.Event) error {
    _, err := broker.PublishDurable(ctx, e.Subject, e.ID, e.Payload)
    return err
}, MaxAttempts: 10}
result, err := worker.RunOne(ctx)
```

Run in a service-owned polling loop with cancellation and idle delay; use stable event IDs and an explicit stream/consumer configuration. `RunOne` holds one DB transaction during publishing, uses `FOR UPDATE SKIP LOCKED` and records retries/dead state. Bound worker concurrency to preserve DB connections; publisher must respect deadlines. A successful result means DB commit succeeded; an error may leave the outcome unknown. At-least-once, not exactly-once: a crash after broker ack but before SQL commit can cause duplicate delivery. Use consumer-side persistent dedupe in the same transaction as effects. Monitor backlog age and dead rows, define redrive/retention policies; no automatic DLQ or cleanup is provided. Do not assume global event ordering.

## JWT rotation

Create a `jwt.KeyRing` with `NewKeyRing(activeKid, map[string]*jwt.Manager{...})` and inject it into `middleware.Authenticate`. Resource services should receive RS256 verification-only managers. Add the new public key to all verifiers before switching active signing key; use a fresh unique `kid`; retain old keys for the maximum token validity and clock skew (and in-flight issuer requests), then remove. `Rotate` swaps the full snapshot atomically and rejects changes in issuer/audience/algorithm/TTL/leeway policy. Missing/unknown `kid` fails closed. Existing Manager API remains unchanged. Never use unverified JWT claims for decisions. Rotation orchestration, secret store/JWKS and revocation are outside this release.

## Rate limiting

Call `cache.AllowRate(ctx, "asset_write", subject, 100, time.Minute)` after authentication; construct subject from unambiguous tenant/branch/actor components (e.g. JSON array encoded to string). For pre-auth routes choose a trusted ingress identifier, never an untrusted forwarded header. `Allowed=false` maps to 429 and `ResetAfter` can determine Retry-After (round up). Redis errors are distinct from denial; explicitly choose fail-closed for sensitive operations or fail-open only for low-risk ones. Fixed windows allow boundary bursts. Limit/window changes should use a new versioned policy name.

## Validation, metrics and resilience

- `validation.DecodeJSON(body, 1<<20, &dto)` accepts an object with strict JSON field checks and duplicate key detection. Use ordinary typed DTOs, avoid custom nested unmarshalling and free-form maps where strictness is required. Validate domain constraints separately. Translate decode errors into approved public errors, not raw parser messages.
- `observability.New("asset_transfer", "outbox_publish")` defines bounded operation names; `finish := registry.Start("asset_transfer")`, then `finish(err)` exactly once (normally deferred); the callback is idempotent. Expose registry on an internal metrics endpoint, never a public endpoint by default. This is basic Prometheus text, not OpenTelemetry or latency histograms.
- `resilience.Retry` accepts a retry classifier and only safe-to-repeat operations; no implicit network retries. `resilience.NewBreaker` isolates a dependency, permits one half-open probe, and should be reused per dependency. Classification must exclude expected business errors and caller cancellation unless intentionally counted.
- `health.Readiness` runs checks concurrently with a bounded response deadline; overlapping or stuck probes report unavailable and cannot spawn unbounded check goroutines. A permanently stuck check still retains one goroutine; dependency implementation must honor context. Register one readiness handler per service, not per request.

## Audit and data ownership

`audit.Append` records action, actor, tenant, branch, resource and request correlation in the business transaction. `audit.New` extracts trusted request scope. Callers creating events manually must supply trusted values. Audit rows are minimal metadata, not immutable/tamper-evident and not a complete compliance system. Use database privileges, retention, redaction and a service-level schema if before/after values or reasons are required.
