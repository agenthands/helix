# Project Retrospective

## Milestone: v1.2 — Performance & Production Hardening

**Shipped:** 2026-04-10
**Phases:** 7 | **Plans:** 25 | **Commits:** 134

### What Was Built
- Benchmark harness with testing.B.Loop, 38-tool benchmarks, CI benchstat regression gate
- Observability foundation: stdlib-only internal/obs/, zero-alloc slog handler, admin listener
- Prometheus /metrics with RED histograms per tool, lspool gauges, bounded-label contract
- End-to-end tracing: otelgrpc propagation, telemetry middleware, per-tool sub-spans, OTLP exporter
- Graceful degradation: per-class timeout budgets, deadline propagation, typed ErrCircuitOpen, GOMEMLIMIT
- Documentation: README.md, USAGE.md, CHANGELOG.md with auto-generated tool/language tables
- Benchmark gate hardening: capture-baseline.yml workflow, blocking benchgate mode

### What Worked
- Benchmarks-first ordering constraint prevented observing-the-benchmarked-thing contamination
- Noop-default provider pattern kept hot-path at zero allocs until metrics/tracing explicitly enabled
- Decoupled MetricsSink interface kept lspool independent of internal/obs with compile-time assertion
- Phase-level benchstat delta gates caught allocation regressions early
- Auto-generated doc tables from registry prevent drift between code and documentation

### What Was Inefficient
- gopls v0.17.1 pinning caused CI build failures on Go 1.25 linux/amd64 — took multiple iterations to diagnose (fork workflow caching compounded the issue)
- Baseline capture required local fallback (darwin/arm64) instead of CI ubuntu-latest — cross-platform comparison is imprecise
- Some SUMMARY.md one-liners captured deviation notes instead of accomplishments, making automated extraction noisy

### Patterns Established
- Tiered benchmark thresholds: PR tier (15%/25%) relaxed for CI noise, release tier (10%/20%) strict
- Admin listener on dedicated loopback port, separate from MCP mux, bind failure non-fatal
- Telemetry middleware runs before profile filter so denied calls are observable
- Kernel spans set no attributes — middleware owns all labels (single responsibility)
- Shutdown ordering: kernel first, then trace flush (separate 5s context), then listener close

### Key Lessons
- Pin dependencies to versions compatible with your Go version before creating CI workflows
- Fork repos may serve stale workflow files from upstream — verify with API before debugging further
- Operational gaps (needing to trigger a workflow) are different from code gaps — don't create code phases for operational tasks

## Milestone: v1.3 — Documentation Catchup

**Shipped:** 2026-04-11
**Phases:** 2 | **Plans:** 4 | **Commits:** 25

### What Was Built
- README.md updated with Production & Observability section, corrected tool count from stale "38+" to accurate "35+"
- CONTRIBUTING.md rewritten as Go-native contributor guide (126 lines, covers dev commands, integration tests, benchmarks, tool/language addition)
- CHANGELOG.md v1.2 entry completed with Phase 15 Benchmark Gate Hardening gap fill
- USAGE.md accuracy gaps fixed: added service_name config key, 2 missing metrics, metric label documentation with PromQL examples, benchmarks subsection
- INSTALL.md created from scratch with per-agent MCP config for 6 coding agents (Claude Code, Codex, OpenCode, Cursor, Gemini CLI, Antigravity) + HTTP mode
- Stale Python-based llms-install.md deleted

