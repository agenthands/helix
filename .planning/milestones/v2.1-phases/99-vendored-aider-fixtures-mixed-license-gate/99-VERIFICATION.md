---
phase: 99-vendored-aider-fixtures-mixed-license-gate
verified: 2026-06-23T00:00:00Z
status: passed
score: 10/10 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: none
  previous_score: none
notes:
  - "Requirement-text SPDX mechanism deviation (inline header → manifest/NOTICE disposition) is intentional, documented, and achieves the requirement intent. Recorded as INFO, not a gap. See Anti-Patterns / Gaps Summary. Override suggestion offered below if a formal record is desired."
---

# Phase 99: Vendored Aider Fixtures + Mixed-License Gate Verification Report

**Phase Goal:** A deterministic, offline, mixed-license vendored fixture tree (MIT Exercism polyglot subset + Apache-2.0 aider edit-format fixtures) lands with correct per-file SPDX disposition, per-track attribution/NOTICE, and a manifest, guarded by an extended hard-fail license gate that goes RED on any tampered or missing header.
**Verified:** 2026-06-23
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #   | Truth | Status | Evidence |
| --- | ----- | ------ | -------- |
| 1 | MIT polyglot subset (9 exercises × py/go/rust) vendored under `fixtures/<lang>/exercises/practice/`, byte-identical @ 7e0611e | ✓ VERIFIED | 9 exercise dirs present in each of python/go/rust. `go test ./bench/datasets/aider-polyglot/... -run Load` PASS. Manifest cites `7e0611e77b54e2dea774cdc0aa00cf9f7ed6144f`. Note: python/wordy slot is the pre-existing hermetic stub (path collision resolved per plan Recommendation a — real upstream wordy vendored for go+rust only). |
| 2 | Pre-existing hermetic stubs (python/wordy, rust/leap) preserved byte-unchanged; loader_test.go green | ✓ VERIFIED | `python/.../wordy/.meta/config.json` carries `authors:["fixture"]`; `git status --short` clean for both stub dirs. `TestLoadExercise` + all 18 loader tests PASS. |
| 3 | MIT provenance cites exercism/<lang>@<sha> in VENDOR-MANIFEST.md + per-track NOTICE | ✓ VERIFIED | python/go/rust NOTICE each cite `exercism/<lang>@<sha>` (d8886cad / 68c309ce / 557695eb). Manifest `upstream_provenance` column carries the same SHAs (9 occurrences of the exercism SHAs). |
| 4 | Apache-2.0 aider edit-format subset under `_aider-edit-format/` (5 small languages/* + trimmed gold excerpt, NOT 28k/100k-line files) + Apache NOTICE/LICENSE | ✓ VERIFIED | 5 `languages/{python,go,rust,java,javascript}/test.<ext>` present; `search-replace-sample.txt` = 119 lines (NOT 27,810); `chat-history.md` absent; `LICENSE` contains "Apache License"; `NOTICE` cites `Aider-AI/aider@5dc9490`. |
| 5 | Every vendored file has a VENDOR-MANIFEST.md row with a REAL crypto/sha256 (no 40-zero placeholders) + MIT\|Apache-2.0 disposition | ✓ VERIFIED | 150 distinct 64-hex digests; zero `0{40}`/`0{64}` placeholders. Independent `sha256sum` of bowling config.json matches manifest digest exactly. Disposition column: 134 MIT, 9 Apache-2.0, 11 internal-stub sentinel rows. |
| 6 | extended verify_licenses.go does dual-disposition (MIT track + Apache fixture) strict-decode + manifest-vs-disk crypto/sha256 walk | ✓ VERIFIED | `verify_licenses.go` contains `FixtureLicense`, `hasTopLevelFixtureKey`, `verifyLicensesFull`, `verifyManifestVsDisk`, `parseManifestRows`, `filepath.WalkDir`, `crypto/sha256`. Audit has 6 `track:` blocks + 1 `fixture:` block (exactly the 5 FixtureLicense keys). No `internal/kernel`/`internal/semantic` imports. |
| 7 | `make verify-licenses` exits 0 on the committed tree | ✓ VERIFIED | `make verify-licenses` → exit 0, "OK (dual disposition + manifest-vs-disk walk)". Makefile wired with `--manifest .../VENDOR-MANIFEST.md --tree .../fixtures`. |
| 8 | ANTI-VACUITY: real tamper of a vendored byte/manifest/license trips the gate RED; revert → green | ✓ VERIFIED | **Real tampers executed (not just reading tests):** (a) mutated 1 byte of a vendored .go → exit 1 "sha256 mismatch"; (b) dropped a manifest row → exit 1 "no manifest entry"; (c) flipped Apache fixture license→MIT → exit 1 "no Apache-2.0 fixture blocks"; (d) removed Apache NOTICE sidecar → exit 1 "missing notice_path". Each reverted → exit 0. 16/16 `VerifyLicenses*` tests (incl. 10 tamper/full cases) PASS. |
| 9 | NO inline SPDX in executed fixture files (.go/.py/.json under exercises/ + Apache languages/*); disposition in manifest/sidecars only | ✓ VERIFIED | `grep -rl SPDX-License-Identifier` over `*/exercises` and `_aider-edit-format/languages` → zero matches. Spot-checks of go bowling, python poker, rust .meta json → no SPDX. Disposition lives in 4 NOTICE sidecars (which DO carry inline SPDX, permitted) + manifest. |
| 10 | The ONLY test failures are known pre-existing (TestRunSubcommandWiresDeltaPass = Phase 80-05 deferred; network HF fetches); phase-99 surface green | ✓ VERIFIED | Full `go test ./cmd/helix-bench/` fails ONLY on `TestRunSubcommandWiresDeltaPass` ("delta pass not wired into runBench"), which originates from commit `05db8792 test(80-05)` — untouched by any phase-99 commit. Phase-99 commits touch only verify_licenses.go/.test/Makefile + fixture/manifest/audit files. The VerifyLicenses + aider-polyglot loader surfaces are fully green. |

**Score:** 10/10 truths verified (0 present, behavior-unverified)

### Roadmap Success Criteria Coverage

| SC | Text | Maps to Truth | Status |
| -- | ---- | ------------- | ------ |
| 1 | MIT Exercism fixtures vendored with SPDX-MIT disposition + per-track NOTICE + VENDOR-MANIFEST.md | 1, 3, 5, 9 | ✓ (disposition via manifest/NOTICE — see INFO note) |
| 2 | Aider Apache-2.0 edit-format fixtures vendored with SPDX-Apache disposition + attribution | 4, 5, 9 | ✓ (disposition via manifest/NOTICE — see INFO note) |
| 3 | make verify-licenses hard-fails over full tree under both dispositions | 6, 7 | ✓ |
| 4 | Anti-vacuity tamper test ships and goes RED | 8 | ✓ |

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `bench/datasets/aider-polyglot/VENDOR-MANIFEST.md` | 7e0611e + 5dc9490 + 150 real-sha256 rows | ✓ VERIFIED | Both pinned SHAs present; 150 digests; no placeholders |
| `bench/datasets/aider-polyglot/LICENSE-AUDIT.md` | 6 MIT track blocks + 1 Apache `fixture:` block | ✓ VERIFIED | `fixture:` key present; exactly 5 FixtureLicense keys |
| `bench/datasets/aider-polyglot/fixtures/_aider-edit-format/LICENSE` | "Apache License" text | ✓ VERIFIED | 4 "Apache License" matches |
| `bench/datasets/aider-polyglot/fixtures/{python,go,rust}/NOTICE` | exercism/<lang> attribution | ✓ VERIFIED | All 3 cite matching exercism SHAs + SPDX |
| `cmd/helix-bench/verify_licenses.go` | FixtureLicense + walk | ✓ VERIFIED | All symbols present; no forbidden imports |
| `cmd/helix-bench/verify_licenses_test.go` | Tamper tests | ✓ VERIFIED | 10 new + 6 pre-existing, all pass |
| `Makefile` | verify-licenses with --manifest/--tree | ✓ VERIFIED | Wired; hard-fail gate (Makefile:337) |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| VENDOR-MANIFEST.md | fixtures/ | sha256 row ↔ on-disk bytes (bidirectional) | ✓ WIRED | Gate walk confirms both directions; tamper proves both fail closed |
| LICENSE-AUDIT.md | _aider-edit-format/ | notice_path/license_path → committed sidecars | ✓ WIRED | Both targets exist; removing NOTICE trips gate RED |
| Makefile | verify_licenses.go | `go run ./cmd/helix-bench verify-licenses ... --manifest --tree` | ✓ WIRED | Recipe present; exit 0 on committed tree |
| verify_licenses.go | fixtures/ | filepath.WalkDir + crypto/sha256 | ✓ WIRED | WalkDir + sha256 present; real recompute confirmed |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Gate green on committed tree | `make verify-licenses` | exit 0 "OK" | ✓ PASS |
| Tamper: mutated byte | append space to vendored .go + `make verify-licenses` | exit 1 "sha256 mismatch" | ✓ PASS |
| Tamper: dropped manifest row | delete row + `make verify-licenses` | exit 1 "no manifest entry" | ✓ PASS |
| Tamper: flipped Apache license | Apache-2.0→MIT in audit + `make verify-licenses` | exit 1 "no Apache-2.0 fixture blocks" | ✓ PASS |
| Tamper: removed NOTICE sidecar | mv NOTICE away + `make verify-licenses` | exit 1 "missing notice_path" | ✓ PASS |
| Manifest digest correctness | `sha256sum` vs manifest row | exact match | ✓ PASS |
| Loader still loads exercises | `go test ./bench/datasets/aider-polyglot/...` | PASS (18 tests) | ✓ PASS |

### Probe Execution

No `scripts/*/tests/probe-*.sh` declared for this phase; the authoritative gate is `make verify-licenses`, executed above (exit 0 committed, exit 1 on every tamper class).

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| VENDOR-01 | 99-01 | MIT Exercism subset vendored + per-track NOTICE + VENDOR-MANIFEST.md | ✓ SATISFIED | Truths 1,2,3,5,9. Per-file MIT disposition + provenance in manifest/NOTICE (intent met; mechanism note below) |
| VENDOR-02 | 99-01 | Apache-2.0 aider edit-format fixtures + attribution, mixed-license tree | ✓ SATISFIED | Truths 4,5,9. Apache disposition + NOTICE + LICENSE present |
| VENDOR-03 | 99-02 | make verify-licenses hard-fails over full tree, dual disposition + tamper RED | ✓ SATISFIED | Truths 6,7,8. Real tampers confirm RED |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| (none) | — | No debt markers (TBD/FIXME/XXX), no stubs, no hollow data introduced by phase 99 | ℹ️ Info | Gate is load-bearing; manifest digests real; fixtures byte-identical |

### Human Verification Required

None. Every phase behavior is automatable and was exercised with real commands (build, vet, gate, tamper, tests, independent digest recompute).

### Gaps Summary

No blocking gaps. The phase goal is achieved in the codebase: the mixed-license vendored tree exists with correct per-file disposition + per-track attribution + a real-sha256 manifest, and the extended `make verify-licenses` gate is genuinely load-bearing — independently tampering a vendored byte, a manifest row, a license value, and a NOTICE sidecar each turned the gate RED, and reverting each returned it to green.

**INFO — intentional mechanism deviation (not a gap):** REQUIREMENTS.md / ROADMAP SC#1–2 phrase the disposition as inline `SPDX-License-Identifier: MIT`/`Apache-2.0` *headers*. The plan deliberately placed per-file SPDX disposition in the VENDOR-MANIFEST.md rows + per-track NOTICE sidecars instead of inline in executed fixtures, because inline SPDX would mutate bench-input bytes, be invalid inside JSON config files, and break byte-reproducibility against upstream (plan threat T-99-05; 99-RESEARCH.md §"SPDX Disposition"). The verification task itself mandates "NO inline SPDX in executed fixtures," confirming this is the intended interpretation. The requirement's *intent* — correct per-file MIT/Apache-2.0 disposition, attribution, manifest, gate-enforced — is fully achieved. This is recorded as INFO; no action required. If a formal acceptance record is desired, add to this file's frontmatter:

```yaml
overrides:
  - must_have: "vendored fixtures carry per-file SPDX-License-Identifier MIT/Apache-2.0 disposition"
    reason: "Inline SPDX in executed fixtures would mutate bench inputs, be invalid in JSON, and break byte-reproducibility (T-99-05). Per-file disposition lives in VENDOR-MANIFEST.md rows + per-track NOTICE sidecars; the gate enforces it. Requirement intent achieved via a byte-preserving mechanism."
    accepted_by: "verifier-recommended"
    accepted_at: "2026-06-23T00:00:00Z"
```

**Out-of-scope pre-existing failure (not introduced by phase 99):** `TestRunSubcommandWiresDeltaPass` fails ("delta pass not wired into runBench") — this is the Phase 80-05 deferred wiring item (commit `05db8792`), untouched by any phase-99 commit. Logged in `deferred-items.md`.

---

_Verified: 2026-06-23_
_Verifier: Claude (gsd-verifier)_
