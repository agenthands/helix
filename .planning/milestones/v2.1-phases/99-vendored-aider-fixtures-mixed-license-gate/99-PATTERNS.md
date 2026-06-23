# Phase 99: Vendored Aider Fixtures + Mixed-License Gate - Pattern Map

**Mapped:** 2026-06-23
**Files analyzed:** 7 (1 MODIFY gate, 1 MODIFY Makefile, 5 NEW data/test artifacts)
**Analogs found:** 7 / 7 (all in-tree; this phase is ~80% data curation + ~20% extending an existing, already-tamper-tested gate)

> **Load-bearing constraint (repeat to planner):** SPDX headers MUST NOT be inlined into executed fixture bytes (`.py`/`.go`/`.rs`/`.json`). The bench (`loader.go` → Phase 100) reads these files from disk and runs them against real toolchains; a `// SPDX...` line mutates inputs and is invalid in `.json`. Provenance is verified via **manifest sha256 + audit block + NOTICE sidecar**, never an inline-comment scan of executable fixtures. Inline SPDX is allowed ONLY on non-executed sidecars (NOTICE, LICENSE copy, the trimmed `.txt` gold excerpt).

> **Open Question 1 — RESOLVED by reading `loader.go`:** there is **NO discovery walk** in the loader. `loadExercise(dir, language)` (loader.go:96) is handed an **explicit per-exercise directory**; nothing in `loader.go`/`loader_test.go` enumerates `fixtures/` or any `<lang>/` root (verified: only one `fixtures` reference in non-test `.go`, and it is a comment — loader.go:320). Therefore an underscore-prefixed sibling `fixtures/_aider-edit-format/` is **safe** — the loader will never pick it up. Placing the Apache-2.0 subtree either under `fixtures/_aider-edit-format/` OR fully outside `fixtures/` both work; underscore-sibling is fine. (Assumption A2 confirmed.)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `cmd/helix-bench/verify_licenses.go` (MODIFY) | gate / CLI subcommand | transform + file-I/O (parse + walk + digest) | self (current `verify_licenses.go`) + `cmd/helix-bench/verify_tos.go` | exact (extend in place) |
| `cmd/helix-bench/verify_licenses_test.go` (MODIFY: add tamper tests) | test (adversarial) | request-response | existing `*FailsClosed` tests in same file | exact |
| `bench/datasets/aider-polyglot/VENDOR-MANIFEST.md` (NEW) | config / data (reproducibility record) | batch (per-file digest table) | `pin.go` (PinnedSHA/isHexSHA1) + `bench/LICENSES.md` table | role-match |
| `bench/datasets/aider-polyglot/LICENSE-AUDIT.md` (MODIFY: add Apache-2.0 blocks) | config / data (license disposition) | batch (`---`-delimited blocks) | self (existing MIT track blocks) | exact |
| `bench/datasets/aider-polyglot/fixtures/<lang>/exercises/practice/<ex>/...` (NEW, MIT) | data (vendored fixtures) | file-I/O (read by loader) | existing hermetic stubs `fixtures/python/.../wordy`, `fixtures/rust/.../leap` | exact (shape match) |
| `fixtures/<lang>/NOTICE` + `fixtures/_aider-edit-format/{NOTICE,LICENSE}` (NEW) | data (attribution sidecars) | file-I/O | existing `LICENSE-AUDIT.md` provenance prose | role-match |
| `Makefile` `verify-licenses` target (MODIFY) | config / build | request-response | existing `verify-licenses` + `verify-tos` targets | exact |

## Pattern Assignments

### `cmd/helix-bench/verify_licenses.go` (gate, extend-in-place) — covers VENDOR-03

**Analog:** itself + sibling `cmd/helix-bench/verify_tos.go` (same package `main`, same strict-decode discipline).

**Imports pattern** (`verify_licenses.go:1-10`) — extend with `io/fs`, `path/filepath`, `crypto/sha256`, `encoding/hex` for the new walk; keep `cobra` + `yaml.v3`:
```go
package main
import (
	"fmt"
	"os"
	"strings"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)
```

