---
author: qa
responsible: architect
phase: 138
status: complete
verdict: passed
---

# VERIFICATION 138 — Origin-space widening + kind-union extension + all-11 unit verification

**VERDICT: `passed`** — build+vet+tests green FRESH, 11/11 matrix zero-skip, m2 guards
proven non-vacuous by independent revert-and-fail (byte-identical restore), deps unchanged,
leaf boundary intact, and the FLOW-05e ledger is honest (every positive claim independently
re-measured true). One conservative, non-blocking advisory for Phase 139 (rust — below).

Verification was performed independently: I re-ran every test uncached (`-count=1`), read
`summary.go` for code reality rather than trusting the SUMMARY, and independently mutated +
restored the source to prove the guards and to re-derive the FLOW-05e HEAD baseline myself.

---

## Step 1 — Fresh build + vet + test (UNCACHED)

| Check | Command | Result |
|---|---|---|
| Build | `go build ./cmd/helix` | **clean** (`BUILD_OK`, exit 0) |
| Vet | `go vet ./internal/semantic/dataflow/... ./internal/semantic/extract/...` | **clean** (`VET_OK`, exit 0) |
| Test | `go test -count=1 ./internal/semantic/dataflow/... ./internal/semantic/extract/...` | **13 packages ok, 1 no-test (testutil)**, exit 0 |

Per-package (uncached): dataflow, extract, extract/{c,cpp,csharp,golang,java,kotlin,php,
python,ruby,rust,typescript} all `ok`; `extract/testutil` `[no test files]`. Re-run AFTER
all my mutations/restores: still 13 ok — no residue left behind.

## Step 2 — Code reality of `summary.go` (read, not trusted)

All six sub-claims **CONFIRMED** against the source:

- **(a) `Origin` discriminated struct** — `summary.go:62-65`: `type Origin struct { Param int;
  Callee string }`, comparable, `Callee==""` ⇒ param origin, `Callee!=""` ⇒ callReturn. Matches spec.
- **(b) No zero-param early bail** — `AnalyzeFlow` (`summary.go:119-158`) seeds param taint,
  **always** calls `walkFlow` (line 142), and returns nil **only** at line 154:
  `if len(out.Params) == 0 && len(out.InBodyFlows) == 0`. The old `len(paramNames)==0` bail is
  gone. Generalized anti-vacuity gate, exactly as the CONTEXT "load-bearing correction" required.
- **(c) `unwrapToCall` STRICT single-child whitelist** — `summary.go:450-461`: loops only over
  `expression_list`/`parenthesized_expression`, **refuses on `NamedChildCount() != 1`** (line 452),
  then requires `callKinds[n.Kind()]`. This is NOT `findFirst` over callKinds (that would be the
  m2 violation — proven fatal in Step 4).
- **(d) `processCall` param vs callReturn split** — `summary.go:302-312`: per arg origin,
  `o.Callee == ""` ⇒ append `CallArgTarget` to `flows[o.Param].CallArgs` (v2.9 path); else append
  `InBodyFlow{Producer:o.Callee, Consumer:callee, ArgPos:i}` to `*inBody`. Correct.
- **(e) Union changes exact** (`summary.go:79-107`, cross-checked against `git diff`):
  `paramListKinds += "method_parameters"`; `assignKinds` replaces `"init_declaration"` (dead typo)
  with `"init_declarator"` **and** adds `"assignment"` + `"property_declaration"`;
  `argListKinds += "value_arguments"`. Exactly the specified delta, nothing else.
- **(f) Determinism** — no Go-map iteration in the OUTPUT path: `out.Params` built by ordered
  slice walk (line 147-152), each `CallArgs` run through `dedupSortCallArgs`; `out.InBodyFlows =
  dedupSortInBody(inBody)` (`summary.go:465-485`, sort by Producer/Consumer/ArgPos then dedup).
  Map iteration exists only inside `collectOrigins` (collection), canonicalized before emit.
  Proven by `TestAnalyzeFlow_InBody_Determinism` (`reflect.DeepEqual` on re-analyze) — GREEN.

## Step 3 — All-11 matrix (M5)

