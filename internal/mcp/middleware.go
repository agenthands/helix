package mcp

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/obs"
)

// traceIDFromCtx extracts the OpenTelemetry trace ID from ctx as a hex string,
// or returns the empty string when no valid span context is attached. Used by
// TelemetryMiddleware to populate the trace_id field on the tap-compatible
// "tool call" JSONL line (F-07 leg A).
func traceIDFromCtx(ctx context.Context) string {
	sc := oteltrace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return ""
	}
	return sc.TraceID().String()
}

// renameStrategySink is the package-level recorder wired by InstallMiddleware.
// It accepts the closed-enum strategy string and increments the corresponding
// Prometheus counter on obs.Metrics. Nil until the first InstallMiddleware call;
// RecordRenameStrategy no-ops until wiring happens (e.g. during test setup).
// Phase 47 D-07: helix_rename_strategy_total bounded-label counter.
var renameStrategySink atomic.Pointer[func(ctx context.Context, strategy string)]

// setRenameStrategySink stores the recorder callback. Called from
// InstallMiddleware with an adapter around provider.Metrics().RenameStrategyInc.
// The sink accepts a ctx so a future OTel tracer can attach span attributes
// without a signature churn on every call-site (Phase 47 REVIEW IN-03).
//
// Process-global state — same parallel-safety caveat as
// SetEditOutcomeSinkForTest applies (WR-05): tests that swap the sink must
// not run in parallel and should restore via t.Cleanup.
func setRenameStrategySink(fn func(ctx context.Context, strategy string)) {
	renameStrategySink.Store(&fn)
}

// RecordRenameStrategy increments the helix_rename_strategy_total counter.
// The strategy string MUST be one of {"lsp-native","rust-client-side"}; any
// other value is dropped by the underlying obs.Metrics.RenameStrategyInc
// (closed-enum cardinality discipline, threat T-47-08). ctx is threaded
// through for future OTel integration; currently the sink adapter discards it.
func RecordRenameStrategy(ctx context.Context, strategy string) {
	p := renameStrategySink.Load()
	if p == nil || *p == nil {
		return
	}
	(*p)(ctx, strategy)
}

// editOutcomeSink is the package-level recorder wired by InstallMiddleware.
// It accepts the closed-enum (toolName, outcome, strategy) tuple and
// increments the corresponding Prometheus counter on obs.Metrics. Nil until
// the first InstallMiddleware call; RecordEditOutcome no-ops until wiring
// happens (e.g. during test setup).
//
// Phase 53 D-16: helix_edit_outcome_total bounded-label counter, mirroring
// the renameStrategySink pattern from Phase 47 D-07. Both families coexist
// — rename_symbol increments BOTH helix_edit_outcome_total (here) and
// helix_rename_strategy_total (above) per D-11.
var editOutcomeSink atomic.Pointer[func(ctx context.Context, toolName, outcome, strategy string)]

// setEditOutcomeSink stores the recorder callback. Called from
// InstallMiddleware with an adapter around provider.Metrics().EditOutcomeInc.
// The sink accepts a ctx so a future OTel tracer can attach span attributes
// without a signature churn on every call-site.
func setEditOutcomeSink(fn func(ctx context.Context, toolName, outcome, strategy string)) {
	editOutcomeSink.Store(&fn)
}

// RecordEditOutcome increments the helix_edit_outcome_total counter. Called
// from edit (internal/kernel/edit/) and fileops (internal/kernel/fileops/)
// tool handlers at return.
//
// Closed enums (Phase 53 D-10 + D-11 + Q-4):
//
//	outcome  ∈ {success, no_match, ambiguous_match, validation_failed, ls_error, internal}
//	strategy ∈ {exact, whitespace_normalized, indentation_flexible, none}
//
// Unknown values are dropped silently at the *obs.Metrics layer
// (EditOutcomeInc), mirroring the closed-enum drop-unknown discipline of
// RenameStrategyInc. Q-4: "failed" is NEVER a valid strategy value at this
// layer — fuzzy.StrategyFailed paths emit outcome="no_match", strategy="none".
//
// Test-only sink wiring: the editOutcomeSink package-level pointer is
// process-global. Tests that mutate it via SetEditOutcomeSinkForTest MUST
// NOT use t.Parallel() — concurrent goroutines would race over the shared
// recorder and observe each other's emissions. WR-05: prefer paired
// t.Cleanup(func() { mcp.SetEditOutcomeSinkForTest(nil) }) in any test
// that installs a custom sink so the leak does not bleed into adjacent
// tests. Same caveat applies to setRenameStrategySink (Phase 47 D-07).
func RecordEditOutcome(ctx context.Context, toolName, outcome, strategy string) {
	p := editOutcomeSink.Load()
	if p == nil || *p == nil {
		return
	}
	(*p)(ctx, toolName, outcome, strategy)
}

