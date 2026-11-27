# Engineering audit V2

Basis: inspected the actual previous enhanced archive. This iteration keeps the module path, minimum Go version and legacy response/pagination APIs. The first audit identifies 10 gaps; V2 adds implementations for all, with verification limits stated below.

| Gap | V2 change | Verification |
|---|---|---|
| SQL lifecycle/transaction | database.Open, WithinTx | offline stub test (commit, rollback, panic); real PG pending |
| Transactional outbox | migration, enqueue, skip-locked worker with retry/dead state | validation test; PG integration pending |
| Idempotency | scoped SQL key, fingerprint, response replay | key tests; concurrent PG test authored, pending |
| Key rotation | JWT key ring, kid, atomic snapshot | rotation regression authored, pending dependency compilation |
| Business audit | transaction-bound append and scoped event | scope tests; SQL integration pending |
| Resilience | bounded jittered retry, circuit breaker | offline race tests |
| Observability | fixed-cardinality Prometheus text metrics | concurrent counter tests |
| Validation | strict typed JSON, limits, duplicate detection | offline regression tests |
| Rate limiting | atomic Redis fixed-window policy | Redis integration authored, pending |
| Readiness isolation | deadline, parallel checks, bounded in-flight, panic isolation | offline stuck/panic tests |

## Second audit: remaining gaps, ordered by platform risk

1. P0 — Release reproducibility: original module requires Go 1.23.4; dependency network and Docker unavailable here; go.sum does not yet contain all added checksums. Full build, integration, security scan and version review must pass before release.
2. P0 — Real service integration and SQL authorization: no Asset Management repositories or auth token fixtures supplied; must prove tenant/branch filtering, permissions and cross-branch transfer invariants. Consider DB RLS where appropriate.
3. P0 — Dedicated consumer inbox/event contract: idempotency is scoped to authenticated commands and not a complete broker inbox. Add consumer + event ID deduplication, versioned event schema, schema validation, and service-specific business constraints.
4. P1 — Production key distribution/session lifecycle: key ring uses explicitly supplied keys; no JWKS cache, KMS integration, refresh-token or revocation workflow. Never distribute signing private keys to resource services.
5. P1 — Full distributed observability: Prometheus-compatible counters/total duration only; no OpenTelemetry traces, histogram percentiles, broker/database instrumentations, dashboards or alerting.
6. P1 — Outbox scaling and recovery operations: one transaction held per publish; no claim lease, batched dispatch, per-aggregate ordering, automated redrive or retention. Test broker outages and tune concurrency before production.
7. P1 — Audit compliance controls: no cryptographic tamper evidence, append-only database privilege provisioning, export/retention workflows or before/after snapshots. Policies depend on service requirements.
8. P1 — Advanced rate limiting/cache safety: fixed-window limits permit boundary bursts; no token bucket, fencing tokens or cache stampede suppression; Redis failover can affect coordination. Use database constraints for asset integrity.
9. P1 — File/object storage: no S3/MinIO adapter, upload policy, checksum verification, quarantine or presigned URL lifecycle. Needed if asset attachments are in scope.
10. P2 — Operational administration: no feature flag, scheduled-job coordination, production migration runner, unified worker lifecycle or configuration schema orchestration. Add based on confirmed service needs rather than making every service import unrelated infrastructure.

These are residual gaps from a source-level re-audit, not defects proven by production testing. V2 deliberately avoids claiming complete production readiness. The next recommended gate is #1 and #2, then the dedicated consumer inbox.
