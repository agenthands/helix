package languages

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// CoverageReport is the per-language declared-vs-covered result (D-11). Declared
// is the count of capability classes the runner statically declares; Covered is
// how many of those are present in the corpus (tagged on at least one task.json);
// Missing lists the declared capabilities with NO covering fixture. A 10/10 Go
// report (Declared==Covered, Missing empty) is the criterion C2 phase gate.
type CoverageReport struct {
	Language string
	Declared int
	Covered  int
	Missing  []Capability
}

// Coverage walks <corpusRoot>/<benchmark>/<lang>/*/task.json, decodes the
// `capability` field of each (the D-06 source of truth — it deliberately does NOT
// parse the task id string, mitigating T-78-11), builds the covered set, and
// reports how many of the runner's `declared` capabilities are covered, with the
// missing ones listed (D-11 "gaps are explicit").
//
// WR-04: the benchmark segment is an explicit parameter, not a hardcoded
// "internal-toolbench" literal. `benchmark` is a first-class axis everywhere else
// in the harness (Cell.Benchmark, cellSeedDir, ExpandMatrix, the Makefile SUITE
// parameter); hardcoding it here would silently read the wrong directory the
// moment a second suite (Rust/TS/Python, anticipated by runner.go) is added.
//
// Only capabilities in `declared` are counted toward Covered: a task tagged with a
// capability the runner does not declare does not inflate the report.
func Coverage(corpusRoot, benchmark, lang string, declared []Capability) (CoverageReport, error) {
	langDir := filepath.Join(corpusRoot, benchmark, lang)

	covered, err := coveredCapabilities(langDir)
	if err != nil {
		return CoverageReport{}, err
	}

	declaredSet := make(map[Capability]bool, len(declared))
	for _, c := range declared {
		declaredSet[c] = true
	}

	// WR-04: count over the DEDUPLICATED declaredSet, not the raw `declared`
	// slice. Iterating the slice would double-count a repeated capability in
	// Covered while Declared (len(declaredSet)) stays deduped, yielding the
	// incoherent Covered > Declared and corrupting the 10/10 gate semantics for a
	// future hand-authored runner that repeats a capability.
	var missing []Capability
	coveredN := 0
	for c := range declaredSet {
		if covered[c] {
			coveredN++
		} else {
			missing = append(missing, c)
		}
	}
	sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })

	return CoverageReport{
		Language: lang,
		Declared: len(declaredSet),
		Covered:  coveredN,
		Missing:  missing,
	}, nil
}

// coveredCapabilities reads the `capability` field from every
// <langDir>/*/task.json and returns the set of capabilities the corpus covers.
// Directories without a task.json are skipped (not every dir is a corpus cell).
func coveredCapabilities(langDir string) (map[Capability]bool, error) {
	entries, err := os.ReadDir(langDir)
	if err != nil {
		return nil, fmt.Errorf("reading corpus dir %s: %w", langDir, err)
	}

	covered := make(map[Capability]bool)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(langDir, e.Name(), "task.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		// Decode ONLY the capability field (D-06 source of truth — NOT the id).
		var meta struct {
			Capability Capability `json:"capability"`
		}
		if err := json.Unmarshal(raw, &meta); err != nil {
			return nil, fmt.Errorf("decoding %s: %w", path, err)
		}
		if meta.Capability != "" {
			covered[meta.Capability] = true
		}
	}
	return covered, nil
}
