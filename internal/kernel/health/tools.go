package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/workspace"
)

// SemanticStoreProbe is the kernel-side seam for the semantic store
// health surface. The daemon implements this against
// internal/semantic/store.Store so the kernel package does not import
// internal/semantic. SC-1.
//
// Available reports whether the store is functional (false when
// semantic_index.enabled=false or under CGO=0 / windows-arm64 stub).
// Probe runs a cheap SELECT 1 against the underlying *sql.DB with a
// bounded context; non-nil error → unhealthy.
type SemanticStoreProbe interface {
	Available() bool
	Probe(ctx context.Context) error
}

// SemanticStoreStatus is the JSON-shaped block surfaced inside the
// get_health report. State ∈ {"disabled", "ready", "unhealthy"}.
//
// Reason is a closed enum (WR-NEW-01) — never the raw error text. The
// daemon-side probe wraps its known failure modes; this helper maps them
// onto the values defined by the SemanticReason* constants below before
// the struct is JSON-marshalled into the MCP-exposed get_health envelope.
// Operators get the full underlying error via slog at warn level.
type SemanticStoreStatus struct {
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

// Closed-enum reasons for SemanticStoreStatus.Reason. The set is locked
// to keep the get_health JSON envelope predictable for MCP clients and
// to eliminate any risk of leaking raw database/sql or DuckDB error text
// (WR-NEW-01).
const (
	SemanticReasonProbeTimeout = "probe_timeout"
	SemanticReasonNilHandle    = "nil_handle"
	SemanticReasonDBError      = "db_error"
	SemanticReasonUnknown      = "unknown"
)

// classifySemanticProbeError maps a probe error onto the closed Reason
// enum (WR-NEW-01). Mapping rules:
//   - errors.Is(err, context.DeadlineExceeded) → "probe_timeout"
//   - the daemon-side adapter's nil-handle sentinels → "nil_handle"
//   - any other non-nil error → "db_error"
//
// nil errors return "" so callers can short-circuit on the empty string.
// The full error text is intentionally NOT returned — log it via slog
// for operators; the MCP-exposed Reason carries only the enum.
func classifySemanticProbeError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return SemanticReasonProbeTimeout
	}
	msg := err.Error()
	// The daemon-side semanticStoreProbe.Probe surfaces these two
	// fmt.Errorf strings when the store handle or its underlying *sql.DB
	// is nil. Substring-match because they are wrapped by Probe's caller.
	if strings.Contains(msg, "DB handle nil") || strings.Contains(msg, "store unavailable") {
		return SemanticReasonNilHandle
	}
	return SemanticReasonDBError
}

// ComputeSemanticStoreStatus runs a bounded probe against the semantic
// store and returns the {disabled, ready, unhealthy} status block. SC-1.
//
// The probe budget is 1s — short enough to never dominate get_health
// latency, long enough to absorb a one-off DuckDB stutter. A timed-out
// probe returns "unhealthy" with reason "probe_timeout".
//
// WR-NEW-01: the Reason field is a closed enum (see SemanticReason*
// constants). The full underlying error is logged via slog at warn level
// for operators; the MCP-exposed Reason never carries raw error text.
func ComputeSemanticStoreStatus(ctx context.Context, p SemanticStoreProbe) SemanticStoreStatus {
	if p == nil || !p.Available() {
		return SemanticStoreStatus{State: "disabled"}
	}
	probeCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	if err := p.Probe(probeCtx); err != nil {
		reason := classifySemanticProbeError(err)
		if reason == "" {
			reason = SemanticReasonUnknown
		}
		// Surface the raw error to operators via slog; the MCP-exposed
		// Reason field above is intentionally a closed enum.
		slog.Warn("semantic store probe failed",
			"reason", reason,
			"err", err,
		)
		return SemanticStoreStatus{State: "unhealthy", Reason: reason}
	}
	return SemanticStoreStatus{State: "ready"}
}

// GetHealthArgs is the input schema for the get_health tool.
type GetHealthArgs struct {
	Verbose bool `json:"verbose,omitempty" jsonschema:"Show all language servers including healthy ones. Default false returns only unhealthy LSes."`
}

