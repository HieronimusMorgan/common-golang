# Delivery verification

## Completed in the delivery environment

- Inspected the complete original archive (three Go source files, README, go.mod and go.sum).
- Preserved the original module name, Go minimum version, exported functions and response contracts. Existing Go files have no semantic edits (line endings/formatting only).
- All Go files formatted successfully with gofmt.
- Ran `go test -race -count=1 ./...` and `go vet ./...` on an isolated copy of the nine standard-library-only packages: apperror, config, health, httpclient, lifecycle, logger, requestctx, scope and pagination's new bounded parser (excluding its existing Gin-dependent parser). All passed, exit status 0. The isolated verification module used the installed Go 1.23.2; the shipped module retains Go 1.23.4.

## Not verified in this environment

The installed Go toolchain is 1.23.2, below the module's original 1.23.4 requirement. `GOTOOLCHAIN=local go test ./...` fails on that version mismatch before compilation. `proxy.golang.org` cannot be resolved from the sandbox, and no dependency cache or Docker/Redis/NATS servers were available. The original go.sum has been retained, but new checksums and indirect requirements were not resolved. Full-project compilation, new Gin/JWT/Redis/NATS tests, Compose startup, and vulnerability scanning have NOT been verified. This is an enhanced source delivery, not a certified production release.

## Required release gate on your machine

Use a supported Go toolchain compatible with the module, with dependency network access. Then run:

```bash
go mod tidy
go mod verify
go vet ./...
go test -race -count=1 ./...
docker compose up -d --wait
COMMON_INTEGRATION=1 go test -race -count=1 ./redis ./nats
docker compose down -v
```

Review dependency versions (pins target the original Go compatibility baseline, not necessarily current security releases), run vulnerability scanning and resolve any findings, review the regenerated go.mod/go.sum, validate the JWT claim contract against the real authentication service, then tag and publish. The GitHub Actions workflow runs the compilation/tests and Compose checks but has not itself been executed here. The actual Asset Management services were not supplied, so no consumer migration or end-to-end integration was performed.

## V2 verification (supersedes previous package-count results)

The V2 implementation retains the original module Go minimum and adds a build-tagged PostgreSQL integration fixture using pgx v5.7.2. The sandbox still cannot resolve proxy.golang.org and has Go 1.23.2, no Docker and no module cache. The actual delivered module cannot compile here; do not infer that the dependencies, full build, SQL syntax/driver behavior, Redis Lua, JWT or NATS have passed.

An isolated module copy (same module path, Go 1.23.2) contains 16 standard-library-only package suites, omitting only the Gin-dependent legacy pagination file and the build-tagged PostgreSQL test. `go test -race -count=1 ./...` and `go vet ./...` passed in that copy (see v2-offline-test.log). SQL transaction tests use a stub driver, and outbox/audit/idempotency tests verify validation/scope only offline. PostgreSQL concurrency, replay, rollback, retry/dead state are covered by the authored, unexecuted integration fixture. Redis and JWT tests remain unexecuted. CI adds PostgreSQL and an advisory scan using a supported Go toolchain; workflow not executed here.

Run in a network-enabled environment with a patched Go toolchain:

```bash
go mod tidy
go mod verify
go vet ./...
go test -race -count=1 ./...
docker compose up -d --wait
COMMON_INTEGRATION=1 go test -race -count=1 ./redis ./nats
COMMON_POSTGRES_INTEGRATION=1 go test -tags postgres_integration -race -count=1 ./database
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
docker compose down -v
```

Commit reviewed go.mod/go.sum after tidy and update any vulnerable dependencies. `@latest` scanner invocation is intentionally dynamic; pin a reviewed version in release CI for reproducibility. Test database credentials are local-only. PostgreSQL fixture creates and drops a unique schema; use a disposable test database only. No real Asset Management service was supplied or modified.

## V3 update

The V3 candidate's independent offline results and remaining release gaps are recorded in [AUDIT_V3.md](AUDIT_V3.md), [v3-offline-test.log](v3-offline-test.log), [v3-full-check.log](v3-full-check.log) and [RELEASE_V3.md](RELEASE_V3.md). Earlier sections refer to prior iterations, not the latest full-project test state.
