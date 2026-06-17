// Package runners mode_resolver.go maps a canonical bench MODE name (e.g.
// "your_agent_full") to a Helix profile name (e.g. "bench-full") by reading the
// YAML frontmatter of bench/runners/<mode>/MODE.md (D-05).
//
// Design notes:
//   - The mapping lives in MODE.md frontmatter, NOT in a hard-coded Go map, so
//     Phase 80 adds the remaining five ablation modes by dropping in new
//     bench/runners/<mode>/MODE.md directories with NO Go change. The lookup is
//     table-driven (the filesystem is the table).
//   - This is deliberately NOT a reuse of internal/eval/runner.profileForMode,
//     which hard-codes the eval modes (baseline/native/semantic/...). Bench
//     modes are a distinct namespace (see 77-RESEARCH anti-patterns).
//   - Mode names flow into filepath.Join as a path segment, so they are
//     validated (reject "..", separators, leading dot, absolute paths) BEFORE
//     any filesystem access — the V5/T-77-01 control, mirroring
//     internal/eval/runner.validateTaskID.
package runners

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// modeFrontmatter is the strict schema of a MODE.md YAML frontmatter block.
// Phase 80 owns the full ABLATE-01 convention; Phase 77 keeps it to the two
// keys the resolver needs (D-05, Claude's discretion). KnownFields(true) makes
// any extra key a hard parse error so malformed frontmatter fails closed.
type modeFrontmatter struct {
	Mode    string `yaml:"mode"`
	Profile string `yaml:"profile"`
}

// runnersRoot returns the absolute path of this package's directory
// (bench/runners), derived from runtime.Caller so the lookup does not depend on
// the process working directory. This is the default root for ResolveProfile.
func runnersRoot() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("mode_resolver: cannot determine package directory via runtime.Caller")
	}
	return filepath.Dir(thisFile), nil
}

// validateModeName rejects mode names that could escape the runners root via
// path traversal. Mirrors internal/eval/runner.validateTaskID (T-77-01 / V5).
// MUST be called before any filepath.Join with the mode name.
func validateModeName(mode string) error {
	if mode == "" {
		return errors.New("mode name is empty")
	}
	if mode != filepath.Clean(mode) || strings.ContainsAny(mode, `/\`) || strings.HasPrefix(mode, ".") {
		return fmt.Errorf("mode name %q contains path separators, parent refs, or a leading dot", mode)
	}
	return nil
}

// ResolveProfile maps a bench mode name to its profile name by reading
// bench/runners/<mode>/MODE.md frontmatter. The runners root is derived from
// the package directory (runtime.Caller), so it works regardless of the caller's
// working directory.
//
// Returns a non-nil error for an unknown mode (no MODE.md dir), a
// path-traversal mode name (rejected before any FS access), or malformed
// frontmatter (strict KnownFields).
func ResolveProfile(mode string) (string, error) {
	root, err := runnersRoot()
	if err != nil {
		return "", err
	}
	return ResolveProfileFromRoot(root, mode)
}

// ResolveProfileFromRoot is ResolveProfile with an injectable runners root, for
// testability. root is the directory that contains the per-mode subdirectories
// (each holding a MODE.md).
func ResolveProfileFromRoot(root, mode string) (string, error) {
	// V5/T-77-01: validate BEFORE joining the mode into a path.
	if err := validateModeName(mode); err != nil {
		return "", fmt.Errorf("mode_resolver: %w", err)
	}

	mdPath := filepath.Join(root, mode, "MODE.md")
	data, err := os.ReadFile(mdPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("mode_resolver: unknown mode %q (no %s)", mode, mdPath)
		}
		return "", fmt.Errorf("mode_resolver: read %q: %w", mdPath, err)
	}

	fm, err := parseFrontmatter(data)
	if err != nil {
		return "", fmt.Errorf("mode_resolver: mode %q: %w", mode, err)
	}
	if strings.TrimSpace(fm.Profile) == "" {
		return "", fmt.Errorf("mode_resolver: mode %q MODE.md frontmatter has no profile", mode)
	}
	return fm.Profile, nil
}

// parseFrontmatter extracts and strictly-decodes the leading "---"-delimited
// YAML frontmatter block of a MODE.md file. Unknown keys are a hard error
// (KnownFields true), mirroring runner.LoadScript (scripted_agent.go:48).
func parseFrontmatter(data []byte) (modeFrontmatter, error) {
	fmBytes, err := extractFrontmatter(data)
	if err != nil {
		return modeFrontmatter{}, err
	}

	var fm modeFrontmatter
	dec := yaml.NewDecoder(bytes.NewReader(fmBytes))
	dec.KnownFields(true)
	if err := dec.Decode(&fm); err != nil {
		return modeFrontmatter{}, fmt.Errorf("parse frontmatter: %w", err)
	}
	return fm, nil
}

// extractFrontmatter returns the bytes between the leading "---" line and the
// next "---" line. The file MUST start with a "---" delimiter.
func extractFrontmatter(data []byte) ([]byte, error) {
	// Normalize CRLF so the delimiter match is line-ending agnostic.
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, errors.New("MODE.md does not start with a '---' frontmatter delimiter")
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return []byte(strings.Join(lines[1:i], "\n")), nil
		}
	}
	return nil, errors.New("MODE.md frontmatter is not terminated by a closing '---'")
}
