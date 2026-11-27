# V3 implementation audit and residual risks

Basis: `common-golang-v2.zip` (67 files). This iteration extends source, tests, docs and CI; no GitHub or deployed service was modified. This is a candidate, not a stable release. Source-level implementation is not evidence of external adapter compilation or integration.

## Original 10 audit areas

| Area | Implemented in V3 | Not completed / verification required |
|---|---|---|
| Release | CI matrix, readonly build and checksums gate, coverage, pinned vulnerability scanner, Dependabot, tag-driven pre-release, templates, module inventory | Resolve module graph and checksums, run CI, choose license, owner settings and CODEOWNERS, full secret content scanning, formal SBOM, automated API diff and conventional commit enforcement |
| Tenant isolation | Scoped transactions, existing-tx binding, scoped idempotency, hashed scope keys, retryable SQLSTATE classification, RLS example | Install actual service RLS and roles, run cross-tenant regression tests; no universal query rewriting, cross-branch business policy or automatic retry |
| Events | Envelope, validation, transactional inbox, NATS consumer adapter, dead outbox redrive | Live DB/NATS checks, schema registry/version dispatch, broker DLQ/replay, strict aggregate ordering |
| Authentication | JWKS fixed HTTPS endpoint, cached verification, context-aware middleware, Redis revocation, opaque refresh tokens, resource policy/role expansion | Full session rotation and reuse detection, KMS, service identities, external identity integration, cache failure backoff |
| Observability | Prometheus histogram, OTel SDK + OTLP HTTP adapter, HTTP spans, trace propagation and log helper | Dependency compilation and collector tests, auto SQL/Redis/NATS instrumentation, server status capture and runtime metrics |
| Cache/coordination | Typed cache-aside, local coalescing, TTL jitter, token bucket, Redis fencing | Live Redis tests, cross-replica stampede, durable fencing, cluster/Sentinel, sliding window and batch APIs |
| Workers | Bounded worker pool, interval scheduler with lease interface, leased outbox claims/completion/recovery/redrive | Durable job source, concrete distributed scheduler, calendar cron, per-aggregate ordering and live crash tests |
| Storage | Scoped provider, size/MIME/checksum checks, MinIO/S3 adapter with SDK multipart support and presigned GET | SDK compile/live integration, virus scanning, quarantine, quota/concurrency enforcement, immutable object versioning |
| Audit/lifecycle | Allowlisted before/after, changed fields, HMAC, key ID, bounded retention, role SQL example | Actual append-only privileges, MAC key lifecycle, deletion/timestamp evidence, export, archive and legal retention |
| Configuration | Tagged struct env loading, bounded JSON overlay, atomic snapshots, scoped feature rollout | YAML, secret provider, watcher/listeners, remote flags and mutable-reference defensive copying |

## Second audit: 10 remaining work items

1. **P0 – Release reproducibility:** download dependencies, tidy/verify and commit checksums, ensure toolchain compatibility, run complete tests and vulnerability scan, license/branch/tag policy. Git tags are visible to Go consumers before a GitHub Release workflow finishes: approve before tagging. Do not publish the candidate as stable.
2. **P0 – Database isolation enforcement:** adapt RLS policies to every business table, provision non-owner/non-BYPASSRLS roles, test cross-tenant queries, direct SQL access, cross-branch transfers and connection reuse. Scope GUCs are not a defense against arbitrary SQL execution by a trusted application role.
3. **P0 – Durable event behavior:** exercise inbox and leased outbox under crashes, retries, network ambiguity and contention. Ensure schema/version routing, authenticated producer ACLs, durable DLQ and ordering rules. No exactly-once transport guarantee.
4. **P0 – Authentication lifecycle:** use supported dependency versions, add refresh rotation/reuse detection, service-to-service auth, JWKS refresh failure backoff and KMS/secret-provider adapters; validate Redis revocation durability and cache-expiry policy.
5. **P1 – Telemetry:** compile and run OpenTelemetry against collector, add SQL/Redis/NATS integrations, server status and runtime metrics. Enforce bounded labels and redaction.
6. **P1 – Distributed cache correctness:** test Redis failover, evictions, cluster/Sentinel and cross-process load control; durable fencing at protected DB/resource. Redis counters may roll back.
7. **P1 – Durable jobs:** introduce a durable queue/lease implementation and schedule history, calendar rules, cancellation and crash recovery; verify dead jobs and concurrent claims. The pool and interval scheduler are process-local abstractions unless an external lease/source is supplied.
8. **P1 – Object storage hardening:** validate adapter against target S3/MinIO, enforce quota and concurrent staging limits, quarantine and malware scanning, download integrity and versioning. Presigned URLs are bearer tokens.
9. **P1 – Audit and retention governance:** implement actual roles, key rotation and independent anchoring/deletion detection, audit export and retention policy. HMAC only covers event fields/change payload, not database timestamp or deletion.
10. **P2 – Configuration and flags lifecycle:** optional YAML/secret backend, immutable or deep-copied config model, reload/watch and change audit, remote rollout provider.

## Verification evidence

- `docs/v3-offline-test.log`: Go 1.23.2, temporary isolated modfile containing only original module path and Go 1.23.2; 25 stdlib-only package suites passed with `-race -count=1` and `go vet`; bounded pagination files passed separately (not full Gin-dependent package).
- Original module minimum remains 1.23.4. No full module compilation, dependency vulnerability scan, live Redis/NATS/PostgreSQL/MinIO/OTLP execution or GitHub Actions run. Network DNS access and Docker are unavailable. `docs/v3-full-check.log` records the local toolchain failure.
- Added tests are not proof of test execution when dependency-backed. Test V3 PostgreSQL and Redis integration fixtures in an isolated environment. MinIO/OTel adapters are source-only pending integration.
- No fabricated go.sum checksums, no remote publication, no license grant, no production-ready claim. New external versions are candidate pins, not an assertion of current security or compatibility.
