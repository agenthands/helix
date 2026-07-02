---
author: qa
responsible: architect
phase: 140
milestone: v2.13
status: complete
verdict: passed
---

# VERIFICATION 140 — All-11 real-binary E2E + function-seeded multi-hop + docs

**Verdict: `passed`.** Independent QA (did NOT write this code). Every acceptance
gate re-exercised against a FRESHLY-BUILT binary and on-disk source reality. The
milestone headline holds: in-body-origin `data_flows` surfaces E2E for **11/11
languages with ZERO skips** through the real `helix` binary; the function-seeded
multi-hop + broken-hop guard are correct; the Path-B daemon-gate wiring did not
regress any pre-existing daemon/skill test.

## 1. Fresh-binary E2E — the headline (11/11, 0 skip)

- Build: `go build -o /tmp/helix-v213-qa ./cmd/helix` → **clean, 3.37s** (176MB binary).
- Run: `HELIX_BIN=/tmp/helix-v213-qa go test -count=1 -v -run TestCLI_E2E_InBody ./internal/cli/`
  → `ok  github.com/agenthands/helix/internal/cli  6.663s`.

Per-language ledger (`TestCLI_E2E_InBodyDataFlow`), each a **real PASS with
non-zero runtime** (not a 0.00s skip):

| # | Lang | Result | Runtime |
|---|---|---|---|
| 1 | go | **PASS** | 0.65s (+determinism guard) |
| 2 | typescript | **PASS** | 0.53s |
| 3 | java | **PASS** | 0.49s |
| 4 | csharp | **PASS** | 0.56s |
| 5 | kotlin | **PASS** | 0.54s (Path-B wired) |
| 6 | php | **PASS** | 0.53s (Path-B wired) |
| 7 | python | **PASS** | 0.57s |
| 8 | ruby | **PASS** | 0.54s (Path-B wired) |
| 9 | rust | **PASS** | 0.53s (Path-B wired) |
| 10 | c | **PASS** | 0.54s |
| 11 | cpp | **PASS** | 0.54s (class-wrapped) |

`--- PASS: TestCLI_E2E_InBodyDataFlow (6.02s)` — **11/11 PASS, 0 SKIP**.
`--- PASS: TestCLI_E2E_InBodyMultiHop (0.62s)` — multi-hop PASS.

HELIX_BIN honored: the harness `t.Skip`s on an empty `resolveHelixBin()`
(cli_e2e_test.go:103-104); 11 PASS (not SKIP) proves the binary was found and
driven. Independently corroborated by the revert-and-fail (§6): removing the
`.rb` arm from the rebuilt binary flipped ruby to `not_found` — the E2E genuinely
exercises the daemon walk, not a stub.

## 2. langFromExt on disk (semantic_wiring.go:1838-1867)

All 4 new arms present and canonical:
`.rs`→`"rust"` (1856-57), `.kt`/`.kts`→`"kotlin"` (1858-59), `.php`→`"php"`
(1860-61), `.rb`→`"ruby"` (1862-63). Doc-comment (1823-1837) **corrected**: states
"since v2.13, Rust/Kotlin/PHP/Ruby" wired for D-BREADTH extraction and that "their
type resolvers remain nil-stubs … adds NO has_type/uses_type edges". **No stale
"stay '' until v2.13 wires them" text remains.** ✓

## 3. E2E test-body non-vacuity (cli_dataflow_inbody_e2e_test.go)

- **Headline asserts FIRST** (313-334): `explainSymbol(consumerParam "s")` must
  resolve `exact` (315-317, else Fatalf) AND yield ≥1 in-body edge (319 `len==0`
  → Fatalf) with `internal_kind=="DATA_FLOWS"`, `To==seed param`, `From` carrying
  the "producer" token. Direction-flip safe (`dataFlowEdgesOf` scans both dirs).
- **Differential negative** (340-348): `pure`'s param `q` (fed literal `pure(0)`)
  must resolve `exact` (341-343, non-vacuous zero) AND yield **exactly 0**
  data_flows edges (345 `!=0` → Fatalf). Real absence, not a missing symbol.
- **Determinism** (355-366, Go): re-`indexFull` → identical in-body edge set
  (`sameEdgeSet`, count + endpoints, order-independent).
- **Multi-hop** (484-536): seeds the producer **FUNCTION `src`** via real
  `helix trace-data-flow` (505); asserts `FallbackReason==""` (real walk, not
  degraded, 506-508) AND `sink.v` reachable by exact symbol_id (509-512).
  **Broken-hop** (`transform(x){return 0}`, 517-535): re-index same workspace →
  `sink.v` must NOT be reachable (532 → Fatalf if still reachable). Non-vacuous
  revert-and-fail baked into the test.
- **Harness reuse confirmed**: `newE2EFixture`+`mcpActivate` (cli_e2e_test.go),
  `switchModeReview`/`indexFull`/`explainSymbol`/`explainDeepResult` (cli_type_
  resolution_e2e_test.go v2.12). The new file only adds domain helpers
  (`dataFlowEdgesOf`, `inBodyEdgesInto`, `newInBodyDataFlowFixture`,
  `traceDataFlow*`, `sameEdgeSet`). **No reinvented harness.** ✓
- Fixture table (130-286): all 11 langs use `local:=producer();sink(local)` +
  negative `pure(0)`; Java/C#/Kotlin/cpp class-wrapped (cpp comment documents the
  extractor's field_identifier-only capture — a pre-existing property, not a
  v2.13 regression).

