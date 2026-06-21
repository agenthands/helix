# Phase 76: Ablation Profiles + Kernel Subsystem Disable Flags - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-16
**Phase:** 76-ablation-profiles-kernel-subsystem-disable-flags
**Areas discussed:** Profile↔flag coupling, Disabled-tool behavior, vet-ablation-leakage strategy, no_lsp no-op enforcement, no_semantic profile content

---

## Profile ↔ kernel-flag coupling

| Option | Description | Selected |
|--------|-------------|----------|
| YAML declares flags | New profile fields; daemon reads at profile-resolve and configures the kernel. One reviewable artifact per arm. | |
| Profiles pure filters; daemon flag separate | Profile only filters tools; kernel flags set via daemon CLI/env at subprocess launch. Two wirings to keep in sync. | |
| Hybrid: YAML + CLI override | YAML is source of truth, daemon also accepts a CLI override for ad-hoc tests. | ✓ |

**User's choice:** Hybrid: YAML + CLI override
**Notes:** Precedence recorded as CLI > profile YAML > default-off, matching the project's existing 4-layer config precedence (CLAUDE.md). Default state = subsystem ENABLED; flags are opt-in disables. Resolution at the daemon composition root (D-01/D-02/D-03).

---

## Disabled structured-edit tool behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Both: filtered + kernel guard | Profile excludes from tools/list (ABLATE-07) AND kernel flag returns typed Unsupported if reached (roadmap #3). Defense-in-depth. | ✓ |
| Excluded from tools/list only | Pure profile filter; nothing ever returns Unsupported (roadmap #3 satisfied only vacuously). | |
| Present-but-Unsupported only | Tool stays listed, returns Unsupported; conflicts with ABLATE-07 "inventory excludes" wording. | |

**User's choice:** Both (recommended) — reconciles the ABLATE-07 ↔ roadmap-#3 tension. `replace_in_file` falls through to exact-match only under the flag (D-04/D-05). Error-kind name left as Claude discretion (D-06).

---

## vet-ablation-leakage strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Import-boundary check | Like noduckdb/nokernel2semantic: assert ablation-gated packages don't import disabled-subsystem packages; testdata violation. Compile-time scope. | ✓ |
| Call-site guard verification | Assert every subsystem entry point is preceded by a flag check. More precise, much harder in go/analysis. | |
| Entry-point registry + guard | Registry of entry points, each must have a paired disable-guard. Middle ground; registry to maintain. | |

**User's choice:** Import-boundary check (recommended) — matches the house lint pattern and Phase 75 D-08 philosophy. Honest scope recorded: catches compile-time import leakage, not every runtime path; kernel Unsupported guard is the runtime backstop (D-07/D-08).

---

## no_lsp no-op enforcement

| Option | Description | Selected |
|--------|-------------|----------|
| Null-object inject at daemon init | No-op EditNotifier, skip SetEnrichFn, no LS pool injection → structurally zero lsp.* spans. Centralized at composition root. | ✓ |
| Guard-at-each-callsite | Kernel flag checked at lspProbeForEdges/OnEdit/SetEnrichFn sites. Explicit but scattered; easy to miss a site. | |
| Don't start LS pool at all | Hardest structural guarantee; risk of breaking degrade-gracefully paths expecting a pool. | |

**User's choice:** Null-object injection at daemon init (recommended). Planner must audit pool-nil/notifier-nil degrade paths (D-09/D-10).

---

## no_semantic profile content (Phase 76 vs Phase 81 boundary)

| Option | Description | Selected |
|--------|-------------|----------|
| Tool-filter now, kernel flag in 81 | bench-no-semantic.yaml excludes semantic-store-backed P1 tools at the profile level now; kernel disable_semantic_subsystem flag deferred to Phase 81. | ✓ |
| Structural placeholder only | Near-clone of bench-full that passes the golden test, no tool filtering; arm not meaningful until 81. | |

**User's choice:** Tool-filter now (recommended) — makes the arm meaningful and testable immediately; kernel back-channel guard (ABLATE-06) added in Phase 81. Exact excluded-tool list is researcher/planner work (D-11/D-12).

---

## Claude's Discretion

- The typed `Unsupported` error-kind constant name + documentation location (D-06).
- CLI override flag naming/shape for the kernel disables (D-01).
- Internal kernel config-struct threading for the two flags (D-03).
- Exact semantic-store-backed tool list excluded by `bench-no-semantic.yaml` (D-12).
- Golden-test structure / how the 4 bench YAMLs share common config.

## Deferred Ideas

- `disable_semantic_subsystem` kernel flag (ABLATE-06) → Phase 81 (Phase 65 un-wiring dependency).
- Call-site guard verification in vet-ablation-leakage → only if a real runtime leak appears.
- `baseline_plain` (ABLATE-03, Phase 80), `baseline_rag` (ABLATE-04, Phase 83) → other phases.
