# Phase 83 — Deferred / Out-of-Scope Items

## `go mod tidy` blocked by pre-existing s2a-go module-graph issue (Plan 01)

- **Discovered:** Plan 01, Task 3 (`go mod tidy` after `go get chromem-go`).
- **Symptom:** `go mod tidy` aborts with:
  ```
  go: finding module for package github.com/google/s2a-go
      github.com/google/s2a-go: module github.com/google/s2a-go@latest found (v0.1.9),
      but does not contain package github.com/google/s2a-go
  ```
- **Root cause:** transitive via the sigstore chain
  (`sigstore/timestamp-authority` → `sigstore/sigstore` kms/gcp →
  `google.golang.org/api/option` → `google.golang.org/api/internal` →
  `github.com/google/s2a-go`). Entirely unrelated to the chromem-go addition.
- **Scope decision:** OUT OF SCOPE for Plan 01 (deviation-rule SCOPE BOUNDARY —
  not caused by this task's changes). chromem-go was instead promoted from
  `// indirect` to a direct `require` by a targeted hand-edit of `go.mod`
  (verified: `go list -m github.com/philippgille/chromem-go` → `v0.7.0`;
  `go build ./...` for the touched packages succeeds; `go test ./bench/ragindex/`
  green). A full-tree `go mod tidy` should be run once the s2a-go graph issue is
  resolved in a dedicated maintenance task.
- **Action required:** dedicated module-hygiene task to resolve the s2a-go
  resolution (likely a `google.golang.org/api` / `s2a-go` version pin), then
  re-run `go mod tidy` to normalize the whole module graph.