// --- help text constants ---

const getHealthHelp = `## Usage Examples

Check workspace health (show only unhealthy servers):
  get_health()

Show all language servers including healthy ones:
  get_health(verbose=true)

## Common Patterns
- Default mode shows only unhealthy workers and non-closed circuits
- Use verbose=true to see all language servers and their states
- Call after activation to verify language servers started successfully`

// SemanticIndexAccessor is the kernel-side seam for the Phase 65 65-07
// semantic_index health surface. INTEG-04. The daemon implements this
// against integSemanticLookup.Status (semantic_wiring.go) so the kernel
// package does not import internal/semantic concretions — only the
// integ value-types boundary (ConfigGate / SemanticStatus / closed-enum
// FallbackReason).
//
// All implementations MUST be read-only (M-readtier mitigation). Any
// reference to the snapshot-write surface (the begin/commit/abort/
// write-facts/on-flush/version-bump methods enumerated in the daemon
// grep canary) in an implementing method body breaks the contract.
type SemanticIndexAccessor interface {
	Status(ctx context.Context, ws workspace.WorkspaceKey) (integ.SemanticStatus, error)
}

// SemanticIndexBlock is the JSON-shaped block surfaced inside the
// get_health envelope per SPEC §24.5. Eight fields plus the `enabled`
// flag — fields are omitempty so an Enabled=false / disabled-state
// block stays compact.
//
// LastError is a closed-enum FallbackReason (WR-NEW-01); raw error text
// from the underlying store / queue / live overlay never reaches this
// field. The daemon-side adapter is responsible for the enum mapping
// (the integSemanticLookup.Status method).
type SemanticIndexBlock struct {
	Enabled                 bool   `json:"enabled"`
	Store                   string `json:"store,omitempty"`
	LatestSnapshotStatus    string `json:"latest_snapshot_status,omitempty"`
	GraphVersion            uint64 `json:"graph_version,omitempty"`
	OverlayActive           bool   `json:"overlay_active"`
	PendingLSPRevalidations int    `json:"pending_lsp_revalidations,omitempty"`
	LastLiveUpdateMs        int64  `json:"last_live_update_ms,omitempty"`
	LastError               string `json:"last_error,omitempty"`
}

// mapSemanticState translates the integ.SemanticStatus closed-enum State
// into the SPEC §24.5 latest_snapshot_status wire string. The mapping is
// total over the integ.Status* constants so unknown values render as
// "missing" (defensive — never surfaces in steady state).
func mapSemanticState(state string) string {
	switch state {
	case integ.StatusReady:
		return "ready"
	case integ.StatusBuilding:
		return "building"
	case integ.StatusError:
		return "error"
	case integ.StatusDisabled, "":
		return "missing"
	default:
		return "missing"
	}
}

// ComputeSemanticIndexBlock runs the read-only Status accessor and maps
// the result onto the SemanticIndexBlock struct. INTEG-04.
//
//   - Nil accessor → Enabled=false zero-block (caller must omit it).
//   - Accessor error → Enabled=true, LatestSnapshotStatus="error",
//     LastError=FallbackReasonIndexError. WR-NEW-01: raw error text is
//     never carried; only the closed-enum reason.
//   - Success → all eight fields populated from the SemanticStatus.
//     Enabled is true unless the underlying State is StatusDisabled (the
//     v1.10 CGO=0 / disabled-flag path), in which case the consumer
//     omits the block by checking Enabled.
func ComputeSemanticIndexBlock(ctx context.Context, accessor SemanticIndexAccessor, ws workspace.WorkspaceKey) SemanticIndexBlock {
	if accessor == nil {
		return SemanticIndexBlock{Enabled: false}
	}
	st, err := accessor.Status(ctx, ws)
	if err != nil {
		// Surface the raw error to operators; the MCP-exposed LastError is
		// the closed-enum FallbackReason (WR-NEW-01).
		slog.Warn("semantic_index status accessor failed",
			"reason", string(integ.FallbackReasonIndexError),
			"err", err,
		)
		return SemanticIndexBlock{
			Enabled:              true,
			LatestSnapshotStatus: "error",
			LastError:            string(integ.FallbackReasonIndexError),
		}
	}
	return SemanticIndexBlock{
		Enabled:                 st.State != integ.StatusDisabled,
		Store:                   st.Store,
		LatestSnapshotStatus:    mapSemanticState(st.State),
		GraphVersion:            st.GraphVersion,
		OverlayActive:           st.OverlayActive,
		PendingLSPRevalidations: st.PendingLSP,
		LastLiveUpdateMs:        st.LastLiveUpdateMs,
		LastError:               st.LastErrorReason,
	}
}

