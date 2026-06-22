# Phase 96: Address v2.0 tech debt - Research

**Researched:** 2026-06-22
**Domain:** Go internals (daemon listener validation, CLI dead-code removal, advisory-hook classifier, generated-doc drift gate)
**Confidence:** HIGH

## Summary

Phase 96 closes four small, well-bounded, non-blocking tech-debt items surfaced by the v2.0 milestone audit, plus a companion Nyquist coverage close (TD-05) that is orchestrated *outside* this code plan via `/gsd-validate-phase`. All four code/doc items were verified to still exist in the tree at research time, and the baseline is green: `go build ./...` clean, `go vet` clean on the touched packages, and `go run ./cmd/docgen --check` reports "README.md is up to date." [VERIFIED: local build/vet/docgen run]

Each fix mirrors an existing, well-understood pattern already in the codebase, so there is no novel design work: TD-01 mirrors `validateGRPCAddr` (CR-01, Phase 94) onto its sibling `validateAdminAddr`; TD-02 is a pure deletion of a now-orphaned helper plus its tests; TD-03 tightens one classifier so a *search pattern* that happens to look like a code path is no longer counted as a file operand; TD-04 rewords a tool Description at source and regenerates README through the docgen drift gate (never a hand-edit). No behavior change to the frozen 50-verb CLI surface, no diff to `api/proto/`, and the retained internal gRPC `StreamMCP` wire is untouched.

**Primary recommendation:** Do all four as independent, single-file (plus test/regen) commits. TD-01, TD-02, TD-03 each ship with a table-driven unit test; TD-04 ships as a source reword + `make docs` regen verified by `make verify-docs`. Run `go build ./... && go vet ./... && go test ./...` and the docgen drift gate as the phase gate. None of these items interact, so they can be planned as parallel tasks in a single wave.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| TD-01 admin-listener bind validation | API/Backend (daemon) | — | `validateAdminAddr` gates the daemon's optional loopback admin HTTP listener (instrumentation surface) |
| TD-02 dead-code removal | Layer 3 setup CLI | — | `mergeJSONConfig` lived in the `helix setup` composition path; orphaned by the Phase-93 setup flip |
| TD-03 nudge classifier | Layer 3 hook/CLI | — | `classifyBashTarget` powers the PreToolUse advisory hook; pure local string classification, no LSP/daemon |
| TD-04 generated tool-help description | Layer 1 kernel help tool + docgen | Docs | Description lives at source in `internal/kernel/help/`; surfaced into generated `README.md` by `cmd/docgen` |

## Project Constraints (from CLAUDE.md)

- **MUST run `go vet ./...` and `go test ./...` before completing any Go task.** (also `go build ./cmd/helix`)
- **GSD workflow enforcement:** file-changing tools only inside a GSD workflow.
- **`helix <verb>` CLI is the only agent-facing surface.** MCP Go SDK + gRPC IPC are *internal daemon plumbing*. Off-message "MCP tool" phrasing in user-visible docs is the exact thing TD-04 corrects.
- **README tool table is auto-generated** by `cmd/docgen` — **do not hand-edit**; it is drift-gated by `make verify-docs` (`docgen --check`).
- **Frozen 50-verb surface** must not change; do not invent verbs. Citations must come from `internal/cli/verbs_gen.go`.
- **No diff to `api/proto/`** and no change to the retained gRPC `StreamMCP` wire.
- Go-only single binary; no Python/Docker/runtime deps.

## User Constraints (from CONTEXT.md)

### Locked Decisions
- **Confirmed scope (user):** All four actionable code/doc items (TD-01..TD-04) AND retroactive Nyquist validation on phases 93/94/95 (TD-05). Pre-existing bench failures explicitly excluded.
- Each code fix (TD-01..TD-04) ships with a regression/unit test where applicable.
- TD-04 must go through docgen regen, never a hand-edit of README.md (drift gate). Keep docgen's blank imports parity-correct (v1.12 docgen-drift lesson).
- Verification target: `go build ./...`, `go vet ./...`, `go test ./...` all green; docgen drift gate green; no diff to `api/proto/`; no change to the frozen verb set.
- TD-05 is orchestrated **outside** the code plan via `/gsd-validate-phase` — **do not include it in the Phase 96 code plan.**

