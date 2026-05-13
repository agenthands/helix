package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/agenthands/helix/internal/semantic"
)

// Deviation (Rule 1 — cycle): Pitfall 1 in 68-RESEARCH.md surfaces a
// structural blocker — internal/semantic/extract already imports
// internal/semantic/store (extract/to_store.go:46), so the plan's
// proposal that PriorFileFact.Symbols be []extract.SymbolFact creates
// an import cycle. Fixed by defining PriorSymbol in the store package
// carrying the subset of extract.SymbolFact fields the Tier-1 diff
// consumes (ID, StableKey, Name, Kind, Signature, Visibility,
// SignatureHash). The handler-side caller (Plan 68-04, package
// live/handler — which CAN import both) is free to convert in either
// direction. ExtractionStatus is a plain string in PriorFileFact for
// the same reason; callers compare against extract.ExtractionStatus*
// string constants.

// seedOverlayFile inserts a row into semantic_live_overlay_files. Use
// fileID=0 to model the Phase 60 P02 placeholder state.
func seedOverlayFile(t *testing.T, ctx context.Context, s *Store, repoID, path, language, status string, fileID uint64) {
	t.Helper()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO semantic_live_overlay_files (
			repo_id, path, file_id, content_hash, language, status, updated_at, write_epoch
		) VALUES (?, ?, ?, '', ?, ?, now(), 1)
	`, repoID, path, fileID, language, status); err != nil {
		t.Fatalf("seedOverlayFile(%q, %q, file=%d, status=%q): %v", repoID, path, fileID, status, err)
	}
}

// seedOverlaySymbol inserts a row into semantic_live_overlay_symbols.
// factJSON is the raw JSON payload that GetLatestFileFact must
// unmarshal into a PriorSymbol.
func seedOverlaySymbol(t *testing.T, ctx context.Context, s *Store, repoID string, symbolID, nodeID, fileID uint64, name, qname, kind, status, factJSON string) {
	t.Helper()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO semantic_live_overlay_symbols (
			repo_id, symbol_id, node_id, file_id, stable_key, name, qualified_name,
			kind, status, validation_state, confidence, fact_json, updated_at, write_epoch
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'validated', 1.0, ?::JSON, now(), 1)
	`, repoID, symbolID, nodeID, fileID, name, name, qname, kind, status, factJSON); err != nil {
		t.Fatalf("seedOverlaySymbol(%q, sym=%d): %v", repoID, symbolID, err)
	}
}

// seedSnapshotFile inserts a row into semantic_files keyed by snapshot_id.
func seedSnapshotFile(t *testing.T, ctx context.Context, s *Store, snapID, fileID uint64, repoID, path, language string) {
	t.Helper()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO semantic_files (
			snapshot_id, file_id, repo_id, path, language, content_hash,
			size_bytes, line_count, generated, ignored, indexed_at
		) VALUES (?, ?, ?, ?, ?, 'h', 100, 10, false, false, now())
	`, snapID, fileID, repoID, path, language); err != nil {
		t.Fatalf("seedSnapshotFile(snap=%d, file=%d, path=%q): %v", snapID, fileID, path, err)
	}
}

// seedSnapshotSymbol inserts a row into semantic_symbols keyed by snapshot_id.
func seedSnapshotSymbol(t *testing.T, ctx context.Context, s *Store, snapID, symbolID, fileID uint64, name, kind string, exported bool) {
	t.Helper()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO semantic_symbols (
			snapshot_id, symbol_id, node_id, file_id, language, kind, name,
			qualified_name, stable_key, start_byte, end_byte, start_line,
			start_col, end_line, end_col, signature, signature_hash, exported,
			visibility, extraction_source, confidence
		) VALUES (?, ?, ?, ?, 'go', ?, ?, ?, ?, 0, 100, 0, 0, 5, 0, ?, '', ?, ?, 'tree-sitter', 1.0)
	`, snapID, symbolID, symbolID, fileID, kind, name, name, name, name+"()", exported, visibilityFromExported(exported)); err != nil {
		t.Fatalf("seedSnapshotSymbol(snap=%d, sym=%d, name=%q): %v", snapID, symbolID, name, err)
	}
}

