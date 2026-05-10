package daemon

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/agenthands/helix/internal/guardrails"
	"github.com/agenthands/helix/internal/guardrails/catalogs"
	"github.com/agenthands/helix/internal/guardrails/rules"
	helixMCP "github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/profile"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/workspace"
)

const guardrailEvalTimeout = 250 * time.Millisecond

// guardrailDepsImpl satisfies helixMCP.MiddlewareDeps for production use.
// It is constructed at daemon step 14b.5 and passed to InstallGuardrailMiddleware.
type guardrailDepsImpl struct {
	lookup          integ.SemanticLookup
	profileResolver profileGuardrailsResolver
	cfg             semantic.GuardrailsConfig
	store           *guardrails.Store
	evaluator       rules.RuleEvaluator
	outline         rules.OutlineProvider
	logger          *slog.Logger
	metrics         *obs.Metrics
	// wsKeyFn returns the active workspace key at evaluation time. Captured
	// from daemon scope so the closure stays consistent with the lazyActivate
	// flow (CR-03). When nil or returning a zero key, evaluation falls back
	// to workspace.WorkspaceKey{}.
	wsKeyFn func() workspace.WorkspaceKey
}

// profileGuardrailsResolver resolves the per-profile guardrail enforcement level.
// Satisfied by profileStoreResolver (below), which wraps *profile.ProfileStore.
type profileGuardrailsResolver interface {
	GuardrailsEnforcement(profileName string) string
}

// newGuardrailDeps constructs the production guardrail middleware deps.
func newGuardrailDeps(
	lookup integ.SemanticLookup,
	resolver profileGuardrailsResolver,
	cfg semantic.GuardrailsConfig,
	store *guardrails.Store,
	evaluator rules.RuleEvaluator,
	outline rules.OutlineProvider,
	logger *slog.Logger,
	metrics *obs.Metrics,
	wsKeyFn func() workspace.WorkspaceKey,
) helixMCP.MiddlewareDeps {
	return &guardrailDepsImpl{
		lookup:          lookup,
		profileResolver: resolver,
		cfg:             cfg,
		store:           store,
		evaluator:       evaluator,
		outline:         outline,
		logger:          logger,
		metrics:         metrics,
		wsKeyFn:         wsKeyFn,
	}
}

// Evaluate runs rule predicates for toolName with a 250 ms soft-deadline
// (T-66-21 fail-open: increment helix_guardrail_eval_timeout_total and allow).
func (d *guardrailDepsImpl) Evaluate(ctx context.Context, toolName string, rawArgs json.RawMessage, profileName string) ([]rules.Decision, error) {
	evalCtx, cancel := context.WithTimeout(ctx, guardrailEvalTimeout)
	defer cancel()

	type evalResult struct {
		decisions []rules.Decision
		err       error
	}
	done := make(chan evalResult, 1)
	go func() {
		dec, err := d.evaluate(evalCtx, toolName, rawArgs, profileName)
		done <- evalResult{dec, err}
	}()

	select {
	case res := <-done:
		return res.decisions, res.err
	case <-evalCtx.Done():
		// T-66-21: fail-open on timeout — increment counter for SRE observability.
		if d.metrics != nil {
			d.metrics.GuardrailEvalTimeoutInc()
		}
		d.logger.Warn("guardrail eval timeout — fail-open",
			"tool", toolName,
			"timeout", guardrailEvalTimeout,
		)
		return nil, nil
	}
}