### Claude's Discretion
- Implementation choices are at Claude's discretion, guided by the audit findings, the existing patterns each fix mirrors, and codebase conventions. Prefer the minimal change that closes the item plus a regression test.

### Deferred Ideas (OUT OF SCOPE)
- Pre-existing `cmd/helix-bench` `TestRunSubcommandWires*` env-gated failures — out of scope; the v1.12 "live execution honestly gated-skip" deferred item. They fail identically on the pre-v2.0 baseline; not a v2.0 regression.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TD-01 | Harden `validateAdminAddr` to reject empty-host/wildcard binds, mirroring CR-01's `validateGRPCAddr` fix | `validateGRPCAddr` fix at `grpc_tcp.go:85-109` + its test table at `grpc_tcp_test.go:27-46`; `validateAdminAddr` gap at `telemetry.go:108-121`; caller at `listenAdmin` (`telemetry.go:40-47`); existing admin test at `telemetry_test.go:18-49` |
| TD-02 | Remove dead `mergeJSONConfig` helper + its now-orphaned tests | Zero non-test callers confirmed; tests at `setup_test.go:17-118`; sibling `removeFromJSONConfig` is still live (keep it) |
| TD-03 | Stop `classifyBashTarget` counting a search *pattern* (e.g. `grep foo.go`) as a file operand | Logic at `nudge.go:337-406`; tests at `nudge_test.go:174-247`; caller `steerMessage` (`nudge.go:158-175`); fail-open contract must be preserved |
| TD-04 | Reword off-message "any MCP tool" `get_tool_help` description at source + regen README | Four source occurrences in `internal/kernel/help/`; docgen reads `HelpSkill.Tools()[].Description` (`skill_adapter.go:31`) → `README.md:332`; gate at `Makefile` `verify-docs` |

## Standard Stack

No new dependencies. All work uses the existing Go stdlib + project packages already imported by the touched files (`net`, `strings`, `path/filepath`, `encoding/json`, `testing`). [VERIFIED: codebase read]

## Package Legitimacy Audit

Not applicable — this phase installs **no external packages**. All changes are in-tree edits to existing Go source and tests.

## Architecture Patterns

### TD-01 — Mirror the CR-01 loopback-validation pattern

**Reference (the hardened sibling), `internal/daemon/grpc_tcp.go:85-109`:** [VERIFIED: codebase read]
```go
func validateGRPCAddr(addr string) error {
	if addr == "" {
		return nil // disabled is valid (unix-socket only — the default)
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid grpc addr %q: %w", addr, err)
	}
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return nil
	case "":
		// ":9099" / ":0" → net.Listen binds ALL interfaces. Refuse.
		return fmt.Errorf("grpc addr must name an explicit loopback host, got wildcard %q ...", addr)
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() && !ip.IsUnspecified() {
		return nil
	}
	return fmt.Errorf("grpc addr must be loopback, got %q ...", host)
}
```

**The gap (the sibling to fix), `internal/daemon/telemetry.go:108-121`:** [VERIFIED: codebase read]
```go
func validateAdminAddr(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid admin addr %q: %w", addr, err)
	}
	switch host {
	case "", "localhost", "127.0.0.1", "::1":   // <-- "" (wildcard host) wrongly accepted
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {   // <-- missing !ip.IsUnspecified()
		return nil
	}
	return fmt.Errorf("admin addr must be loopback, got %q (v1.3 will add auth for non-loopback)", host)
}
```

**Two concrete gaps to close, mirroring CR-01:**
1. **Empty host** (`":9090"` / `":0"`) is in the accept `switch` — it must be *refused* (an empty host makes `net.Listen("tcp", addr)` bind all interfaces). Move `""` out of the accept case into an explicit refusal (or fall through to the non-loopback error).
2. **Missing `!ip.IsUnspecified()` guard** on the `net.ParseIP` branch — add it for defense-in-depth parity with `validateGRPCAddr` (belt-and-suspenders against `0.0.0.0` / `::` should the explicit branch ever be reordered).