func visibilityFromExported(exported bool) string {
	if exported {
		return "exported"
	}
	return "private"
}

// TestGetLatestFileFact_OverlayHit: overlay row + symbol rows present →
// returns PriorFileFact{Symbols: len==2}, true, nil.
func TestGetLatestFileFact_OverlayHit(t *testing.T) {
	t.Parallel()
	s, ctx, _ := openStoreForOverlayTest(t)

	repoID := "r-overlay-hit"
	path := "pkg/foo.go"
	const fileID uint64 = 42
	seedOverlayFile(t, ctx, s, repoID, path, "go", "live", fileID)

	// Construct PriorSymbol-shaped JSON. The accessor unmarshals
	// fact_json directly into PriorSymbol (whose JSON shape is the
	// natural Go-tag-free encoding).
	sym1 := PriorSymbol{
		ID:         semantic.SymbolID(1001),
		StableKey:  "Foo|key",
		Name:       "Foo",
		Kind:       "function",
		Signature:  "func Foo()",
		Visibility: "exported",
	}
	sym2 := PriorSymbol{
		ID:         semantic.SymbolID(1002),
		StableKey:  "bar|key",
		Name:       "bar",
		Kind:       "function",
		Signature:  "func bar()",
		Visibility: "private",
	}
	b1, _ := json.Marshal(sym1)
	b2, _ := json.Marshal(sym2)
	seedOverlaySymbol(t, ctx, s, repoID, 1001, 1001, fileID, "Foo", "Foo", "function", "live", string(b1))
	seedOverlaySymbol(t, ctx, s, repoID, 1002, 1002, fileID, "bar", "bar", "function", "live", string(b2))

	got, ok, err := s.GetLatestFileFact(ctx, repoID, path)
	if err != nil {
		t.Fatalf("GetLatestFileFact: %v", err)
	}
	if !ok {
		t.Fatalf("GetLatestFileFact: ok=false, want true")
	}
	if got.Path != path {
		t.Errorf("Path: got %q, want %q", got.Path, path)
	}
	if got.Language != "go" {
		t.Errorf("Language: got %q, want %q", got.Language, "go")
	}
	if got.ExtractionStatus != "ready" {
		t.Errorf("ExtractionStatus: got %q, want %q", got.ExtractionStatus, "ready")
	}
	if len(got.Symbols) != 2 {
		t.Fatalf("Symbols: got %d, want 2", len(got.Symbols))
	}
}

// TestGetLatestFileFact_SnapshotFallback: no overlay row → snapshot
// path hydrates PriorSymbol from semantic_symbols rows.
func TestGetLatestFileFact_SnapshotFallback(t *testing.T) {
	t.Parallel()
	s, ctx, _ := openStoreForOverlayTest(t)

	repoID := "r-snap-fallback"
	path := "pkg/bar.go"
	const snapID uint64 = 2001
	const fileID uint64 = 73

	seedCommittedSnapshot(t, ctx, s, repoID, snapID, "committed")
	seedSnapshotFile(t, ctx, s, snapID, fileID, repoID, path, "go")
	seedSnapshotSymbol(t, ctx, s, snapID, 5001, fileID, "Alpha", "function", true)
	seedSnapshotSymbol(t, ctx, s, snapID, 5002, fileID, "beta", "function", false)

	got, ok, err := s.GetLatestFileFact(ctx, repoID, path)
	if err != nil {
		t.Fatalf("GetLatestFileFact: %v", err)
	}
	if !ok {
		t.Fatalf("GetLatestFileFact: ok=false, want true")
	}
	if got.Path != path {
		t.Errorf("Path: got %q, want %q", got.Path, path)
	}
	if got.Language != "go" {
		t.Errorf("Language: got %q, want %q", got.Language, "go")
	}
	if len(got.Symbols) != 2 {
		t.Fatalf("Symbols: got %d, want 2", len(got.Symbols))
	}

	// Verify Exported → Visibility mapping per Open Question Q2.
	var sawExported, sawPrivate bool
	for _, sym := range got.Symbols {
		switch sym.Name {
		case "Alpha":
			if sym.Visibility != "exported" {
				t.Errorf("Alpha visibility: got %q, want %q", sym.Visibility, "exported")
			}
			sawExported = true
		case "beta":
			if sym.Visibility != "private" {
				t.Errorf("beta visibility: got %q, want %q", sym.Visibility, "private")
			}
			sawPrivate = true
		}
	}
	if !sawExported || !sawPrivate {
		t.Errorf("symbols: missing exported=%v private=%v", sawExported, sawPrivate)
	}
}

