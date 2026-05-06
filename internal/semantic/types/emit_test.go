package types

import (
	"context"
	"strings"
	"testing"

	semanticstore "github.com/agenthands/helix/internal/semantic/store"
)

// recordingEmitTx captures the EdgeRow batch passed to UpsertEdgesWithMerge.
type recordingEmitTx struct {
	batches [][]semanticstore.EdgeRow
	calls   int
	err     error
}

func (r *recordingEmitTx) UpsertEdgesWithMerge(ctx context.Context, edges []semanticstore.EdgeRow) error {
	r.calls++
	cp := make([]semanticstore.EdgeRow, len(edges))
	copy(cp, edges)
	r.batches = append(r.batches, cp)
	return r.err
}

// TestEmitEdges_PassesToUpsertEdgesWithMerge: ChainResponses turn into rows
// pumped through tx.UpsertEdgesWithMerge in a single batch.
func TestEmitEdges_PassesToUpsertEdgesWithMerge(t *testing.T) {
	refs := []ChainRequest{
		{RepoID: "r", RefNodeID: 1, RefKind: "RESOLVES_TO"},
		{RepoID: "r", RefNodeID: 2, RefKind: "RESOLVES_TO"},
		{RepoID: "r", RefNodeID: 3, RefKind: "CALLS"},
	}
	resps := []ChainResponse{
		{Resolved: true, Target: 10, Confidence: ConfidenceLSP, EvidenceKind: EvidenceLSP, ValidationState: "validated", Source: "lsp.go.text_document_definition"},
		{Resolved: true, Target: 11, Confidence: ConfidenceComment, EvidenceKind: EvidenceComment, ValidationState: "validated", Source: "comment.godoc"},
		{Resolved: false, Target: 0, Confidence: ConfidenceUnknown, EvidenceKind: EvidenceUnknown, ValidationState: "unresolved"},
	}
	tx := &recordingEmitTx{}
	if err := EmitEdges(context.Background(), tx, "r", refs, resps); err != nil {
		t.Fatalf("EmitEdges err: %v", err)
	}
	if tx.calls != 1 {
		t.Fatalf("UpsertEdgesWithMerge calls = %d, want 1", tx.calls)
	}
	if len(tx.batches[0]) != 3 {
		t.Fatalf("batch size = %d, want 3", len(tx.batches[0]))
	}
	got := tx.batches[0]
	if !strings.HasPrefix(got[0].Source, "lsp.") {
		t.Fatalf("row[0].Source = %q, want lsp.* prefix", got[0].Source)
	}
	if !strings.HasPrefix(got[1].Source, "comment.") {
		t.Fatalf("row[1].Source = %q, want comment.* prefix", got[1].Source)
	}
	if got[2].Source == "" {
		t.Fatalf("row[2].Source empty; want a non-empty fallback (e.g., 'unknown')")
	}
}

// TestEmitEdges_CommentNeverExceeds060: even if a per-language resolver
// returned 0.85 with EvidenceComment (out-of-spec), EmitEdges must clamp the
// EdgeRow.Confidence at 0.60 (TYPES-03 invariant).
func TestEmitEdges_CommentNeverExceeds060(t *testing.T) {
	refs := []ChainRequest{{RepoID: "r", RefNodeID: 1, RefKind: "RESOLVES_TO"}}
	resps := []ChainResponse{{
		Resolved: true, Target: 5, Confidence: 0.85, EvidenceKind: EvidenceComment,
		ValidationState: "validated", Source: "comment.godoc",
	}}
	tx := &recordingEmitTx{}
	if err := EmitEdges(context.Background(), tx, "r", refs, resps); err != nil {
		t.Fatalf("EmitEdges err: %v", err)
	}
	row := tx.batches[0][0]
	if row.Confidence != ConfidenceComment {
		t.Fatalf("row.Confidence = %v, want %v (TYPES-03 cap)", row.Confidence, ConfidenceComment)
	}
}

// TestEmitEdges_UnresolvedAlwaysHasState: any Resolved=false response → EdgeRow
// carries ValidationState="unresolved" — never "validated" (TYPES-04
// invariant; defensive even if the per-language resolver mis-set state).
func TestEmitEdges_UnresolvedAlwaysHasState(t *testing.T) {
	refs := []ChainRequest{
		{RepoID: "r", RefNodeID: 1, RefKind: "RESOLVES_TO"},
		{RepoID: "r", RefNodeID: 2, RefKind: "RESOLVES_TO"},
	}
	resps := []ChainResponse{
		// Defensive: resolver sloppily set "validated" but Resolved=false.
		{Resolved: false, Confidence: ConfidenceUnknown, EvidenceKind: EvidenceUnknown, ValidationState: "validated"},
		// Resolver left state empty.
		{Resolved: false, Confidence: ConfidenceHeuristic, EvidenceKind: EvidenceHeuristic, ValidationState: ""},
	}
	tx := &recordingEmitTx{}
	if err := EmitEdges(context.Background(), tx, "r", refs, resps); err != nil {
		t.Fatalf("EmitEdges err: %v", err)
	}
	for i, row := range tx.batches[0] {
		if row.ValidationState != "unresolved" {
			t.Fatalf("row[%d].ValidationState = %q, want unresolved (TYPES-04)", i, row.ValidationState)
		}
	}
}

// TestEmitEdges_EmptyInputNoOp: empty responses → no UpsertEdgesWithMerge call.
func TestEmitEdges_EmptyInputNoOp(t *testing.T) {
	tx := &recordingEmitTx{}
	if err := EmitEdges(context.Background(), tx, "r", nil, nil); err != nil {
		t.Fatalf("EmitEdges err: %v", err)
	}
	if tx.calls != 0 {
		t.Fatalf("UpsertEdgesWithMerge calls = %d, want 0", tx.calls)
	}
}

// TestEmitEdges_EdgeKindFromRefKind: each ChainRequest carries its target edge
// kind (RESOLVES_TO | CALLS | USES_TYPE); EmitEdges threads that through
// EdgeRow.EdgeKind verbatim.
func TestEmitEdges_EdgeKindFromRefKind(t *testing.T) {
	refs := []ChainRequest{
		{RefNodeID: 1, RefKind: "RESOLVES_TO"},
		{RefNodeID: 2, RefKind: "CALLS"},
		{RefNodeID: 3, RefKind: "USES_TYPE"},
	}
	resps := []ChainResponse{
		{Resolved: true, Target: 10, Confidence: ConfidenceLSP, EvidenceKind: EvidenceLSP, ValidationState: "validated", Source: "lsp.x"},
		{Resolved: true, Target: 11, Confidence: ConfidenceLSP, EvidenceKind: EvidenceLSP, ValidationState: "validated", Source: "lsp.x"},
		{Resolved: true, Target: 12, Confidence: ConfidenceLSP, EvidenceKind: EvidenceLSP, ValidationState: "validated", Source: "lsp.x"},
	}
	tx := &recordingEmitTx{}
	if err := EmitEdges(context.Background(), tx, "r", refs, resps); err != nil {
		t.Fatalf("EmitEdges err: %v", err)
	}
	want := []string{"RESOLVES_TO", "CALLS", "USES_TYPE"}
	for i, row := range tx.batches[0] {
		if row.EdgeKind != want[i] {
			t.Fatalf("row[%d].EdgeKind = %q, want %q", i, row.EdgeKind, want[i])
		}
	}
}
