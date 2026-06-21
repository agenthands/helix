package main

import (
	"strings"
	"testing"
)

func TestReplaceSection(t *testing.T) {
	input := "before\n<!-- BEGIN X -->\nold\n<!-- END X -->\nafter"
	got, err := replaceSection(input, "BEGIN X", "END X", "new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "before\n<!-- BEGIN X -->\nnew\n<!-- END X -->\nafter"
	if got != want {
		t.Errorf("replaceSection mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestReplaceSection_PreservesMarkers(t *testing.T) {
	input := "before\n<!-- BEGIN X -->\nold\n<!-- END X -->\nafter"
	first, err := replaceSection(input, "BEGIN X", "END X", "new")
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	second, err := replaceSection(first, "BEGIN X", "END X", "new")
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if first != second {
		t.Errorf("not idempotent:\nfirst:  %q\nsecond: %q", first, second)
	}
}

func TestReplaceSection_MissingMarker(t *testing.T) {
	input := "no markers here"
	_, err := replaceSection(input, "BEGIN X", "END X", "new")
	if err == nil {
		t.Fatal("expected error for missing markers")
	}
}

func TestGenerateToolTable(t *testing.T) {
	table := generateToolTable()
	lines := strings.Split(table, "\n")
	// Count pipe-delimited rows (skip header and separator)
	var dataRows int
	for _, line := range lines {
		if strings.HasPrefix(line, "|") && !strings.Contains(line, "---") && !strings.Contains(line, "Tool") {
			dataRows++
		}
	}
	if dataRows < 30 {
		t.Errorf("expected at least 30 tool rows, got %d", dataRows)
	}
}

func TestGenerateLanguageTable(t *testing.T) {
	table := generateLanguageTable()
	lines := strings.Split(table, "\n")
	var dataRows int
	for _, line := range lines {
		if strings.HasPrefix(line, "|") && !strings.Contains(line, "---") && !strings.Contains(line, "Language") {
			dataRows++
		}
	}
	if dataRows < 40 {
		t.Errorf("expected at least 40 language rows, got %d", dataRows)
	}
}

func TestToolTableContainsKnownTools(t *testing.T) {
	table := generateToolTable()
	knownVerbs := []string{
		"helix go-to-definition",
		"helix replace-symbol-body",
		"helix read-file",
		"helix get-diagnostics",
		"helix write-memory",
		"helix onboard-project",
		"helix switch-mode",
	}
	for _, verb := range knownVerbs {
		if !strings.Contains(table, verb) {
			t.Errorf("tool table missing known verb %q", verb)
		}
	}
}
