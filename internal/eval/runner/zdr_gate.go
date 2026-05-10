package runner

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// CorpusSource identifies the origin of a task corpus.
type CorpusSource string

const (
	// CorpusSynthetic is the default for tasks without a source.yaml or with
	// source: synthetic. Allowed without env-var attestation.
	CorpusSynthetic CorpusSource = "synthetic"
	// CorpusHelixOSS is assigned when the corpus directory resolves under the
	// helix repo root. Allowed without env-var attestation (EVAL-06 secondary).
	CorpusHelixOSS CorpusSource = "helix-oss"
	// CorpusExternal marks tasks whose source.yaml declares source: external.
	// REQUIRES HELIX_EVAL_ZDR_VERIFIED=1 (T-67-06 mitigation).
	CorpusExternal CorpusSource = "external"
)

// corpusManifest is the optional per-task source declaration.
type corpusManifest struct {
	Source CorpusSource `yaml:"source"`
}

// AssertCorpusAllowed enforces the EVAL-06 Zero Data Retention policy:
//
//  1. If corpusDir resolves under the helix repo root → allow + log WARN.
//  2. For each task directory under corpusDir, read optional source.yaml.
//     Default when absent: synthetic.
//  3. If any task has source: external AND HELIX_EVAL_ZDR_VERIFIED != "1" →
//     return a wrapped error listing offending task IDs.
//  4. If HELIX_EVAL_ZDR_VERIFIED=1 is set with external sources → allow +
//     log WARN.
func AssertCorpusAllowed(corpusDir string) error {
	absCorpus, err := filepath.Abs(corpusDir)
	if err != nil {
		return fmt.Errorf("zdr_gate: abs path for corpus: %w", err)
	}

	// Step 1: detect helix-OSS corpus path.
	if isUnderHelixRepo(absCorpus) {
		log.Printf("WARN: zdr_gate: running against helix-OSS corpus %q (EVAL-06 permitted secondary)", absCorpus)
		return nil
	}

	// Step 2: scan task directories.
	entries, err := os.ReadDir(absCorpus)
	if err != nil {
		if os.IsNotExist(err) {
			// Empty / missing corpus — no tasks to check; allow.
			return nil
		}
		return fmt.Errorf("zdr_gate: read corpus dir: %w", err)
	}

	var externalTasks []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		src, err := readTaskSource(filepath.Join(absCorpus, entry.Name()))
		if err != nil {
			return fmt.Errorf("zdr_gate: read task %q source: %w", entry.Name(), err)
		}
		if src == CorpusExternal {
			externalTasks = append(externalTasks, entry.Name())
		}
	}

	// Step 3/4: enforce or allow external tasks.
	if len(externalTasks) == 0 {
		if os.Getenv("HELIX_EVAL_ZDR_VERIFIED") == "1" {
			log.Printf("INFO: zdr_gate: HELIX_EVAL_ZDR_VERIFIED=1 is set but no external tasks found")
		}
		return nil
	}

	if os.Getenv("HELIX_EVAL_ZDR_VERIFIED") != "1" {
		return fmt.Errorf(
			"zdr_gate: corpus contains external tasks [%s]; set HELIX_EVAL_ZDR_VERIFIED=1 after confirming provider retention-zero TOS (EVAL-06)",
			strings.Join(externalTasks, ", "),
		)
	}

	// HELIX_EVAL_ZDR_VERIFIED=1 set — allow with warning.
	log.Printf("WARN: zdr_gate: HELIX_EVAL_ZDR_VERIFIED=1 — running with external tasks [%s]; ensure TOS retention-zero is verified",
		strings.Join(externalTasks, ", "))
	return nil
}

// readTaskSource reads the optional source.yaml in taskDir and returns the
// declared CorpusSource. Returns CorpusSynthetic if the file is absent.
func readTaskSource(taskDir string) (CorpusSource, error) {
	path := filepath.Join(taskDir, "source.yaml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return CorpusSynthetic, nil
	}
	if err != nil {
		return CorpusSynthetic, fmt.Errorf("read source.yaml: %w", err)
	}

	var m corpusManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return CorpusSynthetic, fmt.Errorf("parse source.yaml: %w", err)
	}
	if m.Source == "" {
		return CorpusSynthetic, nil
	}
	return m.Source, nil
}

// isUnderHelixRepo returns true if the given absolute path resolves under the
// helix module root. The module root is located by walking upward from the
// corpus path to find a directory containing go.mod with the helix module
// declaration, or by checking for the .helix-repo-marker sentinel file.
func isUnderHelixRepo(absPath string) bool {
	dir := absPath
	for {
		// Check for .helix-repo-marker first (lightweight sentinel).
		if _, err := os.Stat(filepath.Join(dir, ".helix-repo-marker")); err == nil {
			return true
		}
		// Check for go.mod with the helix module name.
		if isHelixGoMod(filepath.Join(dir, "go.mod")) {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return false
}

// isHelixGoMod returns true if path is a go.mod that declares the helix module.
func isHelixGoMod(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	// Quick scan: look for "module github.com/agenthands/helix".
	return strings.Contains(string(data), "module github.com/agenthands/helix")
}
