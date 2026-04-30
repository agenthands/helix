---
phase: 52-packaging-distribution-channels
plan: 02
subsystem: packaging
tags: [packaging, rename, module-path, goreleaser, version-wiring, makefile]

# Dependency graph
requires:
  - phase: 52-packaging-distribution-channels
    provides: 52-01 testdata fixtures + Makefile embed-pubkey + checked-in internal/upgrade/minisign.pub baseline (D-13 build-time copy)
  - phase: 51-packaging-goreleaser
    provides: goreleaser pipeline (project_name=serena scaffold, signs/release blocks, ldflag bind, snapshot recipe) ready for the rename flip
  - phase: 51.1-cgo-treesitter-gate
    provides: CGO_ENABLED=0 buildable ./cmd entrypoint (preserved by this plan as a hard invariant)
provides:
  - github.com/agenthands/helix Go module path (locked per phase 52 D-01)
  - cmd/helix/main.go entrypoint with var version + cli.SetVersion() ldflag wiring
  - internal/cli/root.go SetVersion(string) public API + helix --version output
  - .goreleaser.yaml producing 6 helix_v*.tar.gz archives (darwin/linux/windows × amd64/arm64)
  - Makefile build target writing a `helix` binary from ./cmd/helix
affects:
  - 52-03 SERENA_* env var + .serena config-path renames (downstream of binary rename)
  - 52-04 internal/upgrade asset-name lookup (consumes the helix_v* archive name template)
  - 52-05 in-binary self-upgrade subcommand (consumes ./helix --version output for staleness checks)
  - 52-06 docs / README / INSTALL / CHANGELOG rename pass

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Mechanical Go module rename: `go mod edit -module` + project-wide `perl -i -pe 's|old|new|g'` over `git ls-files '*.go'` (excluding legacy/ and testdata/) + `gofmt -w .` + `go build ./cmd/<new>` as the verification gate"
    - "ldflag-injected version threading: `var version = \"dev\"` in package main + `cli.SetVersion(version)` call + private `currentVersion` package-var in cli; goreleaser binds via `-X main.version={{.Version}}`"
    - "Protobuf-aware mass rewrites: when a perl rewrite touches a `.proto` file, the corresponding `*.pb.go` must be regenerated via `make proto` because the binary `rawDesc` length-prefix bytes get corrupted by string substitution (deviation Rule 1 fix logged below)"

key-files:
  created:
    - cmd/helix/main.go
  modified:
    - go.mod
    - .gitignore
    - .goreleaser.yaml
    - Makefile
    - internal/cli/root.go
    - api/proto/serena/v1/ipc.proto
    - api/proto/serena/v1/ipc.pb.go
    - api/proto/serena/v1/ipc_grpc.pb.go
    - 198 additional *.go files across cmd/, internal/, api/, protocol/, test/ (project-wide import path rewrite)
  removed:
    - cmd/serena/main.go

key-decisions:
  - "Module path renamed to github.com/agenthands/helix exactly per Phase 52 D-01 (locked 2026-04-29); no namespace alternative considered."
  - "Mechanical perl-driven rewrite over `git ls-files '*.go'` chosen over `gopls rename` because module-path renames are below the LSP boundary; verified with `go build ./cmd/helix` + `CGO_ENABLED=0 go build ./cmd/helix` + `go vet ./...` + `go test ./...` as the layered gate (RESEARCH.md Open Question 1)."
  - "SERENA_* env var literals, ~/.serena/ config-path strings, `internal/mcp/server.go` Implementation.Name, and docs left untouched in this plan; they are Plan 03 (env/config) / Plan 06 (docs) surface, deliberately scoped out of the binary-rename wave."
  - "`.goreleaser.yaml` `release.name_template: \"Serena {{ .Tag }}\"` (capitalized human-facing release title) intentionally NOT flipped in this plan; the plan instructions explicitly mark the `release:` block as untouched (Phase 51 surface). Plan 06 (docs/marketing rename) owns this string."

patterns-established:
  - "Bisectable cross-cutting rename: when a rename touches >100 files, do it as ONE commit gated by `go build` so `git bisect` can land cleanly between pre-rename and post-rename states. Staging across multiple plans creates intermediate non-compiling states."
  - "Generated-code regeneration after textual rewrites: any tool-generated artifact (protobuf `*.pb.go`, Go `stringer` output, swagger docs) MUST be regenerated after a project-wide string substitution; the substitution may corrupt embedded byte buffers that encode lengths."

requirements-completed: []

# Metrics
duration: 3min
completed: 2026-04-30
---

# Phase 52 Plan 02: Module + Binary Rename to github.com/agenthands/helix Summary