## 4. Independent regression — the Path-B risk

`go test -count=1 ./internal/daemon/... ./internal/skill/semantic/...`:
- `ok  github.com/agenthands/helix/internal/daemon        4.321s`
- `ok  github.com/agenthands/helix/internal/skill/semantic 13.238s`

**Both green.** Enabling extraction for the 4 new langs in the daemon full-index
walk broke NO pre-existing daemon or skill/semantic test. ✓

## 5. Guard test (semantic_cfamily_wiring_test.go)

`TestLangFromExt_CFamily` (30-71) now asserts the NEW mappings explicitly:
`{"a.rs","rust"}`, `{"a.kt","kotlin"}`, `{"a.kts","kotlin"}`, `{"a.php","php"}`,
`{"a.rb","ruby"}` (56-61) — it would go RED if the wiring were wrong/reverted (no
longer asserts `→ ""`). Run: `go test -count=1 -run TestLangFromExt ./internal/daemon/`
→ `TestLangFromExt_CFamily PASS` + `TestLangFromExt_RegistryResolves PASS`, `ok  0.072s`. ✓

## 6. Independent revert-and-fail (+ byte-identical restore)

Mutated `langFromExt` `.rb` arm `return "ruby"` → `return ""` (semantic_wiring.go
:1863), **rebuilt the binary** (`/tmp/helix-v213-qa-mut`, required since langFromExt
is compiled into the daemon), ran the ruby subtest:

```
cli_dataflow_inbody_e2e_test.go:315: positive seed "s": resolution="not_found"
    fallback="symbol_not_found", want exact (indexed param)
--- FAIL: TestCLI_E2E_InBodyDataFlow/ruby (0.52s)
```

**RED confirmed** — with `.rb`→"" the daemon walk drops the ruby file, the param
is never indexed, the seed cannot resolve. This proves both (a) the langFromExt
wiring is load-bearing for the 4 Path-B langs and (b) the E2E genuinely drives
the daemon file-walk gate end-to-end. Then **restored byte-identical**:
line 1863 = `return "ruby"` (indentation matches sibling `.php` arm); mutation
binary removed. Working-tree `semantic_wiring.go` diff vs HEAD is the expected
Path-B/139 addition only — no mutation residue.

## 7. Hygiene / deps / schema

- `internal/daemon/zz_scratch_inbody_test.go` — **absent** (`ls` → No such file). ✓
- `git diff --stat go.mod go.sum` — **empty** (zero new deps). ✓
- `git diff --stat internal/semantic/store/` — **empty** (no schema migration). ✓
- `go build ./...` — exit 0. `make vet` — exit 0 (all 8 vettools incl. the
  kernel/semantic boundary + tools-quarantine checks). ✓

## 8. Docs (docs/edge-types.md)

- Families table (:21): `data_flows` row lists all three producers
  (`dataFlowEdges` v2.9, `inBodyDataFlowEdges` v2.13, `returnBridgeEdges` v2.13).
- `data_flows` section (:25-64): all **3 Source markers** documented — `def_use`
  (:29), `def_use_inbody` producer.function→consumer.param (:33-37),
  `def_use_return` param→enclosing-function (:38-42). Honest deferred-surface
  ledger (:46-50: variable-level precision, field/heap, taint deferred).
- **All-11 coverage ledger** (:51-59): "all 11 proven E2E" naming every language;
  honestly notes the 4 new langs' type resolvers stay nil-stubs → no
  has_type/uses_type.
- **Function-seed note** (:60-64 + :104-110): BFS is kind-agnostic, a function
  seed now reaches in-body targets; full function-seed verb contract flagged a
  fast-follow. ✓

## Verdict rationale

`passed` — every gate met: fresh-binary E2E **11/11 + 0 skip**, multi-hop +
broken-hop correct, **regression green** (daemon 4.32s / skill-semantic 13.24s),
guard test asserts the new mappings, scratch file gone, deps + schema unchanged,
docs honest, and the independent revert-and-fail produced a real RED that
restored clean. No gaps found.

## Records

```yaml
gate_result:
  record_author: qa
  accountable_owner: qa
  id: GV213-140
  kind: phase_verification
  gates_subject: phase-140-all11-realbinary-e2e-multihop-docs
  evaluator: qa
  verdict: pass
  status: accepted
  method:
    - built a fresh binary, ran TestCLI_E2E_InBody (11/11 + multi-hop, 0 skip)
    - confirmed langFromExt Path B wiring + corrected doc on disk
  evidence_refs:
    - EV213-140
```

```yaml
evidence:
  record_author: qa
  accountable_owner: qa
  evidence_producer: qa
  commissioned_by: qa
  issuance:
    kind: peer_authored
  id: EV213-140
  subject_ref: GV213-140
  kind: test
  warrant:
    kind: test_pass
    scope:
      - internal/cli/cli_dataflow_inbody_e2e_test.go
      - internal/daemon/semantic_wiring.go
    rationale: >-
      Fresh-binary HELIX_BIN=/tmp/helix-qa-v213 go test -count=1 TestCLI_E2E_InBody ->
      11/11 language subtests PASS + TestCLI_E2E_InBodyMultiHop PASS, 0 skip.
  freshness:
    status: fresh
    checked_against: HEAD
  supports:
    - GV213-140
```