// evaluate is the synchronous evaluation dispatched by Evaluate.
func (d *guardrailDepsImpl) evaluate(ctx context.Context, toolName string, rawArgs json.RawMessage, profileName string) ([]rules.Decision, error) {
	// Parse the raw arguments into a permissive map.
	var args map[string]any
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			d.logger.Debug("guardrail: failed to parse tool args — fail-open",
				"tool", toolName,
				"error", err,
			)
			return nil, nil
		}
	}

	// Build RuleArgs from the parsed argument map.
	ruleArgs := rules.RuleArgs{
		Tool: toolName,
	}
	if args != nil {
		ruleArgs.Path = guardrailStrVal(args, "path")
		ruleArgs.Find = guardrailStrVal(args, "find")
		ruleArgs.Replace = guardrailStrVal(args, "replace")
		ruleArgs.SymbolName = guardrailStrVal(args, "symbol_name")
		ruleArgs.NewName = guardrailStrVal(args, "new_name")
		ruleArgs.NewBody = guardrailStrVal(args, "new_body")
		ruleArgs.SearchBody = guardrailStrVal(args, "search_body")

		// Extract receipts from the uniform receipts: []ReceiptID field (D-08).
		// T-66-20: log only the count, never the full ID strings.
		if receiptsRaw, ok := args["receipts"]; ok {
			if slice, ok := receiptsRaw.([]any); ok {
				for _, item := range slice {
					if s, ok := item.(string); ok {
						if id, err := guardrails.ParseReceiptID(s); err == nil {
							ruleArgs.Receipts = append(ruleArgs.Receipts, id)
						}
					}
				}
			}
		}

		// Numeric fields for G-004 thresholds.
		if v, ok := args["changed_lines"].(float64); ok {
			ruleArgs.ChangedLines = int(v)
		}
		if v, ok := args["file_line_count"].(float64); ok {
			ruleArgs.FileLineCount = int(v)
		}
	}

	// Resolve profile-level enforcement from D-20 precedence stack.
	var profileEnforcement guardrails.EnforcementLevel
	if d.profileResolver != nil {
		rawLevel := d.profileResolver.GuardrailsEnforcement(profileName)
		if rawLevel != "" {
			if lv, err := guardrails.ParseEnforcementLevel(rawLevel); err == nil {
				profileEnforcement = lv
			}
		}
	}
	if profileEnforcement == "" {
		profileEnforcement = guardrails.LevelWarn // safe default (D-20 fallback)
	}

	// Empty catalog map — G-005 predicate loads embedded catalogs when map is nil.
	// TODO(phase-66.x): pre-warm catalogs at startup to avoid per-call load.
	catalogsMap := make(map[string]catalogs.Catalog)

	// Workspace key resolved from daemon scope (CR-03). Falls back to the zero
	// value when no workspace is active (pre-LazyInit), in which case the
	// downstream rule predicates will validate against an empty key — receipts
	// issued before activation are rare and harmless.
	var wsKey workspace.WorkspaceKey
	if d.wsKeyFn != nil {
		wsKey = d.wsKeyFn()
	}

	sc := rules.SessionContext{
		Workspace:          wsKey,
		Profile:            profileName,
		Lookup:             d.lookup,
		Store:              d.store,
		Config:             d.cfg,
		OutlineProvider:    d.outline,
		GraphVersion:       0, // TODO(phase-66.x): wire from active workspace graph version
		Now:                time.Now(),
		Catalogs:           catalogsMap,
		ProfileEnforcement: profileEnforcement,
	}

	return d.evaluator.Evaluate(ctx, ruleArgs, sc), nil
}

// OnGraphVersionAdvance forwards graph-version advance to the receipt store.
// Registered as the graph-version subscriber at daemon step 14b.5.
// TODO(phase-66.x): wire to the actual graph-version publish/subscribe seam.
func (d *guardrailDepsImpl) OnGraphVersionAdvance(ws workspace.WorkspaceKey, newGV uint64) {
	d.store.InvalidateOnGraphVersionAdvance(ws, newGV)
}

// Logger returns the configured logger.
func (d *guardrailDepsImpl) Logger() *slog.Logger { return d.logger }

// guardrailStrVal is a helper that extracts a string value from a map[string]any.
func guardrailStrVal(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// noopOutlineProvider is a no-op OutlineProvider used until the kernel exposes
// a tree-sitter outline seam.
// TODO(phase-66.x): replace with a real adapter wrapping kernel.Kernel.
type noopOutlineProvider struct{}

func (noopOutlineProvider) SymbolsInFile(_ context.Context, _ workspace.WorkspaceKey, _ string) ([]rules.OutlineSymbol, error) {
	return nil, nil
}

// metricsReceiptSink adapts *obs.Metrics to guardrails.MetricsSink.
type metricsReceiptSink struct{ m *obs.Metrics }

func (s metricsReceiptSink) ReceiptIssuedInc(class string)   { s.m.ReceiptIssuedInc(class) }
func (s metricsReceiptSink) ReceiptExpiredInc(reason string) { s.m.ReceiptExpiredInc(reason) }
func (s metricsReceiptSink) ReceiptLookupInc(outcome string) { s.m.ReceiptLookupInc(outcome) }

// profileStoreResolver wraps *profile.ProfileStore to satisfy profileGuardrailsResolver.
// GuardrailsEnforcement returns the profile-level enforcement string for the named
// profile, or "" when the profile is not found or has no guardrails config.
type profileStoreResolver struct{ store *profile.ProfileStore }

func newProfileStoreResolver(store *profile.ProfileStore) profileGuardrailsResolver {
	return profileStoreResolver{store: store}
}

func (r profileStoreResolver) GuardrailsEnforcement(profileName string) string {
	if r.store == nil {
		return ""
	}
	p, ok := r.store.Profile(profileName)
	if !ok || p == nil {
		return ""
	}
	return p.Guardrails.Enforcement
}