**Core strict-decode block-parse engine — REUSE, do not rebuild** (`verify_licenses.go:51-88`). This is the existing MIT-track loop. Pattern to copy for the NEW Apache-2.0 `FixtureLicense` block kind:
```go
segments := splitOnHorizontalRules(string(data))   // shared primitive, verify_tos.go:111
tracks := 0
for _, seg := range segments {
	if !hasTopLevelTrackKey(seg) { continue }      // cheap pre-filter, verify_licenses.go:95
	dec := yaml.NewDecoder(strings.NewReader(seg))
	dec.KnownFields(true)                           // unknown key → error → exit non-zero
	var tl TrackLicense
	if err := dec.Decode(&tl); err != nil {
		return tracks, fmt.Errorf("verify-licenses: strict decode of a track block in %s failed: %w", path, err)
	}
	if tl.Track == "" { continue }
	tracks++
	if tl.License == "" { /* fail closed */ }
	if tl.LicenseSHA256 == "" { /* fail closed */ }
}
if tracks == 0 { return 0, fmt.Errorf("verify-licenses: %s contains no track license blocks", path) }
```

**Shared parse primitives — REUSE verbatim (already in `verify_tos.go`, package-shared):**
- `splitOnHorizontalRules(s string) []string` (`verify_tos.go:111-128`) — splits on lines that are exactly `---`, treating every rule as a separator (immune to stray markdown rules). Used by both gates already.
- `hasTopLevelTrackKey(seg)` (`verify_licenses.go:95-103`) — pre-filter for non-indented `track:`. **Add a parallel `hasTopLevelFixtureKey`** for the Apache-2.0 `fixture:` block kind (mirror it line-for-line, swapping `"track:"`→`"fixture:"`).

**NEW struct to add (second disposition)** — mirror `TrackLicense` (`verify_licenses.go:15-21`):
```go
type FixtureLicense struct {
	Fixture     string `yaml:"fixture"`       // e.g. "_aider-edit-format"
	SourceRepo  string `yaml:"source_repo"`   // Aider-AI/aider@5dc9490...
	License     string `yaml:"license"`       // "Apache-2.0"
	NoticePath  string `yaml:"notice_path"`
	LicensePath string `yaml:"license_path"`
}
```

**NEW manifest-vs-disk walk (fail-closed, bidirectional)** — no existing in-tree walk to copy; build from stdlib mirroring `pin.go`'s digest discipline. Read each file, `sha256` it, cross-check against the manifest map; AND assert every manifest entry exists on disk. Reuse `validatePathSegment` containment idea from `loader.go:81-89`. Sketch in RESEARCH §Code Examples (lines 336-349).

**CLI wiring — extend `newVerifyLicensesCmd`, do NOT mount on root** (`verify_licenses.go:110-133`). It is intentionally not added to the root tree (preserves the `--help` subcommand count — same rule as `newVerifyTOSCmd`). Add optional `--manifest` / `--tree` flags here; keep the positional audit-path arg + default `bench/datasets/aider-polyglot/LICENSE-AUDIT.md`.

---

### `cmd/helix-bench/verify_licenses_test.go` (tamper tests) — covers SC4 anti-vacuity

**Analog:** the existing `*FailsClosed` tests in the SAME file.

**Helper pattern** (`verify_licenses_test.go:11-19`) — `writeTempAudit` writes a temp `.md` and returns the path. Add a `vendorTempTree(t)` that materializes a good MIT+Apache tree + manifest + audit in `t.TempDir()`.

**Good-path control** (`verify_licenses_test.go:41-50`) — `TestVerifyLicenses_GoodFileReturnsCount` asserts nil error + exact count. Mirror with a good-tree-returns-0 control so the gate is not RED-on-everything.