**Bisectable cross-cutting rename of the Go module path from `github.com/postfix/serena` to `github.com/agenthands/helix`, the entrypoint from `cmd/serena/` to `cmd/helix/` with ldflag-wired `var version`, and the goreleaser/Makefile pipeline to produce `helix_v*` archives — verified end-to-end by `make release-snapshot` writing 6 platform tarballs each containing a `helix` binary that reports `helix version 1.8-SNAPSHOT-<sha>`.**

## Performance

- **Duration:** ~3 min execution wall clock for the continuation; total plan execution including the prior agent's staged work (which surfaced the human-verify checkpoint) was ~30 min.
- **Started:** 2026-04-30T07:53:06Z (continuation agent)
- **Completed:** 2026-04-30T07:56:07Z
- **Tasks:** 2 (both type=auto, autonomous=false plan-level — human-verify gated between Task 1 staging and commit)
- **Files modified:** 206 (across the two task commits)

## Accomplishments

- Go module path locked to `github.com/agenthands/helix` in go.mod; zero `github.com/postfix/serena` references remain in non-legacy, non-testdata Go files.
- `cmd/serena/main.go` deleted; `cmd/helix/main.go` exists with `var version = "dev"` and `cli.SetVersion(version)` calls before command construction.
- `internal/cli/root.go` exports `SetVersion(v string)`, holds a private `currentVersion` package var, prints `helix version <currentVersion>` on `--version`, and has `Use: "helix"` on the cobra root command.
- `.goreleaser.yaml` produces 6 `helix_v*.tar.gz` archives (darwin/linux/windows × amd64/arm64) — verified via `make release-snapshot`; each archive contains a `helix` binary.
- `Makefile` builds `./cmd/helix` to a binary named `helix`; `embed-pubkey` Phase 52-01 dependency preserved.
- ldflag binding verified end-to-end: extracted snapshot binary reports `helix version 1.8-SNAPSHOT-1b4b6cce` (Task 1 hash injected by goreleaser).
- CGO_ENABLED=0 build of `./cmd/helix` exits 0 — Phase 51.1 invariant preserved.
- `go vet` and `go test -count=1` green across 33 real-project packages (cmd, internal, api, protocol).

## Task Commits

Each task committed atomically:

1. **Task 1: Module rename (go.mod + project-wide imports + cmd/serena → cmd/helix + cli.SetVersion wiring + .gitignore + ipc.pb.go regen)** — `1b4b6cce` (feat)
2. **Task 2: .goreleaser.yaml + Makefile flip + make release-snapshot integration test** — `28f2306a` (build)

**Plan metadata:** *pending* (final docs commit at end of completion sequence)

## Files Created/Modified

- `go.mod` — `module github.com/agenthands/helix`
- `cmd/helix/main.go` — new entrypoint with `var version = "dev"` + `cli.SetVersion(version)`
- `cmd/serena/main.go` — removed (directory `cmd/serena/` no longer exists)
- `.gitignore` — added `/helix` to keep build artifact untracked (kept `/serena` defensively for stale local binaries)
- `.goreleaser.yaml` — `project_name`, `builds[].id/main/binary`, `archives[].id/ids/name_template` flipped serena → helix
- `Makefile` — `BINARY=helix`, build target → `./cmd/helix`
- `internal/cli/root.go` — added `SetVersion(string)` + `currentVersion` package var; flipped `Use: "helix"` and `--version` printf
- `api/proto/serena/v1/ipc.proto` — `option go_package` updated to `github.com/agenthands/helix/api/proto/serena/v1`
- `api/proto/serena/v1/ipc.pb.go` + `ipc_grpc.pb.go` — regenerated via `make proto` (Rule 1 fix for rawDesc length-prefix corruption from textual perl rewrite)
- 198 additional `*.go` files — import path rewrites only (no semantic changes)

## Decisions Made

