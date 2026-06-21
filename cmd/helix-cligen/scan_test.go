package main

import (
	"testing"
)

// findArg returns the argField with the given json key, or nil.
func findArg(info *argInfo, jsonKey string) *argField {
	if info == nil {
		return nil
	}
	for i := range info.fields {
		if info.fields[i].jsonKey == jsonKey {
			return &info.fields[i]
		}
	}
	return nil
}

// TestScanAllTools is the VERB-04 contract: a go/packages + AST scan over the
// helix module recovers the tool-name -> *Args struct binding for every tool
// that registers via the typed-args mcpsdk.AddTool shape. The scan deliberately
// produces a SUPERSET of the live skill.ToolProviders() name set (it also sees
// the direct-func-literal core tools like activate_project/ping/echo); Task 2's
// main.go intersects against the registry to define the catalog.
func TestScanAllTools(t *testing.T) {
	infos, err := scanToolArgs("github.com/agenthands/helix/internal/...")
	if err != nil {
		t.Fatalf("scanToolArgs: %v", err)
	}
	if len(infos) == 0 {
		t.Fatalf("scan recovered zero tools; expected the typed-args AddTool surface")
	}

	// The scan must recover a SUPERSET that includes a representative tool from
	// every typed-args registration site we know about.
	wantPresent := []string{
		"go_to_definition",    // symbols (WrapToolSpan-wrapped)
		"replace_symbol_body", // edit (WrapToolSpan-wrapped, has []ReceiptID)
		"read_file",           // fileops
		"get_diagnostics",     // diag
		"explain_symbol_deep", // semantic (WrapToolSpan-wrapped)
		"activate_project",    // core (DIRECT func literal, not WrapToolSpan)
		"ping",                // core (direct func literal)
	}
	for _, name := range wantPresent {
		if _, ok := infos[name]; !ok {
			t.Errorf("scan missing tool %q", name)
		}
	}
}

// TestScan_GoToDefinitionFields asserts the args type for go_to_definition is
// recovered with its json keys read from struct tags.
func TestScan_GoToDefinitionFields(t *testing.T) {
	infos, err := scanToolArgs("github.com/agenthands/helix/internal/...")
	if err != nil {
		t.Fatalf("scanToolArgs: %v", err)
	}
	info, ok := infos["go_to_definition"]
	if !ok {
		t.Fatalf("go_to_definition not recovered")
	}
	for _, key := range []string{"path", "line", "column"} {
		if findArg(info, key) == nil {
			t.Errorf("go_to_definition missing json field %q (got %v)", key, jsonKeys(info))
		}
	}
}

// TestScan_RequiredFromOmitempty asserts a field WITHOUT ,omitempty is required
// and a field WITH ,omitempty is optional.
func TestScan_RequiredFromOmitempty(t *testing.T) {
	infos, err := scanToolArgs("github.com/agenthands/helix/internal/...")
	if err != nil {
		t.Fatalf("scanToolArgs: %v", err)
	}
	info, ok := infos["replace_symbol_body"]
	if !ok {
		t.Fatalf("replace_symbol_body not recovered")
	}
	// path has no omitempty -> required
	if f := findArg(info, "path"); f == nil || !f.required {
		t.Errorf("replace_symbol_body path should be required; got %+v", f)
	}
	// search_body has ,omitempty -> optional
	if f := findArg(info, "search_body"); f == nil || f.required {
		t.Errorf("replace_symbol_body search_body should be optional; got %+v", f)
	}
	// receipts has ,omitempty -> optional, and is a non-scalar type
	if f := findArg(info, "receipts"); f == nil || f.required {
		t.Errorf("replace_symbol_body receipts should be optional; got %+v", f)
	}
}

// TestScan_HelpFromJSONSchema asserts a jsonschema tag becomes help text.
func TestScan_HelpFromJSONSchema(t *testing.T) {
	infos, err := scanToolArgs("github.com/agenthands/helix/internal/...")
	if err != nil {
		t.Fatalf("scanToolArgs: %v", err)
	}
	info := infos["go_to_definition"]
	f := findArg(info, "path")
	if f == nil {
		t.Fatalf("go_to_definition path not found")
	}
	if f.help == "" {
		t.Errorf("go_to_definition path help should come from jsonschema tag; got empty")
	}
}

// TestScan_DirectFuncLiteralShape asserts the core tools registered with a
// DIRECT func literal (not WrapToolSpan) yield the same (name, argsType) pair.
func TestScan_DirectFuncLiteralShape(t *testing.T) {
	infos, err := scanToolArgs("github.com/agenthands/helix/internal/...")
	if err != nil {
		t.Fatalf("scanToolArgs: %v", err)
	}
	info, ok := infos["activate_project"]
	if !ok {
		t.Fatalf("activate_project (direct func literal) not recovered")
	}
	// ActivateProjectArgs has a RepoPath field -> json key repo_path.
	if findArg(info, "repo_path") == nil {
		t.Errorf("activate_project should have repo_path field; got %v", jsonKeys(info))
	}
}

// TestScan_TypedToolsHaveFields asserts every typed-args tool the scan resolves
// has at least one field. A typed tool resolving to zero fields is a scan miss.
// (Tools registered via AddSkillTool's map[string]any are recorded with empty
// fields by design and are NOT in the typed-args result here.)
func TestScan_TypedToolsHaveFields(t *testing.T) {
	infos, err := scanToolArgs("github.com/agenthands/helix/internal/...")
	if err != nil {
		t.Fatalf("scanToolArgs: %v", err)
	}
	for name, info := range infos {
		if len(info.fields) == 0 {
			t.Errorf("typed-args tool %q resolved to zero fields — scan miss", name)
		}
	}
}

func jsonKeys(info *argInfo) []string {
	if info == nil {
		return nil
	}
	var ks []string
	for _, f := range info.fields {
		ks = append(ks, f.jsonKey)
	}
	return ks
}
