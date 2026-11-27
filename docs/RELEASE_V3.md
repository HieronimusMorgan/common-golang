# V3 release gate

Candidate only. Preserve original module path and Go minimum for now. The owner must choose a license (no license grant has been made here), add LICENSE, configure branch/tag protection, CODEOWNERS using real GitHub accounts, private vulnerability reporting, secret scanning and any external contribution policy. A private repository may be used for review before a public release.

1. Use supported Go locally; run `go mod tidy` and commit the resolved `go.mod` / `go.sum`. Check whether new transitive dependencies raise the minimum version; update CI matrix and docs deliberately if so. No checksum is fabricated.
2. `go mod verify`, `go build -mod=readonly ./...`, `go vet -mod=readonly ./...`, `go test -mod=readonly -race -count=1 ./...`.
3. Run `docker compose up -d --wait`, `make integration` on a disposable database, and verify MinIO/OTLP against target providers. `docker compose down -v` deletes local fixture volumes.
4. Run `go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...`; review and resolve findings. Review scanner version before release; pinned does not mean vulnerability-free. Enable full content secret scanning. Check licenses, compatibility and Go toolchain support.
5. Configure required checks and tag protection. Commit release notes. Tag `v0.3.0` only after approval. Tag workflow produces GitHub pre-release and module inventory only after passing CI and requiring LICENSE. Inventory is not a formal SPDX/CycloneDX SBOM. Workflow uses major-version action tags; pin immutable commit SHAs through owner policy.

CI was authored but not executed on GitHub. No repository was created, modified or published. Stable version and API compatibility certification require further verification.
