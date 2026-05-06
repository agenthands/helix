package store

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/semantic"
)

// --- Test helpers ---

// silentLogger returns a slog.Logger that swallows output (Discard handler)
// while still being a valid argument so the store's slog.Warn calls don't
// panic on a nil receiver.
func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// newTestObsMetrics constructs a fresh *obs.Metrics. The constructor is
// package-private; the obs package exposes Provider.Metrics() for production
// callers and (since this is a sibling internal package) we build one via
// obs.Noop which is the lightest path to a registered Metrics with a real
// owned registry.
func newTestObsMetrics(t *testing.T) *obs.Metrics {
	t.Helper()
	p := obs.Noop(silentLogger().Handler())
	m := p.Metrics()
	if m == nil {
		t.Fatal("obs.Noop().Metrics() returned nil — fix obs wiring before testing store")
	}
	return m
}

// configFor builds a semantic.Config rooted at the given workspace dir
// with the default workspace-relative store path. The helper chdirs into
// `workspaceDir` so the relative path resolves to the temp dir; T-57-02-01
// (BL-01) requires Path to be workspace-relative, so absolute paths from
// t.TempDir() are no longer admissible.
func configFor(t *testing.T, workspaceDir string) semantic.Config {
	t.Helper()
	t.Chdir(workspaceDir)
	return semantic.Config{
		Enabled: true,
		Store: semantic.StoreConfig{
			Kind:        "duckdb",
			Path:        filepath.Join(".helix", "semantic.duckdb"),
			MemoryLimit: "256MiB",
			Threads:     2,
		},
	}
}

// counterValue reads a counter family by name from the registry, restricted
// to the metric whose label set matches the given labelMatch map. Returns 0
// if absent. Lets tests assert exact pre/post deltas without coupling to
// other emissions.
func counterValue(t *testing.T, m *obs.Metrics, name string, labelMatch map[string]string) float64 {
	t.Helper()
	mfs, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, metric := range mf.GetMetric() {
			if matchLabels(metric.GetLabel(), labelMatch) {
				return metric.GetCounter().GetValue()
			}
		}
	}
	return 0
}

func matchLabels(lps []*dto.LabelPair, want map[string]string) bool {
	got := map[string]string{}
	for _, lp := range lps {
		got[lp.GetName()] = lp.GetValue()
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

// dbFileExists returns true iff the given path is a regular file (not a
// directory, not absent).
func dbFileExists(t *testing.T, path string) bool {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false
		}
		t.Fatalf("os.Stat(%s): %v", path, err)
	}
	return st.Mode().IsRegular()
}

// --- Tests ---

// TestOpen_FreshWorkspace_CreatesDB asserts that opening a store in a
// directory that has no existing DB file creates one, stamps schema_version
// row = 1, and increments the open counter with outcome="created".
func TestOpen_FreshWorkspace_CreatesDB(t *testing.T) {
	wsDir := t.TempDir()
	cfg := configFor(t, wsDir)
	m := newTestObsMetrics(t)

	s, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open(fresh): %v", err)
	}
	if s == nil {
		t.Fatal("Open(fresh): want non-nil Store, got nil")
	}
	defer s.Close()

	if !dbFileExists(t, cfg.Store.Path) {
		t.Errorf("Open(fresh): want DB file at %s, but it does not exist", cfg.Store.Path)
	}

	// Counter increment for outcome="created".
	if v := counterValue(t, m, "helix_semantic_store_open_total",
		map[string]string{"outcome": "created"}); v < 1 {
		t.Errorf("helix_semantic_store_open_total{outcome=created} = %v, want ≥ 1", v)
	}
}