`go test -count=1 -run TestAnalyzeFlow_AllElevenMatrix -v ./internal/semantic/dataflow/`:

```
--- PASS: TestAnalyzeFlow_AllElevenMatrix (0.14s)
    go, typescript, java, csharp, kotlin, php, python, ruby, rust, c, cpp  — all PASS
```

**11/11 PASS, ZERO skips.** Verified non-vacuous by reading the test
(`dataflow_matrix_test.go:56-107`): every case has empty `skipReason` (so the `t.Skipf` branch
at line 85-87 never fires), and each runs the full `hasInBody(s, {producer,sink,0})` assertion
(line 103) after a non-nil summary check. No language silently degraded to a skip.

## Step 4 — Anti-vacuity is NOT theater (INDEPENDENT revert-and-fail)

I independently weakened `unwrapToCall` to the m2-forbidden naive form
`return findFirst(n, func(c){ return callKinds[c.Kind()] })` and re-ran the m2 guards:

```
--- FAIL: TestAnalyzeFlow_InBody_SelectorRHS_NoOrigin
    dataflow_test.go:266: selector RHS must yield no in-body origin; got [{Producer:producer Consumer:sink ArgPos:0}]
--- FAIL: TestAnalyzeFlow_InBody_MultiCallRHS_NoOrigin
    dataflow_test.go:275: multi-call RHS must yield no in-body origin; got [{Producer:f Consumer:sink ArgPos:0}]
```

Both guards went **RED** — the naive form falsely bound `producer→sink` (descended into the
`selector_expression` RHS) and `f→sink` (took the first call of a multi-child `expression_list`).
This proves the strict whitelist genuinely constrains the engine; the guards are load-bearing,
not decorative.

