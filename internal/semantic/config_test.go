// config_test.go — Tests for GuardrailsConfig D-22 shape decoding.
// Phase 66 Plan 02.
package semantic

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// d22YAMLFixture is the verbatim D-22 LOCKED config shape from
// .planning/phases/66-agent-guardrails/66-CONTEXT.md.
// This fixture serves as the acceptance gate: if the struct shape drifts
// from D-22, the decode-and-assert test below fails at the first mismatch.
const d22YAMLFixture = `
guardrails:
  enforcement: warn
  receipt_ttl: 5m
  rules:
    G-001:
      enforcement: warn
    G-002:
      enforcement: enforce
    G-003:
      enforcement: enforce
    G-004:
      enforcement: warn
    G-005:
      enforcement: enforce
  tools:
    rename_symbol:
      enforcement: enforce
    safe_delete_symbol:
      enforcement: enforce
    replace_symbol_body:
      enforcement: enforce
    fuzzy_edit:
      enforcement: warn
    replace_in_file:
      enforcement: warn
  G-004:
    max_changed_lines: 50
    max_files: 1
    max_file_change_ratio: 0.30
    enable_ratio_trigger: true
  G-005:
    path_globs:
      - "**/auth/*.go"
      - "**/crypto/*.go"
    identifier_patterns:
      - "^(?i)(password|secret|token)"
    import_patterns:
      go:
        - "crypto/tls"
        - "golang.org/x/crypto"
`

// TestGuardrailsConfig_D22Decode decodes the verbatim D-22 YAML fixture into
// GuardrailsConfig and asserts every field that the plan acceptance criteria
// require. If any koanf tag is wrong or a field is missing, this test fails.
func TestGuardrailsConfig_D22Decode(t *testing.T) {
	// Write the fixture to a temp file so we can use the file provider
	// (rawbytes provider is not in go.mod; file provider is already imported).
	dir := t.TempDir()
	fixturePath := filepath.Join(dir, "guardrails_d22.yaml")
	if err := os.WriteFile(fixturePath, []byte(d22YAMLFixture), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	k := koanf.New(".")
	if err := k.Load(file.Provider(fixturePath), yaml.Parser()); err != nil {
		t.Fatalf("koanf load: %v", err)
	}

	var cfg GuardrailsConfig
	if err := k.Unmarshal("guardrails", &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// Global enforcement + receipt TTL.
	if cfg.Enforcement != "warn" {
		t.Errorf("Enforcement: got %q, want %q", cfg.Enforcement, "warn")
	}
	if cfg.ReceiptTTL.String() != "5m0s" {
		t.Errorf("ReceiptTTL: got %q, want %q", cfg.ReceiptTTL, "5m0s")
	}

	// Per-rule enforcement.
	wantRules := map[string]string{
		"G-001": "warn",
		"G-002": "enforce",
		"G-003": "enforce",
		"G-004": "warn",
		"G-005": "enforce",
	}
	for id, want := range wantRules {
		got, ok := cfg.Rules[id]
		if !ok {
			t.Errorf("Rules[%q]: missing", id)
			continue
		}
		if got.Enforcement != want {
			t.Errorf("Rules[%q].Enforcement: got %q, want %q", id, got.Enforcement, want)
		}
	}

	// Per-tool enforcement.
	wantTools := map[string]string{
		"rename_symbol":      "enforce",
		"safe_delete_symbol": "enforce",
		"replace_symbol_body": "enforce",
		"fuzzy_edit":          "warn",
		"replace_in_file":     "warn",
	}
	for tool, want := range wantTools {
		got, ok := cfg.Tools[tool]
		if !ok {
			t.Errorf("Tools[%q]: missing", tool)
			continue
		}
		if got.Enforcement != want {
			t.Errorf("Tools[%q].Enforcement: got %q, want %q", tool, got.Enforcement, want)
		}
	}

	// G004 thresholds.
	if cfg.G004.MaxChangedLines != 50 {
		t.Errorf("G004.MaxChangedLines: got %d, want 50", cfg.G004.MaxChangedLines)
	}
	if cfg.G004.MaxFiles != 1 {
		t.Errorf("G004.MaxFiles: got %d, want 1", cfg.G004.MaxFiles)
	}
	if cfg.G004.MaxFileChangeRatio != 0.30 {
		t.Errorf("G004.MaxFileChangeRatio: got %f, want 0.30", cfg.G004.MaxFileChangeRatio)
	}
	if !cfg.G004.EnableRatioTrigger {
		t.Error("G004.EnableRatioTrigger: got false, want true")
	}

	// G005 patterns.
	if len(cfg.G005.PathGlobs) == 0 {
		t.Error("G005.PathGlobs: empty, want non-empty")
	}
	if len(cfg.G005.IdentifierPatterns) == 0 {
		t.Error("G005.IdentifierPatterns: empty, want non-empty")
	}
	goPatterns, ok := cfg.G005.ImportPatterns["go"]
	if !ok || len(goPatterns) == 0 {
		t.Error("G005.ImportPatterns[\"go\"]: empty or missing, want populated")
	}
}

// TestRuleConfig_EnforcementField confirms that RuleConfig.Enforcement is
// accessible and zero-valued when absent (koanf does not error on missing keys).
func TestRuleConfig_EnforcementField(t *testing.T) {
	var r RuleConfig
	if r.Enforcement != "" {
		t.Errorf("zero-value RuleConfig.Enforcement: got %q, want empty", r.Enforcement)
	}
}

// TestToolConfig_EnforcementField is the parallel test for ToolConfig.
func TestToolConfig_EnforcementField(t *testing.T) {
	var tc ToolConfig
	if tc.Enforcement != "" {
		t.Errorf("zero-value ToolConfig.Enforcement: got %q, want empty", tc.Enforcement)
	}
}

// TestG004Config_ZeroValues confirms the G004Config zero values compile and
// are distinct from the D-16 defaults (which are populated via defaults.go).
func TestG004Config_ZeroValues(t *testing.T) {
	var g G004Config
	if g.MaxChangedLines != 0 || g.MaxFiles != 0 || g.MaxFileChangeRatio != 0.0 || g.EnableRatioTrigger {
		t.Errorf("G004Config zero-value unexpected: %+v", g)
	}
}

// TestG005Config_ZeroValues confirms the G005Config zero values compile cleanly.
func TestG005Config_ZeroValues(t *testing.T) {
	var g G005Config
	if g.PathGlobs != nil || g.IdentifierPatterns != nil || g.ImportPatterns != nil {
		t.Errorf("G005Config zero-value unexpected: %+v", g)
	}
}