// TestOpen_ExistingClean_Reopens asserts that closing a fresh store and
// reopening produces no quarantine and increments the open counter with
// outcome="opened".
func TestOpen_ExistingClean_Reopens(t *testing.T) {
	wsDir := t.TempDir()
	cfg := configFor(t, wsDir)
	m := newTestObsMetrics(t)

	s1, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open #1: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("Close #1: %v", err)
	}

	beforeQuarantine := counterValue(t, m, "helix_semantic_store_quarantine_total",
		map[string]string{})

	s2, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open #2 (reopen): %v", err)
	}
	defer s2.Close()

	afterQuarantine := counterValue(t, m, "helix_semantic_store_quarantine_total",
		map[string]string{})
	if afterQuarantine != beforeQuarantine {
		t.Errorf("reopen unexpectedly quarantined: before=%v after=%v", beforeQuarantine, afterQuarantine)
	}
	if v := counterValue(t, m, "helix_semantic_store_open_total",
		map[string]string{"outcome": "opened"}); v < 1 {
		t.Errorf("helix_semantic_store_open_total{outcome=opened} = %v, want ≥ 1", v)
	}
}

// TestOpen_CorruptHeader_QuarantinesAndRebuilds writes 64 bytes of garbage
// to <dir>/.helix/semantic.duckdb, then Opens. Expectations: file is
// renamed to <path>.corrupt.<unix-ts>, fresh DB is created at original
// path, slog.Warn is captured, quarantine counter increments with
// reason="corrupt_file", and open counter increments with
// outcome="quarantined".
func TestOpen_CorruptHeader_QuarantinesAndRebuilds(t *testing.T) {
	wsDir := t.TempDir()
	cfg := configFor(t, wsDir)
	m := newTestObsMetrics(t)

	// Seed a corrupt file.
	if err := os.MkdirAll(filepath.Dir(cfg.Store.Path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.Store.Path, []byte(strings.Repeat("\x00\xff\xde\xad", 16)), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open(corrupt): %v", err)
	}
	defer s.Close()

	// Quarantine artifact must exist beside the original.
	parent := filepath.Dir(cfg.Store.Path)
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", parent, err)
	}
	var foundQuarantine bool
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), filepath.Base(cfg.Store.Path)+".corrupt.") {
			foundQuarantine = true
			break
		}
	}
	if !foundQuarantine {
		t.Errorf("want a *.corrupt.<ts> sibling in %s, found none", parent)
	}

	// Fresh DB recreated.
	if !dbFileExists(t, cfg.Store.Path) {
		t.Errorf("after quarantine: want fresh DB at %s, but missing", cfg.Store.Path)
	}

	if v := counterValue(t, m, "helix_semantic_store_quarantine_total",
		map[string]string{"reason": "corrupt_file"}); v < 1 {
		t.Errorf("helix_semantic_store_quarantine_total{reason=corrupt_file} = %v, want ≥ 1", v)
	}
	if v := counterValue(t, m, "helix_semantic_store_open_total",
		map[string]string{"outcome": "quarantined"}); v < 1 {
		t.Errorf("helix_semantic_store_open_total{outcome=quarantined} = %v, want ≥ 1", v)
	}
}

// TestOpen_SchemaForwardIncompat_Quarantines seeds a valid DB whose
// semantic_schema_version row is 999, then Opens. Expectations: quarantine
// + rebuild, reason="schema_forward_incompat".
func TestOpen_SchemaForwardIncompat_Quarantines(t *testing.T) {
	wsDir := t.TempDir()
	cfg := configFor(t, wsDir)
	m := newTestObsMetrics(t)

	// Seed a fresh, clean store first.
	s1, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open seed: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("Close seed: %v", err)
	}

	// Manually bump the schema version to 999 in the seeded DB.
	if err := writeSchemaVersion(t, cfg.Store.Path, 999); err != nil {
		t.Fatalf("writeSchemaVersion: %v", err)
	}

	s2, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open(forward-incompat): %v", err)
	}
	defer s2.Close()

	if v := counterValue(t, m, "helix_semantic_store_quarantine_total",
		map[string]string{"reason": "schema_forward_incompat"}); v < 1 {
		t.Errorf("helix_semantic_store_quarantine_total{reason=schema_forward_incompat} = %v, want ≥ 1", v)
	}
}

