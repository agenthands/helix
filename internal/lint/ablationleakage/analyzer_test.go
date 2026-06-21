package ablationleakage_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/agenthands/helix/internal/lint/ablationleakage"
)

// TestAnalyzer_RejectsBenchRunnerImportingDisabledSubsystem asserts the
// analyzer fires when a package rooted under `bench/runners` imports a
// disabled-subsystem package (here internal/semantic/store).
func TestAnalyzer_RejectsBenchRunnerImportingDisabledSubsystem(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), ablationleakage.Analyzer,
		"github.com/agenthands/helix/bench/runners/badrunner")
}

// TestAnalyzer_AllowsBenchRunnerWithoutForbiddenImport asserts the analyzer
// stays silent on a bench-runner package that imports only permitted packages.
func TestAnalyzer_AllowsBenchRunnerWithoutForbiddenImport(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), ablationleakage.Analyzer,
		"github.com/agenthands/helix/bench/runners/goodrunner")
}

// TestAnalyzer_AllowsSlashBoundaryLookalike is the slash-boundary regression
// guard: an import of `internal/semantic/storehouse` shares the bare-`HasPrefix`
// collision surface with the forbidden `internal/semantic/store` prefix but
// lacks the slash boundary. The analyzer MUST stay silent. Without this test, a
// future bare-`HasPrefix` regression would silently over-flag legitimate
// sibling packages.
func TestAnalyzer_AllowsSlashBoundaryLookalike(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), ablationleakage.Analyzer,
		"github.com/agenthands/helix/bench/runners/siblingrunner")
}

// TestAnalyzer_RejectsUngatedSemanticRead is the D-06 call-site RED fixture: a
// package that calls a SemanticLookup read method (`.ExpandFrom(`) directly,
// without first routing through `integ.ChooseSource`, MUST be flagged. The
// badgate fixture carries the matching `// want` comment.
func TestAnalyzer_RejectsUngatedSemanticRead(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), ablationleakage.Analyzer,
		"github.com/agenthands/helix/badgate")
}

// TestAnalyzer_AllowsSemanticReadBehindChooseSource is the D-06 call-site GREEN
// fixture: the same `.ExpandFrom(` read routed through `integ.ChooseSource` MUST
// NOT be flagged. The goodgate fixture has no `// want` directive, so
// analysistest fails if the analyzer emits any diagnostic.
func TestAnalyzer_AllowsSemanticReadBehindChooseSource(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), ablationleakage.Analyzer,
		"github.com/agenthands/helix/goodgate")
}