**Fail-closed shape to mirror** (`verify_licenses_test.go:52-109`) — each test feeds a sabotaged input and asserts `err != nil`. Copy this exact shape for the required tamper cases (RESEARCH lines 437-444):
```go
func TestVerifyLicenses_TamperedFixtureFailsClosed(t *testing.T) {
	tree := vendorTempTree(t)
	mutateOneByte(t, filepath.Join(tree, "python/exercises/practice/wordy/wordy.py"))
	if _, err := verifyVendored(tree); err == nil {
		t.Fatalf("expected RED on tampered fixture, got nil (vacuous gate)")
	}
}
```
Required RED cases: (1) flip `license:` MIT→GPL in an audit block; (2) delete a manifest entry for an on-disk file; (3) mutate one fixture byte (sha256 mismatch); (4) remove a per-track NOTICE / the Apache-2.0 LICENSE copy; (5) zero Apache-2.0 blocks (dual-disposition regression).

**Committed-artifact passes** (`verify_licenses_test.go:111-124`) — `TestVerifyLicenses_CommittedAuditPasses` reads the REAL `../../bench/datasets/aider-polyglot/LICENSE-AUDIT.md` and asserts it passes with `n>0`. Add a sibling that runs the full vendored-tree gate against the real committed tree + manifest.

---

### `bench/datasets/aider-polyglot/VENDOR-MANIFEST.md` (NEW) — covers VENDOR-01

**Analogs:** `pin.go` (the SHA discipline) + `bench/LICENSES.md` (the markdown audit-table shape).

**Pinned-SHA discipline to mirror** (`pin.go:6-36`):
```go
const RepoURL = "https://github.com/Aider-AI/polyglot-benchmark"
const PinnedSHA = "7e0611e77b54e2dea774cdc0aa00cf9f7ed6144f"  // 40-hex, real HEAD, never a mutable ref
func isHexSHA1(s string) bool { /* exactly 40 lowercase hex; total, no regexp */ }
```
Manifest records: per upstream repo URL + pinned SHA (`polyglot-benchmark@7e0611e`, `aider@5dc9490`) + the exact (language, exercise) / (fixture-path) selection + a per-file `sha256` content digest column. The 2 pre-existing hermetic stubs (`wordy`, `leap`) are marked distinctly as **repo-authored / internal — repo license**, NOT Exercism MIT (RESEARCH line 154).

**Table shape to mirror** (`bench/LICENSES.md:12-16`) — pipe-delimited `| ... |` rows with a `sha256_or_pin` column; note `bench/LICENSES.md` itself uses 40-zero placeholder digests for deferred pins — the manifest must use REAL `crypto/sha256` digests (the gate verifies them, so placeholders fail).

---

### `bench/datasets/aider-polyglot/LICENSE-AUDIT.md` (MODIFY) — covers VENDOR-02

**Analog:** the existing MIT track blocks in the same file.

**Existing block shape to mirror** (`LICENSE-AUDIT.md:30-37`, the `cpp`/`go` blocks):
```yaml
---
track: go
source_repo: github.com/exercism/go@68c309cef65b6140646270de0910581332a60f44
license: MIT
license_sha256: e52f804e74f0fbd34e8927962319df346166b8194094309a0aee318693df44df
redistribution_clause_excerpt: "Permission is hereby granted, free of charge, ..."
---
```
ADD parallel Apache-2.0 fixture blocks keyed by top-level `fixture:` (decoded by the new `FixtureLicense` struct + `hasTopLevelFixtureKey` pre-filter). All vendored MIT provenance cites `exercism/<lang>@<sha>` (NOT polyglot-benchmark, which has no LICENSE for 5/6 tracks — see audit prose lines 14-20). Keep the existing header prose contract that names `make verify-licenses` as the HARD-FAIL gate.

---

### `bench/datasets/aider-polyglot/fixtures/<lang>/...` (NEW vendored tree) — covers VENDOR-01

**Analog:** the 2 existing committed stubs (verified on disk):
```
fixtures/python/exercises/practice/wordy/{.meta/config.json,.meta/example.py,wordy.py,wordy_test.py}
fixtures/rust/exercises/practice/leap/{.meta/config.json,.meta/example.rs,src/lib.rs,tests/leap.rs}
```
**Shape the loader requires** (`loader.go:96-124`): `<dir>/.meta/config.json` with `files.solution` / `files.test` / `files.example` (the `Config` struct, loader.go:40-52). Per-exercise vendored files (exclude `.docs/`, `.approaches/`): `.meta/config.json`, `.meta/example.<ext>`, the stub (`<ex>.<ext>` / `src/lib.rs`), test file(s), toolchain files (`Cargo.toml`/`go.mod`).

