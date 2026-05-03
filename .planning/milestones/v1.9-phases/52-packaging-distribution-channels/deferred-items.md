# Phase 52 — Deferred Items (out-of-scope discoveries)

## Local-only `tmp/graphify/` cgo noise in `go vet ./...`

**Discovered during:** Plan 52-01 Task 2 verification.

**What:** Running `go vet ./...` from the repo root surfaces:

> `package github.com/postfix/serena/tmp/graphify/tests/fixtures: C source files not allowed when not using cgo or SWIG: sample.c`

**Why it's out of scope:** `tmp/` is in `.gitignore`; the `tmp/graphify/` tree is
local scratch produced by the user's `graphify` skill, not committed code, and
not part of any plan in Phase 52. The Go toolchain walks gitignored directories
when resolving `./...`, so the warning surfaces locally even though the files
never enter the repo.

**Workaround applied during Plan 01 verification:** filtered package list to
real subtrees:

```sh
go list ./internal/... ./cmd/... ./api/... ./protocol/... | xargs go vet
go list ./internal/... ./cmd/... ./api/... ./protocol/... | xargs go test -count=1
```

Both pass cleanly. The noise is environmental, not a regression introduced by
Plan 52-01.

**Suggested follow-up (not for this phase):** if the graphify skill becomes
durable scratch space, either move it under `tmp/.graphify-cache/` (which Go
already excludes via the `.` prefix rule) or have the skill set `GOFLAGS=-tags=novet`
locally. No change required from Phase 52.
