package mcp

import (
	"context"
	"errors"
	"log/slog"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	serr "github.com/postfix/serena/internal/errors"
	"github.com/postfix/serena/internal/obs"
)

// InstallMiddleware wires Serena's receiving middleware onto the MCP SDK server
// (MCP-04 + METRIC-02).
//
// TelemetryMiddleware absorbs the previous Phase 8 logging closure: it
// preserves the structured log lines for every method AND emits RED metrics
// for method == "tools/call". It fully replaces the Phase 8 log-only
// middleware.
//
// NOTE: ProfileFilterMiddleware only touches tools/list; TelemetryMiddleware
// only emits metrics on tools/call. Ordering between the two is independent,
// so callers may install them in either order. The D-06 "before ProfileFilter"
// constraint from CONTEXT.md was written assuming ProfileFilter had a deny
// path at tool-call time; since it does not in v1.2, that ordering constraint
// is obsolete here.
// BudgetFunc returns the timeout budget for a tool name. A nil BudgetFunc
// disables deadline injection (all calls pass through without a timeout).
// Wired from degrade.BudgetFor in daemon.go to avoid an import cycle
// (mcp -> config -> profile -> mcp).
type BudgetFunc func(toolName string) time.Duration

func InstallMiddleware(server *mcpsdk.Server, provider *obs.Provider, resolver ProfileResolver, getSession func(ctx context.Context) *SessionInfo, budgetFn BudgetFunc, registry *ToolRegistry, logger *slog.Logger) {
	server.AddReceivingMiddleware(TelemetryMiddleware(provider, getSession, budgetFn, logger))
	if resolver != nil {
		var briefDescs map[string]string
		if registry != nil {
			briefDescs = registry.BriefDescriptions()
		}
		server.AddReceivingMiddleware(ProfileFilterMiddleware(resolver, getSession, briefDescs, logger))
	}
}

// Outcome enum for the "outcome" metric label on serena_tool_calls_total.
//
// NOTE: a deny-outcome bucket is intentionally absent. ProfileFilterMiddleware
// only filters tools/list in v1.2; there is no rejection path at tools/call
// time. Reintroduce a deny bucket here if per-call filtering lands in v1.3.
//
// Finer-grained classification (invalid_args, not_found, ls_crash) is a v1.3
// concern and requires typed errors from the kernel. For v1.2 the hot path
// only produces {success, timeout, circuit_open, internal}; the remaining
// enum values are pre-declared so downstream dashboards can rely on the
// closed vocabulary.
const (
	outcomeSuccess     = "success"
	outcomeInvalidArgs = "invalid_args" // TODO v1.3: wire from typed validation errors
	outcomeNotFound    = "not_found"    // TODO v1.3: wire from symbol-lookup misses
	outcomeCircuitOpen = "circuit_open"
	outcomeLSCrash     = "ls_crash" // TODO v1.3: wire from lspool crash signals
	outcomeTimeout     = "timeout"
	outcomeInternal    = "internal"
)

// outcomeEnum is the authoritative closed-enum list for CI assertions and tests.
var outcomeEnum = []string{
	outcomeSuccess,
	outcomeInvalidArgs,
	outcomeNotFound,
	outcomeCircuitOpen,
	outcomeLSCrash,
	outcomeTimeout,
	outcomeInternal,
}

// classifyOutcome maps a (result, err) pair to one of the 7 closed enum values.
// Hot path: no allocations and no err.Error() text ever reaches the label.
//
// Check order matters: timeout wins over circuit_open wins over generic error
// wins over IsError result. This keeps the cheapest checks first and ensures
// context-cancellation errors do not get buried under the generic "internal"
// bucket.
func classifyOutcome(result mcpsdk.Result, err error) string {
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return outcomeTimeout
		}
		if errors.Is(err, serr.ErrCircuitOpen) {
			return outcomeCircuitOpen
		}
		return outcomeInternal
	}
	if ctr, ok := result.(*mcpsdk.CallToolResult); ok && ctr != nil && ctr.IsError {
		// v1.2: without typed errors from tool handlers we cannot distinguish
		// invalid_args / not_found / ls_crash here. Bucket as "internal" and
		// refine in v1.3.
		return outcomeInternal
	}
	return outcomeSuccess
}

// extractToolName pulls the tool name out of a tools/call request. Falls back
// to "unknown" if the request is not a *CallToolRequest or its Params are nil.
func extractToolName(req mcpsdk.Request) string {
	ctr, ok := req.(*mcpsdk.CallToolRequest)
	if !ok || ctr == nil || ctr.Params == nil {
		return "unknown"
	}
	return ctr.Params.Name
}

