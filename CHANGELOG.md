# Changelog

## Unreleased — v0.3.0 candidate

- Add scoped PostgreSQL transactions, tenant-scoped keys, event envelope and transactional inbox.
- Add cached HTTPS JWKS verification, revocation interface, opaque refresh token primitives and resource authorization.
- Add cache-aside load coalescing, Redis token buckets and fencing, bounded worker pools and interval scheduling.
- Add leased outbox batch claims, ownership-checked completion and dead-event redrive.
- Add histogram and opt-in OpenTelemetry tracing, scoped object storage and MinIO adapter.
- Add allowlisted audit changes with HMAC evidence, explicit retention, typed config and scoped flags.
- Add migration 002, CI release gate, integration fixtures and engineering documentation.
- Legacy package APIs retained; new dependencies must be resolved before release. Do not mix legacy and leased outbox workers.

## v0.2.0 candidate

Database, outbox, idempotency, audit, resilience, metrics, validation, keyring and Redis fixed-window rate limiting.
