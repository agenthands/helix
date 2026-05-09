package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/guardrails/rules"
	"github.com/agenthands/helix/internal/workspace"
)

// MiddlewareDeps is the dependency interface consumed by GuardrailMiddleware.
// It abstracts the production receipt store, rule evaluator, and profile resolver
// so the middleware can be tested with lightweight fakes.
//
// OnGraphVersionAdvance is invoked by the daemon's graph-version watcher
// (wired in daemon.go step 14b.5) whenever a workspace's graph_version
// advances, invalidating receipts whose GraphVersion < newGV (T-66-02).
//
// Evaluate wraps the synchronous rule evaluation in a 250 ms soft-deadline
// (T-66-21 fail-open mitigation). Returns nil,nil on timeout.
type MiddlewareDeps interface {
	// Evaluate runs rule predicates for the given tool call and returns 0..N
	// decisions. profile is the active agent profile name (e.g. "claude-code").
	// rawArgs is the JSON-encoded arguments map from the tool call.
	Evaluate(ctx context.Context, toolName string, rawArgs json.RawMessage, profile string) ([]rules.Decision, error)

	// OnGraphVersionAdvance forwards a graph-version advance to the receipt
	// store so stale receipts are invalidated.
	OnGraphVersionAdvance(ws workspace.WorkspaceKey, newGV uint64)

	// Logger returns the configured logger.
	Logger() *slog.Logger
}

// guardrailWarningSentinel is the text prefix used to tag a TextContent
// block as a guardrail warning. hasGuardrailWarning in middleware.go scans
// for it so classifyOutcome can detect warn-mode results without a
// parallel metadata channel.
//
// Channel rationale: a TextContent block whose Text begins with this prefix
// lets clients display the warning verbatim, and TelemetryMiddleware can detect
// it cheaply (O(n) scan over Content, usually 0 or 1 item). The sentinel is
// stable (documented here) so future readers do not invent an alternative.
const guardrailWarningSentinel = "__guardrail_warning__:"

// InstallGuardrailMiddleware wires the guardrail middleware onto the MCP SDK server.
//
// MUST be installed AFTER InstallSuggestionMiddleware and BEFORE InstallLazyInitMiddleware
// so LIFO execution order is LazyInit → Guardrail → Suggestion → ProfileFilter →
// Telemetry → handler. LazyInit-last invariant preserved
// (see internal/mcp/lazy_init.go:106-112).
//
// getSession is the same closure wired to TelemetryMiddleware and ProfileFilterMiddleware
// (single source of truth for session state, per D-01 thread-safety invariant).
func InstallGuardrailMiddleware(server *mcpsdk.Server, deps MiddlewareDeps, getSession func(ctx context.Context) *SessionInfo, logger *slog.Logger) {
	server.AddReceivingMiddleware(GuardrailMiddleware(deps, getSession, logger))
}