**DO NOT overwrite `wordy`/`leap`** — `loader_test.go:14` hard-codes `filepath.Join("fixtures","python","exercises","practice","wordy")` and asserts `Files.Solution==[wordy.py]` (loader_test.go:28-36). Vendor the upstream 9-exercise subset (×{python,go,rust}, java optional) as ADDITIONAL dirs alongside the hermetic stubs (RESEARCH Recommendation (a) / Open Question 2).

**Subset (recorded in manifest):** `book-store bowling forth pig-latin poker react two-bucket variable-length-quantity wordy` × {python, go, rust} (java reserved for Phase 100). Apache-2.0 subset under `fixtures/_aider-edit-format/`: `languages/{python,go,rust,java,javascript}/test.<ext>` + a trimmed (≤~150-line) SEARCH/REPLACE gold excerpt + NOTICE + Apache-2.0 LICENSE copy.

---

### `Makefile` `verify-licenses` target (MODIFY) — supports VENDOR-03

**Analog:** the existing `verify-licenses` + `verify-tos` targets.

**Current target** (`Makefile:330-331`):
```makefile
verify-licenses:
	go run ./cmd/helix-bench verify-licenses bench/datasets/aider-polyglot/LICENSE-AUDIT.md
```
The target ALREADY EXISTS (and is in the `.PHONY` set, Makefile:1). Extend the recipe with any new `--manifest`/`--tree` flags and update the comment block (Makefile:324-329) to describe the dual MIT+Apache-2.0 disposition + manifest-vs-disk walk. `verify-tos` (Makefile:321-322) is the sibling precedent for a hard-fail `go run ./cmd/helix-bench` gate.

## Shared Patterns

### Strict-decode block parser (REUSE — already shipped + tamper-tested)
**Source:** `cmd/helix-bench/verify_tos.go:111-128` (`splitOnHorizontalRules`) + `verify_licenses.go:62-63` (`yaml.NewDecoder` + `dec.KnownFields(true)`).
**Apply to:** both the existing MIT track loop AND the new Apache-2.0 fixture loop. Unknown key → error → non-zero exit. Do NOT hand-roll a new parser.

### Pinned-SHA / hex digest provenance discipline
**Source:** `bench/datasets/aider-polyglot/pin.go:6-36` (`PinnedSHA` const + `isHexSHA1` total no-regexp guard).
**Apply to:** `VENDOR-MANIFEST.md` (pinned SHAs for both upstream repos) + the per-file `crypto/sha256` digest column the gate verifies.

### Path-segment containment (defense in depth on fixture walk)
**Source:** `bench/datasets/aider-polyglot/loader.go:81-89` (`validatePathSegment`) + `copyFile` clean check (loader.go:130-132).
**Apply to:** the new manifest-vs-disk `filepath.WalkDir` so a crafted path cannot escape the fixtures root.

### Fail-closed test shape (anti-vacuity)
**Source:** `cmd/helix-bench/verify_licenses_test.go:52-109` (`*FailsClosed` / `*FailsStrictDecode` / `*ZeroTracksIsError`) + good-path control `:41-50`.
**Apply to:** every new tamper test — each sabotage class MUST return non-nil; one good-tree control MUST return nil.

## No Analog Found

None. Every file this phase touches has a direct in-tree analog (the gate, its tests, the manifest discipline, the audit blocks, the fixture shape, the Makefile target all already exist — this phase EXTENDS them).

## Metadata

**Analog search scope:** `cmd/helix-bench/`, `bench/datasets/aider-polyglot/`, `bench/*.md`, `Makefile`.
**Files scanned:** `verify_licenses.go`, `verify_licenses_test.go`, `verify_tos.go`, `pin.go`, `loader.go`, `loader_test.go`, `LICENSE-AUDIT.md`, `bench/LICENSES.md`, `Makefile`, on-disk `fixtures/` tree.
**Pattern extraction date:** 2026-06-23
