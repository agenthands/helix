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

// RegisterTools registers the get_health MCP tool with the server. The
// semProbe argument is the daemon-supplied semantic store seam (SC-1);
// pass nil to disable the semantic_store block (CGO=0 / no daemon
// wiring scenario — the block then renders as state="disabled").
func RegisterTools(server *mcp.SerenaMCPServer, k *kernel.Kernel, semProbe SemanticStoreProbe) {
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

		// SC-1: surface semantic store readiness alongside LS health. Use
		// an envelope so the existing report shape is preserved verbatim
		// and `semantic_store` is additive — downstream consumers parsing
		// only `workspaces` are unaffected; new consumers see the field.
		semStatus := ComputeSemanticStoreStatus(ctx, semProbe)
		envelope := struct {
			*lspool.HealthReport
			SemanticStore SemanticStoreStatus `json:"semantic_store"`
		}{
			HealthReport:  report,
			SemanticStore: semStatus,
		}

		jsonBytes, err := json.MarshalIndent(envelope, "", "  ")
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