// healthEnvelope is the on-the-wire shape returned by get_health. The
// embedded *lspool.HealthReport carries the SC-0 workspaces/circuits
// payload; the explicit Source / FallbackReason fields stamp the INTEG-05
// uniform-envelope contract (closed-enum top-level fields shared with
// get_repo_map / get_context / analyze_blast_radius); SemanticStore is
// the Phase 57 SC-1 block (preserved verbatim, never renamed —
// Pitfall §4); SemanticIndex is the Phase 65 INTEG-04 additive block.
//
// SemanticIndex is a pointer so omitempty fires when ComputeSemanticIndexBlock
// returns an Enabled=false zero block (the nil-accessor / disabled path).
type healthEnvelope struct {
	*lspool.HealthReport
	Source         string              `json:"source"`
	FallbackReason string              `json:"fallback_reason,omitempty"`
	SemanticStore  SemanticStoreStatus `json:"semantic_store"`
	SemanticIndex  *SemanticIndexBlock `json:"semantic_index,omitempty"`
}

// nilIfDisabled returns nil when the block carries Enabled=false so the
// json:"semantic_index,omitempty" tag elides it. Otherwise returns a
// pointer to a copy of the block.
func nilIfDisabled(b SemanticIndexBlock) *SemanticIndexBlock {
	if !b.Enabled {
		return nil
	}
	cp := b
	return &cp
}

// BuildEnvelopeJSON marshals the get_health envelope. Exposed for
// kernel-side tests that pin the SPEC §24.5 + INTEG-05 contracts without
// spinning up a full MCP server. Production callers (RegisterTools below)
// invoke this on every request.
//
// report may be nil (the no-workspaces path elides the embed entirely;
// downstream the json marshaller renders an empty object for the
// embedded fields). semStore is rendered verbatim (Phase 57 SC-1).
// semIdx is rendered when Enabled=true (INTEG-04 additive). source +
// reason are stamped at the top level (INTEG-05 uniform-envelope).
func BuildEnvelopeJSON(
	report *lspool.HealthReport,
	semStore SemanticStoreStatus,
	semIdx SemanticIndexBlock,
	source integ.Source,
	reason integ.FallbackReason,
) ([]byte, error) {
	env := healthEnvelope{
		HealthReport:   report,
		Source:         string(source),
		FallbackReason: string(reason),
		SemanticStore:  semStore,
		SemanticIndex:  nilIfDisabled(semIdx),
	}
	return json.MarshalIndent(env, "", "  ")
}

