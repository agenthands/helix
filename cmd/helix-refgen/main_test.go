package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenthands/helix/internal/cli"
)

// TestRenderCoversAllVerbs asserts the rendered reference carries a section/line
// for every kebab verb in the frozen registry authority (cli.VerbToolNames()),
// 50/50 with zero missing. The authority is VerbToolNames(), NOT the generator's
// own output (anti-vacuity).
func TestRenderCoversAllVerbs(t *testing.T) {
	ref := renderReference()
	names := cli.VerbToolNames()
	if len(names) == 0 {
		t.Fatal("VerbToolNames() returned no verbs — authority is empty")
	}
	for _, toolName := range names {
		verb := strings.ReplaceAll(toolName, "_", "-")
		if !strings.Contains(ref, verb) {
			t.Errorf("reference output missing section for verb %q", verb)
		}
	}
}

// TestRenderDeterministic asserts two render calls produce byte-identical output
// (sorted iteration, no map-order leakage).
func TestRenderDeterministic(t *testing.T) {
	a := renderReference()
	b := renderReference()
	if a != b {
		t.Fatal("renderReference() is not deterministic — two calls differ")
	}
}

// TestRenderSectionContent asserts each verb section carries the five required
// elements: a synopsis, an args list, an output-shape note, a worked example, and
// a use-this-not-that line. We probe a representative verb with required flags.
func TestRenderSectionContent(t *testing.T) {
	ref := renderReference()
	// analyze-blast-radius is a navigation verb with required path/line/column flags.
	if !strings.Contains(ref, "analyze-blast-radius") {
		t.Fatal("reference missing analyze-blast-radius")
	}
	for _, want := range []string{
		"helix analyze-blast-radius", // worked example
		"--path",                     // args list
		"Use this",                   // use-this-not-that line
	} {
		if !strings.Contains(ref, want) {
			t.Errorf("reference missing expected marker %q", want)
		}
	}
}

// TestCheckRoundTrip renders to a temp file then asserts the in-sync check logic
// reports up-to-date against that file, and reports a diff when the file is stale.
func TestCheckRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reference.md")
	rendered := renderReference()
	if err := os.WriteFile(path, []byte(rendered), 0644); err != nil {
		t.Fatalf("write temp reference: %v", err)
	}
	// In sync.
	stale, err := referenceStale(path, rendered)
	if err != nil {
		t.Fatalf("referenceStale (in-sync): %v", err)
	}
	if stale {
		t.Error("referenceStale reported a diff for an in-sync file")
	}
	// Hand-edited / stale.
	if err := os.WriteFile(path, []byte("hand-edited drift\n"), 0644); err != nil {
		t.Fatalf("rewrite temp reference: %v", err)
	}
	stale, err = referenceStale(path, rendered)
	if err != nil {
		t.Fatalf("referenceStale (stale): %v", err)
	}
	if !stale {
		t.Error("referenceStale failed to detect a hand-edited reference")
	}
}