### What Worked
- Parallel executor agents for independent documentation plans (USAGE.md and INSTALL.md modified different files)
- Research phase caught 4 verified accuracy gaps by reading actual source code before planning
- Explicit "do NOT include" constraints (D-05: no CI gate details in USAGE benchmark section) prevented content duplication with CONTRIBUTING.md
- Per-agent config format research (especially OpenCode's different `"mcp"` key) caught a pitfall that would have produced broken examples

### What Was Inefficient
- Phase 16 summaries lacked `requirements-completed` frontmatter, requiring manual cross-referencing during milestone audit
- Integration checker found README.md has no links to INSTALL.md, USAGE.md, or CONTRIBUTING.md — this cross-reference gap should have been caught during planning
- HTTP mode flag style inconsistency across docs (--serve vs --mode=http) — not caught until integration check

### Patterns Established
- Documentation accuracy audits should always verify against source code, not prior documentation
- Per-agent MCP config sections should note format differences explicitly (OpenCode vs others)
- USAGE.md benchmark content covers local workflow only; CI gate details belong in CONTRIBUTING.md

### Key Lessons
- Documentation phases benefit from research that reads actual source code — found gaps that planning-only approaches would miss
- Cross-document link graphs should be verified as part of planning, not discovered during milestone audit
- Tool count claims drift over time — auto-generated tables are the source of truth

## Milestone: v1.4 — Integration Testing v2

**Shipped:** 2026-04-14
**Phases:** 4 | **Plans:** 11

### What Was Built
- Extracted importable test harness (`test/harness/`) with Runner, golden store, fixture helpers, build tag taxonomy
- Protocol oracle tests (MCP handshake, tools/list, session isolation, reconnect)
- Contract oracle tests (JSON Schema validation, 23 golden outputs, error contracts, selectability heuristics)
- Scenario oracle matrix (14+ full-cycle agent workflows across Go, Python, TypeScript, C++, Swift, Zig, JavaScript, PHP, SQL, Markdown, polyglot, unsupported, collision, degraded)
- LLM behavioral tests with multi-provider support (Anthropic + DeepSeek) and judge scoring infrastructure

### What Worked
- Multi-oracle architecture separated deterministic from LLM tests cleanly
- Extension-based language detection fallback enabled marker-free languages like Markdown
- DeepSeek fallback achieved 97.4% pass rate, proving provider-agnostic tool descriptions
- LLM tests gated behind build tags, never blocking merge

### What Was Inefficient
- Quick tasks (C++/Swift/Zig/JS fixtures) were done separately from Phase 20 scenario work — could have been bundled
- REQUIREMENTS.md checkboxes drifted unchecked despite satisfied requirements

### Patterns Established
- Five oracle layers as separate packages under test/oracle/
- Marksman quirk adapter pattern for language-specific LS behavioral hooks

### Key Lessons
- Provider-agnostic tool descriptions pay off — multi-LLM test coverage validates tool design quality
- Quick task workflow is effective for fixture additions that don't warrant full phase planning

## Milestone: v1.5 — Typed Errors & Hardening

**Shipped:** 2026-04-15
**Phases:** 3 | **Plans:** 12

### What Was Built
- `internal/errors/` package with 7 error kinds, builder pattern, JSON serialization, cause-chain wrapping
- Full migration of all 38+ MCP tools from raw `fmt.Errorf` to typed `serr.New`/`serr.Wrap` across 10 packages
- Inline input validation at all 24 kernel tool handlers (empty-string checks before workspace/LS work)
- Kind-level test assertions with `extractKind` helper and 4 typed golden files

### What Worked
- Clean 3-phase decomposition: taxonomy → migration → testing was a natural dependency chain
- Builder pattern for errors (`.WithTool()`, `.WithDetail()`) made migration mechanical across packages
- Research phase identified the right error kind set upfront — no mid-milestone taxonomy changes
- 8-plan parallel migration (Phase 23) was effective — each package independent

### What Was Inefficient
- REQUIREMENTS.md checkboxes never checked despite all requirements being satisfied (tracking gap, caught by audit)
- Summary one-liner extraction from gsd-tools produced garbled output (tool parsing bug)
- 3 golden files deferred because error conditions can't be triggered deterministically without live LS

### Patterns Established
- Inline validation before workspace check — fail fast on invalid params, avoid LS startup for bad input
- `extractKind` helper for test assertions — handles both SDK validation and inline validation error formats
- Detail string flattening for complex metadata (CircuitOpen language/failures/backoff → single string)

### Key Lessons
- Error taxonomy design should happen once, upfront — the 7-kind set proved sufficient for all 38+ tools
- Mechanical migrations (same pattern across many files) benefit from high plan parallelism
- Golden file coverage is limited by testability — some error paths require live infrastructure

### Cost Observations
- 12 plans completed in ~1 day
- Highly parallel Phase 23 (8 plans) was the bulk of the work
- Sessions: ~3

## Milestone: v1.9 — Polish & Infra

**Shipped:** 2026-05-03
**Phases:** 12 (46–56, including emergent 51.1) | **Plans:** 51 | **Commits:** 326
**Files changed:** 517 (+57,759 / -3,120 LOC) | **Timeline:** 7 days (2026-04-24 → 2026-05-01)

### What Was Built
- All 4 known LSP/tooling bugs closed (BUG-01..BUG-04): repomap PageRank starvation on polyglot workspaces (Phase 46), rust-analyzer rename via experimental/serverStatus readiness + RenameOverride QuirkAdapter (Phase 47), jdtls warm cache for `go test ./...` Java integration (Phase 48), single canonical `GrammarRegistry` (Phase 49)
- Go 1.25 + gopls green on `ubuntu-latest`; benchmark harness converted to local-only — `bench.yml` / `capture-baseline.yml` / `*-github-hosted.txt` baselines all removed (Phase 50)
- Reproducible multi-arch signed release pipeline via goreleaser — 6 archives × darwin/linux/windows × amd64/arm64 with minisign signing (Phase 51)
- CGO=0 build path via `//go:build cgo` stubs across treesitter/repomap/edit; daemon refuses CGO=0 with remediation (Phase 51.1, emergent)
- **Product rename `serena → helix` as a hard-cut breaking change**: binary, module path `github.com/agenthands/helix`, env vars `SERENA_* → HELIX_*`, config dir `~/.serena/ → ~/.helix/`, MCP server identity (Phase 52)
- In-binary self-upgrade — `helix update` (read-only) + `helix upgrade` (minisign verify + atomic swap + downgrade refusal + daemon-aware re-launch) (Phase 52)
- EMBED-AUDIT.md manifest classifying every runtime asset; build-time-synced `minisign.pub` embed with CI gate (Phase 52)
- 5 new Prometheus metric families with bounded labels (lspool/repomap cache hit-rate, repomap extract latency histogram, session lifecycle, edit outcomes); cardinality test enforced (Phase 53)
- 2 Grafana dashboards (`helix-overview.json`, `helix-engine.json`) + 4 runbooks (ErrCircuitOpen, deadline-timeouts, ls-crash-restart, memory-pressure-eviction); registry-driven PromQL validator (Phase 54)
- Per-MCP-tool + per-outbound-LS-call trace coverage; TRACE-AUDIT.md hygiene review (no PII, bounded cardinality); real Jaeger smoke trace captured (Phase 55)
- LS notification dispatch wired in production: `jsonrpc.Conn.OnNotification` set in `Worker.Start` with regression assertion; `JdtlsAdapter.WaitUntilJavaReady(ctx)` deterministic gate (Phase 56, emergent)

### What Worked
- **One phase per bug** for BUG-01..BUG-04 — small blast radius, easy rollback, clear ownership per LS quirk; all 4 closed cleanly
- **Mechanical perl rewrite + go build verification gate** for the module path rename — gopls rename does not operate on module paths; layered build/vet/test caught misses across 200+ files
- **Build-time embed-copy with CI gate** for `minisign.pub` — placeholder pubkey fails loudly at release time, can't accidentally ship
- **Registry-driven PromQL validator (fail-closed)** for dashboards/runbooks — every PromQL expression must reference a real registered metric, enforced in `go test`
- **OBS-03 metrics before OBS-01/02 dashboards** — natural dependency ordering, dashboards never reference nonexistent metrics
- **Phase 51.1 inserted emergently** when DEF-51-01 (CGO=0 cross-compile) blocked goreleaser SC-1 — small, focused phase rather than rolling fix into Phase 51
- **Phase 56 inserted emergently** after Phase 55 trace audit surfaced that `jsonrpc.Conn.OnNotification` was never wired in production — audit found a real prod bug, not just doc drift

### What Was Inefficient
- **Phase 51 needed 4 gap-closure plans (51-03..51-06)** after `gaps_found` audit — original 2-plan scope underestimated CGO + signing + CI hardening
- **Phase 52 rescope mid-milestone** (PKG-02/03/04 → self-contained-binary + self-upgrade) was the right call but cost discussion-phase time; could have been caught in `/gsd-discuss-phase` for v1.9 if the binary-first story had been articulated upfront
- **Multiple SUMMARY.md decision entries logged with `[Phase ?]`** instead of phase number — STATE.md decision log entries 79–91 (the entire Phase 52 sub-decision set) lost phase attribution; structural problem in how SUMMARY → STATE flows
- **Protobuf rawDesc invalidated by perl rewrite** during Phase 52-02 module rename — required `make proto` follow-up; should be a documented post-rewrite hook
- **PKG-01 SC-3 was always going to be deployment-gated** but wasn't called out as such until the audit — the success criterion as written was unverifiable in CI by definition; better to mark deployment-gated SCs as such at planning time

### Patterns Established
- **Emergent phases get a sub-decimal numbering** (Phase 51.1, not Phase 56-promoted-from-50.1) — preserves the original phase numbering and keeps the "this came from a discovered gap" semantics readable
- **`//go:build cgo` + `//go:build !cgo` stub pair** for any package that must support both build modes — daemon refuses CGO=0 startup with a clear remediation message; CGO=1 path remains byte-identical
- **CI pre-flight grep gate for placeholder values** (`release.yml:25-37` PLACEHOLDER detection) — turns deployment misconfiguration into a blocking failure rather than a silent bad release
- **Registry-driven validators (PromQL, trace coverage)** as fail-closed `go test` gates — compile-time-equivalent guarantees for dashboards/runbooks/spans
- **Asset-name template constant pinned with parity test against `.goreleaser.yaml`** — drift detected at PR-review time
- **Single canonical error literal at all return sites** (Phase 51 "signature verification FAILED") — keeps grep-based CI gates auditable as a guard against future refactors
- **Test fixtures use `https://example.invalid/...` per RFC 6761** — tests that miss the httptest stub fail loudly with DNS errors instead of silently leaking the runner IP

### Key Lessons
- **Verification audits find real bugs** — Phase 55 trace audit surfaced Phase 56's production LS-dispatch bug. Audit work is feature work.
- **Mark deployment-gated success criteria as such at planning time** — PKG-01 SC-3 was never engineering-completable; calling it deployment-gated upfront would have saved audit cycles
- **Hard-cut renames are easier than transition periods** — `serena → helix` shipped clean as a breaking-change with a CHANGELOG `### Breaking Changes` section; trying to support both would have been months of compat code
- **Small emergent phases (Phase 51.1, Phase 56) > rolling unbounded scope into the parent phase** — easier to verify, audit, and roll back
- **Local-only benchmark rule must be enforced architecturally** — Phase 50 had to delete CI bench plumbing accumulated since v1.2; the rule existed but the codebase had drifted
- **Forwarder-level tracing is genuinely architectural** — Phase 55 forwarder.tools.call deferral to v1.10 isn't an implementation problem, it's a v1.2 boundary decision (gRPC server span as root) that needs revisiting

### Cost Observations
- 51 plans across 12 phases in 7 days — by far the largest milestone by plan count and LOC
- Phase 52 (rename + self-upgrade + embed-audit) and Phase 56 (LS dispatch + jdtls) were the heaviest individual phases
- 2 emergent phases (51.1, 56) added ~25% to original phase count (from 10 planned to 12 shipped)
- Audit work (`/gsd-audit-milestone`, `/gsd-eval-review`, integration check) consumed ~20% of milestone wall-clock — well-spent given Phase 56 discovery

## Milestone: v2.2 — Agent-Facing Skill Quality & Prompt Tuning

**Shipped:** 2026-06-24
**Phases:** 4 (103–106) | **Plans:** 5

### What Was Built
Closed-set skill bundle (ships only `{SKILL.md, reference.md}`) + non-vacuous exact-count (==50) reference contract (103); per-verb `cmd/helix-refgen` override map fixing the generated-`reference.md` group-collapse, 10 verbs corrected (104); hand-authored `SKILL.md` decision-matrix rewrite — QUERY/ACTION split, "Not this" on every row, indexed-graph prereqs, capability grouping (105); and a quarantined dev-time DSPy offline-tuning spike with a parity-pinned Python scorer, overfit/leakage guards, and a `make vet` import-boundary analyzer — concluding a documented **no-ship** (106). Zero new Go deps; single binary stays 100% Python-free.

### What Worked
- **Code review as a real correctness gate, not a formality.** The biggest wins came from the review step *catching what the plan-checker passed*: a vacuous Guard B in 104 (a `x == x` tautology masquerading as a break-the-invariant test) and a genuine Python↔Go parity bug in 106 (`if/elif` prompt-stripping vs Go's two unconditional `TrimPrefix` calls, invisible because the corpus lacked a `$ > ` case). Both were found by **mutation-testing the guard itself**, not by reading it.
- **Folding review+fix BEFORE verification** (vs after) meant the verifier validated post-fix code, and every fix was re-confirmed green + the anti-vacuity guards mutation-tested independently by the verifier.
- **Research-grounded planning on the spike.** 106's deep research (the exact `scorecard.go` classifier, GEPA selection, the inverted-`ablationleakage` analyzer pattern) made a 13-file phase land cleanly with all invariants holding.

### What Was Inefficient
- **The `## Open Questions (RESOLVED)` doc-gate fired as a blocker/warning on 3 of 4 phases** — a convention the researcher didn't apply, forcing an orchestrator touch-up each time. Cheap to fix, but recurring.
- **VALIDATION.md `nyquist_compliant` draft flag** had to be flipped manually per phase (the planner left the template default).
- **Phase 103 shipped in a prior session without a VERIFICATION.md**, surfacing as an audit gap that needed a retroactive verifier pass at close.

### Key Lessons
- **A break-the-invariant test must itself be mutation-tested.** A guard named `TestOverrideDiffersFromGroupDefault` can still be a tautology; the only proof it bites is to break the invariant and watch it go RED. The plan-checker reviews test *structure/intent*; only execution-time mutation catches a vacuous discriminator. In an anti-vacuity milestone, the code-review + verifier mutation passes are the load-bearing gate.
- **A point-wise golden corpus proves agreement only on the points it pins.** The 106 parity bug lived in the gap between corpus cases; a "verbatim port" claim is only as strong as the corpus's domain coverage — scope the claim or widen the corpus.
- **No-ship is a real deliverable.** The DSPy spike's honest "MinTasks=5 is too small to trust" conclusion, documented in a committed REPORT.md, is a success-meeting outcome — the value was the parity-pinned, quarantined harness + the guards, not a shipped optimization.

### Cost Observations
- 4 phases, 5 plans; per phase a full pipeline ran (research → pattern-map → plan → plan-check → execute → code-review+fix → verify), plus the milestone lifecycle (audit → integration-check → complete).
- Code-review caught 2 genuine defects (1 per the two code-heavy phases) that would otherwise have shipped — the review spawns paid for themselves.
- Run on Opus 4.8 (1M context) end-to-end autonomously.

## Milestone: v2.3 — Task-Success-Driven Skill Optimization

**Shipped:** 2026-06-24
**Phases:** 4 (107–110) | **Verdict:** NO-SHIP by design

### What Was Built
A dev-time DeepSeek/OpenAI ReAct agent driving the real `helix` CLI via subprocess with a first-class steering ON/OFF switch (107); an honest Aider-polyglot task-success oracle (0-tests=hard-ERROR, anti-tamper gold-test restore) that swapped the gameable `choice_rate` GEPA reward for real task-success behind a sequestered `val_size>50` split (108); the heaviest grader — SWE-bench via the upstream `swebench==4.1.0` harness on Podman, a parity mirror of `harness.go` — plus an ON-vs-OFF attribution delta with per-arm cost (109); and human-gated `helix-refgen --check` adoption + end-to-end boundary re-verification + the ship/no-ship REPORT (110). Zero new Go deps; the whole pipeline is dev-venv Python under `tools/dspy-tune/`.

### What Worked
- **Honest gating over a shippable-looking result.** The milestone reached the *same* NO-SHIP conclusion as v2.2 — but this time through real task-success machinery, not the gameable proxy. The `val_size>50` gate (a single source of truth shared between `optimize.py` and `attribution.py`) refused to fabricate a delta on a 3-task corpus.
- **Parity mirrors pinned on both sides.** `grade_swebench.py` ↔ `harness.go` (via `golden/swebench_argv.json` + `argv_parity_test.go`) reused the exact discipline that caught the v2.2 parity bug — no silent Go/Python drift.
- **Mutation-testing the new guards before sign-off.** Three Phase-109 guards (vacuous-pass refusal, resolution contract, val_size gate) were each broken → confirmed RED → reverted; the Phase-110 adoption gate was proven by desyncing `reference.md` and watching `helix-refgen --check` exit 1.
- **Inline execution adapted cleanly** when the gsd-* executor/verifier subagents turned out to be absent from the roster — phases 107–110 + the lifecycle all ran in the main context, producing the identical artifacts.

### What Was Inefficient
- **The roster mismatch** (skills assume `gsd-planner`/`gsd-executor`/`gsd-verifier` that aren't installed) meant the autonomous flow couldn't dispatch subagents; it degraded to inline execution. Correct outcome, but the skills' background-dispatch paths were dead weight.
- **The milestone-complete CLI is a partial handler** (again): it under-counted plans/tasks in the MILESTONES entry and left ROADMAP/REQUIREMENTS/STATE for the orchestrator to finish — a known recurring touch-up.
- **The open-artifact audit false-positive recurred** — it flagged a CONTEXT "Open questions resolved at implementation time" section as open questions (same class as the prior backtick/config false positive).

### Key Lessons
- **NO-SHIP, reached honestly twice, is a finding — not a stall.** The corpus size (`val_size>50`) is the real blocker; until TUNE-FUT-01 grows it, no benchmark plumbing changes the verdict. Building the full gated pipeline so the *next* corpus can be judged trustworthy is the deliverable.
- **A re-verification phase still needs evidence, not assertion.** Phase 110 added no production code, but every ADOPT-03/04 claim was backed by a live command (`go.mod` last-touch commit, `make vet`, the desync exit-1 demo) — "already true" is only credible when re-proven.

### Cost Observations
- 4 phases executed inline with `uv` (no subagent fan-out); the milestone lifecycle (audit → complete → cleanup) also ran inline.
- 51 dev-venv pytest + Go parity/refgen tests as the green bar; `make vet` (incl. `toolsquarantine`) as the boundary gate.
- Run on Opus 4.8 (1M context) end-to-end autonomously.

## Cross-Milestone Trends

| Metric | v1.0 | v1.1 | v1.2 | v1.3 | v1.4 | v1.5 | v1.9 |
|--------|------|------|------|------|------|------|------|
| Phases | 5 | 3 | 7 | 2 | 4 | 3 | 12 |
| Plans | 20 | 11 | 25 | 4 | 11 | 12 | 51 |
| Duration | 2 days | 1 day | 2 days | 1 day | 4 days | 1 day | 7 days |
| Go LOC (cumulative) | ~26K | ~30K | ~36K | ~36K | ~41K | ~48K | ~70K+ |
| Test coverage focus | Unit | Integration | Benchmark + E2E | Documentation | Oracle tests | Error contracts | Trace coverage + dashboards/runbooks |
| Emergent phases | 0 | 0 | 0 | 0 | 0 | 0 | 2 (51.1, 56) |
