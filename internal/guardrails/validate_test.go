package guardrails

import (
	"testing"
	"time"

	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/workspace"
)

var (
	baseWS  = workspace.WorkspaceKey{RepoRoot: "/repo", Language: "go", Toolchain: "1.25"}
	otherWS2 = workspace.WorkspaceKey{RepoRoot: "/other", Language: "go", Toolchain: "1.25"}
)

func makeReceipt(class ReceiptClass, scope ReceiptScope, ws workspace.WorkspaceKey, gv uint64, now time.Time) *Receipt {
	return &Receipt{
		ID:           "rcpt_aaaaaaaaaaaaaaaaaaaaaaaaa2",
		Class:        class,
		WorkspaceKey: ws,
		GraphVersion: gv,
		IssuedAt:     now.Add(-1 * time.Minute),
		ExpiresAt:    now.Add(4 * time.Minute),
		Scope:        scope,
	}
}

func TestValidateReceiptForOperation_PriorityOrder(t *testing.T) {
	t.Parallel()

	now := time.Now()

	// A receipt that fails on MULTIPLE criteria simultaneously.
	// Priority: ErrWrongWorkspace first.
	rcpt := &Receipt{
		Class:        ClassReferencesChecked,
		WorkspaceKey: otherWS2,           // wrong workspace
		GraphVersion: 1,                  // stale (currentGV=100)
		IssuedAt:     now.Add(-10 * time.Minute),
		ExpiresAt:    now.Add(-1 * time.Minute), // expired
		Scope:        ReferencesCheckedScope{SymbolID: "sym-A"},
	}

	target := Target{
		SymbolID:      "sym-B", // scope mismatch
		RequiredClass: ClassReferencesChecked,
	}

	err := ValidateReceiptForOperation(rcpt, target, 100, baseWS, now)
	if err != ErrWrongWorkspace {
		t.Errorf("priority order violated: got %v; want ErrWrongWorkspace", err)
	}

	// Same receipt but right workspace → should get ErrGraphVersionMismatch.
	rcpt2 := *rcpt
	rcpt2.WorkspaceKey = baseWS
	err2 := ValidateReceiptForOperation(&rcpt2, target, 100, baseWS, now)
	if err2 != ErrGraphVersionMismatch {
		t.Errorf("priority order violated: got %v; want ErrGraphVersionMismatch", err2)
	}

	// Fix graph version → should get ErrReceiptExpired.
	rcpt3 := rcpt2
	rcpt3.GraphVersion = 100
	err3 := ValidateReceiptForOperation(&rcpt3, target, 100, baseWS, now)
	if err3 != ErrReceiptExpired {
		t.Errorf("priority order violated: got %v; want ErrReceiptExpired", err3)
	}

	// Fix expiry → scope mismatch (sym-A != sym-B).
	rcpt4 := rcpt3
	rcpt4.ExpiresAt = now.Add(4 * time.Minute)
	err4 := ValidateReceiptForOperation(&rcpt4, target, 100, baseWS, now)
	if err4 != ErrReceiptScopeMismatch {
		t.Errorf("priority order violated: got %v; want ErrReceiptScopeMismatch", err4)
	}

	// Fix scope → nil (success).
	rcpt5 := rcpt4
	rcpt5.Scope = ReferencesCheckedScope{SymbolID: "sym-B"}
	target5 := target
	target5.SymbolID = "sym-B"
	err5 := ValidateReceiptForOperation(&rcpt5, target5, 100, baseWS, now)
	if err5 != nil {
		t.Errorf("expected nil on fully valid receipt; got %v", err5)
	}
}

func TestValidateReceiptForOperation_WrongClass(t *testing.T) {
	t.Parallel()
	now := time.Now()

	rcpt := makeReceipt(ClassReferencesChecked, ReferencesCheckedScope{}, baseWS, 1, now)
	target := Target{RequiredClass: ClassImpactChecked}

	err := ValidateReceiptForOperation(rcpt, target, 1, baseWS, now)
	if err != ErrWrongReceiptClass {
		t.Errorf("got %v; want ErrWrongReceiptClass", err)
	}
}

func TestValidateScope_ReferencesChecked_ScopeMismatch(t *testing.T) {
	t.Parallel()
	now := time.Now()

	// Receipt for sym-A cannot satisfy operation on sym-B.
	rcpt := makeReceipt(ClassReferencesChecked, ReferencesCheckedScope{SymbolID: "sym-A"}, baseWS, 1, now)
	target := Target{
		SymbolID:      "sym-B",
		RequiredClass: ClassReferencesChecked,
	}

	err := ValidateReceiptForOperation(rcpt, target, 1, baseWS, now)
	if err != ErrReceiptScopeMismatch {
		t.Errorf("got %v; want ErrReceiptScopeMismatch", err)
	}
}

func TestValidateScope_ImpactChecked_ScopeMismatch(t *testing.T) {
	t.Parallel()
	now := time.Now()

	rcpt := makeReceipt(ClassImpactChecked, ImpactCheckedScope{SymbolID: "sym-A"}, baseWS, 1, now)
	target := Target{
		SymbolID:      integ.SymbolID("sym-B"),
		RequiredClass: ClassImpactChecked,
	}

	err := ValidateReceiptForOperation(rcpt, target, 1, baseWS, now)
	if err != ErrReceiptScopeMismatch {
		t.Errorf("got %v; want ErrReceiptScopeMismatch", err)
	}
}

