# Phase 99: Vendored Aider Fixtures + Mixed-License Gate - Research

**Researched:** 2026-06-23
**Domain:** Third-party data vendoring + redistribution-compliance license gating (Go, offline bench fixtures, SPDX/NOTICE conventions)
**Confidence:** HIGH (every layout claim verified by live clone of both upstream repos at the pinned SHAs; every gate/loader claim verified by direct in-tree source read)

## Summary

Phase 99 lands a deterministic, offline, **mixed-license** vendored fixture tree under `bench/datasets/aider-polyglot/fixtures/` and extends the existing `make verify-licenses` HARD-FAIL gate to scan it under both MIT (Exercism polyglot subset) and Apache-2.0 (aider edit-format fixtures) dispositions, with an adversarial tamper test. The substrate already exists: Phase 85 shipped `clone.go`/`loader.go`/`pin.go` + `LICENSE-AUDIT.md`, and `cmd/helix-bench/verify_licenses.go` already implements the strict-decode (`KnownFields(true)`) hard-fail gate that mirrors Phase 75's `verify_tos.go`. This phase ADDS committed fixture data + a second license disposition + a manifest + extended scanning — it does NOT rebuild the loader or the gate engine.

**The single most important discovered constraint** is that fixtures are loaded **from disk by relative path** (`loader.go` reads `fixtures/<lang>/exercises/practice/<name>/...`), and the Phase 100 bench will execute the solution/test files verbatim against real language toolchains. Therefore **SPDX headers must NOT be injected inline into fixture source bytes the bench executes** — doing so would alter test inputs (and a `// SPDX...` line is not even valid inside every fixture file type). The gate must verify provenance via a **manifest-driven / sidecar disposition** (an extended `LICENSE-AUDIT.md`-style audit + a `VENDOR-MANIFEST.md`), NOT via inline per-file comment scanning of executable fixtures. This is the deterministic, byte-preserving approach.

Two correction flags from milestone research are now upstream-verified and BINDING: (1) the polyglot fixtures are **MIT** (Exercism redistribution), not Apache-2.0 — the brief's "Apache-2.0 fixtures" conflates the two repos; (2) the polyglot-benchmark repo ships **NO LICENSE file at repo or track root** for 5 of 6 tracks (only `javascript/` retains 49 per-exercise Exercism LICENSE files) — so MIT provenance is established against the `exercism/<lang>@<sha>` source repos (exactly what the existing `LICENSE-AUDIT.md` already does), not against polyglot-benchmark itself.

**Primary recommendation:** Vendor a recorded subset of **9 exercises common to python+go+rust+java** (the Helix first-class LSP languages) from `polyglot-benchmark@7e0611e` (MIT), plus a **small curated subset of aider's `tests/fixtures/languages/*` + one trimmed SEARCH/REPLACE edit-format sample** from `aider@5dc9490` (Apache-2.0). Record exact SHAs + per-file selection in `VENDOR-MANIFEST.md`. Extend the existing audit to a **dual-disposition** structure (MIT track blocks + Apache-2.0 fixture blocks) and extend `verify_licenses.go` to assert the manifest covers every vendored file on disk + every audit block round-trips, with a tamper test that flips/removes an entry and asserts RED.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Vendored fixture data (MIT subset) | Database / Storage (committed tree under `bench/datasets/`) | — | Static offline data; consumed by `loader.go` from disk |
| Vendored edit-format fixtures (Apache-2.0) | Database / Storage | — | Static offline data; consumed by Phase 100 edit bench |
| License audit + SPDX/NOTICE disposition | API/Backend (the `verify-licenses` gate logic in `cmd/helix-bench`) | Database/Storage (the `*.md` audit + manifest files) | Gate is code; the recorded dispositions are data |
| Manifest (subset + pinned SHAs) | Database / Storage (`VENDOR-MANIFEST.md`) | — | Mirrors `pin.go` discipline; the reproducibility contract |
| Tamper test (anti-vacuity) | API/Backend (Go test in `cmd/helix-bench`) | — | Adversarial revert-and-fail proof the gate is real |

This phase is entirely a **storage + offline-tooling** concern. No daemon, no kernel, no LSP, no network at run time (network is needed once, at vendoring time, to clone the pinned SHAs).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
The discuss phase was skipped (`workflow.skip_discuss`); all implementation choices are at Claude's discretion. The following constraints are carried from research (SUMMARY.md / STACK.md / PITFALLS.md) and the user's ratified scope, and are treated as locked:

- **Mixed-license tree (user ratified):** vendor BOTH (VENDOR-01) the MIT-licensed Exercism polyglot fixtures (from `Aider-AI/polyglot-benchmark`, which redistributes Exercism content under MIT — byte-verified in `bench/datasets/aider-polyglot/LICENSE-AUDIT.md`) AND (VENDOR-02) Aider's Apache-2.0 edit-format fixtures (from the `Aider-AI/aider` tool repo). Per-file `SPDX-License-Identifier:` disposition: MIT for the polyglot subset, Apache-2.0 for the aider edit-format fixtures.
- **Vendor a recorded, deterministic SUBSET** (not all 225 tasks / 6 tracks) — record the exact selection in a `VENDOR-MANIFEST.md`. Subset must drive the Phase 100 polyglot edit benchmark + the Phase 102 corpora, but stay lean. Pin upstream commit SHAs (mirror `bench/datasets/aider-polyglot/pin.go`'s pinned-sha + `isHexSHA1` discipline).
- **Reuse, don't rebuild:** Phase 85 adapter (`bench/datasets/aider-polyglot/`) already has clone/loader/pin + a `LICENSE-AUDIT.md`. This phase ADDS a committed `fixtures/` tree + the Apache-2.0 edit-format fixtures + extends the gate. Do NOT rebuild the loader.
- **Extended `make verify-licenses` (VENDOR-03):** the existing hard-fail license gate (Phase 85, mirrors Phase 75 `verify_tos.go` strict-decode discipline) is extended to scan the FULL vendored tree under BOTH dispositions (MIT + Apache-2.0), failing RED on a missing/incorrect SPDX header or NOTICE. Ship an adversarial tamper test (flip/remove a header → gate RED) — anti-vacuity (the v1.12 lesson).
- **Network dependency:** vendoring requires fetching upstream repos at pinned SHAs. Research MUST confirm network/clone availability and the exact upstream layout; if a clone isn't possible the plan surfaces a blocker rather than fabricating fixture content.
- Per-track NOTICE/attribution files preserving the upstream copyright + license text (Exercism per-track licenses are MIT; aider is Apache-2.0 — include the Apache-2.0 NOTICE requirements).

### Claude's Discretion
All implementation choices (exact exercise selection within the recommended subset, exact gate-extension mechanism, manifest schema, disposition mechanism) are at Claude's discretion, guided by ROADMAP success criteria + codebase conventions.

### Deferred Ideas (OUT OF SCOPE)
None — discuss phase skipped. (Note: EDITBENCH-04 "full 6-track/225-task expansion" is a v2 requirement, explicitly deferred in REQUIREMENTS.md — do NOT vendor all six tracks.)
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| VENDOR-01 | MIT Exercism polyglot fixtures (recorded deterministic subset) vendored under `bench/datasets/aider-polyglot/fixtures/` with `SPDX-License-Identifier: MIT`, per-track NOTICE/attribution, and a `VENDOR-MANIFEST.md` | Verified upstream layout of `polyglot-benchmark@7e0611e` (§Upstream Layout — Polyglot); recommended 9-exercise python+go+rust+java subset (§Recommended Subset); MIT provenance from `exercism/<lang>@<sha>` matching existing `LICENSE-AUDIT.md`; byte-preserving disposition (§SPDX Disposition) |
| VENDOR-02 | Aider's Apache-2.0 edit-format fixtures vendored with `SPDX-License-Identifier: Apache-2.0` + attribution, producing a documented mixed-license tree | Verified `aider@5dc9490` LICENSE.txt = Apache-2.0, no root NOTICE; located edit-format fixtures at `tests/fixtures/` (§Upstream Layout — Aider); recommended small curated subset (§Recommended Subset); Apache-2.0 NOTICE obligations (§NOTICE/Attribution) |
| VENDOR-03 | `make verify-licenses` extended to hard-fail over the full vendored tree (dual MIT + Apache-2.0), with a tamper test proving RED on a missing/incorrect SPDX header or NOTICE | Existing `verify_licenses.go` strict-decode engine (§Gate Extension Points); dual-disposition audit schema; manifest-vs-disk coverage check; tamper-test pattern mirroring `verify_licenses_test.go` (§Validation Architecture) |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **Go single binary, native concurrency.** No new runtime deps. This phase adds zero Go dependencies (vendoring is `git clone` at vendor-time + committed data + an extended Go gate).
- **Always run `go vet ./...` and `go test ./...` before completing any Go task.** The gate lives in `cmd/helix-bench`; `make verify-licenses` is the integration target.
- **`bench/` leaf-import boundary (`vet-ablation-leakage`):** the gate is in `cmd/helix-bench` (already a bench-adjacent command), not a `bench/evaluators/*` leaf, so the no-kernel-import leaf rule does not directly bind here — but do not introduce `internal/kernel`/`internal/semantic` imports into the gate.
- **Local-only benches; no CI benchstat gate.** `verify-licenses` is a hard-fail build gate (it runs in CI per the Makefile `.PHONY` set), but it is a pure parse/structural gate (no binary, no network at run time), so it is CI-safe.
- **GSD workflow enforcement:** all edits go through a GSD command (this is plan→execute-phase work).

## Standard Stack

No new libraries. Everything is already in the tree.

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `gopkg.in/yaml.v3` | (in `go.mod`) | strict-decode (`KnownFields(true)`) the audit frontmatter blocks | Already used by `verify_licenses.go` + `verify_tos.go` |
| `github.com/spf13/cobra` | v1.9.1 | the `verify-licenses` subcommand | Already used; the subcommand is NOT mounted on root (preserves `--help` count) |
| stdlib `os`/`path/filepath`/`strings`/`crypto/sha256` | go 1.25.1 | read the vendored tree, walk it, compute content digests | Mirrors `pin.go`/`isHexSHA1` and the existing `license_sha256` provenance |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `git` (CLI, vendor-time only) | system | shallow clone `polyglot-benchmark@7e0611e` + `aider@5dc9490`, copy the subset | One-time, at vendoring; the committed tree is then offline-self-sufficient |

**Installation:** None. `go build ./cmd/helix-bench` + `make verify-licenses` already work; the change is additive Go + committed data.

## Package Legitimacy Audit

> Not applicable — this phase installs **zero** external packages. All code reuses `gopkg.in/yaml.v3`, `cobra`, and stdlib already in `go.mod`. Vendored content is **data** (test fixtures), audited for redistribution license, not executable dependencies. The redistribution audit IS the per-file legitimacy gate for this phase (see Validation Architecture).

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Upstream Layout — Polyglot Benchmark (MIT) [VERIFIED: live clone @ 7e0611e]

`https://github.com/Aider-AI/polyglot-benchmark` @ `7e0611e77b54e2dea774cdc0aa00cf9f7ed6144f` (matches `pin.go` `PinnedSHA`; confirmed reachable via `git ls-remote`).

**Repo root:** only `README.md` + the six track dirs (`cpp go java javascript python rust`). **No repo-root LICENSE.**

**Track layout:** `<lang>/exercises/practice/<exercise-name>/`. Per-track exercise counts (verified):

| Track | Exercises |
|-------|-----------|
| python | 34 |
| rust | 30 |
| go | 39 |
| javascript | 49 |
| java | 47 |
| cpp | 26 |

**Per-exercise shape** (verified across python/go/rust):
- `.meta/config.json` — `files.solution` / `files.test` / `files.example` (+ `authors`, `contributors`, `blurb`, sometimes `source`/`source_url`). This is exactly the `loader.go` `Config` struct's subset.
- `.meta/example.<ext>` — reference solution (the "answer").
- `<exercise>.<ext>` (python: `wordy.py`) OR `src/lib.rs` (rust) OR `<exercise>.go` (go) — the **stub** the agent edits.
- `<exercise>_test.<ext>` / `tests/<exercise>.rs` / `<exercise>_test.go`+`cases_test.go` — the test file(s), restored pristine each attempt (WR-01).
- Rust adds `Cargo.toml`; go adds `go.mod` (+ sometimes `.meta/gen.go`).
- `.docs/`, `.approaches/` — instructional markdown. **NOT needed by the bench** — exclude from the vendored subset to keep the tree lean.

**LICENSE provenance — CRITICAL FINDING [VERIFIED: live clone]:** the polyglot-benchmark repo carries **NO `LICENSE` file at repo root or any track root**. Only the `javascript/` track retains the original Exercism per-exercise `LICENSE` files (49 of them); python/go/rust/java/cpp tracks have **zero** LICENSE files in-tree. The repo's `README.md` "Source Attribution" / "Credits" section points to `github.com/exercism/<lang>` and states all content is "© Exercism … used in accordance with Exercism's open source licenses." The existing `bench/datasets/aider-polyglot/LICENSE-AUDIT.md` already handles this correctly: it records each track's MIT `license_sha256` fetched from `exercism/<lang>@<pinned-sha>/LICENSE` (all six identical, sha256 `e52f804e…df44df`), NOT from polyglot-benchmark. **The vendored NOTICE must cite the `exercism/<lang>@<sha>` source repo for MIT provenance, mirroring the existing audit.** [VERIFIED: in-tree LICENSE-AUDIT.md + live polyglot clone]

## Upstream Layout — Aider Tool Repo (Apache-2.0) [VERIFIED: live clone @ 5dc9490]

`https://github.com/Aider-AI/aider` @ `5dc9490bb35f9729ef2c95d00a19ccd30c26339c` (confirmed reachable; matches additional_context).

- **`LICENSE.txt` at repo root = Apache License 2.0** [VERIFIED]. **No `NOTICE` file at repo root** — so Apache-2.0 §4(d) "if the Work includes a NOTICE file" obligation does not transitively apply (there is no upstream NOTICE to propagate); attribution + a copy of the Apache-2.0 license text + the SPDX disposition is the obligation we satisfy. [VERIFIED: `git ls-tree` shows only `LICENSE.txt`]

**Edit-format fixtures live under `tests/fixtures/`:**

| Path | What it is | Size | Recommendation |
|------|-----------|------|----------------|
| `tests/fixtures/chat-history-search-replace-gold.txt` | The canonical SEARCH/REPLACE edit-format gold transcript (`@@@ SEARCH: <file> @@@` … `@@@@@@` blocks) | **~27,810 lines** | Too large to vendor whole — vendor a **trimmed excerpt** (2–4 representative blocks) OR re-derive native blocks. If vendored verbatim, it is the Apache-2.0 anchor. |
| `tests/fixtures/chat-history.md` | A massive sample chat transcript used by repomap/editblock tests | **~99,961 lines** | Do NOT vendor (oversized; not needed for the edit bench). |
| `tests/fixtures/languages/<lang>/test.<ext>` | 40 small per-language sample source files (python ~26 lines, java ~16, js ~26) used to exercise edit application across languages | small | **Vendor the subset of languages the edit bench targets** (python/go/rust/java/javascript). These are the ideal small Apache-2.0 edit-format targets. |
| `tests/fixtures/sample-code-base/` + `sample-code-base-repo-map.txt` | repomap sample | — | Not needed for Phase 99/100 edit bench; skip. |

**What "edit-format fixtures" means for Phase 100:** aider's editblock tests (`tests/basic/test_editblock.py`, `eb.find_original_update_blocks(edit)`) parse SEARCH/REPLACE blocks out of an LLM reply. The vendored Apache-2.0 material that drives the Phase 100 EDIT-verb bench is best served by **(a) the small `tests/fixtures/languages/*` source files** (concrete targets to apply edits against) **plus (b) a trimmed SEARCH/REPLACE block sample** from the gold file. This keeps the Apache-2.0 footprint tiny and auditable.

> **Recommendation (carry the STACK.md "native re-derivation" option as discretion):** STACK.md preferred re-deriving the drift/edit-format corpus *natively* to avoid mixing licenses. The user has now **ratified** vendoring the Apache-2.0 fixtures (mixed-license tree), so vendor a **minimal** Apache-2.0 subset (the small `languages/*` files + a trimmed gold excerpt) — just enough to anchor the mixed-license gate and the Phase 100 bench — rather than the multi-megabyte gold/chat files.

## Recommended Subset (deterministic, recorded in VENDOR-MANIFEST.md)

### MIT polyglot subset (VENDOR-01)

**9 exercises common to python + go + rust + java** [VERIFIED via `comm` across the four tracks @ 7e0611e]:

```
book-store  bowling  forth  pig-latin  poker  react  two-bucket  variable-length-quantity  wordy
```

(For a 3-language python+go+rust intersection, add `scale-generator` → 10. No exercise is common to all six tracks, so a 4-language intersection is the practical multi-language floor.)

**Recommendation:** vendor these **9 exercises across the 4 Helix first-class LSP languages** (python, go, rust, java) = up to 36 exercise dirs. To keep the tree lean while still "multi-language," a defensible **minimum floor** is the 9 exercises × {python, go, rust} = 27 dirs (rust+go+python are the 3 fully first-class langs with simplest toolchains for the bench), with java reserved for Phase 100 expansion. **Record the exact (language, exercise) matrix in `VENDOR-MANIFEST.md`** so the subset is reproducible and the committed baseline is stable.

**Per-exercise vendored files (exclude `.docs/`, `.approaches/`):** `.meta/config.json`, `.meta/example.<ext>`, the stub (`<ex>.<ext>` / `src/lib.rs`), the test file(s), and toolchain files (`Cargo.toml` / `go.mod`). This matches what `loader.go` reads.

> **Reconcile with existing fixtures:** the two fixtures already in-tree (`python/wordy`, `rust/leap`) are **hand-authored hermetic stubs** (`config.json` `authors:["fixture"]`, `raise NotImplementedError("solve the wordy exercise")`), NOT byte-vendored from upstream. The plan must decide: (a) keep them as separate hermetic loader-test fixtures and vendor the real upstream subset alongside (recommended — preserves `loader_test.go` which hard-codes `fixtures/python/.../wordy`), or (b) replace them with the real upstream `wordy`/`leap`. **Recommendation (a)** — do not break `loader_test.go`'s pinned path; vendor the upstream subset as additional dirs and mark the two pre-existing hermetic stubs distinctly in the manifest (they are repo-authored, "internal — repo license," not Exercism MIT).

### Apache-2.0 aider subset (VENDOR-02)

- `tests/fixtures/languages/{python,go,rust,java,javascript}/test.<ext>` (5 small files), AND
- a **trimmed** `chat-history-search-replace-gold` excerpt (2–4 SEARCH/REPLACE blocks, ≤ ~150 lines) saved as e.g. `edit-format/search-replace-sample.txt`.

Place under a clearly Apache-2.0-dispositioned subdir, e.g. `bench/datasets/aider-polyglot/fixtures/_aider-edit-format/` (leading-underscore to keep it out of the per-language exercise walk in `loader.go`, which globs `<lang>/exercises/practice/`).

## SPDX Disposition — byte-preserving (the load-bearing design decision)

**Constraint [VERIFIED: `loader.go` reads fixtures from disk by relative path; Phase 100 executes them]:** the bench runs the vendored solution/test files against real toolchains (`pytest`, `cargo test`, `go test`, etc.). Injecting `// SPDX-License-Identifier: MIT` into a `.py`/`.go`/`.rs`/`.json` fixture would (a) change the bytes the bench executes, (b) be invalid in some file types (`.json` has no comment syntax; `config.json` is parsed by `loader.go`), and (c) break byte-reproducibility of any committed baseline. **Therefore: do NOT stamp inline SPDX headers into executable fixture files.**

**Recommended deterministic, byte-preserving disposition:**

1. **Manifest + audit-based disposition (primary).** Record every vendored file's license in two committed text artifacts the gate strict-decodes:
   - **`VENDOR-MANIFEST.md`** — the reproducibility record: upstream repo URL + pinned SHA + the exact (language, exercise) / (fixture path) selection + per-file `sha256` content digest (mirrors `pin.go` + `license_sha256` provenance discipline). This is the "which bytes, from which commit" contract.
   - **Extended `LICENSE-AUDIT.md`** (or a sibling `VENDOR-AUDIT.md`) — the per-disposition license record: MIT track blocks (already present) + new Apache-2.0 fixture blocks, each strict-decoded by the gate.
2. **NOTICE sidecars (attribution).** Per-track `NOTICE` (MIT) under `fixtures/<lang>/NOTICE` citing `exercism/<lang>@<sha>` + the MIT copyright/permission notice; an `fixtures/_aider-edit-format/NOTICE` (Apache-2.0) + a copy of the Apache-2.0 `LICENSE` text. NOTICE/LICENSE files are NOT executed by the bench, so they carry the human-readable attribution without touching fixture bytes.
3. **Optional inline SPDX only where safe.** If inline SPDX is desired for the **non-executed** files (NOTICE sidecars, the trimmed Apache-2.0 edit-format `.txt` sample, README-style docs), it is fine to add a comment line there — but never to the solution/test/config files the loader feeds the toolchain.

> **`SPDX-License-Identifier:` semantics for the gate:** the gate verifies the *recorded disposition* (manifest entry + audit block + NOTICE presence) for each vendored file — NOT an inline comment scan of executable fixtures. "Missing/incorrect SPDX header" in VENDOR-03 is satisfied by "missing/incorrect manifest+audit disposition," which is deterministically checkable without mutating fixture bytes. This is the only design that keeps the Phase 100 bench reproducible.

## Architecture Patterns

### System Data Flow

```
VENDOR-TIME (once, network):
  git clone --depth1 polyglot-benchmark@7e0611e ──┐
  git clone --depth1 aider@5dc9490 ───────────────┤
                                                   ▼
              copy recorded subset (no .docs/.approaches)
                                                   ▼
        bench/datasets/aider-polyglot/fixtures/   (committed, offline)
          ├── <lang>/exercises/practice/<ex>/...        [MIT]
          ├── <lang>/NOTICE                              [MIT attribution → exercism/<lang>@sha]
          └── _aider-edit-format/...                     [Apache-2.0]
              └── NOTICE + LICENSE (Apache-2.0 text)
                                                   ▼
        compute sha256 per file ──► VENDOR-MANIFEST.md  (SHA + selection)
        record license disposition ─► LICENSE-AUDIT.md  (MIT blocks + Apache-2.0 blocks)

RUN-TIME (offline, CI-safe):
  make verify-licenses
    └─► go run ./cmd/helix-bench verify-licenses <audit> [--manifest <manifest> --tree <fixtures>]
          ├─ strict-decode every audit block (KnownFields(true))      [existing]
          ├─ assert MIT blocks + Apache-2.0 blocks both present       [NEW: dual disposition]
          ├─ walk fixtures tree; assert every file is covered by a    [NEW: manifest-vs-disk]
          │    manifest entry with matching sha256 + a license block
          └─ HARD-FAIL (non-zero) on: missing/extra file, sha mismatch,
               missing disposition, empty/zero blocks                 [extends fail-closed engine]

  go test ./cmd/helix-bench/...
    └─ TAMPER TEST: flip a license / drop a manifest entry / mutate a
         fixture byte → assert verifyLicenses returns non-nil error   [NEW: anti-vacuity]
```

### Recommended Structure

```
bench/datasets/aider-polyglot/
├── fixtures/
│   ├── python/exercises/practice/<ex>/...        # MIT (vendored subset)
│   ├── go/exercises/practice/<ex>/...            # MIT
│   ├── rust/exercises/practice/<ex>/...          # MIT
│   ├── java/exercises/practice/<ex>/...          # MIT (optional, Phase-100 expansion)
│   ├── python/NOTICE  go/NOTICE  rust/NOTICE      # MIT per-track attribution
│   └── _aider-edit-format/                        # Apache-2.0 subtree (underscore = excluded from loader glob)
│       ├── languages/{python,go,rust,java,javascript}/test.*
│       ├── search-replace-sample.txt
│       ├── NOTICE
│       └── LICENSE                                # Apache-2.0 text copy
├── VENDOR-MANIFEST.md                             # NEW: SHA + subset selection (mirrors pin.go)
├── LICENSE-AUDIT.md                               # EXTENDED: MIT track blocks + Apache-2.0 blocks
├── clone.go loader.go pin.go                      # UNCHANGED (reuse)
└── ...
```

### Pattern: strict-decode hard-fail gate (existing — extend, don't replace)
**What:** the gate splits the audit `.md` on `---` horizontal rules, pre-filters segments with a top-level key, strict-decodes each (`KnownFields(true)` → unknown key errors), and fails closed on empty fields / zero blocks.
**When to use:** extend it to (a) recognize a second block kind for Apache-2.0 fixtures, and (b) cross-check the manifest against the on-disk tree.
**Example (existing engine, from `cmd/helix-bench/verify_licenses.go`):**
```go
// Source: in-tree cmd/helix-bench/verify_licenses.go:62-88
dec := yaml.NewDecoder(strings.NewReader(seg))
dec.KnownFields(true) // unknown key → error → exit non-zero
var tl TrackLicense
if err := dec.Decode(&tl); err != nil {
    return tracks, fmt.Errorf("verify-licenses: strict decode of a track block in %s failed: %w", path, err)
}
if tl.License == "" { /* fail closed */ }
if tl.LicenseSHA256 == "" { /* fail closed */ }
```

### Anti-Patterns to Avoid
- **Inline SPDX comments in executed fixture files** — mutates bench inputs, breaks byte-reproducibility, invalid in `.json`. Use manifest/audit/NOTICE disposition instead.
- **Vendoring all 6 tracks / 225 tasks** — bloated, unauditable; v2 EDITBENCH-04, explicitly deferred. Vendor the recorded subset only.
- **Vendoring the multi-megabyte aider gold/chat fixtures whole** — vendor a trimmed excerpt + the small `languages/*` files only.
- **Sourcing MIT provenance from polyglot-benchmark** — it has no LICENSE for 5/6 tracks; source from `exercism/<lang>@<sha>` (as the existing audit already does).
- **A green-only gate** — every gate this milestone adds MUST ship a break-the-invariant → assert-RED test (the v1.12 four-CRITICAL lesson).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Strict YAML frontmatter decode | A bespoke parser | existing `verify_licenses.go` + `yaml.v3 KnownFields(true)` + `splitOnHorizontalRules` | Already shipped, already tamper-tested |
| Pinned-SHA hex validation | A regexp | existing `pin.go` `isHexSHA1` | Explicit, total, no-regexp guard already in-tree |
| Content-digest provenance | A new hashing scheme | `crypto/sha256` + the `license_sha256` convention | Mirrors the existing audit's digest discipline |
| Fixture loading | A new loader for vendored files | existing `loader.go` `Config`/`RunExercise` | The vendored tree is shaped to match `loader.go`'s `<lang>/exercises/practice/<ex>/.meta/config.json` |
| Cobra subcommand wiring | A new CLI surface | extend `newVerifyLicensesCmd` (NOT mounted on root, preserves `--help` count) | The `make verify-licenses` target already invokes it |

**Key insight:** Phase 99 is ~80% data curation + ~20% extending a gate that already exists and is already tamper-tested. The risk is not "how to build a gate" — it's "does the extended gate actually go RED on tampered/missing vendored content," which is the anti-vacuity tamper test.

## Runtime State Inventory

> This is a vendoring/data-addition phase (no rename), but the "what state besides files" discipline still applies to the gate + loader coupling.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | The committed `fixtures/` tree (currently 2 hermetic stubs: `python/wordy`, `rust/leap`). Loader reads by relative path from disk. | Add vendored subset dirs; decide whether to keep or replace the 2 hermetic stubs (recommend KEEP — `loader_test.go` hard-codes `fixtures/python/.../wordy`). |
| Live service config | None — no daemon/external service touches this; gate runs offline in CI + `make`. | None. |
| OS-registered state | None. | None. |
| Secrets/env vars | None. The clone is anonymous public GitHub; no token. (`HELIX_BIN` is for bench surfaces, not this gate.) | None. |
| Build artifacts | `make verify-licenses` target (Makefile `.PHONY` list, line ~331); `cmd/helix-bench` binary; CI step invoking the target. | Extend the target/args if the gate gains `--manifest`/`--tree` flags; update the Makefile comment block to describe dual disposition. |

**Coupling to verify:** `loader_test.go` hard-codes `filepath.Join("fixtures","python","exercises","practice","wordy")` and asserts `Files.Solution == [wordy.py]`. Any change to the existing `wordy` fixture breaks this test. **Vendor alongside, don't overwrite.** The `_aider-edit-format/` underscore-prefixed dir keeps the Apache-2.0 subtree out of `loader.go`'s per-language exercise glob — verify the glob/walk does not pick it up (it walks `<lang>/exercises/practice/`, so an underscore-prefixed top-level sibling is safe).

## Common Pitfalls

### Pitfall 1: SPDX header mutates bench inputs (byte-reproducibility break)
**What goes wrong:** stamping `// SPDX...` into `.py`/`.go`/`.rs`/`.json` fixtures changes the bytes Phase 100 executes and breaks any committed baseline; `.json` can't even hold a comment.
**Why it happens:** "per-file SPDX header" reads as "inline comment in every file."
**How to avoid:** manifest + audit + NOTICE disposition (see §SPDX Disposition); inline SPDX only on non-executed sidecar/doc files.
**Warning signs:** a diff that touches a fixture solution/test/config file's content; `loader_test.go` or a Phase 100 baseline diff turns red after "adding headers."

### Pitfall 2: Wrong license stamped (Apache-2.0 on MIT fixtures)
**What goes wrong:** the brief says "Apache-2.0 fixtures"; the polyglot fixtures are MIT. Stamping Apache-2.0 on them is non-compliant.
**Why it happens:** "Aider" conflates the tool repo (Apache-2.0) with the fixtures repo (Exercism MIT).
**How to avoid:** MIT disposition + NOTICE → `exercism/<lang>@<sha>` for polyglot; Apache-2.0 disposition + NOTICE + LICENSE copy ONLY for the `_aider-edit-format/` subtree. [VERIFIED: both upstream LICENSEs read at pinned SHAs]
**Warning signs:** any Apache-2.0 disposition on a file under `<lang>/exercises/practice/`.

### Pitfall 3: Gate is vacuous (green-only)
**What goes wrong:** the extended gate passes whether or not the vendored tree is correctly dispositioned; a tampered/removed header still passes.
**Why it happens:** authors write the happy-path test only (the v1.12 four-CRITICAL pattern).
**How to avoid:** ship the tamper test — flip a license value, drop a manifest entry, mutate a fixture byte (sha mismatch), remove a NOTICE — each MUST make `verifyLicensesCount` return a non-nil error. Mirror the existing `TestVerifyLicenses_EmptyLicenseFailsClosed` / `_UnknownKeyFailsStrictDecode` / `_ZeroTracksIsError` shape.
**Warning signs:** no test in `verify_licenses_test.go` is *expected* to be RED on a sabotaged vendored tree; coverage of the manifest-vs-disk cross-check is absent.

### Pitfall 4: Oversized / unauditable tree
**What goes wrong:** vendoring all six tracks or the multi-MB aider gold/chat files bloats the diff and makes the NOTICE unauditable.
**How to avoid:** recorded 9-exercise × {python,go,rust(,java)} subset + the small `languages/*` + trimmed gold excerpt; everything enumerated in `VENDOR-MANIFEST.md`.
**Warning signs:** `chat-history.md` (~100k lines) or the full `*-gold.txt` (~28k lines) appears in the tree; exercise dirs the bench never references.

### Pitfall 5: Manifest drifts from disk
**What goes wrong:** a file is added/removed under `fixtures/` but the manifest isn't updated, so the committed baseline + license audit silently diverge.
**How to avoid:** the gate's manifest-vs-disk walk is bidirectional — every on-disk vendored file must have a manifest entry with a matching sha256, AND every manifest entry must exist on disk. Both directions hard-fail.
**Warning signs:** the gate passes after you `rm` a fixture file or add one without a manifest row.

## Code Examples

### Extending the audit struct for a second disposition (sketch)
```go
// Source: extends in-tree cmd/helix-bench/verify_licenses.go TrackLicense
// MIT track block (existing) — keep as-is.
type TrackLicense struct {
    Track         string `yaml:"track"`
    SourceRepo    string `yaml:"source_repo"`
    License       string `yaml:"license"`          // "MIT"
    LicenseSHA256 string `yaml:"license_sha256"`
    RedistributionClauseExcerpt string `yaml:"redistribution_clause_excerpt"`
}
// NEW: Apache-2.0 fixture block — distinguished by a top-level `fixture:` key
// (so hasTopLevelFixtureKey pre-filters it, parallel to hasTopLevelTrackKey).
type FixtureLicense struct {
    Fixture     string `yaml:"fixture"`           // e.g. "_aider-edit-format"
    SourceRepo  string `yaml:"source_repo"`       // Aider-AI/aider@5dc9490...
    License     string `yaml:"license"`           // "Apache-2.0"
    NoticePath  string `yaml:"notice_path"`       // fixtures/_aider-edit-format/NOTICE
    LicensePath string `yaml:"license_path"`      // fixtures/_aider-edit-format/LICENSE
}
```

### Manifest-vs-disk coverage (the new fail-closed check, sketch)
```go
// Walk the vendored tree; every file must be covered by a manifest entry whose
// recorded sha256 matches the on-disk bytes. Fail closed on miss/mismatch.
err := filepath.WalkDir(treeRoot, func(p string, d fs.DirEntry, err error) error {
    if err != nil || d.IsDir() { return err }
    sum := sha256Hex(mustRead(p))
    entry, ok := manifest[rel(p)]
    if !ok { return fmt.Errorf("verify-licenses: vendored file %s has no manifest entry", p) }
    if entry.SHA256 != sum { return fmt.Errorf("verify-licenses: %s sha256 mismatch (tamper)", p) }
    return nil
})
// And the reverse: every manifest entry must exist on disk.
```

### Tamper test (anti-vacuity, mirrors existing test shape)
```go
// Source: mirrors cmd/helix-bench/verify_licenses_test.go pattern
func TestVerifyLicenses_TamperedFixtureFailsClosed(t *testing.T) {
    tree := vendorTempTree(t)                 // good MIT+Apache tree + manifest + audit
    // flip one byte in a vendored fixture so its sha256 no longer matches the manifest
    mutateOneByte(t, filepath.Join(tree, "python/exercises/practice/wordy/wordy.py"))
    if _, err := verifyVendored(tree); err == nil {
        t.Fatalf("expected RED on tampered fixture, got nil (vacuous gate)")
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Clone-at-runtime, network-gated, skips offline | Vendored committed subset, offline-self-sufficient | This phase (B1/99) | Offline-reproducible Phase 100 baseline |
| Single MIT disposition (cloned tracks) | Dual MIT + Apache-2.0 disposition over the vendored tree | This phase (VENDOR-03) | Mixed-license tree, gate scans both |
| `license_sha256` per track only | + per-file content `sha256` in `VENDOR-MANIFEST.md` | This phase | Tamper detection on individual vendored bytes |

**Deprecated/outdated:**
- The milestone-brief claim "Aider's fixtures are Apache-2.0" — corrected: polyglot fixtures are MIT; only the aider tool repo (and its `tests/fixtures/*`) is Apache-2.0. [VERIFIED at both pinned SHAs]

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The 4-language (python+go+rust+java) 9-exercise subset is enough to drive Phase 100's edit bench + Phase 102 corpora | Recommended Subset | If Phase 100 needs more languages/exercises, expand the manifest (cheap, additive). Floor is defensible (multi-language, real upstream). |
| A2 | `loader.go`'s per-language walk globs only `<lang>/exercises/practice/` and will not pick up the underscore-prefixed `_aider-edit-format/` sibling | SPDX Disposition / Runtime State | If the loader walks the whole `fixtures/` root, the Apache subtree could be mis-loaded as an exercise. **Plan must verify the loader's walk root before placing the Apache subtree.** (Verifiable by reading `loader.go`'s discovery code — flagged for the planner.) |
| A3 | Keeping the 2 pre-existing hermetic stubs (`wordy`,`leap`) alongside the vendored upstream subset is preferable to replacing them | Recommended Subset / Runtime State | If replaced, `loader_test.go`'s hard-coded `fixtures/python/.../wordy` path + `Files.Solution==[wordy.py]` assertions break. Recommendation avoids that. |
| A4 | A trimmed 2–4-block excerpt of the SEARCH/REPLACE gold file is sufficient Apache-2.0 edit-format anchor for the bench | Recommended Subset (Apache) | If Phase 100 needs the full gold transcript, vendor more (still Apache-2.0, still auditable). |

**These are LOW/MEDIUM-confidence planning choices, not facts.** Every upstream layout + license fact in this document is `[VERIFIED]` by live clone. The assumptions are about *scope selection*, resolvable at plan time.

## Open Questions

1. **Does `loader.go`'s exercise-discovery walk start at `fixtures/<lang>/` or at `fixtures/`?**
   - What we know: `loader_test.go` references `fixtures/python/exercises/practice/wordy` explicitly; the loader's `Config` decode is per-exercise.
   - What's unclear: whether any code enumerates all of `fixtures/` (which would see `_aider-edit-format/`).
   - Recommendation: the planner reads `loader.go`'s discovery function first; if it walks `fixtures/` root, place the Apache subtree OUTSIDE `fixtures/` (e.g. `bench/datasets/aider-polyglot/edit-format-fixtures/`) instead of an underscore sibling.

2. **Keep vs replace the 2 hermetic stubs?**
   - Recommendation: KEEP (preserves `loader_test.go`); mark them in the manifest as repo-authored hermetic fixtures (not Exercism MIT), distinct from the vendored upstream subset.

3. **Trimmed gold excerpt vs full gold file for Apache-2.0 anchor?**
   - Recommendation: trimmed excerpt (≤~150 lines) + the small `languages/*` files; expand only if Phase 100 demands.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `git` (clone at pinned SHA) | Vendoring step (one-time) | ✓ | system git; network reachable [VERIFIED `git ls-remote` both repos] | None needed — confirmed reachable |
| Network to github.com | Vendoring step (one-time) | ✓ | — [VERIFIED: both SHAs resolved] | If unreachable at execute time, surface as a BLOCKER (do not fabricate fixtures) |
| Go toolchain | gate build + tests | ✓ | go 1.25.1 | — |
| `gopkg.in/yaml.v3`, `cobra` | gate | ✓ (in go.mod) | — | — |

**Missing dependencies with no fallback:** none (network confirmed reachable this session).
**Missing dependencies with fallback:** none. **Note:** if the execute-time environment loses network, the clone step blocks — the plan must gate the vendoring step on a reachability check and surface a blocker rather than fabricate.

## Validation Architecture

> `workflow.nyquist_validation` is not disabled — this section is included. Test framework = Go `testing` (no external runner). Gate target = `make verify-licenses`.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib) |
| Config file | none (go test) |
| Quick run command | `go test ./cmd/helix-bench/...` |
| Full suite command | `go test ./...` |
| Gate command | `make verify-licenses` (→ `go run ./cmd/helix-bench verify-licenses ...`) |

### Phase Requirements → Test Map (4 success criteria)
| SC | Behavior | Test Type | Automated Command | File Exists? |
|----|----------|-----------|-------------------|-------------|
| SC1 (VENDOR-01) | MIT subset present, shaped for `loader.go`, with `VENDOR-MANIFEST.md` + per-track NOTICE | unit + gate | `go test ./bench/datasets/aider-polyglot/... -run Loader` ; `make verify-licenses` | ❌ Wave 0 (manifest, vendored dirs, NOTICEs) |
| SC2 (VENDOR-02) | Apache-2.0 edit-format subset present + NOTICE + LICENSE copy; mixed-license tree documented | gate | `make verify-licenses` (asserts an Apache-2.0 block exists) | ❌ Wave 0 (Apache subtree + audit block) |
| SC3 (VENDOR-03) | Gate hard-fails over the FULL vendored tree under both dispositions (manifest-vs-disk + audit) | gate | `make verify-licenses` returns 0 on good tree; `go test ./cmd/helix-bench/... -run VerifyLicenses` | ❌ Wave 0 (extend `verify_licenses.go`) |
| SC4 (anti-vacuity) | Tamper (flip/remove a header/NOTICE, mutate a fixture byte, drop a manifest entry) → gate RED | unit (adversarial) | `go test ./cmd/helix-bench/... -run Tamper` | ❌ Wave 0 (new tamper tests) |

### Sampling Rate
- **Per task commit:** `go vet ./... && go test ./cmd/helix-bench/...`
- **Per wave merge:** `go test ./...` + `make verify-licenses`
- **Phase gate:** `make verify-licenses` green on the good tree AND every tamper test RED on its sabotaged input, before `/gsd-verify-work`.

### Anti-vacuity tamper test (the load-bearing SC4 proof)
Each of these MUST cause `verifyLicensesCount`/`verifyVendored` to return a non-nil error (mirror the existing `*FailsClosed` test shape):
- flip a `license:` value (MIT→GPL) in an audit block → RED
- delete a manifest entry for an on-disk file → RED ("no manifest entry")
- mutate one byte of a vendored fixture → sha256 mismatch → RED
- remove a per-track `NOTICE` or the Apache-2.0 `LICENSE` copy → RED ("missing notice/license path")
- zero Apache-2.0 blocks (regression to single-disposition) → RED (dual-disposition assertion)
A reverse control: the unmodified good tree → exit 0 (so the gate isn't RED-on-everything).

### Wave 0 Gaps
- [ ] `bench/datasets/aider-polyglot/fixtures/<lang>/exercises/practice/<ex>/...` — vendored MIT subset (covers VENDOR-01)
- [ ] `bench/datasets/aider-polyglot/fixtures/<lang>/NOTICE` — MIT per-track attribution → `exercism/<lang>@<sha>`
- [ ] `bench/datasets/aider-polyglot/fixtures/_aider-edit-format/` (or sibling outside `fixtures/`) — Apache-2.0 subset + NOTICE + LICENSE (covers VENDOR-02)
- [ ] `bench/datasets/aider-polyglot/VENDOR-MANIFEST.md` — subset selection + pinned SHAs + per-file sha256 (covers VENDOR-01)
- [ ] Extended `bench/datasets/aider-polyglot/LICENSE-AUDIT.md` — MIT track blocks (exist) + new Apache-2.0 fixture blocks
- [ ] Extended `cmd/helix-bench/verify_licenses.go` — dual disposition + manifest-vs-disk walk + fail-closed (covers VENDOR-03)
- [ ] New tamper tests in `cmd/helix-bench/verify_licenses_test.go` (covers SC4 anti-vacuity)
- [ ] `Makefile` `verify-licenses` target/comment updated for dual disposition (+ any `--manifest`/`--tree` flags)

## Security Domain

> `security_enforcement` not disabled — included.

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | anonymous public clone; no creds |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | yes | strict-decode (`KnownFields(true)`) of audit blocks; `isHexSHA1` SHA validation; sha256 content-digest verification of vendored bytes |
| V6 Cryptography | yes (integrity, not secrecy) | `crypto/sha256` for content-digest provenance — standard lib, never hand-rolled |
| V12 Files & Resources | yes | path-segment containment on fixture load (reuse existing `validatePathSegment`/`isHexSHA1` guards, Phase 84/85 precedent); no path traversal out of the fixtures dir |

### Known Threat Patterns for vendored-data + gate
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Mutable-ref tampering (upstream moves a branch under us) | Tampering | Pin to 40-hex SHA via `isHexSHA1`; clone fetches THAT sha (existing `pin.go` discipline); manifest records the sha |
| Silently swapped fixture bytes | Tampering | Per-file `sha256` in `VENDOR-MANIFEST.md`; gate fails closed on mismatch |
| Wrong-license redistribution | Repudiation / legal | Dual-disposition audit + per-track NOTICE + Apache-2.0 LICENSE copy; hard-fail gate + tamper test |
| Path traversal on fixture load | Tampering | Reuse existing `validatePathSegment` containment (Phase 84/85) |
| Vacuous gate (fails open) | Tampering / repudiation | Adversarial tamper test asserts RED on every sabotage class (the v1.12 lesson) |

## Sources

### Primary (HIGH confidence)
- **Live clone `Aider-AI/polyglot-benchmark@7e0611e77b54e2dea774cdc0aa00cf9f7ed6144f`** — track layout, per-exercise shape, exercise counts (py34/rs30/go39/js49/java47/cpp26), cross-track intersections (9 common to py+go+rust+java), and the critical "no LICENSE file at repo/track root for 5/6 tracks; 49 per-exercise LICENSEs only in javascript/" finding.
- **Live clone `Aider-AI/aider@5dc9490bb35f9729ef2c95d00a19ccd30c26339c`** — `LICENSE.txt` = Apache-2.0, no root NOTICE; edit-format fixtures at `tests/fixtures/` (`chat-history-search-replace-gold.txt` ~27,810 lines; `chat-history.md` ~99,961 lines; `tests/fixtures/languages/*` 40 small files); editblock test usage (`eb.find_original_update_blocks`).
- In-tree direct read: `cmd/helix-bench/verify_licenses.go` + `verify_licenses_test.go` (the strict-decode hard-fail engine + existing fail-closed tests), `bench/datasets/aider-polyglot/{pin.go,loader.go,LICENSE-AUDIT.md}`, `bench/LICENSES.md`, `Makefile` (verify-licenses/verify-tos targets), existing `fixtures/` tree (2 hermetic stubs).
- `git ls-remote` both repos — network reachable; both pinned SHAs resolve to `refs/heads/main` HEAD.

### Secondary (MEDIUM confidence)
- Subset-size recommendations (9 exercises × 3–4 langs; trimmed gold excerpt) — derived from "lean but multi-language" intent + Phase 100/102 needs; resolvable at plan time.
- "Place Apache subtree under `_aider-edit-format/` to dodge the loader glob" — depends on `loader.go`'s walk root (Open Question 1).

### Tertiary (LOW confidence)
- None — all factual claims verified this session.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new deps; all reuse verified in `go.mod` + in-tree gate.
- Upstream layout (both repos): HIGH — live clone at the exact pinned SHAs.
- License facts (MIT polyglot / Apache-2.0 aider): HIGH — both LICENSEs read at pinned SHAs; matches in-tree byte-verified audit.
- SPDX/byte-preserving disposition: HIGH — driven by the verified disk-load + bench-execution constraint.
- Subset selection + loader-walk-root: MEDIUM — planning choices flagged in Assumptions Log / Open Questions.

**Research date:** 2026-06-23
**Valid until:** ~2026-07-23 (stable; pinned SHAs are immutable, so layout facts do not expire — only re-pinning would change them).