// GuardrailMiddleware returns a middleware function that gates destructive tool
// calls on prior evidence (receipts). Non-destructive tools and non-tools/call
// methods pass through immediately.
//
// Decision aggregation (D-12):
//   - Any Block decision → return serr.NewGuardrailViolation.
//     TelemetryMiddleware classifies as outcomeGuardrailBlocked.
//   - Any Warn decision → forward call, attach warning content block(s) to result.
//     TelemetryMiddleware classifies as outcomeGuardrailWarned via hasGuardrailWarning.
//   - All Allow (nil/empty decisions) → forward as-is.
func GuardrailMiddleware(deps MiddlewareDeps, getSession func(ctx context.Context) *SessionInfo, logger *slog.Logger) mcpsdk.Middleware {
	return func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
		return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
			// Early-out for non-tools/call methods.
			if method != "tools/call" {
				return next(ctx, method, req)
			}

			// Extract the tool request.
			ctr, ok := req.(*mcpsdk.CallToolRequest)
			if !ok || ctr == nil || ctr.Params == nil {
				return next(ctx, method, req)
			}

			// Early-out for non-destructive tools — no guardrail evaluation needed.
			if !isDestructiveTool(ctr.Params.Name) {
				return next(ctx, method, req)
			}

			// Resolve active profile name for enforcement-level resolution.
			profileName := ""
			if getSession != nil {
				if sess := getSession(ctx); sess != nil {
					snap := sess.Snapshot()
					profileName = snap.Profile
				}
			}

			// T-66-20: log only count, not receipt ID values or raw arg content.
			logger.Debug("guardrail: evaluating destructive tool",
				"tool", ctr.Params.Name,
				"receipt_count", receiptCount(ctr.Params.Arguments),
			)

			// Evaluate rules against the tool's arguments.
			decisions, err := deps.Evaluate(ctx, ctr.Params.Name, ctr.Params.Arguments, profileName)
			if err != nil {
				// Evaluation error: fail-open (log and pass through).
				logger.Warn("guardrail: evaluation error — fail-open",
					"tool", ctr.Params.Name,
					"error", err,
				)
				return next(ctx, method, req)
			}

			// Aggregate decisions.
			var blockDecisions []rules.Decision
			var warnDecisions []rules.Decision
			for _, d := range decisions {
				switch d.Action {
				case rules.Block:
					blockDecisions = append(blockDecisions, d)
				case rules.Warn:
					warnDecisions = append(warnDecisions, d)
				}
			}

			// Block path: return a typed GuardrailViolation error (first block decision).
			if len(blockDecisions) > 0 {
				d := blockDecisions[0]
				required := make([]serr.ReceiptClassRef, len(d.RequiredReceipts))
				for i, rr := range d.RequiredReceipts {
					required[i] = string(rr.Class)
				}
				seeAlso := make([]serr.SeeAlsoRef, len(d.SeeAlso))
				for i, sa := range d.SeeAlso {
					seeAlso[i] = serr.SeeAlsoRef{Tool: sa.Tool, Args: sa.Args}
				}
				return nil, serr.NewGuardrailViolation(d.Rule, d.Message, required, d.SuggestedTools, seeAlso)
			}

			// Warn path: forward to next, then attach warning content blocks.
			if len(warnDecisions) > 0 {
				result, nextErr := next(ctx, method, req)
				if nextErr != nil {
					return result, nextErr
				}
				// Attach warning content to the tool result.
				if ctrResult, ok := result.(*mcpsdk.CallToolResult); ok && ctrResult != nil && !ctrResult.IsError {
					for _, d := range warnDecisions {
						warningText := guardrailWarningSentinel + formatWarning(d)
						ctrResult.Content = append(ctrResult.Content, &mcpsdk.TextContent{
							Text: warningText,
						})
					}
				}
				return result, nil
			}

			// All Allow: pass through unchanged.
			return next(ctx, method, req)
		}
	}
}

// isDestructiveTool returns true for the six destructive tool names (D-08 LOCKED).
// This list is closed-enum; do not add tool names without a corresponding
// guard rule implementation and test.
func isDestructiveTool(name string) bool {
	switch name {
	case "rename_symbol":
		return true
	case "safe_delete_symbol":
		return true
	case "replace_symbol_body":
		return true
	case "fuzzy_edit":
		return true
	case "replace_in_file":
		return true
	case "delete_file":
		return true
	}
	return false
}

// formatWarning serialises a Warn decision into a human-readable JSON string.
// The guardrailWarningSentinel prefix is prepended by the caller.
func formatWarning(d rules.Decision) string {
	type warningPayload struct {
		Kind           string   `json:"kind"`
		Rule           string   `json:"rule"`
		Message        string   `json:"message"`
		SuggestedTools []string `json:"suggested_tools,omitempty"`
		SeeAlso        []struct {
			Tool string            `json:"tool"`
			Args map[string]string `json:"args,omitempty"`
		} `json:"see_also,omitempty"`
	}
	p := warningPayload{
		Kind:           "guardrail_warning",
		Rule:           d.Rule,
		Message:        d.Message,
		SuggestedTools: d.SuggestedTools,
	}
	for _, sa := range d.SeeAlso {
		p.SeeAlso = append(p.SeeAlso, struct {
			Tool string            `json:"tool"`
			Args map[string]string `json:"args,omitempty"`
		}{Tool: sa.Tool, Args: sa.Args})
	}
	b, err := json.Marshal(p)
	if err != nil {
		return fmt.Sprintf("guardrail warning: rule=%s message=%s", d.Rule, d.Message)
	}
	return string(b)
}

// receiptCount extracts the receipt count from raw tool args for INFO-level logging.
// Returns 0 on parse failure. T-66-20: never log IDs themselves at INFO.
func receiptCount(rawArgs json.RawMessage) int {
	if len(rawArgs) == 0 {
		return 0
	}
	var args map[string]any
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return 0
	}
	if receiptsRaw, ok := args["receipts"]; ok {
		if slice, ok := receiptsRaw.([]any); ok {
			return len(slice)
		}
	}
	return 0
}
