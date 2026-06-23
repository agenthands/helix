//go:build ignore

// aider_edit_baseline_regen.go is the LOCAL-ONLY regenerator for the committed
// polyglot-edit baseline (BASELINE-01). It is invoked by `make bench-aider-edit`
// (NOT compiled into any binary — the //go:build ignore tag excludes it from the
// package build). It drives the REAL runAiderEditCell over the vendored go/wordy
// fixture against the warm daemon (HELIX_BIN), captures the deterministic-metrics-only
// result.v2.json, renders the fixed-format BENCH-RESULTS.md, and writes both into the
// force-tracked bench/reports/aider-edit-baseline/ dir. The artifact is byte-
// reproducible: re-running this regenerator over the same fixture + binary yields
// byte-identical bytes (asserted by bench/runtime TestAiderEditBaseline* tests).
//
// Usage (from repo root):
//
//	go build -o ./helix ./cmd/helix
//	HELIX_BIN="$(pwd)/helix" go run bench/runtime/aider_edit_baseline_regen.go
//
// It stays local-only — there is no CI benchstat gate for it (STATE.md
// local-only-benches constraint). The committed artifact under version control IS the
// CI contract; this script only regenerates it on demand.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/agenthands/helix/bench/aggregator"
	benchruntime "github.com/agenthands/helix/bench/runtime"
)

func main() {
	helixBin := os.Getenv("HELIX_BIN")
	if helixBin == "" {
		fmt.Fprintln(os.Stderr, "HELIX_BIN must be set (build with: go build -o ./helix ./cmd/helix)")
		os.Exit(2)
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "getwd:", err)
		os.Exit(1)
	}
	fixtureDir := filepath.Join(repoRoot, "bench", "datasets", "aider-polyglot",
		"fixtures", "go", "exercises", "practice", "wordy")
	baselineDir := filepath.Join(repoRoot, "bench", "reports", "aider-edit-baseline")
	if err := os.MkdirAll(baselineDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mkdir baseline:", err)
		os.Exit(1)
	}
	outDir, err := os.MkdirTemp("", "aider-edit-regen-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "mkdtemp:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(outDir)

	cfg := benchruntime.CellConfig{
		RunID:       "aider-edit-baseline",
		Benchmark:   "aider-polyglot",
		Language:    "go",
		Task:        "wordy",
		Mode:        "aider_edit",
		RunIndex:    0,
		HelixBin:    helixBin,
		SeedDir:     fixtureDir,
		OutDir:      outDir,
		RunnersRoot: filepath.Join(repoRoot, "bench", "runners"),
		Agent:       "scripted",
	}
	res, err := benchruntime.RunCell(context.Background(), cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "runcell:", err)
		os.Exit(1)
	}
	resultBytes, err := os.ReadFile(res.ResultPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read result.v2.json:", err)
		os.Exit(1)
	}
	summary, err := aggregator.RenderAiderEditBaseline(resultBytes)
	if err != nil {
		fmt.Fprintln(os.Stderr, "render summary:", err)
		os.Exit(1)
	}

	if err := os.WriteFile(filepath.Join(baselineDir, "result.v2.json"), resultBytes, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write result.v2.json:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(baselineDir, "BENCH-RESULTS.md"), summary, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write BENCH-RESULTS.md:", err)
		os.Exit(1)
	}
	fmt.Printf("regenerated committed baseline at %s\n", baselineDir)
}