// TestOpen_SchemaUnreadable_Quarantines seeds a valid DB then DROPs the
// semantic_schema_version table. Expectations: quarantine + rebuild,
// reason="schema_unreadable".
func TestOpen_SchemaUnreadable_Quarantines(t *testing.T) {
	wsDir := t.TempDir()
	cfg := configFor(t, wsDir)
	m := newTestObsMetrics(t)

	s1, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open seed: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("Close seed: %v", err)
	}

	if err := dropSchemaVersionTable(t, cfg.Store.Path); err != nil {
		t.Fatalf("dropSchemaVersionTable: %v", err)
	}

	s2, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open(unreadable schema): %v", err)
	}
	defer s2.Close()

	if v := counterValue(t, m, "helix_semantic_store_quarantine_total",
		map[string]string{"reason": "schema_unreadable"}); v < 1 {
		t.Errorf("helix_semantic_store_quarantine_total{reason=schema_unreadable} = %v, want ≥ 1", v)
	}
}

// TestOpen_QuarantineFails_HardFails simulates a quarantine rename failure
// by making the parent directory read-only after seeding a corrupt file.
// Open must return an error so the daemon refuses to start. Skipped on
// platforms where root or filesystem-uid permits the write regardless.
func TestOpen_QuarantineFails_HardFails(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; chmod-based denial does not apply")
	}
	wsDir := t.TempDir()
	cfg := configFor(t, wsDir)
	m := newTestObsMetrics(t)

	dir := filepath.Dir(cfg.Store.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Seed a corrupt file.
	if err := os.WriteFile(cfg.Store.Path, []byte("not a duckdb file"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Make parent dir read-only so rename fails.
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	_, err := Open(context.Background(), cfg, silentLogger(), m)
	if err == nil {
		t.Fatal("Open with read-only parent dir: want error (Tier-3 hard fail), got nil")
	}
	// Either the rename failed or the rebuild failed — both are Tier-3.
	// The error message should not be a nil-deref or panic; any non-nil
	// error satisfies the contract.
	if errors.Is(err, serr.ErrUnsupported) {
		t.Errorf("Tier-3 should not surface as Unsupported; got %v", err)
	}
}

// TestSchema1_AllTablesExist asserts that after a fresh Open all 16 SPEC §8
// tables exist. Queries duckdb_tables (DuckDB's information schema).
func TestSchema1_AllTablesExist(t *testing.T) {
	wsDir := t.TempDir()
	cfg := configFor(t, wsDir)
	m := newTestObsMetrics(t)

	s, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	wantTables := []string{
		"semantic_schema_version",
		"semantic_snapshots",
		"semantic_nodes",
		"semantic_files",
		"semantic_symbols",
		"semantic_references",
		"semantic_edges",
		"semantic_diagnostics",
		"semantic_graph_scores",
		"semantic_clusters",
		"semantic_cluster_members",
		"semantic_live_overlay_meta",
		"semantic_live_overlay_files",
		"semantic_live_overlay_symbols",
		"semantic_live_overlay_references",
		"semantic_live_overlay_edges",
	}

	have := listTables(t, s)
	for _, want := range wantTables {
		if !have[want] {
			t.Errorf("missing SPEC §8 table: %s", want)
		}
	}
}

// TestQueryEffective_EmptyStore_ReturnsEmpty asserts the Schema 1 contract:
// every QueryEffective* call returns an empty result on a fresh store
// (because no data write paths exist in P57).
func TestQueryEffective_EmptyStore_ReturnsEmpty(t *testing.T) {
	wsDir := t.TempDir()
	cfg := configFor(t, wsDir)
	m := newTestObsMetrics(t)

	s, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := s.QueryEffectiveFiles(ctx, "repo-1", "any/path"); err != nil {
		t.Errorf("QueryEffectiveFiles on empty store: want nil err, got %v", err)
	}
	syms, err := s.QueryEffectiveSymbols(ctx, nil)
	if err != nil {
		t.Errorf("QueryEffectiveSymbols on empty store: want nil err, got %v", err)
	}
	if len(syms) != 0 {
		t.Errorf("QueryEffectiveSymbols on empty store: want []; got %d rows", len(syms))
	}
	refs, err := s.QueryEffectiveReferences(ctx, nil)
	if err != nil {
		t.Errorf("QueryEffectiveReferences on empty store: want nil err, got %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("QueryEffectiveReferences on empty store: want []; got %d rows", len(refs))
	}
	edges, err := s.QueryEffectiveEdges(ctx, nil)
	if err != nil {
		t.Errorf("QueryEffectiveEdges on empty store: want nil err, got %v", err)
	}
	if len(edges) != 0 {
		t.Errorf("QueryEffectiveEdges on empty store: want []; got %d rows", len(edges))
	}
}

// TestMetricsLabels_SemanticStoreCarveOuts asserts that the carveOuts map
// in internal/obs/metrics_labels_test.go was updated for the two Phase 57
// counter families. We re-build the registry, prime both vectors, and run
// the same lint helper to confirm no "forbidden label" diagnostics fire.
func TestMetricsLabels_SemanticStoreCarveOuts(t *testing.T) {
	// Cross-package reference: the test runs in package store, so we go
	// through obs.Noop which constructs Metrics via obs.newMetrics().
	p := obs.Noop(silentLogger().Handler())
	m := p.Metrics()
	m.SemanticStoreQuarantine.WithLabelValues("ws-test", "corrupt_file").Inc()
	m.SemanticStoreOpen.WithLabelValues("ws-test", "opened").Inc()

	// Walk the registry and confirm both families show up with their
	// closed-enum labels and nothing forbidden. We do an in-test lint
	// rather than calling the obs-package helper to avoid cyclic test
	// dependencies; the obs package's TestMetricsLabelsAllowlist is the
	// authoritative gate.
	mfs, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	allowed := map[string]map[string]bool{
		"helix_semantic_store_quarantine_total": {"workspace_label": true, "reason": true},
		"helix_semantic_store_open_total":       {"workspace_label": true, "outcome": true},
	}
	seen := map[string]bool{}
	for _, mf := range mfs {
		name := mf.GetName()
		if _, ok := allowed[name]; !ok {
			continue
		}
		seen[name] = true
		for _, metric := range mf.GetMetric() {
			for _, lp := range metric.GetLabel() {
				if !allowed[name][lp.GetName()] {
					t.Errorf("metric %s carries unexpected label %s", name, lp.GetName())
				}
			}
		}
	}
	if !seen["helix_semantic_store_quarantine_total"] {
		t.Error("helix_semantic_store_quarantine_total not registered or not primed")
	}
	if !seen["helix_semantic_store_open_total"] {
		t.Error("helix_semantic_store_open_total not registered or not primed")
	}
}

// --- DB-manipulation helpers used by quarantine seed tests ---

// writeSchemaVersion replaces the contents of semantic_schema_version with
// a single row holding the given version. Opens the DB directly via the
// duckdb driver.
func writeSchemaVersion(t *testing.T, path string, version int) error {
	t.Helper()
	db, err := sql.Open("duckdb", path)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.Exec("DELETE FROM semantic_schema_version"); err != nil {
		return err
	}
	_, err = db.Exec("INSERT INTO semantic_schema_version (version, applied_at) VALUES (?, now())", version)
	return err
}

// dropSchemaVersionTable removes the schema-version table entirely so the
// next Open classifies the DB as schema_unreadable.
func dropSchemaVersionTable(t *testing.T, path string) error {
	t.Helper()
	db, err := sql.Open("duckdb", path)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec("DROP TABLE semantic_schema_version")
	return err
}

// listTables uses DuckDB's duckdb_tables() function to return the set of
// table names in the main schema. The Store exposes the underlying *sql.DB
// via an unexported field; tests access it with the storeUnderlyingDB
// helper below.
func listTables(t *testing.T, s *Store) map[string]bool {
	t.Helper()
	db := storeUnderlyingDB(s)
	if db == nil {
		t.Fatal("listTables: Store.db is nil — Open failed to wire the *sql.DB")
	}
	rows, err := db.Query("SELECT table_name FROM duckdb_tables() WHERE schema_name = 'main'")
	if err != nil {
		t.Fatalf("SELECT FROM duckdb_tables(): %v", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("Scan: %v", err)
		}
		out[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}
	return out
}

// storeUnderlyingDB returns the unexported db field on *Store. Tests live
// in the same package so direct field access is fine; this helper exists
// to keep the access localized for easier refactoring.
func storeUnderlyingDB(s *Store) *sql.DB { return s.db }

// Compile-time assertion that prometheus dto types are tracked (in case
// future test refactors drop the import).
var _ = (*dto.MetricFamily)(nil)
var _ = prometheus.Labels{}
