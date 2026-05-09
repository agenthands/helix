package guardrails

import (
	"time"

	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/workspace"
)

// ReceiptClass is a closed enum of capability receipt types.
// Each class corresponds to a specific read-side tool that issues the receipt.
type ReceiptClass string

const (
	// ClassReferencesChecked is issued by find_references on success.
	ClassReferencesChecked ReceiptClass = "references_checked"
	// ClassImpactChecked is issued by analyze_blast_radius on success.
	ClassImpactChecked ReceiptClass = "impact_checked"
	// ClassContextGathered is issued by get_context / get_semantic_context on success.
	ClassContextGathered ReceiptClass = "context_gathered"
	// ClassStructuralOverview is issued by get_repo_map on success.
	ClassStructuralOverview ReceiptClass = "structural_overview"
	// ClassDiagnosticsClean is issued by get_diagnostics / verify_edit / run_diagnostics
	// when ErrorCount == 0.
	ClassDiagnosticsClean ReceiptClass = "diagnostics_clean"
)

// receiptClassEnum is the canonical ordered slice of all ReceiptClass values.
// Used by exhaustiveness tests and the validation switch.
var receiptClassEnum = []ReceiptClass{
	ClassReferencesChecked,
	ClassImpactChecked,
	ClassContextGathered,
	ClassStructuralOverview,
	ClassDiagnosticsClean,
}

// Receipt is the server-side capability receipt issued by a read-side tool
// and stored in the per-workspace in-memory store. Agents carry only the
// ReceiptID — the full struct body never leaves the daemon.
type Receipt struct {
	ID              ReceiptID
	Class           ReceiptClass
	SchemaVersion   uint32
	WorkspaceKey    workspace.WorkspaceKey
	SnapshotID      uint64
	GraphVersion    uint64
	Freshness       string // closed-enum mirror of integ.Freshness values
	ScoreStatus     string
	ClusterStatus   string
	PendingLSPFiles int
	LSPCoverage     float64
	IssuedAt        time.Time
	ExpiresAt       time.Time
	IssuingTool     string
	IssuingCallID   string
	TraceID         string
	Scope           ReceiptScope
}

// ReceiptScope is the marker interface for per-class scope structs.
// All concrete scope types implement isReceiptScope() via a method receiver.
type ReceiptScope interface {
	isReceiptScope()
}

// ReferencesCheckedScope is the scope for ClassReferencesChecked receipts,
// issued by find_references.
type ReferencesCheckedScope struct {
	SymbolID     integ.SymbolID
	RefCount     int
	FilePath     string
	IncludeTests bool
}

func (ReferencesCheckedScope) isReceiptScope() {}

// ImpactCheckedScope is the scope for ClassImpactChecked receipts,
// issued by analyze_blast_radius.
type ImpactCheckedScope struct {
	SymbolID        integ.SymbolID
	RefCount        int
	PublicAPI       bool
	BlastNodes      int
	MaxDepth        int
	IncludedCallers bool
	IncludedTypes   bool
}

func (ImpactCheckedScope) isReceiptScope() {}

// ContextGatheredScope is the scope for ClassContextGathered receipts,
// issued by get_context / get_semantic_context.
type ContextGatheredScope struct {
	FileSet         []string
	TargetSymbols   []integ.SymbolID
	TaskHash        string
	TokenBudgetUsed int
	MaxTokens       int
}

func (ContextGatheredScope) isReceiptScope() {}

// StructuralOverviewScope is the scope for ClassStructuralOverview receipts,
// issued by get_repo_map.
type StructuralOverviewScope struct {
	RootPath  string
	Depth     int
	FileCount int
	MaxTokens int
}

func (StructuralOverviewScope) isReceiptScope() {}

// DiagnosticsCleanScope is the scope for ClassDiagnosticsClean receipts,
// issued by get_diagnostics / verify_edit / run_diagnostics when ErrorCount == 0.
type DiagnosticsCleanScope struct {
	FileSet         []string
	DiagnosticCount int
	ErrorCount      int
	WarningCount    int
	Tool            string
}

func (DiagnosticsCleanScope) isReceiptScope() {}

// Compile-time interface assertions.
var _ ReceiptScope = (*ReferencesCheckedScope)(nil)
var _ ReceiptScope = (*ImpactCheckedScope)(nil)
var _ ReceiptScope = (*ContextGatheredScope)(nil)
var _ ReceiptScope = (*StructuralOverviewScope)(nil)
var _ ReceiptScope = (*DiagnosticsCleanScope)(nil)
