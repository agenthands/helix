package guardrails

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/workspace"
)

// Typed sentinel errors for ValidateReceiptForOperation.
// Priority order (D-05 LOCKED):
//   ErrWrongWorkspace > ErrGraphVersionMismatch > ErrReceiptExpired >
//   ErrReceiptTooStale > ErrWrongReceiptClass > ErrReceiptScopeMismatch
var (
	ErrWrongWorkspace      = errors.New("receipt workspace does not match current workspace")
	ErrGraphVersionMismatch = errors.New("receipt graph_version is stale (graph_version advanced)")
	ErrReceiptExpired      = errors.New("receipt has expired (TTL elapsed)")
	ErrReceiptTooStale     = errors.New("receipt freshness below required minimum")
	ErrWrongReceiptClass   = errors.New("receipt class does not satisfy required class")
	ErrReceiptScopeMismatch = errors.New("receipt scope does not match operation target")
)

// Target describes the destructive operation being validated.
type Target struct {
	// SymbolID is the symbol being modified (for symbol-anchored operations).
	SymbolID integ.SymbolID
	// Path is the file/directory path being modified (for structural ops).
	Path string
	// TouchedFiles is the full set of files touched by the operation.
	TouchedFiles []string
	// SecuritySensitiveTouchedFiles is the security-relevant subset of TouchedFiles.
	SecuritySensitiveTouchedFiles []string
	// RequiredClass is the receipt class the operation requires.
	RequiredClass ReceiptClass
	// MinFreshness is the minimum acceptable freshness value (closed-enum string).
	// Empty = no freshness constraint.
	MinFreshness string
}

// ValidateReceiptForOperation validates that receipt satisfies the operation target.
// Returns nil on success; returns a typed sentinel on failure (priority order above).
// currentGV is the current graph_version from the live workspace;
// currentWS is the current workspace key;
// now is the current time (injectable for tests).
func ValidateReceiptForOperation(rcpt *Receipt, target Target, currentGV uint64, currentWS workspace.WorkspaceKey, now time.Time) error {
	// 1. Wrong workspace (T-66-06 mitigation).
	if rcpt.WorkspaceKey != currentWS {
		return ErrWrongWorkspace
	}

	// 2. Graph version mismatch (T-66-02 mitigation).
	if rcpt.GraphVersion < currentGV {
		return ErrGraphVersionMismatch
	}

	// 3. Expired (TTL elapsed).
	if now.After(rcpt.ExpiresAt) {
		return ErrReceiptExpired
	}

	// 4. Stale freshness (MinFreshness constraint).
	if target.MinFreshness != "" && !freshnessAcceptable(rcpt.Freshness, target.MinFreshness) {
		return ErrReceiptTooStale
	}

	// 5. Wrong receipt class.
	if target.RequiredClass != "" && rcpt.Class != target.RequiredClass {
		return ErrWrongReceiptClass
	}

	// 6. Scope mismatch (per-class STRICT check, D-09).
	if err := validateScope(rcpt.Class, rcpt.Scope, target); err != nil {
		return err
	}

	return nil
}

// validateScope performs per-class STRICT scope validation.
// A default-case panic ensures exhaustiveness when new classes are added.
func validateScope(class ReceiptClass, scope ReceiptScope, target Target) error {
	switch class {
	case ClassReferencesChecked:
		s, ok := scope.(ReferencesCheckedScope)
		if !ok {
			return ErrReceiptScopeMismatch
		}
		if target.SymbolID != "" && s.SymbolID != target.SymbolID {
			return ErrReceiptScopeMismatch
		}

	case ClassImpactChecked:
		s, ok := scope.(ImpactCheckedScope)
		if !ok {
			return ErrReceiptScopeMismatch
		}
		if target.SymbolID != "" && s.SymbolID != target.SymbolID {
			return ErrReceiptScopeMismatch
		}

	case ClassContextGathered:
		s, ok := scope.(ContextGatheredScope)
		if !ok {
			return ErrReceiptScopeMismatch
		}
		if !FileSetContainsAll(s.FileSet, target.TouchedFiles) {
			return ErrReceiptScopeMismatch
		}

	case ClassStructuralOverview:
		s, ok := scope.(StructuralOverviewScope)
		if !ok {
			return ErrReceiptScopeMismatch
		}
		if target.Path != "" && !PathIsWithin(target.Path, s.RootPath) {
			return ErrReceiptScopeMismatch
		}

	case ClassDiagnosticsClean:
		s, ok := scope.(DiagnosticsCleanScope)
		if !ok {
			return ErrReceiptScopeMismatch
		}
		if !FileSetContainsAll(s.FileSet, target.SecuritySensitiveTouchedFiles) {
			return ErrReceiptScopeMismatch
		}
		if s.ErrorCount != 0 {
			return ErrReceiptScopeMismatch
		}

	default:
		// This panic signals a missing case when a new ReceiptClass is added.
		// Tests will catch this via TestValidateScope_AllClasses.
		panic("guardrails: unhandled receipt class in validateScope — closed enum violation: " + string(class))
	}
	return nil
}

// FileSetContainsAll returns true if the fileSet contains every file in required.
// An empty required set is always satisfied.
func FileSetContainsAll(fileSet, required []string) bool {
	if len(required) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(fileSet))
	for _, f := range fileSet {
		set[f] = struct{}{}
	}
	for _, r := range required {
		if _, ok := set[r]; !ok {
			return false
		}
	}
	return true
}

// PathIsWithin returns true if path is rooted at or within rootPath.
// Uses filepath.Rel to detect ".." escapes (T-66-06 scope boundary).
func PathIsWithin(path, rootPath string) bool {
	if rootPath == "" || path == "" {
		return false
	}
	rel, err := filepath.Rel(rootPath, path)
	if err != nil {
		return false
	}
	// rel starting with ".." means path is outside rootPath.
	return !strings.HasPrefix(rel, "..")
}

// freshnessAcceptable returns true if the receipt freshness meets the minimum.
// Freshness is a closed enum; this implementation uses a simple ordered comparison
// based on the Phase 62 freshness ladder.
func freshnessAcceptable(actual, minimum string) bool {
	// Order: exact > approximate > stale > missing (higher index = worse).
	order := map[string]int{
		"exact":       0,
		"approximate": 1,
		"stale":       2,
		"missing":     3,
	}
	a, aOK := order[actual]
	m, mOK := order[minimum]
	if !aOK || !mOK {
		// Unknown freshness values: fail safe by rejecting.
		return false
	}
	// acceptable if actual is at least as fresh as minimum (lower or equal index).
	return a <= m
}
