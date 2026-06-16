package mcp_test

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/obs"
)

// TestRecordEditOutcome covers the package-level RecordEditOutcome sink
// scaffolding (Phase 53 Plan 04 Task 4.1, mirror of Phase 47 D-07's
// RecordRenameStrategy). Three sub-tests:
//   - noop_without_sink: RecordEditOutcome is a safe no-op when no sink is
//     wired.
//   - roundtrip_through_installed_sink: a recorder closure observes the
//     (toolName, outcome, strategy) triple verbatim.
//   - install_middleware_routes_to_obs_metrics: after InstallMiddleware runs
//     with a real obs.Provider, RecordEditOutcome increments the underlying
//     helix_edit_outcome_total counter on the provider's registry.
func TestRecordEditOutcome(t *testing.T) {
	t.Run("noop_without_sink", func(t *testing.T) {
		// Reset the sink to nil. Must not panic, must not block.
		mcp.SetEditOutcomeSinkForTest(nil)
		mcp.RecordEditOutcome(context.Background(), "replace_symbol_body", "success", "exact")
	})

	t.Run("roundtrip_through_installed_sink", func(t *testing.T) {
		type observed struct {
			toolName, outcome, strategy string
		}
		var (
			mu  sync.Mutex
			got observed
		)
		mcp.SetEditOutcomeSinkForTest(func(_ context.Context, toolName, outcome, strategy string) {
			mu.Lock()
			defer mu.Unlock()
			got = observed{toolName, outcome, strategy}
		})
		mcp.RecordEditOutcome(context.Background(), "rename_symbol", "success", "none")
		mu.Lock()
		defer mu.Unlock()
		if got.toolName != "rename_symbol" {
			t.Errorf("toolName = %q, want %q", got.toolName, "rename_symbol")
		}
		if got.outcome != "success" {
			t.Errorf("outcome = %q, want %q", got.outcome, "success")
		}
		if got.strategy != "none" {
			t.Errorf("strategy = %q, want %q", got.strategy, "none")
		}
	})

	t.Run("install_middleware_routes_to_obs_metrics", func(t *testing.T) {
		// Reset for cleanliness; InstallMiddleware will rewire it.
		mcp.SetEditOutcomeSinkForTest(nil)

		// Real provider with its own registry; no sharing across tests.
		provider := obs.Noop(slog.NewTextHandler(io.Discard, nil))

		// InstallMiddleware needs an *mcpsdk.Server. The sink-wiring branch
		// runs unconditionally on the (provider, m := provider.Metrics())
		// guard — no profile / session / budget code path exercised here.
		server := mcpsdk.NewServer(
			&mcpsdk.Implementation{Name: "helix-test", Version: "0.0.0"},
			nil,
		)
		mcp.InstallMiddleware(
			server,
			provider,
			nil, // resolver
			func(ctx context.Context) *mcp.SessionInfo { return nil },
			nil, // budgetFn
			nil, // registry
			slog.New(slog.NewTextHandler(io.Discard, nil)),
		)

		// After wiring, calling RecordEditOutcome must increment the
		// underlying obs.Metrics counter.
		mcp.RecordEditOutcome(context.Background(), "fuzzy_edit", "success", "exact")
		got := testutil.ToFloat64(provider.Metrics().EditOutcome.WithLabelValues("fuzzy_edit", "success", "exact"))
		if got != 1 {
			t.Errorf("EditOutcome counter = %v, want 1", got)
		}
	})
}

// TestEditOutcomeEnumForTest_Closed asserts the closed enum for the outcome
// label on helix_edit_outcome_total has exactly 7 values and contains the
// expected closed set (Phase 53 D-10 + Phase 76 "unsupported").
func TestEditOutcomeEnumForTest_Closed(t *testing.T) {
	enum := mcp.EditOutcomeEnumForTest()
	want := map[string]bool{
		"success":           true,
		"no_match":          true,
		"ambiguous_match":   true,
		"validation_failed": true,
		"ls_error":          true,
		"internal":          true,
		"unsupported":       true,
	}
	if len(enum) != len(want) {
		t.Fatalf("EditOutcomeEnumForTest len = %d, want %d", len(enum), len(want))
	}
	for _, v := range enum {
		if !want[v] {
			t.Errorf("unexpected enum value: %q", v)
		}
	}
}

// TestStrategyEnumForTest_Closed asserts the closed enum for the strategy
// label on helix_edit_outcome_total has exactly 4 values and does NOT
// contain "failed" (Phase 53 D-11 + Q-4: failed maps to outcome=no_match,
// strategy=none).
func TestStrategyEnumForTest_Closed(t *testing.T) {
	enum := mcp.StrategyEnumForTest()
	want := map[string]bool{
		"exact":                 true,
		"whitespace_normalized": true,
		"indentation_flexible":  true,
		"none":                  true,
	}
	if len(enum) != len(want) {
		t.Fatalf("StrategyEnumForTest len = %d, want %d", len(enum), len(want))
	}
	for _, v := range enum {
		if !want[v] {
			t.Errorf("unexpected enum value: %q", v)
		}
		if v == "failed" {
			t.Errorf("strategy enum must not contain %q (Q-4)", v)
		}
	}
}