**Restore is byte-identical.** Pre-mutation `summary.go` sha256 =
`de62a6ceb29168fb6cf8334b4e34544a6ec36ec51b18d254b4c5fb7fcbe208f9`; post-restore sha256 =
identical. The two m2 guards re-run GREEN after restore. (This reproduces — independently — the
engineer's claimed RED evidence in SUMMARY §"Revert-and-fail evidence".)

## Step 5 — FLOW-05e ledger honesty (load-bearing for Phase 139)

I independently re-derived the HEAD→post-138 param→param (`CallArgs`) truth table with a
throwaway 11-language probe (`f(x){sink(x)}` direct + `f(x){y=x;sink(y)}` transitive), measuring
post-138 as-is, then reverting **only the union block** to HEAD state (the def_use param→param
result depends solely on the paramList/assign/argList kind unions) and re-measuring. Probe
removed and `summary.go` restored byte-identical afterward (sha256 re-confirmed).

| Lang | Measured HEAD (direct/transitive) | Ledger HEAD claim | Post-138 | Match? |
|---|---|---|---|---|
| ruby | **false / false** | false / false (WHOLESALE NEW) | true/true | ✅ |
| kotlin | **false / false** | false / false (WHOLESALE NEW) | true/true | ✅ |
| python | **true / false** | true / false (PARTIALLY NEW) | true/true | ✅ |
| c | **true / false** | true / false (PARTIALLY NEW) | true/true | ✅ |
| cpp | **true / false** | true / false (PARTIALLY NEW) | true/true | ✅ |
| go / typescript / java / csharp / php | **true / true** | true / true (UNCHANGED) | true/true | ✅ |

**Every positive claim in the ledger is independently verified true.** The dangerous direction
— claiming a language UNCHANGED when it actually changed — does NOT occur: all five
claimed-stable languages (go/ts/java/csharp/php) are byte-unchanged as measured. The core
load-bearing preservation property ("v2.9 param→param semantics preserved for the stable
languages") holds. The ledger is **honest** — no false statement.

### Advisory (Phase 139 FLOW-05e, non-blocking) — rust omitted from the stable set

I measured **rust = true/true at BOTH HEAD and post-138** (rust's kinds — `parameters`,
`let_declaration`, `arguments` — are untouched by the union diff, so its def_use SET is
byte-unchanged). The ledger's UNCHANGED enumeration lists only **5** langs (go/ts/java/csharp/php)
and its guidance says "assert def_use SET unchanged **ONLY for** go/typescript/java/csharp/php,"
yet the ledger's own parenthetical says "the **6** already-working langs." Rust is the missing
6th. This is a **conservative** imprecision: it under-claims stability, so it CANNOT cause a
Phase 139 false-pass (worst case, rust's stability simply isn't locked by an assertion).
**Recommendation for the architect:** widen the FLOW-05e "assert def_use unchanged" set to the
full **6** (add rust) in Phase 139. Not a Phase-138 defect — the engine, tests, and every factual
ledger claim are correct — but the actionable scoping line should say 6, not 5.

## Step 6 — Deps + leaf boundary

- **Deps unchanged:** `git diff --stat go.mod go.sum` — **empty**. Zero new deps.
- **Leaf boundary intact:** the only non-test file, `summary.go`, imports exactly
  `sort`, `strings`, and `github.com/tree-sitter/go-tree-sitter`. No daemon/kernel/
  extract-provider imports in production code. (`dataflow` pkg = summary.go + 2 test files.)
- **Carry-seam VERIFIED, not re-plumbed:** `fact.go` / `fingerprint.go` are **unedited**
  (`git status` clean for both) — the widened `*dataflow.Summary` flows through the pointer,
  confirmed by the green extract suite. Matches the SUMMARY's carry-seam claim.
- **Working tree** (non-`.planning`): exactly the 3 claimed files — `summary.go` (M),
  `dataflow_test.go` (M), `dataflow_matrix_test.go` (??). Left dirty, uncommitted, as required.

### Minor SUMMARY doc inaccuracy (immaterial)
SUMMARY §"Diff stat" and §"dataflow_matrix_test.go (NEW, +122)" claim the matrix file is
**+122**; `git diff --stat` reports **+108**. The file exists, compiles, and passes 11/11 — the
line-count label is cosmetically off, no correctness impact.

---

## Acceptance ledger

| Criterion | Verdict |
|---|---|
| Build + vet + tests green FRESH (`-count=1`) | ✅ PASS (13 pkgs ok) |
| Code reality (a)–(f) matches contract | ✅ PASS |
| All-11 matrix 11/11, zero skips | ✅ PASS |
| m2 guards non-vacuous (independent revert→RED→byte-identical restore) | ✅ PASS |
| FLOW-05e ledger honest (no false claim; every positive claim re-measured true) | ✅ PASS (rust advisory below) |
| Zero new deps (go.mod/go.sum) | ✅ PASS (empty diff) |
| Leaf boundary + carry-seam intact | ✅ PASS |

**VERDICT: `passed`.** All hard acceptance bars clear on independent re-verification. The single
finding (rust omitted from the FLOW-05e "assert-unchanged" enumeration) is a conservative,
non-blocking scoping note routed to the architect for Phase 139 — it makes no false claim and
cannot cause a downstream false-pass.

## Records

```yaml
gate_result:
  record_author: qa
  accountable_owner: qa
  id: GV213-138
  kind: phase_verification
  gates_subject: phase-138-origin-space-widening-unions
  evaluator: qa
  verdict: pass
  status: accepted
  method:
    - ran go test -count=1 TestAnalyzeFlow_AllElevenMatrix (11/11, 0 skip)
    - ran m2 RED guards + anti-vacuity + determinism fresh (all pass)
    - read summary.go Origin/InBodyFlow/unwrapToCall + unions against the contract
  evidence_refs:
    - EV213-138
```

```yaml
evidence:
  record_author: qa
  accountable_owner: qa
  evidence_producer: qa
  commissioned_by: qa
  issuance:
    kind: peer_authored
  id: EV213-138
  subject_ref: GV213-138
  kind: test
  warrant:
    kind: test_pass
    scope:
      - internal/semantic/dataflow/dataflow_matrix_test.go
      - internal/semantic/dataflow/dataflow_test.go
      - internal/semantic/dataflow/summary.go
    rationale: >-
      go test -count=1 TestAnalyzeFlow_AllElevenMatrix -> 11/11 subtests PASS, 0 skip;
      m2 RED guards + anti-vacuity + determinism green on a fresh run.
  freshness:
    status: fresh
    checked_against: HEAD
  supports:
    - GV213-138
```
