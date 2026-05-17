# Phase 71: P1 Single-Symbol Read Tools — Pattern Map

**Mapped:** 2026-05-17
**Files analyzed:** 14 (3 new handlers + 4 modify-in-place + 2 store + 5 tests)
**Analogs found:** 14 / 14 (every new file has a precise in-tree analog)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/skill/semantic/tools_explain_symbol.go` | handler (MCP tool) | request-response (read) | `internal/skill/semantic/tools_context.go` | exact |
| `internal/skill/semantic/tools_find_related.go` | handler (MCP tool) | request-response (read, ranked) | `internal/skill/semantic/tools_context.go` | exact |
| `internal/skill/semantic/tools_validate_edge.go` | handler (MCP tool) | request-response (read, envelope-rich) | `internal/skill/semantic/tools_status.go` | exact |
| `internal/skill/semantic/edge_kind_surface.go` | constants + mapper (utility) | transform (string→enum) | `internal/skill/semantic/envelope.go` (closed-enum block) | role-match |
| `internal/skill/semantic/seed_resolve.go` | utility (shared helper) | transform (input→ResolvedSeed) | `internal/skill/semantic/handler_helpers.go` (`validatePaths`) + resolution switch in `tools_context.go` | role-match |
| `internal/skill/semantic/envelope.go` (MODIFY) | model (response shapes) | request-response | existing `CommonEnvelope` + `RetrievalStatus` block | self-extend |
| `internal/skill/semantic/accessors.go` (MODIFY) | model (interface declarations) | seam | existing `StoreAccessor`/`SchedulerAccessor`/`RetrievalAccessor` interfaces | self-extend |
| `internal/skill/semantic/skill.go` (MODIFY) | provider (post-init setters) | wiring | existing `SetStore`/`SetScheduler`/... setter block | self-extend |
| `internal/skill/semantic/register.go` (MODIFY) | wiring (MCP registration) | request-response | existing `RegisterAll` body | self-extend |
| `internal/semantic/store/effective_graph.go` (MODIFY: add `QuerySymbolByName`) | model (SQL accessor) | CRUD (read-only) | `Store.QuerySymbolByLocation` (same file lines 537-582) | exact |
| `internal/semantic/store/*.go` (MODIFY: add `LatestExtractorRunID`) | model (SQL accessor) | CRUD (read-only) | `Store.LatestCommittedSnapshot` (same file line 385) | role-match |
| `internal/skill/semantic/tools_*_test.go` (3 new test files) | test (table-driven + recorder) | test fixtures | `internal/skill/semantic/tools_refresh_test.go` | exact |
| `internal/skill/semantic/seed_resolve_test.go` | test | test fixtures | `tools_refresh_test.go` mock construction | role-match |
| `internal/skill/semantic/edge_kind_surface_test.go` | test (table-driven round-trip) | test fixtures | `internal/skill/semantic/envelope_test.go` | role-match |
| `internal/skill/semantic/populated_graph_fixture_test.go` | test fixture builder | test fixtures | (none — multi-language graph fixture is new in 71) | no analog |
| `internal/skill/semantic/integration_test.go` (EXTEND with `TestThreeTools_*`) | test (integration) | cross-tool | existing `integration_test.go` cases | self-extend |
| `internal/semantic/store/effective_graph_test.go` (EXTEND) | test | CRUD | existing `TestQuerySymbolByLocation*` and `TestLatestCommittedSnapshot*` | self-extend |

## Pattern Assignments

### `internal/skill/semantic/tools_explain_symbol.go` (handler, request-response)

**Analog:** `internal/skill/semantic/tools_context.go`

**Imports pattern** (tools_context.go:1-17):
```go
package semantic

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"

	"github.com/agenthands/helix/internal/guardrails"
	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/semantic/retrieval"
)
```

**Typed-args struct + help const pattern** (tools_context.go:31-104):
```go
const contextHelp = `## Usage Examples
...
## Mode Tier
read+ — every session passes; this is the read-tier semantic retrieval
front door for agents.

## Determinism
Same (task, files, symbols, max_tokens) inputs produce byte-identical
candidate ordering across runs ...`

type GetSemanticContextArgs struct {
	Task          string   `json:"task,omitempty"           jsonschema:"natural-language task description"`
	Files         []string `json:"files,omitempty"          jsonschema:"anchor files for personalized PageRank"`
	Symbols       []string `json:"symbols,omitempty"        jsonschema:"anchor symbol IDs"`
	MaxTokens     int      `json:"max_tokens,omitempty"     jsonschema:"token budget (default 2048; clamped to [64, 32768])"`
	FreshnessMode string   `json:"freshness_mode,omitempty" jsonschema:"allow_stale | require_current | validate_live"`
}
```

**Register-function pattern** (tools_context.go:110-124) — copy verbatim, rename only:
```go
func registerGetSemanticContext(server *mcp.SerenaMCPServer, s *SemanticSkill, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "get_semantic_context",
		Description: "Ranked, evidence-backed semantic context (read+).",
	}, kernel.WrapToolSpan(tracer, "get_semantic_context",
		func(ctx context.Context, req *mcpsdk.CallToolRequest, args GetSemanticContextArgs) (*mcpsdk.CallToolResult, any, error) {
			return s.handleGetSemanticContext(ctx, args), nil, nil
		}))
	server.Registry().Register(&mcp.ToolDef{
		Name:             "get_semantic_context",
		Description:      "Ranked, evidence-backed semantic context (read+).",
		BriefDescription: "Semantic context retrieval",
		HelpText:         contextHelp,
	})
}
```

**Handler entry pattern (mode check + workspace + path validation)** (tools_context.go:151-163):
```go
func (s *SemanticSkill) handleGetSemanticContext(ctx context.Context, args GetSemanticContextArgs) *mcpsdk.CallToolResult {
	// 1. Mode-tier check (read+ — every session passes; retained for
	//    code-review visibility and Phase 66 GuardrailMiddleware precedent).
	snap := s.sessionSnapshot(ctx)
	if err := checkMode(snap, modeTierRead); err != nil {
		return errorResult(err.Error())
	}

	// 2. Resolve workspace + path-traversal validation (T-64-07-01).
	ws := s.workspaceKey(ctx)
	if err := validatePaths(args.Files, ws.RepoRoot); err != nil {
		return errorResult(err.Error())
	}
```

**Fan-out accessor read with per-accessor tolerated errors** (tools_context.go:194-210):
```go
if s.store != nil {
	if gv, err := s.store.CurrentGraphVersion(ctx, repoID); err == nil {
		graphVersion = gv
	} else if s.logger != nil {
		s.logger.Warn("get_semantic_context: CurrentGraphVersion read failed",
			"repo_id", repoID, "err", err)
	}
	overlayActive = s.store.OverlayHasPendingRows(repoID)
}
```

**Receipt issuance on success path (Phase 66 GUARD-03)** (tools_context.go:313-326):
```go
guardrails.IssueReceiptOnSuccess(ctx, guardrails.ClassContextGathered,
	guardrails.ContextGatheredScope{
		FileSet:         args.Files,
		TargetSymbols:   targetSymbols,
		TaskHash:        computeSemanticTaskHash(args),
		TokenBudgetUsed: len(candidates),
		MaxTokens:       budget,
	}, "get_semantic_context")
return result
```

---

### `internal/skill/semantic/tools_find_related.go` (handler, request-response)

**Analog:** `internal/skill/semantic/tools_context.go` (most lines copy verbatim — the fusion entrypoint is identical, only the post-fuse cluster-boost and `paths`-filter steps are new).

**Reuse the exact fusion pattern** (tools_context.go:226-264) — `find_related_symbols` uses the same RRF entrypoint:
```go
// 7. Fan out QueryBleve + PersonalizedPageRank.
anchors := append(append([]string{}, args.Files...), args.Symbols...)

var textRanksRaw []TextRank
var graphRanksRaw []GraphRank
if s.retrieval != nil {
	if tr, err := s.retrieval.QueryBleve(args.Task, anchors); err == nil {
		textRanksRaw = tr
	} ...
	if gr, err := s.retrieval.PersonalizedPageRank(ctx, repoID, anchors); err == nil {
		graphRanksRaw = gr
	} ...
}

// Translate skill-package TextRank/GraphRank → retrieval-package types ...
gvLookup := func(symbolID string) uint64 { return graphVersion }
fused := retrieval.Fuse(textForFuse, graphForFuse, retrieval.DefaultRRFConfig(), gvLookup)
```

**Clamp pattern for k** — mirrors token-budget clamp (tools_context.go:166-175):
```go
budget := args.MaxTokens
if budget <= 0 {
	budget = retrieval.DefaultContextBudget
}
if budget < retrieval.MinTokenBudget {
	budget = retrieval.MinTokenBudget
}
if budget > retrieval.MaxTokenBudget {
	budget = retrieval.MaxTokenBudget
}
```

For `find_related_symbols.k`: default 20, clamp [1, 100] — identical shape, different constants.

---

### `internal/skill/semantic/tools_validate_edge.go` (handler, envelope-rich)

**Analog:** `internal/skill/semantic/tools_status.go`

**Closed-enum status fan-out + freshness selection** (tools_status.go:120-232) — copy structure for evidence assembly:
```go
func (s *SemanticSkill) handleGetSemanticGraphStatus(ctx context.Context, _ GetSemanticGraphStatusArgs) *mcpsdk.CallToolResult {
	// 1. Mode-tier check (read+ ...).
	snap := s.sessionSnapshot(ctx)
	if err := checkMode(snap, modeTierRead); err != nil {
		return errorResult(err.Error())
	}

	// 2. Resolve workspace + repoID.
	ws := s.workspaceKey(ctx)
	repoID := ws.Hash()

	// 3. Fan-out accessor reads ... per-accessor nil-guarded ...

	// 4. Closed-enum freshness selection (SPEC §26.2).
	var freshness Freshness
	switch {
	case retrievalPending:
		freshness = FreshnessStale
	case overlayActive && pendingLSPFiles > 0:
		freshness = FreshnessStructurallyFreshSemanticallyPending
	case overlayActive:
		freshness = FreshnessOverlayActive
	default:
		freshness = FreshnessFresh
	}
	// 5. Marshal envelope (SPEC §23.3).
	return jsonResult(StatusResult{ ... })
}
```

`validate_graph_edge` mirrors this same shape, replacing the freshness switch with the **confidence cap selector** from D4 + Phase 62 TYPES-04. Evidence assembly is the new logic; envelope marshalling is identical.

---

### Shared: D-09 Read-Only Invariant Header (REQUIRED on all 3 new handler files)

**Source:** `internal/skill/semantic/tools_refresh.go:15-23` — copy this header comment block to the top of each of `tools_explain_symbol.go`, `tools_find_related.go`, `tools_validate_edge.go` (adjust the tool name):

```go
// INVARIANT (D-09 / D-13): explain_symbol_deep MUST NOT touch the
// snapshot-write surface of *Store. Specifically: no Begin/Commit/Abort/Write
// methods on snapshots, and no compactor flush trigger. The grep gate in CI
// enforces the absence of those identifier tokens in this file; the recorder
// mocks in tools_explain_symbol_test.go enforce it under unit test. Read+
// stays read-only with respect to committed state.
```

---

### `internal/skill/semantic/edge_kind_surface.go` (NEW, closed enum + mapper)

**Analog:** `internal/skill/semantic/envelope.go` (closed-enum convention)

**Closed-enum constant block pattern** (envelope.go:14-32):
```go
type Freshness string

const (
	FreshnessFresh Freshness = "fresh"
	FreshnessStale Freshness = "stale"
	FreshnessStructurallyFreshSemanticallyPending Freshness = "structurally_fresh_semantically_pending"
	FreshnessOverlayActive Freshness = "overlay_active"
)
```

Copy this shape for `EdgeKindSurface` (research §"Edge kind surface mapping" already drafts it verbatim — see 71-RESEARCH.md lines 319-351). The `MapInternalKind` function is a plain `switch` on internal string with `default: return EdgeKindOther`.

---

### `internal/skill/semantic/seed_resolve.go` (NEW, shared helper)

**Analog:** `internal/skill/semantic/handler_helpers.go` (`validatePaths` — same role: input validation/translation shared across handlers)

**Helper-file convention** (handler_helpers.go:46-67):
```go
// validatePaths rejects path-traversal attempts and absolute paths outside the
// workspace root. ...
func validatePaths(paths []string, root string) error {
	for _, p := range paths {
		if strings.Contains(p, "..") {
			return serr.New(serr.InvalidArgs,
				fmt.Sprintf("path %q contains '..' (path traversal not allowed)", p))
		}
		if filepath.IsAbs(p) {
			if root == "" || !strings.HasPrefix(p, root) {
				return serr.New(serr.InvalidArgs,
					fmt.Sprintf("path %q is outside workspace root", p))
			}
		}
	}
	return nil
}
```

Same `serr.New(serr.InvalidArgs, ...)` shape applies to seed resolution errors for malformed input. Output struct shape mirrors `Resolution`/`ResolvedSeed` already drafted in 71-RESEARCH.md lines 261-303.

---

### `internal/skill/semantic/envelope.go` (MODIFY: add FreshnessV2 + Seed envelope)

**Analog:** existing `ClusterStatus` / `RetrievalStatus` structs in same file

**Struct pattern with `omitempty` for additive extension** (envelope.go:68-85):
```go
type ClusterStatus struct {
	State string `json:"state"`
	Reason string `json:"reason,omitempty"`
	ComputedAt int64 `json:"computed_at,omitempty"`
	MemberCount int `json:"member_count,omitempty"`
}
```

Use this shape for `FreshnessV2` (with new fields `SnapshotID`, `ExtractorRunID`, `AsOfUnixMs`, `Status` — all `omitempty` to preserve Phase 64 envelope shape for existing tools).

---

### `internal/skill/semantic/accessors.go` (MODIFY: add 3 narrow interfaces)

**Analog:** existing narrow interfaces in same file

**Narrow-interface convention** (accessors.go:71-79):
```go
// QueueAccessor is the narrow seam to Phase 61's LSPQueue.
type QueueAccessor interface {
	// DepthAll returns the total number of pending LSP enrichment items
	// across all lanes for the given workspace.
	DepthAll(ws workspace.WorkspaceKey) int
	// LastEnqueueAt returns the unix-millis timestamp of the most-recent
	// enqueue for the given workspace, or 0 if the queue has been idle.
	LastEnqueueAt(ws workspace.WorkspaceKey) int64
}
```

Each Phase 71 accessor is its own minimal interface:
- `SymbolByNameAccessor` — 1 method: `QuerySymbolByName(ctx, repoID, path, name) ([]integ.SymbolID, error)`
- `ExtractorRunAccessor` — 1 method: `LatestExtractorRunID(ctx, repoID) (string, error)`
- `ClusterMembershipAccessor` — 1 method: `ClusterIDOf(ctx, repoID, symbolID) (clusterID uint64, size int, err error)`

Each gets a corresponding `Set*` post-init setter in `skill.go` mirroring the SetStore/SetScheduler block (skill.go:60-115). All setters use the same `s.mu.Lock(); defer s.mu.Unlock()` shape.

---

### `internal/skill/semantic/register.go` (MODIFY: extend `RegisterAll`)

**Analog:** existing `RegisterAll` body (register.go:19-27)

**Extension pattern** — append three new `register*` calls inside `RegisterAll`, in declaration order:
```go
func RegisterAll(server *mcp.SerenaMCPServer, s *SemanticSkill, tracer trace.Tracer) {
	if server == nil || s == nil {
		return
	}
	registerIndexSemanticGraph(server, s, tracer)
	registerRefreshSemanticGraph(server, s, tracer)
	registerGetSemanticGraphStatus(server, s, tracer)
	registerGetSemanticContext(server, s, tracer)
	// Phase 71 additions:
	registerExplainSymbolDeep(server, s, tracer)
	registerFindRelatedSymbols(server, s, tracer)
	registerValidateGraphEdge(server, s, tracer)
}
```

---

### `internal/semantic/store/effective_graph.go` (MODIFY: add `QuerySymbolByName`)

**Analog:** `Store.QuerySymbolByLocation` in same file (lines 537-582)

**Exact pattern to copy** — same nil guard, `LatestCommittedSnapshot` lookup, SQL JOIN shape, `sql.ErrNoRows` → clean miss:
```go
// effective_graph.go:537-582
func (s *Store) QuerySymbolByLocation(
	ctx context.Context, repoID, path string, line, col uint32,
) (string, bool, error) {
	if s == nil || s.db == nil {
		return "", false, errors.New("QuerySymbolByLocation: nil store")
	}
	latest, err := s.LatestCommittedSnapshot(ctx, repoID)
	if err != nil {
		return "", false, fmt.Errorf("QuerySymbolByLocation: %w", err)
	}
	if latest == 0 {
		return "", false, nil
	}
	const q = `
		SELECT sym.stable_key
		  FROM semantic_symbols AS sym
		  JOIN semantic_files AS f
		    ON f.snapshot_id = sym.snapshot_id AND f.file_id = sym.file_id
		 WHERE sym.snapshot_id = ?
		   AND f.path          = ?
		   ...
		 LIMIT 1
	`
	...
}
```

`QuerySymbolByName(ctx, repoID, path, name) ([]string, error)` follows the same shape but:
- Returns `[]string` (not single `string`) — D1 ambiguity needs all matches.
- WHERE clause filters on `sym.name = ?` (or `qualified_name`, planner to confirm column).
- Drop `LIMIT 1`; cap at 6 (so caller can detect `>5` and truncate per D1 cap of 5).

---

### `internal/semantic/store/*.go` (MODIFY: add `LatestExtractorRunID`)

**Analog:** `Store.LatestCommittedSnapshot` (effective_graph.go:385)

The exact existing `LatestCommittedSnapshot(ctx, repoID) (uint64, error)` pattern applies — single-row SELECT over a per-repo monotonic counter or `MAX(run_id)` over a `semantic_extractor_runs` table. Researcher A4 flags this column/table may not yet exist; planner decides storage shape.

---

### Tests — `internal/skill/semantic/tools_*_test.go` (3 new files)

**Analog:** `internal/skill/semantic/tools_refresh_test.go`

**Recorder-style mock pattern with D-09 canary methods** (tools_refresh_test.go:38-50, 113-135):
```go
// recorderStoreAccessor is a recorder-style mock StoreAccessor. The methods
// declared in the interface return injected values; the extra forbidden
// methods (BeginSnapshot et al.) are canary methods whose bodies call t.Fatal
// if ever invoked.
type recorderStoreAccessor struct {
	t *testing.T
	graphVersion    uint64
	overlayHasPending bool
	latestSnapshot  uint64
	...
}

// Forbidden methods — D-09 / D-13 invariants. These methods are NOT part of
// the StoreAccessor interface; they only exist on this recorder type so we
// can fail loudly if a future regression ever reaches them via type assertion.
func (r *recorderStoreAccessor) BeginSnapshot(ctx context.Context, repoID string) (uint64, error) {
	r.beginSnapshotCalls.Add(1)
	r.t.Fatalf("D-09 violation: refresh handler must NOT call BeginSnapshot")
	return 0, nil
}
func (r *recorderStoreAccessor) CommitSnapshot(ctx context.Context, snapID uint64) error {
	r.commitSnapshotCalls.Add(1)
	r.t.Fatalf("D-09 violation: refresh handler must NOT call CommitSnapshot")
	return nil
}
func (r *recorderStoreAccessor) AbortSnapshot(ctx context.Context, snapID uint64) error {
	r.abortSnapshotCalls.Add(1)
	r.t.Fatalf("D-09 violation: refresh handler must NOT call AbortSnapshot")
	return nil
}
func (r *recorderStoreAccessor) WriteSnapshotFacts(ctx context.Context, snapID uint64, facts any) error {
	r.writeSnapshotFactsCalls.Add(1)
	r.t.Fatalf("D-09 violation: refresh handler must NOT call WriteSnapshotFacts")
	return nil
}
```

All three new test files MUST install this same recorder pattern. Adjust the `Fatalf` message to name the handler under test.

**Test imports block** (tools_refresh_test.go:1-16) — copy header:
```go
package semantic

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/skill"
	"github.com/agenthands/helix/internal/workspace"
)
```

---

## Shared Patterns

### Pattern A: Handler entry (mode check → workspace → validation)
**Source:** `internal/skill/semantic/tools_context.go:151-163` and `tools_status.go:120-132`
**Apply to:** all 3 new handler files (`tools_explain_symbol.go`, `tools_find_related.go`, `tools_validate_edge.go`)

```go
snap := s.sessionSnapshot(ctx)
if err := checkMode(snap, modeTierRead); err != nil {
	return errorResult(err.Error())
}
ws := s.workspaceKey(ctx)
repoID := ws.Hash()
```

For tools that accept `paths` filter (`find_related_symbols`): immediately follow with `validatePaths(args.Paths, ws.RepoRoot)`.

### Pattern B: D-09 read-only invariant header
**Source:** `internal/skill/semantic/tools_refresh.go:15-23`
**Apply to:** all 3 new handler files (REQUIRED; CI grep-gate enforces this)

Header MUST be present and MUST NOT mention `Begin/Commit/Abort/Write` snapshot tokens in any non-comment code.

### Pattern C: Recorder-style test mocks with D-09 canary methods
**Source:** `internal/skill/semantic/tools_refresh_test.go:38-50, 113-135`
**Apply to:** all 3 new `tools_*_test.go` files + the optional `seed_resolve_test.go` if it touches a store accessor

### Pattern D: Error envelope shape
**Source:** `internal/skill/semantic/handler_helpers.go:22-29` + `internal/errors.serr.New`
**Apply to:** all 3 new handlers' error-return paths

```go
return errorResult(err.Error())
// for richer errors:
return errorResult(serr.New(serr.InvalidArgs, "seed requires symbol_id OR (file_path AND symbol_name)").Error())
```

### Pattern E: JSON envelope marshalling
**Source:** `internal/skill/semantic/handler_helpers.go:35-45`
**Apply to:** all 3 new handlers' success-return paths

```go
return jsonResult(ExplainSymbolDeepResult{
	CommonEnvelope: CommonEnvelope{...},
	Freshness: freshnessV2,  // new V2 envelope with snapshot_id/extractor_run_id/as_of_unix_ms
	...
})
```

### Pattern F: Narrow accessor interface convention
**Source:** `internal/skill/semantic/accessors.go:71-79` (`QueueAccessor`)
**Apply to:** 3 new accessor interfaces (`SymbolByNameAccessor`, `ExtractorRunAccessor`, `ClusterMembershipAccessor`)

Each interface declares ONLY the methods the Phase 71 handlers consume — no speculative additions.

### Pattern G: Post-init setter convention
**Source:** `internal/skill/semantic/skill.go:60-106`
**Apply to:** 3 new setters added to `skill.go` for the new accessors

```go
func (s *SemanticSkill) SetSymbolByName(a SymbolByNameAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.symbolByName = a
}
```

### Pattern H: Store accessor SQL shape
**Source:** `internal/semantic/store/effective_graph.go:537-582` (`QuerySymbolByLocation`)
**Apply to:** new `QuerySymbolByName` in `effective_graph.go` and new `LatestExtractorRunID` (wherever planner places it)

- Nil guard: `if s == nil || s.db == nil { return ..., errors.New(...) }`
- Lookup `LatestCommittedSnapshot` if reading from snapshot-scoped tables
- Return `sql.ErrNoRows` as clean miss (`return ..., nil, nil` — not an error)
- Wrap real errors with `fmt.Errorf("FuncName(args): %w", err)`

### Pattern I: Closed-enum const block
**Source:** `internal/skill/semantic/envelope.go:14-32, 36-46, 50-60`
**Apply to:** `edge_kind_surface.go` (`EdgeKindSurface`), the new `Resolution` enum in `seed_resolve.go`, the new `FreshnessStatus` (current/stale/unknown) in `envelope.go`

```go
type EnumName string
const (
	EnumNameValue1 EnumName = "value1"
	EnumNameValue2 EnumName = "value2"
)
```

---

## No Analog Found

| File | Role | Reason |
|------|------|--------|
| `internal/skill/semantic/populated_graph_fixture_test.go` | test fixture builder | Researcher A6: no existing multi-language graph fixture in-tree. Phase 64 P07 builder is bleve-recovery-focused, not multi-language. New construction required. Planner should consult `internal/semantic/store/*_test.go` for SQL insertion helpers when building the fixture. |

---

## Metadata

**Analog search scope:**
- `internal/skill/semantic/` (full directory)
- `internal/semantic/store/effective_graph.go` (symbol lookup accessors)
- `internal/semantic/store/scheduler_store.go` (lock-free read context)
- `internal/semantic/integ/` (researcher already covered this in 71-RESEARCH.md §"Sources")

**Files scanned:** 12 (semantic skill package + 2 in `internal/semantic/store/`)

**Pattern extraction date:** 2026-05-17

**Key conventions extracted:**
1. Every Phase 64 tool follows the same 5-element shape: typed-args struct, `const ___Help`, `register___` func, `handle___` method, package-internal helper file. Phase 71 mirrors this exactly.
2. The `D-09 read-only invariant comment` is the CI grep-gate's anchor; mirror it on each new handler file.
3. Recorder-style mocks with `t.Fatal` canary methods provide compile-time + runtime D-09 enforcement; this is the test pattern Phase 71 must replicate.
4. Each accessor interface is narrow (1-4 methods); never extend an existing interface for a Phase 71 need — declare a new one.
5. Store SQL accessors follow a strict shape: nil-guard → `LatestCommittedSnapshot` lookup → SQL → `sql.ErrNoRows` clean-miss handling.