// TelemetryMiddleware emits RED metrics for every tools/call and preserves the
// Phase 8 structured log lines for all methods. The Phase 8 log closure has
// been absorbed here so we only traverse the middleware chain once per request.
//
// Metric emission is gated on method == "tools/call"; tools/list, initialize,
// and all other methods are pure log pass-through. This matches the v1.2
// scope: RED metrics are per-tool-call only (T-11-09 "accept" disposition).
func TelemetryMiddleware(provider *obs.Provider, getSession func(ctx context.Context) *SessionInfo, budgetFn BudgetFunc, logger *slog.Logger) mcpsdk.Middleware {
	m := provider.Metrics()     // closure-captured once; Metrics() is never nil per obs.Noop
	tracer := provider.Tracer() // captured once — noop when tracing is off, cheap on the hot path (D-01: never otel.GetTracerProvider)
	return func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
		return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
			// Non-tool methods: existing log/metrics path unchanged — NO span.
			if method != "tools/call" {
				start := time.Now()
				result, err := next(ctx, method, req)
				duration := time.Since(start)

				// Preserve Phase 8 logging behavior for ALL methods.
				if err != nil {
					logger.Warn("request failed",
						"method", method,
						"duration", duration,
						"error", err,
					)
				} else {
					logger.Info("request handled",
						"method", method,
						"duration", duration,
					)
				}
				return result, err
			}

			// D-01: inject per-class deadline BEFORE the tracing span so the
			// timeout covers both the span and the handler execution.
			toolName := extractToolName(req)
			if budgetFn != nil {
				if budget := budgetFn(toolName); budget > 0 {
					var budgetCancel context.CancelFunc
					ctx, budgetCancel = context.WithTimeout(ctx, budget)
					defer budgetCancel()
				}
			}

			// tools/call: create a tracing span (TRACE-02).
			ctx, span := tracer.Start(ctx, "daemon.mcp.tools.call")
			defer span.End()

			start := time.Now()
			result, err := next(ctx, method, req)
			duration := time.Since(start)

			// Preserve Phase 8 logging behavior.
			if err != nil {
				logger.Warn("request failed",
					"method", method,
					"duration", duration,
					"error", err,
				)
			} else {
				logger.Info("request handled",
					"method", method,
					"duration", duration,
				)
			}

			outcome := classifyOutcome(result, err)

			var profile, mode, language string
			if sess := getSession(ctx); sess != nil {
				// Snapshot holds RLock over all field reads so profile/mode/
				// language come from one point in time even under concurrent
				// SetLanguage / switch_mode (T-11-07 mitigation).
				snap := sess.Snapshot()
				profile, mode, language = snap.Profile, snap.Mode, snap.Language
			}

			// Gate attribute setting behind IsRecording — avoids attribute
			// allocation when tracing is off (D-17 budget protection).
			if span.IsRecording() {
				span.SetAttributes(
					attribute.String("tool_name", toolName),
					attribute.String("profile", profile),
					attribute.String("mode", mode),
					attribute.String("language", language),
					attribute.String("outcome", outcome),
				)
				if err != nil {
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
				}
			}

			// Metrics emission byte-for-byte identical to Phase 11.
			m.ToolCalls.WithLabelValues(toolName, profile, mode, language, outcome).Inc()
			m.ToolDuration.WithLabelValues(toolName, profile, mode, language).Observe(duration.Seconds())

			return result, err
		}
	}
}

// ClassifyOutcomeForTest exposes classifyOutcome to the _test package.
func ClassifyOutcomeForTest(result mcpsdk.Result, err error) string {
	return classifyOutcome(result, err)
}

// OutcomeEnumForTest returns a copy of the closed outcome enum for test
// assertions that the 7-value vocabulary is preserved.
func OutcomeEnumForTest() []string {
	out := make([]string, len(outcomeEnum))
	copy(out, outcomeEnum)
	return out
}

// ProfileResolver provides profile information for middleware filtering.
// This interface avoids a circular import between mcp and profile packages.
type ProfileResolver interface {
	// ToolDescriptionOverrides returns the description override map for the named profile.
	// Returns nil if the profile has no overrides or is not found.
	ToolDescriptionOverrides(profileName string) map[string]string
}

// ProfileFilterMiddleware creates middleware that filters tool listings based on
// the active session's AllowedTools and applies description overrides from the
// profile (PRF-03). For tools/list requests it filters and rewrites descriptions;
// all other methods pass through unchanged.
func ProfileFilterMiddleware(resolver ProfileResolver, getSession func(ctx context.Context) *SessionInfo, briefDescs map[string]string, logger *slog.Logger) mcpsdk.Middleware {
	return func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
		return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
			result, err := next(ctx, method, req)
			if err != nil {
				return result, err
			}

			if method != "tools/list" {
				return result, nil
			}

			session := getSession(ctx)
			if session == nil {
				return result, nil
			}

			listResult, ok := result.(*mcpsdk.ListToolsResult)
			if !ok {
				return result, nil
			}

			// Take a consistent snapshot so AllowedTools and Profile come from
			// the same point in time even if a concurrent switch_mode is racing
			// with this tools/list (threat T-08-08 mitigation).
			snap := session.Snapshot()

			// Apply AllowedTools filtering if the session has a whitelist.
			if snap.AllowedTools != nil {
				allowed := make(map[string]bool, len(snap.AllowedTools))
				for _, name := range snap.AllowedTools {
					allowed[name] = true
				}
				filtered := make([]*mcpsdk.Tool, 0, len(listResult.Tools))
				for _, tool := range listResult.Tools {
					if allowed[tool.Name] {
						filtered = append(filtered, tool)
					}
				}
				listResult.Tools = filtered
			}

			// Apply brief descriptions (DESC-01, D-02). Applied before profile
			// overrides so ToolDescriptionOverrides win when both are present.
			if briefDescs != nil {
				for _, tool := range listResult.Tools {
					if brief, ok := briefDescs[tool.Name]; ok && brief != "" {
						tool.Description = brief
					}
				}
			}

			// Apply description overrides from the profile.
			if resolver != nil {
				overrides := resolver.ToolDescriptionOverrides(snap.Profile)
				if len(overrides) > 0 {
					for _, tool := range listResult.Tools {
						if desc, ok := overrides[tool.Name]; ok {
							tool.Description = desc
						}
					}
				}
			}

			return listResult, nil
		}
	}
}