func TestValidateScope_ContextGathered_ScopeMismatch(t *testing.T) {
	t.Parallel()
	now := time.Now()

	rcpt := makeReceipt(ClassContextGathered, ContextGatheredScope{
		FileSet: []string{"a.go", "b.go"},
	}, baseWS, 1, now)
	// Operation touches c.go which is not in the receipt's file set.
	target := Target{
		TouchedFiles:  []string{"a.go", "c.go"},
		RequiredClass: ClassContextGathered,
	}

	err := ValidateReceiptForOperation(rcpt, target, 1, baseWS, now)
	if err != ErrReceiptScopeMismatch {
		t.Errorf("got %v; want ErrReceiptScopeMismatch", err)
	}
}

func TestValidateScope_StructuralOverview_ScopeMismatch(t *testing.T) {
	t.Parallel()
	now := time.Now()

	rcpt := makeReceipt(ClassStructuralOverview, StructuralOverviewScope{
		RootPath: "/repo/pkg",
	}, baseWS, 1, now)
	// Operation targets a file outside the root path.
	target := Target{
		Path:          "/other/pkg/file.go",
		RequiredClass: ClassStructuralOverview,
	}

	err := ValidateReceiptForOperation(rcpt, target, 1, baseWS, now)
	if err != ErrReceiptScopeMismatch {
		t.Errorf("got %v; want ErrReceiptScopeMismatch", err)
	}
}

func TestValidateScope_DiagnosticsClean_ScopeMismatch(t *testing.T) {
	t.Parallel()
	now := time.Now()

	t.Run("missing security file", func(t *testing.T) {
		t.Parallel()
		rcpt := makeReceipt(ClassDiagnosticsClean, DiagnosticsCleanScope{
			FileSet:    []string{"auth.go"},
			ErrorCount: 0,
		}, baseWS, 1, now)
		target := Target{
			SecuritySensitiveTouchedFiles: []string{"auth.go", "crypto.go"}, // crypto.go not in scope
			RequiredClass:                 ClassDiagnosticsClean,
		}
		err := ValidateReceiptForOperation(rcpt, target, 1, baseWS, now)
		if err != ErrReceiptScopeMismatch {
			t.Errorf("got %v; want ErrReceiptScopeMismatch", err)
		}
	})

	t.Run("errors not zero", func(t *testing.T) {
		t.Parallel()
		rcpt := makeReceipt(ClassDiagnosticsClean, DiagnosticsCleanScope{
			FileSet:    []string{"auth.go"},
			ErrorCount: 2, // not clean
		}, baseWS, 1, now)
		target := Target{
			SecuritySensitiveTouchedFiles: []string{"auth.go"},
			RequiredClass:                 ClassDiagnosticsClean,
		}
		err := ValidateReceiptForOperation(rcpt, target, 1, baseWS, now)
		if err != ErrReceiptScopeMismatch {
			t.Errorf("got %v; want ErrReceiptScopeMismatch", err)
		}
	})
}

func TestValidateScope_AllClasses(t *testing.T) {
	t.Parallel()

	now := time.Now()

	// For each class in the enum, create a matching receipt and target.
	// validateScope must handle all classes without hitting the default panic.
	classScopes := map[ReceiptClass]ReceiptScope{
		ClassReferencesChecked:  ReferencesCheckedScope{},
		ClassImpactChecked:      ImpactCheckedScope{},
		ClassContextGathered:    ContextGatheredScope{},
		ClassStructuralOverview: StructuralOverviewScope{RootPath: "/"},
		ClassDiagnosticsClean:   DiagnosticsCleanScope{ErrorCount: 0},
	}

	for _, class := range receiptClassEnum {
		class := class
		t.Run(string(class), func(t *testing.T) {
			t.Parallel()
			scope, ok := classScopes[class]
			if !ok {
				t.Fatalf("no scope type registered for class %q in test; update TestValidateScope_AllClasses", class)
			}
			rcpt := makeReceipt(class, scope, baseWS, 1, now)
			target := Target{RequiredClass: class}
			// Should not panic. Error or nil is acceptable.
			_ = ValidateReceiptForOperation(rcpt, target, 1, baseWS, now)
		})
	}
}

func TestFileSetContainsAll(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		fileSet  []string
		required []string
		want     bool
	}{
		{"empty required always satisfied", []string{"a.go"}, nil, true},
		{"exact match", []string{"a.go", "b.go"}, []string{"a.go", "b.go"}, true},
		{"superset satisfied", []string{"a.go", "b.go", "c.go"}, []string{"a.go", "b.go"}, true},
		{"missing file", []string{"a.go"}, []string{"a.go", "b.go"}, false},
		{"empty fileSet", nil, []string{"a.go"}, false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FileSetContainsAll(tc.fileSet, tc.required)
			if got != tc.want {
				t.Errorf("FileSetContainsAll(%v, %v) = %v; want %v", tc.fileSet, tc.required, got, tc.want)
			}
		})
	}
}

func TestPathIsWithin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		rootPath string
		want     bool
	}{
		{"within root", "/repo/pkg/file.go", "/repo", true},
		{"is root", "/repo", "/repo", true},
		{"outside root (sibling)", "/other/file.go", "/repo", false},
		{"path traversal", "/repo/../other/file.go", "/repo", false},
		{"empty path", "", "/repo", false},
		{"empty root", "/repo/file.go", "", false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := PathIsWithin(tc.path, tc.rootPath)
			if got != tc.want {
				t.Errorf("PathIsWithin(%q, %q) = %v; want %v", tc.path, tc.rootPath, got, tc.want)
			}
		})
	}
}
