package report

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"
)

// RunMetadata captures per-run environment information per D-03 (record, don't pin).
// EnvKeys captures KEYS only — never values (T-67-Pitfall-1 mitigation).
type RunMetadata struct {
	RunID         string    `json:"run_id"`
	StartedAt     time.Time `json:"started_at"`
	EndedAt       time.Time `json:"ended_at"`
	ClaudeVersion string    `json:"claude_version"` // verbatim `claude --version` output; "" if not on PATH
	HelixVersion  string    `json:"helix_version"`
	GOOS          string    `json:"goos"`
	GOARCH        string    `json:"goarch"`
	Modes         []string  `json:"modes"`
	EnvKeys       []string  `json:"env_keys"` // KEYS only; never values (T-67-Pitfall-1)
	CorpusDir     string    `json:"corpus_dir"`
}

// CaptureRunMetadata collects the run environment. It invokes `claude --version`
// once and records the output verbatim. If claude is not on PATH, ClaudeVersion
// is left empty (not an error per D-03).
func CaptureRunMetadata(modes []string, corpusDir string) RunMetadata {
	now := time.Now().UTC()

	// Read claude version — best effort.
	claudeVersion := ""
	if claudePath, err := exec.LookPath("claude"); err == nil {
		out, err := exec.Command(claudePath, "--version").Output()
		if err == nil {
			claudeVersion = strings.TrimSpace(string(out))
		}
	}

	// Helix version via runtime/debug.ReadBuildInfo.
	helixVersion := "(unknown)"
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			helixVersion = info.Main.Version
		} else {
			// Scan for vcs.revision in build settings.
			for _, s := range info.Settings {
				if s.Key == "vcs.revision" && s.Value != "" {
					helixVersion = "git-" + s.Value[:min(s.Value, 12)]
					break
				}
			}
			if helixVersion == "(unknown)" {
				helixVersion = "(devel)"
			}
		}
	}

	// Collect env keys (not values) — T-67-Pitfall-1 mitigation.
	envKeys := captureEnvKeys()

	// Generate run-id from timestamp (caller can override).
	runID := now.Format("20060102T150405Z")

	return RunMetadata{
		RunID:         runID,
		StartedAt:     now,
		EndedAt:       now, // caller updates EndedAt when run completes
		ClaudeVersion: claudeVersion,
		HelixVersion:  helixVersion,
		GOOS:          runtime.GOOS,
		GOARCH:        runtime.GOARCH,
		Modes:         modes,
		EnvKeys:       envKeys,
		CorpusDir:     corpusDir,
	}
}

// captureEnvKeys returns a sorted, deduplicated list of environment variable
// names present in the current process environment. Values are never captured
// (T-67-Pitfall-1 mitigation).
func captureEnvKeys() []string {
	seen := make(map[string]struct{})
	for _, kv := range os.Environ() {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) >= 1 && parts[0] != "" {
			seen[parts[0]] = struct{}{}
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// WriteRunMetadata serialises m to JSON and writes it to path with mode 0600.
func WriteRunMetadata(path string, m RunMetadata) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("report.WriteRunMetadata marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("report.WriteRunMetadata write %q: %w", path, err)
	}
	return nil
}

// min returns the minimum of len(s) and n, used for truncating git SHA.
func min(s string, n int) int {
	if len(s) < n {
		return len(s)
	}
	return n
}