// TestGetLatestFileFact_ColdStart: empty store, neither overlay nor
// snapshot row exists → (zero, false, nil).
func TestGetLatestFileFact_ColdStart(t *testing.T) {
	t.Parallel()
	s, ctx, _ := openStoreForOverlayTest(t)

	got, ok, err := s.GetLatestFileFact(ctx, "r-cold", "pkg/missing.go")
	if err != nil {
		t.Fatalf("GetLatestFileFact cold-start: %v", err)
	}
	if ok {
		t.Errorf("GetLatestFileFact cold-start: ok=true, want false")
	}
	if len(got.Symbols) != 0 {
		t.Errorf("GetLatestFileFact cold-start: got %d symbols, want 0", len(got.Symbols))
	}
}

// TestGetLatestFileFact_OverlayPlaceholder: overlay row exists but
// file_id==0 (Phase 60 P02 placeholder); accessor must fall through to
// the snapshot branch and return the snapshot result.
func TestGetLatestFileFact_OverlayPlaceholder(t *testing.T) {
	t.Parallel()
	s, ctx, _ := openStoreForOverlayTest(t)

	repoID := "r-placeholder"
	path := "pkg/baz.go"
	// Placeholder overlay row: file_id=0 means "not yet linked".
	seedOverlayFile(t, ctx, s, repoID, path, "go", "live", 0)

	// Seed a snapshot so the fallback has something to return.
	const snapID uint64 = 3001
	const fileID uint64 = 81
	seedCommittedSnapshot(t, ctx, s, repoID, snapID, "committed")
	seedSnapshotFile(t, ctx, s, snapID, fileID, repoID, path, "go")
	seedSnapshotSymbol(t, ctx, s, snapID, 7001, fileID, "Gamma", "function", true)

	got, ok, err := s.GetLatestFileFact(ctx, repoID, path)
	if err != nil {
		t.Fatalf("GetLatestFileFact placeholder: %v", err)
	}
	if !ok {
		t.Fatalf("GetLatestFileFact placeholder: ok=false, want true (snapshot fallback)")
	}
	if len(got.Symbols) != 1 {
		t.Fatalf("Symbols: got %d, want 1 (from snapshot)", len(got.Symbols))
	}
	if got.Symbols[0].Name != "Gamma" {
		t.Errorf("snapshot symbol: got %q, want %q", got.Symbols[0].Name, "Gamma")
	}
}

// TestGetLatestFileFact_NilStore: nil-receiver guard.
func TestGetLatestFileFact_NilStore(t *testing.T) {
	t.Parallel()
	var s *Store
	_, _, err := s.GetLatestFileFact(context.Background(), "r", "p")
	if err == nil {
		t.Fatal("GetLatestFileFact on nil *Store: want error, got nil")
	}
	if !strings.Contains(err.Error(), "GetLatestFileFact: nil store") {
		t.Errorf("nil store: got %q, want substring %q", err.Error(), "GetLatestFileFact: nil store")
	}
}

// TestGetLatestFileFact_EmptyRepoID: empty-repoID guard.
func TestGetLatestFileFact_EmptyRepoID(t *testing.T) {
	t.Parallel()
	s, ctx, _ := openStoreForOverlayTest(t)
	_, _, err := s.GetLatestFileFact(ctx, "", "p")
	if err == nil {
		t.Fatal("GetLatestFileFact empty repoID: want error, got nil")
	}
	if !strings.Contains(err.Error(), "empty repoID") {
		t.Errorf("empty repoID: got %q, want substring %q", err.Error(), "empty repoID")
	}
}
