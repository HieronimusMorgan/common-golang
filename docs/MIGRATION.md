# Migration and compatibility

Module path remains `github.com/HieronimusMorgan/common-golang`. Minimum Go version remains 1.23.4.

The existing `response.SendResponse`, `response.SendResponseList`, `response.ErrorResponse`, `response.Response`, `response.ListResponse`, `response.PagedData`, `pagination.GetPageIndexPageSize`, `pagination.PageIndex` and `pagination.PageSize` APIs retain their original behavior and JSON keys. In particular, the list timestamp remains the legacy GMT+7 `time.DateTime` string (not ISO 8601 despite the old comment); a future versioned response contract may change this. No existing function silently clamps page size or changes error payload structure.

For new or migrated endpoints use `pagination.Parse(c.Request.URL.Query(), pagination.Config{})` with default size 10, max size 100 and max offset 1,000,000. It rejects duplicate/empty/invalid values and overflow. Use a service-owned cursor pagination design for very large or rapidly changing data sets.

Replace unsafe raw errors in new endpoints with `response.Fail(c, apperror.Wrap(...))`; this aborts Gin and exposes only an approved public message, error code and correlation ID. Unknown/internal errors are normalized to 500. Do not pass sensitive details as a public message for non-500 codes. Existing response functions are intentionally unchanged and can still leak data if callers send raw errors.

The default JWT example claim names are `sub`, `iss`, `aud`, `exp`, `iat`, `nbf`, `jti`, `token_use`, `tenant_id`, `branch_ids` and `permissions`. This is a proposed shared claim contract, not verified against the actual authentication service. Adapt it explicitly if current token formats differ. No automatic migration of auth, Redis keys, event subjects or database schema occurs.

After `go mod tidy`, review module changes, run all tests, perform vulnerability scanning (e.g. `govulncheck ./...` with a compatible current toolchain), and publish a tagged release before updating production consumer services.

## V2 opt-in migration

Apply `migrations/001_common.sql` via the service migration system before using SQL helpers. Existing tables and endpoints are not changed. JWT `Manager` retains its legacy contract; `KeyRing` requires `kid`, so distribute new public keys and migrate token issuance/verification in a coordinated rollout. Health readiness retains the JSON response shape but overlapping checks may report unavailable rather than wait. See V2 guide for authorization, retention and transaction requirements.