**Critical contract difference vs validateGRPCAddr — DO NOT introduce a regression:**
- `validateGRPCAddr` treats an *entirely empty `addr` string* (`""`) as "disabled → valid" because the empty-addr no-op is handled differently.
- `validateAdminAddr` does **not** special-case `addr == ""` — the empty-addr no-op is handled *upstream* in `listenAdmin` (`telemetry.go:42-44`: `if addr == "" { return nil }` *before* calling `validateAdminAddr`). The existing admin test asserts `{name: "empty", addr: "", wantErr: true}` because `net.SplitHostPort("")` errors. [VERIFIED: `telemetry_test.go:25`, `telemetry.go:42`]
- **Therefore:** do NOT add an `if addr == "" { return nil }` short-circuit to `validateAdminAddr`. Closing the *empty-host* gap (`":9090"`) is different from the *empty-addr* case (`""`) — keep `addr == ""` erroring inside the validator (its no-op lives in the caller). Update the existing `"empty"` test case intent comment if needed but keep `wantErr: true`.

**Lower severity (audit-confirmed):** the admin listener exposes only instrumentation (`/healthz`, `/readyz`, `/metrics`, opt-in pprof) — not the tool surface. The fix is for parity/defense-in-depth, not an active exploit closure. Keep the existing `v1.3`/auth-roadmap error reference in the message (the admin path references the v1.3 auth roadmap, not REMOTE-01). [VERIFIED: `telemetry.go:120`]

### TD-02 — Pure dead-code deletion with orphan sweep

`mergeJSONConfig` (`setup_clients.go:60-91`) has **zero non-test callers** — only its definition, its doc comment, and four test calls in `setup_test.go` reference it. [VERIFIED: `grep -rn mergeJSONConfig`]

```
internal/cli/setup_clients.go:60  func mergeJSONConfig(...)        <- definition (remove)
internal/cli/setup_test.go:23,57,77,104  mergeJSONConfig(...)      <- tests (remove)
```

**Sibling helper `removeFromJSONConfig` is STILL LIVE — do NOT remove it.** It has ~15 production callers across `setup_clients.go` (it is the teardown path for the Phase-93 setup flip). [VERIFIED: `grep -rn removeFromJSONConfig`]

**Orphan sweep after removal:**
- Remove the `mergeJSONConfig`-specific tests in `setup_test.go` (lines ~17-118, the `// --- mergeJSONConfig tests ---` block; stop before the `// --- removeFromJSONConfig tests ---` block at line 120).
- Check `internal/cli/setup_clients.go` imports (`encoding/json`, `os`, `fmt`, `path/filepath`) are still used by `removeFromJSONConfig` and other functions — they are, since `removeFromJSONConfig` uses the identical set. Do **not** prune imports blindly; let `go build`/`go vet` confirm.
- Confirm no `setup_test.go` import becomes unused once the merge tests go.

### TD-03 — Distinguish search PATTERN from file operand

`classifyBashTarget` (`nudge.go:337-406`) recognizes `grep/rg/ag/sed/cat/find/egrep/fgrep`, then loops over **all** non-flag tokens, classifying any token with a code extension as a code file operand. [VERIFIED: codebase read]

**The bug:** for `grep foo.go`, the *pattern* `foo.go` has ext `.go ∈ codeExtensions` → counted as a code operand → hook over-triggers (suggests a helix verb when the user is searching for the literal string "foo.go", not reading a file). [VERIFIED: read of loop at `nudge.go:355-394`]

**Recommended fix (minimal, pattern-aware):** For the search commands that take a leading PATTERN argument (`grep`, `rg`, `ag`, `egrep`, `fgrep` — NOT `cat`/`sed`/`find`, whose first operand is a file/expr), skip the first non-flag, non-option-value token (the pattern). The remaining tokens are the file operands. Edge cases to handle:
- Flags consume the leading position before the pattern (`grep -i foo.go file.go` → pattern is `foo.go`, file is `file.go`).
- `grep -e PATTERN file` / `grep -f PATFILE file` — `-e`/`-f` take an argument; the current loop skips `-`-prefixed tokens but does NOT skip the *value* after `-e`. The doc comment at `nudge.go:357-358` already acknowledges this is "acceptable" for pattern-finding; the new logic must not regress it. Keep it simple: treat the *first* non-flag token after the command as the pattern for search commands.
- `cat`/`sed`/`find` keep current behavior (no leading pattern to skip).

