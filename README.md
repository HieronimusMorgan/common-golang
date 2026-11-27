# Common Golang — V3 candidate

Shared infrastructure for Asset Management and other Go services. No business entities, ORM, service globals, or implicit network initialization. Includes opt-in infrastructure-only PostgreSQL migrations. Existing module path and response/pagination contracts are retained.

## V3 additions

See [V3 integration guide](docs/V3_GUIDE.md), [release checklist](docs/RELEASE_V3.md), and [second audit](docs/AUDIT_V3.md). Apply migration 002 explicitly for inbox, leased outbox and audit changes. New packages include `event`, `inbox`, `auth`, `rbac`, `cache`, `worker`, `scheduler`, `storage`, `retention`, `featureflag`, and `observability/otel`. Existing packages remain available. This archive is not a verified public release.

## Requirements and setup

- Go 1.23.4 or newer (original module minimum; use a supported, patched Go toolchain in production).
- Docker Compose is optional for local PostgreSQL, Redis and NATS integration tests.
- First run `go mod tidy` and `go mod verify` from this directory. The supplied go.sum retains its original entries; newly added dependency checksums and transitive module requirements must be resolved with network access. Commit the resulting go.mod and go.sum before tagging the library. The delivery environment cannot download modules; see [verification](docs/VERIFICATION.md).

For a local service using this module, add both modules to a Go workspace, or temporarily use a local replace directive in the consumer:

```go
replace github.com/HieronimusMorgan/common-golang => ../common-golang
```

Then import the individual packages you need. Do not publish the local replace directive as part of a released consumer application.

## Packages

| Package | Responsibility |
|---|---|
| `response` | Legacy Gin envelopes, safe public error responses |
| `pagination` | Legacy query parsing and new bounded offset pagination |
| `apperror` | Typed errors, HTTP mapping, preserved internal causes |
| `config` | Typed environment reads, explicit invalid-value errors |
| `logger` | Standard-library structured slog, request correlation, common-key redaction |
| `requestctx` | Validated correlation IDs and context propagation |
| `scope` | Verified identity, tenant and branch selection, exact permission checks |
| `jwt` | HS256 and RS256 access tokens, strict issuer/audience/expiry/algorithm checks, key ring |
| `middleware` | Gin request ID, access logs, recovery, bearer authentication, branch and permission enforcement, size limits |
| `redis` | Namespaced TTL-based JSON cache, token-owned lease locks, raw client escape hatch, atomic fixed-window rate limiting |
| `nats` | Core NATS, request/reply, JetStream acknowledged publication, explicit-ack consumption with delayed retry |
| `health` | Isolated concurrent HTTP readiness, liveness |
| `database` | SQL connection pool, transaction callback |
| `outbox` | PostgreSQL atomic enqueue and bounded at-least-once worker |
| `idempotency` | Transactional scoped duplicate-command prevention |
| `audit` | Transactional business audit records |
| `resilience` | Jittered retries and circuit breaker |
| `observability` | Static-operation Prometheus-compatible metrics |
| `validation` | Strict JSON and field violations |
| `httpclient` | Bounded JSON HTTP calls and correlation propagation; no automatic retries or redirects |
| `lifecycle` | HTTP server timeouts, graceful shutdown |

## Quick start

```go
l, err := logger.New(logger.Config{Service: "asset-service", Environment: "prod", Level: "INFO", JSON: true})
if err != nil { return err }
r := gin.New()
r.Use(middleware.RequestID(false), middleware.AccessLog(l), middleware.Recovery(l), middleware.MaxBodyBytes(1<<20))
r.GET("/livez", gin.WrapF(health.Liveness))
r.GET("/readyz", gin.WrapF(health.Readiness(2*time.Second,
    health.Dependency{Name: "redis", Check: cache.Ping},
    health.Dependency{Name: "nats", Check: broker.Ping},
)))
r.GET("/assets", middleware.Authenticate(verifier), middleware.RequireBranch(), middleware.RequirePermission("asset.read"), listAssets)
```

`cache`, `broker`, `verifier`, and `listAssets` are initialized by the service. See `examples/api/main.go` for a runnable example with RS256 public-key verification, and [integration guide](docs/INTEGRATION.md) for concrete client initialization and worker examples. The example is intentionally not a complete Asset Management application.

## Development and tests

```bash
go mod tidy
go mod verify
go vet ./...
go test -race -count=1 ./...
docker compose up -d --wait
COMMON_INTEGRATION=1 go test -race -count=1 ./redis ./nats
COMMON_POSTGRES_INTEGRATION=1 go test -tags postgres_integration -race -count=1 ./database
docker compose down -v
```

PowerShell integration test equivalent: `$env:COMMON_INTEGRATION = '1'; go test -race -count=1 ./redis ./nats`.

See [architecture and boundaries](docs/ARCHITECTURE.md), [migration notes](docs/MIGRATION.md), and [verification](docs/VERIFICATION.md).

## V2 review

See [V2 guide](docs/V2_GUIDE.md), [engineering audit and residual gaps](docs/AUDIT_V2.md), and [verification](docs/VERIFICATION.md). PostgreSQL migrations are explicit opt-in files; they are not executed at library startup.
