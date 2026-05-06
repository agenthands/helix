package health

import (
	"context"
	"encoding/json"
	"fmt"

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
type SemanticStoreStatus struct {
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

// ComputeSemanticStoreStatus is a RED-stage stub. Replaced by the GREEN
// commit with the real probe-and-classify logic.
func ComputeSemanticStoreStatus(ctx context.Context, p SemanticStoreProbe) SemanticStoreStatus {
	_ = ctx
	_ = p
	return SemanticStoreStatus{}
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

// RegisterTools registers the get_health MCP tool with the server.
func RegisterTools(server *mcp.SerenaMCPServer, k *kernel.Kernel) {
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

		jsonBytes, err := json.MarshalIndent(report, "", "  ")
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
