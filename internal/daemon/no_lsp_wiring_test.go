package daemon

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/agenthands/helix/internal/config"
	"github.com/agenthands/helix/internal/obs"
	repomapSkill "github.com/agenthands/helix/internal/skill/repomap"
)

// no_lsp_wiring_test.go is the Phase 76 ABLATE-05 regression gate. It proves
// the no_lsp ablation arm is achieved by what the daemon does NOT wire
// (D-09 structural null-object injection), not by scattered per-callsite flag
// checks, and that the result emits ZERO lspool.lsp.* OTel spans (ROADMAP
// success criterion #2 hard fail).

// newNoLSPTestConfig returns a minimal SerenaConfig with the semantic index
// OFF (so no DuckDB is opened) and the LSP-subsystem disable flag set to the
// supplied value. The repomap skill is a process-global singleton resolved
// via repomapSkill.GetRepoMapSkill(), so each test re-runs daemon construction
// and re-reads the skill's wiring state.
func newNoLSPTestConfig(disableLSP bool) *config.SerenaConfig {
	return &config.SerenaConfig{
		DisableLSPSubsystem: disableLSP,
		// Profile resolves to "full" when empty; that arm sets neither flag,
		// so the effective flag is driven purely by DisableLSPSubsystem here.
	}
}

// buildNoLSPDaemon constructs a Daemon with a tracetest-backed obs.Provider so
// the test can inspect every span the daemon (and its kernel) emit. Semantic
// index is OFF so construction stays dependency-light (no DuckDB).
func buildNoLSPDaemon(t *testing.T, disableLSP bool) (*Daemon, *tracetest.InMemoryExporter) {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exp),
	)
	provider := obs.NewForTest(tp)
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))

	d, err := NewWithObsProvider(newNoLSPTestConfig(disableLSP), logger, provider)
	if err != nil {
		t.Fatalf("NewWithObsProvider(disableLSP=%v): %v", disableLSP, err)
	}
	return d, exp
}

// TestNoLSPWiring asserts the D-09 null-object decisions at the composition
// root under effDisableLSP=true: the kernel reports the flag, no LSP enrichment
// callback is installed on the repomap skill, and the pool-leasing repomap
// fallback deps (the D-10-audited AcquireFn lease path) are not wired.
func TestNoLSPWiring(t *testing.T) {
	d, _ := buildNoLSPDaemon(t, true)

	if !d.KernelForTest().LSPSubsystemDisabled() {
		t.Fatal("kernel.LSPSubsystemDisabled() = false; want true under disable_lsp_subsystem")
	}
	if d.KernelForTest().EditNotifier() != nil {
		t.Fatal("kernel.EditNotifier() != nil under no_lsp; want nil/no-op (live notifier must be skipped)")
	}

	rs := repomapSkill.GetRepoMapSkill()
	if rs == nil {
		t.Fatal("repomap skill not registered; cannot assert SetEnrichFn skip")
	}
	if rs.HasEnrichFn() {
		t.Error("repomap SetEnrichFn was installed under no_lsp; want skipped (D-09 repomap → tree-sitter fallback)")
	}
	if rs.HasFallbackDeps() {
		t.Error("repomap pool-leasing fallback deps were wired under no_lsp; want skipped (D-10 audit)")
	}
}

// TestNoLSPZeroSpans is the trace-tap hard-fail gate. With effDisableLSP=true
// and an in-memory exporter on the daemon tracer, NO span whose name has the
// "lspool.lsp." prefix may be recorded — the no_lsp arm cannot reach a live LS
// worker by construction (no pool lease ⇒ no conn.Call ⇒ no span).
func TestNoLSPZeroSpans(t *testing.T) {
	d, exp := buildNoLSPDaemon(t, true)
	_ = d // constructed; representative LSP seams are now neutralized.

	for _, span := range exp.GetSpans() {
		if strings.HasPrefix(span.Name, "lspool.lsp.") {
			t.Fatalf("ROADMAP criterion #2 violation: recorded lspool.lsp.* span %q under no_lsp arm", span.Name)
		}
	}
}

// TestNoLSPDefaultArmUnchanged proves the no_lsp path is purely additive: with
// effDisableLSP=false the daemon wires the repomap LSP enrichment callback and
// the pool-leasing fallback deps exactly as before.
func TestNoLSPDefaultArmUnchanged(t *testing.T) {
	_, _ = buildNoLSPDaemon(t, false)

	rs := repomapSkill.GetRepoMapSkill()
	if rs == nil {
		t.Fatal("repomap skill not registered")
	}
	if !rs.HasEnrichFn() {
		t.Error("repomap SetEnrichFn was NOT installed in the default arm; the no_lsp skip must be additive")
	}
	if !rs.HasFallbackDeps() {
		t.Error("repomap fallback deps were NOT wired in the default arm; the no_lsp skip must be additive")
	}
}