**Existing test expectations to preserve / extend** (`nudge_test.go:174-247`): [VERIFIED: codebase read]
- `TestClassifyBashTarget_MixedTargetsConservative` asserts `grep x main.go README.md` → `(false, true)` (non-code wins). Note the pattern here is `x` (no ext) so it is already correctly skipped today by the "unknown ext, no separator → ignore" default branch. After the fix, `main.go` and `README.md` are still both operands.
- `TestClassifyBashTarget_PureDataNoExec` asserts `grep x f.go && rm -rf /` does not execute and that `grep x $(whoami).go` is handled. `$(whoami).go` has ext `.go`; today it would be (wrongly, but harmlessly) counted as a code operand — verify the existing assertion (`ok2`) and ensure the pattern-skip change keeps it sane (with the fix, `$(whoami).go` is the file operand since `x` is the pattern → still classified code; the test only asserts no-exec + a boolean, so confirm exact expected values when editing).
- **New cases to add (the TD-03 regression):**
  - `grep foo.go` → `(false, false)` — pattern only, no file operand → fail open (was wrongly `(true, true)` before).
  - `grep foo.go bar.go` → `(true, true)` — `foo.go` is the pattern (skipped), `bar.go` is a code file.
  - `grep -i foo.go util.go` → `(true, true)` — flag, then pattern `foo.go`, then file `util.go`.
  - `rg pattern.go` → `(false, false)` (rg pattern only).
  - control: `cat main.go` → `(true, true)` (cat has no leading pattern; first operand is the file).

**Fail-open contract is sacred:** `classifyBashTarget` returns `(false, false)` (= "no signal → caller stays silent") for anything it cannot positively identify. The PreToolUse hook exits 0 regardless (`steerMessage` returns `emit=false` on `!ok || !isCode`, `nudge.go:169-171`). The fix must only *narrow* what counts as a code operand — it must never cause a non-zero exit. [VERIFIED: `nudge.go:158-175`]

### TD-04 — Reword at source, regenerate through the drift gate

The off-message phrase "documentation for any MCP tool" appears at **four** source locations in `internal/kernel/help/`: [VERIFIED: `grep -rn "any MCP tool"`]

| File:Line | Field | Surfaced where |
|-----------|-------|----------------|
| `tools.go:27` | `mcpsdk.Tool.Description` (runtime MCP registration) | MCP tool metadata (internal plumbing) |
| `tools.go:81` | `mcp.ToolDef.Description` (registry Register) | runtime registry |
| `skill_adapter.go:31` | `mcp.ToolDef.Description` (HelpSkill.Tools()) | **→ `README.md:332` via docgen** |
| `skill_adapter.go:31` | `BriefDescription: "...for an MCP tool"` | profile-filter brief desc (also off-message) |

**Docgen reads `skill.ToolProviders()` → `tp.Tools()[].Description`** (`cmd/docgen/main.go:110,118-119`). [VERIFIED: codebase read] The HelpSkill ToolProvider returns the def at `skill_adapter.go:31`, so **that `Description` field is the one that renders into README:332.** To make `make verify-docs` reflect the reword, `skill_adapter.go:31`'s `Description` MUST change.

**Recommendation:** reword all four occurrences for CLI-first consistency (drop "MCP tool", say e.g. "any Helix tool" / "a Helix tool"), even though only `skill_adapter.go:31` Description drives the README. The two `tools.go` Descriptions and the `skill_adapter.go` BriefDescription are user/agent-visible metadata that should carry the same CLI-first identity. Keep all four wordings consistent to avoid future drift confusion.

**Regen workflow (NEVER hand-edit README.md):**
```bash
# 1. edit source descriptions in internal/kernel/help/{tools.go,skill_adapter.go}
make docs            # = go run ./cmd/docgen  → rewrites README tables
make verify-docs     # = go run ./cmd/docgen --check  → must exit 0
```
The README markers and table layout are managed by docgen; the only README diff should be the single `get-tool-help` row text.

**Blank-import parity lesson (v1.12 docgen-drift):** docgen's blank-import list (`cmd/docgen/main.go:32-42`) must keep the same *enumerated tool SET* as the daemon (`internal/daemon/imports.go`). The lists are intentionally *not literally identical* (health/help are non-blank in the daemon; semantic/extract has D-02 double-import constraints), but the resulting tool set must match — the `docgen --check` gate enforces this. [VERIFIED: `cmd/docgen/main.go:21-42`] **No import change is needed for TD-04** (the `help` package is already blank-imported at `main.go:36`); this lesson is a guard against accidentally touching the import list during the edit. The memory note `helix-tool-docs-drift.md` documents the prior incident: docgen was missing the `internal/skill/semantic` blank import → docs drifted. That import is present today (`main.go:41`).