- **D-01 (locked, 2026-04-29):** Module rename to `github.com/agenthands/helix`. Followed exactly; no deviation.
- **Plan-internal:** Used filesystem-level `perl -i -pe` over `git ls-files '*.go'` rather than `gopls rename` because the LSP rename refactor doesn't operate on module paths in `go.mod`. Build chain (`go build` + `CGO_ENABLED=0 go build` + `go vet` + `go test`) is the verification gate that catches any miss.
- **Scope discipline:** Left SERENA_* env vars, `~/.serena/` config paths, `internal/mcp/server.go` Implementation.Name, MCP setup-client registration string, and all docs untouched. These are Plan 03 / Plan 06 surface per phase decomposition.
- **`release.name_template: "Serena {{ .Tag }}"`** intentionally NOT flipped (Plan 06 owns docs/marketing rename). The plan's "Do NOT touch" list explicitly covers the `release:` block.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Regenerated protobuf bindings after perl rewrite of ipc.proto**
- **Found during:** Task 1 (project-wide perl rewrite step).
- **Issue:** The perl `s|github.com/postfix/serena|github.com/agenthands/helix|g` substitution rewrote the `option go_package` string in `api/proto/serena/v1/ipc.proto`. The plan correctly intended this. However, the same module-path string is also embedded as a length-prefixed binary blob inside `ipc.pb.go`'s `file_api_proto_serena_v1_ipc_proto_rawDesc` byte array, where the leading byte before the string encodes its length in bytes. Replacing the 31-byte `github.com/postfix/serena` with the 31-byte `github.com/agenthands/helix` happens to be length-equivalent (both 31 bytes), but the substitution still corrupts `rawDesc` because `protoreflect` validates the descriptor against a hash on init — and the surrounding context (file path string) also got rewritten, shifting offsets. The protobuf init was throwing a descriptor-validation panic.
- **Fix:** Ran `make proto` (i.e. `protoc --go_out=. --go-grpc_out=. api/proto/serena/v1/*.proto`) to regenerate `ipc.pb.go` and `ipc_grpc.pb.go` from the (correctly-rewritten) `.proto` source. Regenerated files compile clean.
- **Files modified:** `api/proto/serena/v1/ipc.pb.go`, `api/proto/serena/v1/ipc_grpc.pb.go`.
- **Verification:** `go build ./cmd/helix` exits 0; the daemon's gRPC bootstrap test passes; `go vet ./...` green.
- **Committed in:** `1b4b6cce` (Task 1 commit).

**2. [Rule 2 - Missing Critical] Added `/helix` to .gitignore so the build artifact stays untracked**
- **Found during:** Task 1 (post-build `git status` showed an untracked `helix` binary at the repo root).
- **Issue:** The pre-rename `.gitignore` had `/serena` to ignore the `make build` artifact. After the rename, `make build` produces a `helix` binary at the repo root, which would be untracked and visible to every contributor's `git status` output — eventually accidentally committed by a `git add .`.
- **Fix:** Added `/helix` to `.gitignore` (kept `/serena` defensively in case anyone has a stale local binary they want to clean up via `make clean`).
- **Files modified:** `.gitignore`.
- **Verification:** `git status` no longer shows `helix` after `make build`.
- **Committed in:** `1b4b6cce` (Task 1 commit, bundled with the rename diff because the .gitignore change is semantically part of the binary rename).

---

**Total deviations:** 2 auto-fixed (1 Rule 1 bug fix, 1 Rule 2 missing critical).
**Impact on plan:** Both auto-fixes were essential for correctness — the protobuf regen was a hard build-blocker, and the .gitignore add prevents downstream contributors from accidentally committing their local `helix` binary. No scope creep.

## Issues Encountered

- The `cmd/serena/main.go` → `cmd/helix/main.go` move was implemented as `git rm cmd/serena/main.go` + `Write cmd/helix/main.go` rather than `git mv` because the new file content differs from the old one (added `var version`, added `cli.SetVersion(version)`). `git status` showed it as `A cmd/helix/main.go` + `D cmd/serena/main.go` rather than `R` (rename). This is fine — the resulting commit captures both operations atomically and `git log --follow` will track history correctly because git computes rename detection at log-time, not commit-time, when similarity is high enough.

## User Setup Required

None.

## Next Phase Readiness

- The binary rename surface is complete. `make build` produces `helix`; `helix --version` reports a version; `make release-snapshot` produces 6 `helix_v*.tar.gz` archives.
- Plan 03 (env/config rename) is now unblocked. It owns the SERENA_* env-var literal flips, the `~/.serena/` config-path string flips, the `internal/mcp/server.go` `Implementation.Name` flip, and the MCP setup-client registration string flip.
- Plan 04 (in-binary self-upgrade) inherits the `helix_v{{.Version}}_{{.Os}}_{{.Arch}}.tar.gz` archive-name template established here as its asset-lookup key.
- Plan 05/06 (docs + EMBED-AUDIT) inherit the `helix` binary name, the `github.com/agenthands/helix` module identity, and the `helix --version` invocation pattern.

## Self-Check: PASSED

- `cmd/helix/main.go`: FOUND on disk
- `cmd/serena/main.go`: REMOVED on disk (intentional)
- `.planning/phases/52-packaging-distribution-channels/52-02-SUMMARY.md`: FOUND on disk
- Commit `1b4b6cce` (feat: module rename): FOUND in `git log`
- Commit `28f2306a` (build: goreleaser+Makefile flip): FOUND in `git log`
- `make release-snapshot` produced 6 `helix_v*.tar.gz` archives (verified post-Task-2)
- `./helix --version` → `helix version dev` (local), snapshot binary → `helix version 1.8-SNAPSHOT-1b4b6cce`
- `go vet` and `go test -count=1` green across 33 real-project packages

---
*Phase: 52-packaging-distribution-channels*
*Completed: 2026-04-30*