// InstallMiddleware wires Helix's receiving middleware onto the MCP SDK server
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
	// Phase 47 D-07: wire the rename dispatcher strategy recorder to the
	// provider's metrics sink so edit/tools.go can call
	// mcp.RecordRenameStrategy without reaching into obs directly.
	if provider != nil {
		if m := provider.Metrics(); m != nil {
			// Adapter closure discards ctx for now; future OTel integration
			// can read span context from ctx here without touching callers.
			setRenameStrategySink(func(_ context.Context, strategy string) {
				m.RenameStrategyInc(strategy)
			})
			// Phase 53 D-16: parallel sink for edit-tool outcomes.
			// Both edit (internal/kernel/edit/) and fileops
			// (internal/kernel/fileops/) tool handlers route through here.
			setEditOutcomeSink(func(_ context.Context, toolName, outcome, strategy string) {
				m.EditOutcomeInc(toolName, outcome, strategy)
			})
		}
	}
}

// Outcome enum for the "outcome" metric label on helix_tool_calls_total.
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
	// Phase 66 (GUARD-01/GUARD-04): two new outcome values for the guardrail path.
	// outcomeGuardrailWarned: tool call proceeded but a Warn decision attached a warning block.
	// outcomeGuardrailBlocked: GuardrailMiddleware blocked the tool call via GuardrailViolation.
	outcomeGuardrailWarned  = "guardrail_warned"  // NEW Phase 66
	outcomeGuardrailBlocked = "guardrail_blocked" // NEW Phase 66
)

// outcomeEnum is the authoritative closed-enum list for CI assertions and tests.
// Phase 66 adds outcomeGuardrailWarned and outcomeGuardrailBlocked (GUARD-04/05).
var outcomeEnum = []string{
	outcomeSuccess,
	outcomeInvalidArgs,
	outcomeNotFound,
	outcomeCircuitOpen,
	outcomeLSCrash,
	outcomeTimeout,
	outcomeInternal,
	outcomeGuardrailWarned,  // Phase 66
	outcomeGuardrailBlocked, // Phase 66
}

// editOutcomeEnum is the closed-enum vocabulary for the helix_edit_outcome_total
// "outcome" label (Phase 53 D-10). Mirrors the outcomeEnum discipline. Six
// values:
//
//   - success           — edit applied (any strategy)
//   - no_match          — fuzzy cascade exhausted (fuzzy.StrategyFailed)
//   - ambiguous_match   — >1 fuzzy candidate refused with diff
//   - validation_failed — post-edit verifier flagged regression
//   - ls_error          — upstream LS failure
//   - internal          — catch-all for unclassified errors
//   - unsupported       — structured-edit ablation guard rejected the call
//     (Phase 76 DisableStructuredEditSubsystem)
//
// Q-3 (RESOLVED 2026-04-30): "missing required field" validations bucket as
// "internal" — preserves the locked D-10 enum; classified as a known
// under-classification scheduled for v1.3 typed-error work (parallel to the
// outcomeEnum invalid_args TODO at line 102).
const (
	editOutcomeSuccess          = "success"
	editOutcomeNoMatch          = "no_match"
	editOutcomeAmbiguousMatch   = "ambiguous_match"
	editOutcomeValidationFailed = "validation_failed"
	editOutcomeLSError          = "ls_error"
	editOutcomeInternal         = "internal"
	editOutcomeUnsupported      = "unsupported"
)

var editOutcomeEnum = []string{
	editOutcomeSuccess,
	editOutcomeNoMatch,
	editOutcomeAmbiguousMatch,
	editOutcomeValidationFailed,
	editOutcomeLSError,
	editOutcomeInternal,
	editOutcomeUnsupported,
}

// strategyEnum is the closed-enum vocabulary for the helix_edit_outcome_total
// "strategy" label (Phase 53 D-11 + Q-4 correction).
//
// Q-4: NO "failed" entry — fuzzy.StrategyFailed paths emit outcome="no_match"
// with strategy="none" instead of propagating "failed" as a strategy label
// value. Cardinality bound 7 tools × 7 outcomes × 4 strategies = 196 (per
// AMENDED D-11/D-12 in 53-CONTEXT.md, dropping ellipsis from the enum;
// Phase 76 added the "unsupported" outcome lifting 6→7 outcomes).
const (
	strategyExact           = "exact"
	strategyWhitespaceNorm  = "whitespace_normalized"
	strategyIndentationFlex = "indentation_flexible"
	strategyNone            = "none"
)