### Anti-Patterns to Avoid
- **Hand-editing README.md for TD-04** — breaks the drift gate; always regenerate.
- **Adding `if addr == "" { return nil }` to `validateAdminAddr`** — that would flip the existing `"empty" → wantErr:true` test and the empty-addr no-op is the *caller's* job (`listenAdmin`), not the validator's.
- **Removing `removeFromJSONConfig` along with `mergeJSONConfig`** — the remove helper is live (teardown path). Only the merge helper is dead.
- **Pruning imports by hand in TD-02** — let `go build` tell you; `removeFromJSONConfig` keeps all four imports alive.
- **Making the nudge classifier exit non-zero or throw** — the advisory hook is fail-open/exit-0 by contract.
- **Changing `classifyBashTarget` to skip patterns for `cat`/`sed`/`find`** — only the grep-family commands lead with a pattern.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| README tool table | Hand-edit the markdown | `make docs` (docgen) | Drift gate + parity rule; hand edits break `make verify-docs` |
| Loopback validation | A bespoke IP check | Copy the `validateGRPCAddr` shape (`net.SplitHostPort` + switch + `net.ParseIP().IsLoopback() && !IsUnspecified()`) | Proven sibling; identical semantics expected |
| Bash command parsing for the classifier | A shell parser / regex backtracking on attacker input | `strings.Fields` bounded split (already in place) | T-93-05: never execute/regex-backtrack attacker input; tokenize as DATA |

**Key insight:** Every one of these four fixes has an in-tree precedent. The right move is to copy the existing pattern precisely, not invent.

## Common Pitfalls

