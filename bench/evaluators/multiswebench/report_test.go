package multiswebench

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseFinalReport (Task 2, T-88-01-04 / A2): ParseFinalReport over
// testdata/final_report.json yields the resolved_ids/unresolved_ids/
// total_instances; an unknown extra key is tolerated. An empty body, an oversized
// body, and a type-mismatched resolved field each error.
func TestParseFinalReport(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "final_report.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	fr, err := ParseFinalReport(b)
	if err != nil {
		t.Fatalf("ParseFinalReport on valid fixture: %v", err)
	}
	if fr.TotalInstances != 4 {
		t.Errorf("total_instances = %d, want 4", fr.TotalInstances)
	}
	if len(fr.ResolvedIDs) != 2 || fr.ResolvedIDs[0] != "golang__go-1001" {
		t.Errorf("resolved_ids = %v, want 2 ids starting golang__go-1001", fr.ResolvedIDs)
	}
	if len(fr.UnresolvedIDs) != 2 {
		t.Errorf("unresolved_ids = %v, want 2", fr.UnresolvedIDs)
	}

	// Empty body is an error.
	if _, err := ParseFinalReport(nil); err == nil {
		t.Error("ParseFinalReport(nil) must error (a missing report is not an empty report)")
	}

	// Oversized body is an error (bound BEFORE unmarshal).
	big := make([]byte, maxReportBytes+1)
	if _, err := ParseFinalReport(big); err == nil {
		t.Error("ParseFinalReport over the size cap must error before unmarshal")
	}

	// Type-mismatched field is an error (total_instances as a string).
	if _, err := ParseFinalReport([]byte(`{"total_instances":"four"}`)); err == nil {
		t.Error("ParseFinalReport with a type-mismatched field must error")
	}
}

// TestParseInstanceReport (Task 2): ParseInstanceReport over a per-instance fixture
// round-trips the resolved bool; a literal null body is an error; a non-bool
// resolved errors.
func TestParseInstanceReport(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "report.go.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	rep, err := ParseInstanceReport(b)
	if err != nil {
		t.Fatalf("ParseInstanceReport on valid fixture: %v", err)
	}
	got, ok := rep["golang__go-1001"]
	if !ok {
		t.Fatalf("instance golang__go-1001 missing from parsed report")
	}
	if !got.Resolved {
		t.Errorf("golang__go-1001 resolved = %v, want true", got.Resolved)
	}
	if unres, ok := rep["golang__go-1003"]; !ok || unres.Resolved {
		t.Errorf("golang__go-1003 resolved = %v (present=%v), want false", unres.Resolved, ok)
	}

	// Null body is an error.
	if _, err := ParseInstanceReport([]byte("null")); err == nil {
		t.Error("ParseInstanceReport(null) must error")
	}
	// Empty body is an error.
	if _, err := ParseInstanceReport(nil); err == nil {
		t.Error("ParseInstanceReport(nil) must error")
	}
	// Non-bool resolved is an error.
	if _, err := ParseInstanceReport([]byte(`{"x":{"resolved":"yes"}}`)); err == nil {
		t.Error("ParseInstanceReport with a non-bool resolved must error")
	}
}