var strategyEnum = []string{
	strategyExact,
	strategyWhitespaceNorm,
	strategyIndentationFlex,
	strategyNone,
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
		// Phase 66 (GUARD-04): GuardrailViolation classified BEFORE the generic
		// outcomeInternal fallback so blocked calls are visible in dashboards.
		if errors.Is(err, serr.ErrGuardrailViolation) {
			return outcomeGuardrailBlocked
		}
		return outcomeInternal
	}
	if ctr, ok := result.(*mcpsdk.CallToolResult); ok && ctr != nil {
		if ctr.IsError {
			// v1.2: without typed errors from tool handlers we cannot distinguish
			// invalid_args / not_found / ls_crash here. Bucket as "internal" and
			// refine in v1.3.
			return outcomeInternal
		}
		// Phase 66 (GUARD-05): warn-mode results carry a guardrailWarningSentinel
		// prefix on a TextContent block appended by GuardrailMiddleware. Detect it
		// here so dashboards can track warns separately from plain successes.
		// Channel documented at guardrail_middleware.go guardrailWarningSentinel.
		if hasGuardrailWarning(ctr) {
			return outcomeGuardrailWarned
		}
	}
	return outcomeSuccess
}

// hasGuardrailWarning returns true if the CallToolResult contains a content block
// appended by GuardrailMiddleware in warn mode. The sentinel prefix is defined in
// guardrail_middleware.go as guardrailWarningSentinel = "__guardrail_warning__:".
// Scans Content in O(n); in practice n ≤ 2 (one tool result + one warning).
func hasGuardrailWarning(ctr *mcpsdk.CallToolResult) bool {
	for _, c := range ctr.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			if len(tc.Text) > len(guardrailWarningSentinel) &&
				tc.Text[:len(guardrailWarningSentinel)] == guardrailWarningSentinel {
				return true
			}
		}
	}
	return false
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

			var profile, mode, language, sessionID string
			if sess := getSession(ctx); sess != nil {
				// Snapshot holds RLock over all field reads so profile/mode/
				// language come from one point in time even under concurrent
				// SetLanguage / switch_mode (T-11-07 mitigation).
				snap := sess.Snapshot()
				profile, mode, language, sessionID = snap.Profile, snap.Mode, snap.Language, snap.SessionID
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

			// F-07 leg A: emit tap-compatible JSONL "tool call" line for the
			// subprocess eval daemon-tap consumer (internal/eval/trace/tap.go).
			// MUST match the daemonLogLine schema: msg="tool call",
			// duration_ms as int64 milliseconds, pid matching expectedPid,
			// guardrail nil (populated by F-08 emission path elsewhere).
			logger.Info("tool call",
				"tool", toolName,
				"outcome", outcome,
				"duration_ms", duration.Milliseconds(),
				"pid", os.Getpid(),
				"trace_id", traceIDFromCtx(ctx),
				"session_id", sessionID,
				"guardrail", nil,
			)

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

// EditOutcomeEnumForTest returns a copy of the closed edit-outcome enum
// (Phase 53 D-10 + Phase 76 "unsupported") for test assertions that the
// 7-value vocabulary is preserved.
func EditOutcomeEnumForTest() []string {
	out := make([]string, len(editOutcomeEnum))
	copy(out, editOutcomeEnum)
	return out
}

// StrategyEnumForTest returns a copy of the closed strategy enum (Phase 53
// D-11 + Q-4) for test assertions that the 4-value vocabulary is preserved.
// Note: "failed" is intentionally absent — see strategyEnum doc.
func StrategyEnumForTest() []string {
	out := make([]string, len(strategyEnum))
	copy(out, strategyEnum)
	return out
}

// SetEditOutcomeSinkForTest exposes setEditOutcomeSink to external tests
// (e.g. internal/kernel/edit/tools_test.go) that need to install a recording
// recorder without going through InstallMiddleware. Production code MUST
// continue to wire via InstallMiddleware.
//
// WR-05: this mutates process-global state. Callers MUST NOT use
// t.Parallel() on tests that touch the sink, and SHOULD pair the call with
// t.Cleanup(func() { SetEditOutcomeSinkForTest(nil) }) so the recorder
// does not leak into adjacent tests in the same package.
func SetEditOutcomeSinkForTest(fn func(ctx context.Context, toolName, outcome, strategy string)) {
	setEditOutcomeSink(fn)
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