// RegisterTools registers the get_health MCP tool with the server.
//
// Arguments:
//
//   - semProbe (Phase 57 SC-1): semantic_store readiness seam. Pass nil
//     to render state="disabled" (CGO=0 / no daemon wiring path).
//   - semIndex (Phase 65 INTEG-04): semantic_index block accessor. Pass
//     nil to omit the block entirely (additive — Pitfall §4).
//   - cfgGate (Phase 65 INTEG-05): koanf-resolved feature gate. Required
//     for the top-level source field; nil-safe via integ.ChooseSource
//     (which renders SourceTreeSitter on nil cfgGate).
//   - semLookup (Phase 65 INTEG-05): SemanticLookup probe used only by
//     integ.ChooseSource to decide source ∈ {semantic | tree_sitter |
//     fallback}. Nil-safe (defaults to NoopLookup behaviour through the
//     ladder).
//   - wsKeyFn: returns the active workspace key (already constructed in
//     daemon.go step 6f.2). Used to address ComputeSemanticIndexBlock at
//     the per-request workspace.
func RegisterTools(
	server *mcp.SerenaMCPServer,
	k *kernel.Kernel,
	semProbe SemanticStoreProbe,
	semIndex SemanticIndexAccessor,
	cfgGate integ.ConfigGate,
	semLookup integ.SemanticLookup,
	wsKeyFn func() workspace.WorkspaceKey,
) {
	tracer := k.Tracer()

	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "get_health",
		Description: "Get workspace health status and language server states",
	}, kernel.WrapToolSpan(tracer, "get_health", func(ctx context.Context, req *mcpsdk.CallToolRequest, args GetHealthArgs) (*mcpsdk.CallToolResult, any, error) {
		report := k.HealthStatus()

		if len(report.Workspaces) == 0 {
			return textResult("No workspaces activated. Call activate_project first."), nil, nil
		}

		FilterReport(report, args.Verbose)

		// SC-1: surface semantic store readiness (Phase 57). Phase 65 INTEG-04
		// adds the semantic_index block; INTEG-05 stamps the top-level
		// source field via integ.ChooseSource. The semantic_store block is
		// preserved verbatim — additive evolution (Pitfall §4).
		semStatus := ComputeSemanticStoreStatus(ctx, semProbe)
		var ws workspace.WorkspaceKey
		if wsKeyFn != nil {
			ws = wsKeyFn()
		}
		semIdxBlock := ComputeSemanticIndexBlock(ctx, semIndex, ws)
		// D-04 + Pitfall §3: cfgGate is the FIRST decision. ChooseSource
		// emits SourceTreeSitter when the feature is off (steady-state
		// v1.9), SourceFallback + index_disabled when feature on but
		// lookup unavailable (defensive D-05 wiring-bug path), and
		// SourceSemantic in the success arm. err is nil here because the
		// envelope is stamped before any per-tool semantic call (no
		// integ-classified error to surface).
		src, reason := integ.ChooseSource(cfgGate, semLookup, nil)

		jsonBytes, err := BuildEnvelopeJSON(report, semStatus, semIdxBlock, src, reason)
		if err != nil {
			return errorResult(fmt.Sprintf("marshaling health report: %v", err)), nil, nil
		}

		return textResult(string(jsonBytes)), nil, nil
	}))

	server.Registry().Register(&mcp.ToolDef{
		Name:             "get_health",
		Description:      "Get workspace health status and language server states",
		BriefDescription: "Check workspace health and language server status",
		HelpText:         getHealthHelp,
	})
}

// FilterReport applies verbose/default filtering to a HealthReport in-place.
// When verbose is true, the report is returned as-is.
// When verbose is false, only unhealthy workers and non-closed circuits are kept.
// If all workers are healthy, Summary is set to "All N language servers healthy".
func FilterReport(report *lspool.HealthReport, verbose bool) {
	if verbose {
		return
	}

	totalWorkers := report.TotalWorkers()
	allHealthy := true

	for i := range report.Workspaces {
		ws := &report.Workspaces[i]

		// Filter workers to unhealthy only.
		var unhealthy []lspool.WorkerHealth
		for _, w := range ws.Workers {
			if w.State != "healthy" {
				unhealthy = append(unhealthy, w)
				allHealthy = false
			}
		}
		ws.Workers = unhealthy

		// Filter circuits to non-closed only.
		var nonClosed []lspool.CircuitHealth
		for _, c := range ws.Circuits {
			if c.State != "closed" {
				nonClosed = append(nonClosed, c)
				allHealthy = false
			}
		}
		ws.Circuits = nonClosed
	}

	if allHealthy && totalWorkers > 0 {
		report.Summary = fmt.Sprintf("All %d language servers healthy", totalWorkers)
	}
}

// textResult returns a successful MCP tool result with the given text.
func textResult(text string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: text},
		},
	}
}

// errorResult returns an error MCP tool result with the given message.
func errorResult(msg string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: msg},
		},
		IsError: true,
	}
}