### Pitfall 1: Confusing empty-addr with empty-host in TD-01
**What goes wrong:** Adding an `addr == ""` early-return to `validateAdminAddr` to "match" `validateGRPCAddr`.
**Why it happens:** The two validators look like twins but have different empty-handling contracts (admin's no-op is upstream in `listenAdmin`).
**How to avoid:** Only close the *empty-host* (`":9090"`) and `IsUnspecified()` gaps; leave `addr == ""` erroring. Keep the existing `"empty" → wantErr:true` test case.
**Warning signs:** The `TestValidateAdminAddr/empty` case flips to `wantErr:false`, or `TestListenAdmin_Disabled` behavior changes.

### Pitfall 2: TD-04 source edit doesn't move the README
**What goes wrong:** You edit `tools.go:27/81` but not `skill_adapter.go:31`, run `make verify-docs`, and it still passes (README unchanged) — but the row text didn't update.
**Why it happens:** docgen renders the ToolProvider `Tools()[].Description` (= `skill_adapter.go:31`), not the runtime `mcpsdk.Tool.Description`.
**How to avoid:** Edit `skill_adapter.go:31`'s `Description`; then `make docs` should produce a one-line README diff, and `make verify-docs` should pass post-regen.
**Warning signs:** `make docs` produces no diff after your edit → you edited the wrong field.

### Pitfall 3: TD-03 regresses the conservative/fail-open behavior
**What goes wrong:** Skipping the pattern too aggressively (e.g. for `cat`/`sed`) or counting a directory-only grep as code.
**Why it happens:** Over-generalizing the pattern-skip beyond the grep family.
**How to avoid:** Gate the pattern-skip on the grep-family command set only; keep the existing default branches for unknown/dir-only operands; re-run the full `nudge_test.go` suite.
**Warning signs:** `TestClassifyBashTarget_MixedTargetsConservative` or `_FailOpen` flips.

## Code Examples

### TD-01 test table (extend the existing admin test, mirror grpc cases)
```go
// Source: internal/daemon/grpc_tcp_test.go:34-46 (template) + telemetry_test.go:18-49 (target)
// Add to TestValidateAdminAddr cases — the wildcard/empty-host rejections CR-01 added:
{name: "wildcard_empty_host",      addr: ":9090",        wantErr: true},  // binds all ifaces
{name: "wildcard_empty_host_zero", addr: ":0",           wantErr: true},
{name: "wildcard_v6",              addr: "[::]:9090",    wantErr: true},
// existing rows kept: empty(wantErr:true), loopback_v4/v6, localhost, auto_port,
// zero_bind(0.0.0.0, "v1.3"), lan_bind, dns_host, malformed
```

### TD-03 test cases (add to nudge_test.go)
```go
// Source: internal/cli/nudge_test.go:174-247 (existing table-driven style)
// grep family leads with a PATTERN that must NOT count as a file operand:
{cmd: `grep foo.go`,            wantCode: false, wantOk: false}, // pattern only → fail open
{cmd: `grep foo.go bar.go`,     wantCode: true,  wantOk: true},  // bar.go is the file
{cmd: `grep -i foo.go util.go`, wantCode: true,  wantOk: true},  // flag, pattern, file
{cmd: `rg pattern.go`,          wantCode: false, wantOk: false},
{cmd: `cat main.go`,            wantCode: true,  wantOk: true},   // cat: first operand IS the file
```

## State of the Art

Not applicable — no external technology version changes are involved. This is internal cleanup against patterns introduced in the same v2.0 milestone (Phases 93/94/95).

## Runtime State Inventory

> This phase is a code/doc cleanup, not a rename/migration. No stored data, live-service config, OS-registered state, secrets, or build artifacts carry a string that this phase changes.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — no datastore keys/IDs touched | None |
| Live service config | None | None |
| OS-registered state | None | None |
| Secrets/env vars | None | None |
| Build artifacts | README.md is a generated artifact (TD-04) — regenerated via `make docs`, not hand-edited | Run `make docs` after the source reword |

**Nothing found in categories 1-4** — verified: TD-01/02/03 are pure Go source+test edits; TD-04 changes a Go string literal and regenerates one README row. No external state.

## Validation Architecture

> Nyquist validation is ENABLED. These are small, well-bounded fixes — validation is lightweight, concrete, and per-item.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard `testing` (table-driven) |
| Config file | none (Go toolchain) |
| Quick run command | `go test ./internal/cli/... ./internal/daemon/...` |
| Full suite command | `go test ./...` |
| Doc gate | `make verify-docs` (= `go run ./cmd/docgen --check`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TD-01 | `validateAdminAddr` rejects empty-host/wildcard binds, accepts loopback | unit (table) | `go test ./internal/daemon/ -run TestValidateAdminAddr -x` | ✅ extend `telemetry_test.go:18` |
| TD-02 | `mergeJSONConfig` removed; build+vet green; no dangling refs | build/vet + suite | `go build ./... && go vet ./... && go test ./internal/cli/...` | ✅ remove merge tests from `setup_test.go` |
| TD-03 | grep PATTERN not counted as file operand; fail-open preserved | unit (table) | `go test ./internal/cli/ -run TestClassifyBashTarget -x` | ✅ extend `nudge_test.go:174` |
| TD-04 | README `get-tool-help` row reworded; drift gate green post-regen | drift gate | `make docs && make verify-docs` | ✅ docgen `--check` |

### Sampling Rate
- **Per task commit:** the item's targeted `-run` command above (sub-second).
- **Per wave merge:** `go build ./... && go vet ./... && go test ./internal/cli/... ./internal/daemon/... ./internal/kernel/help/...` + `make verify-docs`.
- **Phase gate:** `go build ./...`, `go vet ./...`, `go test ./...` all green; `make verify-docs` green; `git diff --stat api/proto/` empty; `internal/cli/verbs_gen.go` unchanged (frozen verb set).

### Wave 0 Gaps
- None — existing test files cover all four items; the work is *extending* `telemetry_test.go` (TD-01) and `nudge_test.go` (TD-03), *removing* merge tests from `setup_test.go` (TD-02), and the docgen `--check` gate already exists (TD-04). No new test framework or fixture is required.

**Note on TD-05:** Nyquist coverage on phases 93/94/95 is closed via `/gsd-validate-phase 93|94|95`, NOT this code plan — out of scope per CONTEXT.md.

## Security Domain

> `security_enforcement` default (absent = enabled). TD-01 is itself a security-hardening item; the others are non-security cleanups.

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V1 Architecture (network exposure) | yes (TD-01) | Loopback-only bind validation; refuse wildcard/empty-host so the admin instrumentation listener never binds `0.0.0.0`/`::` by default |
| V5 Input Validation | partial (TD-03) | `classifyBashTarget` reads command strings as DATA only (no exec, bounded `strings.Fields`, no regex backtracking) — must be preserved |
| V2/V3/V4/V6 (authn/session/access/crypto) | no | Not touched by this phase |

### Known Threat Patterns for the touched surfaces
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Admin listener binds all interfaces via empty-host (`:9090`) → LAN-exposed `/metrics`,`/pprof` | Information Disclosure | TD-01: refuse empty-host + `!IsUnspecified()` guard (mirror `validateGRPCAddr`) |
| Hook treats command string as executable / regex DoS | Tampering / DoS | Already mitigated (DATA-only tokenize, T-93-05); TD-03 must not regress it |

**Severity note (audit-confirmed):** the admin listener exposes *instrumentation only* (`/healthz`, `/readyz`, `/metrics`, opt-in pprof) — not the tool surface. TD-01 is parity/defense-in-depth, lower severity than the CR-01 grpc fix (which guarded the full MCP tool surface). The admin error message keeps its `v1.3` auth-roadmap reference (not REMOTE-01).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| — | (none) | — | All claims verified by direct codebase read + local build/vet/docgen run |

**This table is empty:** Every factual claim in this research was verified against the live tree at research time (file reads, grep for callers, and a clean `go build` / `go vet` / `docgen --check` baseline run). No `[ASSUMED]` claims remain.

## Open Questions (RESOLVED)

RESOLVED: reword all four per recommendation (adopted in 96-01 Task 4).

1. **Should the two `tools.go` Descriptions and the `skill_adapter.go` BriefDescription also be reworded, or only the README-driving `skill_adapter.go` Description?**
   - What we know: only `skill_adapter.go:31` `Description` renders into README:332; the audit item names "any MCP tool" generically.
   - What's unclear: whether the runtime MCP-metadata strings (internal plumbing) are in scope for the CLI-first reword.
   - Recommendation: reword all four for consistency (CLI-first identity, avoid future drift), since they are user/agent-visible metadata and the change is trivial. Low risk; Claude's discretion per CONTEXT.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | all four items | ✓ | host go | — |
| `make` | TD-04 regen + gate | ✓ (Makefile present) | — | `go run ./cmd/docgen [--check]` directly |

**Missing dependencies:** none. No external tools, services, or network access required. `zig`/CGO release tooling is NOT needed (these are source/test/doc edits buildable with `make build` / `go build`).

## Sources

### Primary (HIGH confidence)
- `internal/daemon/grpc_tcp.go:85-109` + `internal/daemon/grpc_tcp_test.go:27-46` — CR-01 reference pattern + test table [VERIFIED]
- `internal/daemon/telemetry.go:40-47,108-121` + `internal/daemon/telemetry_test.go:18-49` — TD-01 target + existing tests [VERIFIED]
- `internal/cli/setup_clients.go:60-91,95-` + `internal/cli/setup_test.go:17-150` — TD-02 dead helper + live sibling + tests [VERIFIED]
- `internal/cli/nudge.go:158-175,290-406` + `internal/cli/nudge_test.go:174-247` — TD-03 classifier + caller + tests [VERIFIED]
- `internal/kernel/help/tools.go:25-91`, `internal/kernel/help/skill_adapter.go:20-33`, `cmd/docgen/main.go:21-42,108-119`, `Makefile:77-84`, `README.md:332` — TD-04 source, docgen path, gate [VERIFIED]
- `.planning/v2.0-MILESTONE-AUDIT.md` frontmatter `tech_debt:` — canonical item list [CITED]
- Local run: `go build ./...` (exit 0), `go vet ./internal/{cli,daemon}/... ./internal/kernel/help/...` (clean), `go run ./cmd/docgen --check` ("README.md is up to date") [VERIFIED]

### Secondary / Tertiary
- None needed — no external docs or web sources; this is purely in-tree cleanup.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new deps; stdlib + existing packages.
- Architecture: HIGH — every fix mirrors a verified in-tree precedent; anchors read directly.
- Pitfalls: HIGH — each pitfall derived from a concrete contract difference observed in the code (empty-addr vs empty-host; docgen field selection; fail-open contract).

**Research date:** 2026-06-22
**Valid until:** 2026-07-22 (stable internal code; only invalidated if Phases 93/94/95 source is refactored before this phase executes)
