package rules_test

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/guardrails"
	"github.com/agenthands/helix/internal/guardrails/rules"
	"github.com/agenthands/helix/internal/semantic"
)

func makeSCWithG004Config(t *testing.T, g004 semantic.G004Config) rules.SessionContext {
	t.Helper()
	sc := makeSC(t, "enforce", &fakeOutlineProvider{})
	sc.Config.G004 = g004
	return sc
}

func TestG004_BelowThreshold_Allows(t *testing.T) {
	sc := makeSCWithG004Config(t, semantic.G004Config{MaxChangedLines: 50, MaxFiles: 1})
	args := rules.RuleArgs{
		Tool:         "fuzzy_edit",
		ChangedLines: 49,
		TouchedFiles: []string{"file.go"},
	}
	d := rules.EvaluateG004(context.Background(), args, sc)
	if d.Action != rules.Allow {
		t.Errorf("49 changed lines (threshold 50): expected Allow, got %v", d.Action)
	}
}

func TestG004_ExceedsLinesThreshold_Blocks(t *testing.T) {
	sc := makeSCWithG004Config(t, semantic.G004Config{MaxChangedLines: 50, MaxFiles: 1})
	args := rules.RuleArgs{
		Tool:         "fuzzy_edit",
		ChangedLines: 51,
		TouchedFiles: []string{"file.go"},
	}
	d := rules.EvaluateG004(context.Background(), args, sc)
	if d.Action != rules.Block {
		t.Errorf("51 changed lines (threshold 50): expected Block, got %v", d.Action)
	}
	if d.Rule != "G-004" {
		t.Errorf("expected Rule=G-004, got %q", d.Rule)
	}
	// Single file → context_gathered
	hasContext := false
	for _, rr := range d.RequiredReceipts {
		if rr.Class == guardrails.ClassContextGathered {
			hasContext = true
		}
	}
	if !hasContext {
		t.Errorf("single-file large edit: expected context_gathered in RequiredReceipts, got %v", d.RequiredReceipts)
	}
}

func TestG004_MultiFile_TwoFiles_Blocks_RequiresStructural(t *testing.T) {
	sc := makeSCWithG004Config(t, semantic.G004Config{MaxChangedLines: 50, MaxFiles: 1})
	args := rules.RuleArgs{
		Tool:         "fuzzy_edit",
		ChangedLines: 10,
		TouchedFiles: []string{"file1.go", "file2.go"},
	}
	d := rules.EvaluateG004(context.Background(), args, sc)
	if d.Action != rules.Block {
		t.Errorf("2 touched files (threshold 1): expected Block, got %v", d.Action)
	}
	hasStructural := false
	for _, rr := range d.RequiredReceipts {
		if rr.Class == guardrails.ClassStructuralOverview {
			hasStructural = true
		}
	}
	if !hasStructural {
		t.Errorf("multi-file: expected structural_overview in RequiredReceipts, got %v", d.RequiredReceipts)
	}
}

func TestG004_MultiFile_ThreeFiles_Blocks(t *testing.T) {
	sc := makeSCWithG004Config(t, semantic.G004Config{MaxChangedLines: 50, MaxFiles: 1})
	args := rules.RuleArgs{
		Tool:         "replace_in_file",
		ChangedLines: 10,
		TouchedFiles: []string{"a.go", "b.go", "c.go"},
	}
	d := rules.EvaluateG004(context.Background(), args, sc)
	if d.Action != rules.Block {
		t.Errorf("3 touched files: expected Block, got %v", d.Action)
	}
}

func TestG004_RatioTrigger_Enabled_Blocks(t *testing.T) {
	sc := makeSCWithG004Config(t, semantic.G004Config{
		MaxChangedLines:    50,
		MaxFiles:           1,
		MaxFileChangeRatio: 0.30,
		EnableRatioTrigger: true,
	})
	args := rules.RuleArgs{
		Tool:          "fuzzy_edit",
		ChangedLines:  20,
		TouchedFiles:  []string{"file.go"},
		FileLineCount: 50, // ratio = 20/50 = 0.40 > 0.30
	}
	d := rules.EvaluateG004(context.Background(), args, sc)
	if d.Action != rules.Block {
		t.Errorf("ratio trigger (0.40 > 0.30): expected Block, got %v", d.Action)
	}
}

func TestG004_RatioTrigger_Disabled_Allows(t *testing.T) {
	sc := makeSCWithG004Config(t, semantic.G004Config{
		MaxChangedLines:    50,
		MaxFiles:           1,
		MaxFileChangeRatio: 0.30,
		EnableRatioTrigger: false, // disabled
	})
	args := rules.RuleArgs{
		Tool:          "fuzzy_edit",
		ChangedLines:  20,
		TouchedFiles:  []string{"file.go"},
		FileLineCount: 50, // ratio = 0.40, but trigger disabled
	}
	d := rules.EvaluateG004(context.Background(), args, sc)
	if d.Action != rules.Allow {
		t.Errorf("ratio trigger disabled: expected Allow, got %v", d.Action)
	}
}

func TestG004_ConfigOverride_RaisesThreshold(t *testing.T) {
	// cfg.G004.MaxChangedLines=200 means 51 is below threshold.
	sc := makeSCWithG004Config(t, semantic.G004Config{MaxChangedLines: 200, MaxFiles: 1})
	args := rules.RuleArgs{
		Tool:         "fuzzy_edit",
		ChangedLines: 51,
		TouchedFiles: []string{"file.go"},
	}
	d := rules.EvaluateG004(context.Background(), args, sc)
	if d.Action != rules.Allow {
		t.Errorf("with threshold 200: 51 lines should Allow, got %v", d.Action)
	}
}

func TestG004_ZeroConfig_UsesDefaults(t *testing.T) {
	// Zero G004Config: predicates use sensible defaults (50 lines, 1 file).
	sc := makeSC(t, "enforce", &fakeOutlineProvider{})
	args := rules.RuleArgs{
		Tool:         "fuzzy_edit",
		ChangedLines: 51,
		TouchedFiles: []string{"file.go"},
	}
	d := rules.EvaluateG004(context.Background(), args, sc)
	if d.Action != rules.Block {
		t.Errorf("zero config (default 50 threshold): 51 lines should Block, got %v", d.Action)
	}
}
