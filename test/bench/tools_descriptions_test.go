package bench_test

// tools_descriptions_test.go provides golden-file snapshot tests for tool
// descriptions, gating description changes per DESC-03. Three tests:
//
//   - TestToolDescriptionsComplete: every registered tool has a non-empty
//     BriefDescription
//   - TestToolDescriptionsTokenLimit: every BriefDescription is under 100
//     tokens (using word count < 80 as a conservative proxy)
//   - TestToolDescriptionsGoldenFile: tool names + brief descriptions match
//     a committed golden file baseline

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update", false, "update golden files")

// TestToolDescriptionsComplete asserts every registered tool has a non-empty
// BriefDescription (D-10a).
func TestToolDescriptionsComplete(t *testing.T) {
	bd := startBenchDaemon(t)
	descs := bd.BriefDescriptions()
	names := bd.RegistryNames()

	var missing []string
	for _, name := range names {
		if desc, ok := descs[name]; !ok || desc == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("tools missing BriefDescription: %v", missing)
	}
}

// TestToolDescriptionsTokenLimit asserts every BriefDescription is under 100
// tokens, using word count < 80 as a conservative proxy (D-10b, Assumption A1).
func TestToolDescriptionsTokenLimit(t *testing.T) {
	bd := startBenchDaemon(t)
	descs := bd.BriefDescriptions()

	const maxWords = 80 // ~100 tokens for English text
	var overLimit []string
	for name, desc := range descs {
		words := len(strings.Fields(desc))
		if words >= maxWords {
			overLimit = append(overLimit, fmt.Sprintf("%s (%d words)", name, words))
		}
	}
	if len(overLimit) > 0 {
		sort.Strings(overLimit)
		t.Fatalf("BriefDescriptions exceeding %d-word limit: %v", maxWords, overLimit)
	}
}

// TestToolDescriptionsGoldenFile compares tool names + brief descriptions
// against a golden file baseline (D-08, D-10c, DESC-03). A description
// change that doesn't update the golden file fails this test.
//
// To update the golden file after intentional description changes:
//
//	go test ./test/bench/... -run TestToolDescriptionsGoldenFile -update
//
// On first run (golden file missing), the file is auto-created.
func TestToolDescriptionsGoldenFile(t *testing.T) {
	bd := startBenchDaemon(t)
	descs := bd.BriefDescriptions()
	names := bd.RegistryNames()
	sort.Strings(names)

	var lines []string
	for _, name := range names {
		desc := descs[name]
		if desc == "" {
			desc = "(no brief description)"
		}
		lines = append(lines, fmt.Sprintf("%s: %s", name, desc))
	}
	actual := strings.Join(lines, "\n") + "\n"

	goldenPath := filepath.Join("testdata", "tool_descriptions.golden")

	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("creating testdata dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(actual), 0o644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
		t.Logf("Golden file updated: %s", goldenPath)
		return
	}

	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		// First run: create the golden file automatically.
		t.Logf("Golden file not found, creating: %s", goldenPath)
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("creating testdata dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(actual), 0o644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
		return
	}

	if string(golden) != actual {
		t.Fatalf("tool descriptions golden file mismatch.\n"+
			"Update golden: go test ./test/bench/... -run TestToolDescriptionsGoldenFile -update\n"+
			"Or manually update %s\n\n"+
			"EXPECTED (golden):\n%s\n\nACTUAL:\n%s",
			goldenPath, string(golden), actual)
	}
}
