//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

// TestConcurrency_MixedScenarios (ADV-03 Tier 1): saturates the worker pool
// with different tool calls via t.Parallel() subtests. Exercises share-until-dirty
// across heterogeneous workloads. Pass = no races, no deadlocks, no errors.
func TestConcurrency_MixedScenarios(t *testing.T) {
	requireGopls(t)
	fixture := PrepareFixture(t, "go")
	td := StartTestDaemon(t, Options{
		WorkspaceDir: fixture,
		MaxWorkers:   8,
	})

	scenarios := []struct {
		name string
		tool string
		args map[string]any
	}{
		{"search_symbol_helper", "search_symbols", map[string]any{"query": "Helper"}},
		{"search_symbol_greeter", "search_symbols", map[string]any{"query": "Greeter"}},
		{"search_func", "search_in_files", map[string]any{"pattern": "func "}},
		{"search_type", "search_in_files", map[string]any{"pattern": "type "}},
		{"list_dir_root", "list_directory", map[string]any{"path": "."}},
		{"list_dir_pkg", "list_directory", map[string]any{"path": "pkg"}},
		{"find_file_go", "find_files", map[string]any{"pattern": "*.go"}},
		{"symbols_overview", "get_symbol_overview", map[string]any{"path": "main.go"}},
	}

	for _, sc := range scenarios {
		sc := sc // D-07: capture loop variable before t.Parallel
		t.Run(sc.name, func(t *testing.T) {
			t.Parallel()
			callTool(t, td.Session, sc.tool, sc.args)
		})
	}
}

// TestConcurrency_PoolSaturation (ADV-03 Tier 2): explicit fan-out targeting
// pool AcquireLease / ReleaseLease on the hot path. Provokes rare interleavings
// that t.Parallel() subtests don't hit. Baseline N=100 for normal runs; CI stress
// job raises -count and reruns.
func TestConcurrency_PoolSaturation(t *testing.T) {
	requireGopls(t)
	fixture := PrepareFixture(t, "go")
	td := StartTestDaemon(t, Options{
		WorkspaceDir: fixture,
		MaxWorkers:   8,
	})

	const N = 100
	g, ctx := errgroup.WithContext(context.Background())
	for i := 0; i < N; i++ {
		g.Go(func() error {
			_, err := td.Session.CallTool(ctx, &mcp.CallToolParams{
				Name:      "search_symbols",
				Arguments: map[string]any{"query": "Helper"},
			})
			return err
		})
	}
	require.NoError(t, g.Wait(), "fan-out failed under saturation")
}

// TestConcurrency_ModeSwitchRace mixes mode switches with tool calls to catch
// races on session.AllowedTools (security domain: Tampering / EoP).
// The session starts in admin mode (set via cfg.Mode, bypassing transition rules),
// then the race cycles through transitions that ARE allowed from the full
// profile's transition graph: admin -> read -> edit -> read -> review -> read.
func TestConcurrency_ModeSwitchRace(t *testing.T) {
	requireGopls(t)
	fixture := PrepareFixture(t, "go")
	td := StartTestDaemon(t, Options{
		WorkspaceDir: fixture,
		MaxWorkers:   8,
		Profile:      "full",
		Mode:         "admin",
	})

	g, ctx := errgroup.WithContext(context.Background())
	// Fan out read-only tool calls that remain valid in every mode.
	for i := 0; i < 20; i++ {
		g.Go(func() error {
			_, err := td.Session.CallTool(ctx, &mcp.CallToolParams{
				Name:      "list_directory",
				Arguments: map[string]any{"path": "."},
			})
			return err
		})
	}
	// Interleave mode switches that are reachable per full profile transition graph
	// (admin->read, read->edit, edit->read, read->review, review->read). Errors on
	// disallowed transitions are tolerated because concurrent switches can move the
	// current mode out from under a pending call — the race-free property we want
	// is "no data race / no deadlock / no panic", not "every switch succeeds".
	switchChain := []string{"read", "edit", "read", "review", "read"}
	for _, m := range switchChain {
		m := m
		g.Go(func() error {
			// Intentionally ignore per-call errors: we're racing transitions and
			// some may be invalid from the observed-but-stale current mode. We
			// only care that CallTool itself doesn't blow up (protocol error).
			_, err := td.Session.CallTool(ctx, &mcp.CallToolParams{
				Name:      "switch_mode",
				Arguments: map[string]any{"target_mode": m},
			})
			if err != nil {
				return err
			}
			return nil
		})
	}
	require.NoError(t, g.Wait(), "mode switch race: protocol-level failure")
}
