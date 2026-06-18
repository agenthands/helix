// Package patch_validator computes the git-derived patch metrics
// (files_modified, edit_locality per METRIC-04/D-04, edit_distance_patch per
// METRIC-02) by inspecting the cell's repo working tree. It carries the genuinely
// net-new edit_locality formula of the phase.
//
// Every git invocation uses exec.CommandContext with a FIXED argv and
// cmd.Dir = repoDir — never a shell, never string interpolation of repo content
// into a command line (T-79-02-01 command-injection mitigation, mirroring
// bench/runtime/cell.go runVerify and bench/languages/go.GoRunner.RunTests). A ctx
// cancellation/timeout is an infra error (returned as a MetricError); a valid repo
// that simply has no changes yields an empty result, not a panic.
//
// edit_locality = 1 − modified/total where:
//   - total (denominator) = git ls-files (tracked files only, D-04). Untracked
//     scratch files do NOT inflate the denominator.
//   - modified (numerator) = git diff --name-only ∩ tracked. Paths are resolved
//     under repoDir before counting (T-79-02-02 path-prefix invariant, mirroring
//     internal/eval/trace/merge.go:122-139); a path escaping the subtree is not
//     counted toward files_modified.
//
// Interpretation (A3): a single-file edit in an N-file repo yields 1 − 1/N, which
// approaches 1.0 as N grows — "locality" measures how surgical the patch is
// relative to the tracked tree. A zero-tracked repo has an undefined denominator
// and is nulled with a MetricError.
package patch_validator

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/agenthands/helix/bench/evaluators"
)

const graderName = "patch_validator"

// EditLocality returns (edit_locality, files_modified, *MetricError). The
// locality is nil with a MetricError when the repo has zero tracked files
// (undefined denominator); files_modified is still reported in that case.
func EditLocality(ctx context.Context, repoDir string) (*float64, *int, *evaluators.MetricError) {
	tracked, err := gitLines(ctx, repoDir, "ls-files")
	if err != nil {
		return nil, nil, &evaluators.MetricError{
			Metric: "edit_locality",
			Grader: graderName,
			Reason: "git ls-files failed: " + err.Error(),
		}
	}
	trackedSet := make(map[string]struct{}, len(tracked))
	for _, p := range tracked {
		trackedSet[p] = struct{}{}
	}

	changed, err := gitLines(ctx, repoDir, "diff", "--name-only")
	if err != nil {
		return nil, nil, &evaluators.MetricError{
			Metric: "edit_locality",
			Grader: graderName,
			Reason: "git diff --name-only failed: " + err.Error(),
		}
	}

	modified := 0
	for _, p := range changed {
		if _, ok := trackedSet[p]; !ok {
			continue // untracked / not in denominator
		}
		if !underRepo(repoDir, p) {
			continue // path-prefix invariant: escapes the subtree → not counted
		}
		modified++
	}

	n := len(trackedSet)
	if n == 0 {
		m := modified
		return nil, &m, &evaluators.MetricError{
			Metric: "edit_locality",
			Grader: graderName,
			Reason: "zero git-tracked files (undefined denominator)",
		}
	}

	loc := 1.0 - float64(modified)/float64(n)
	m := modified
	return &loc, &m, nil
}

// EditDistancePatch returns the deterministic patch edit distance as the sum of
// added + deleted lines reported by `git diff --numstat` across the working tree
// (METRIC-02). This is the chosen deterministic definition Plan 04 documents in
// METRICS.md. Binary files (numstat reports "-\t-") contribute 0.
func EditDistancePatch(ctx context.Context, repoDir string) (*int, *evaluators.MetricError) {
	lines, err := gitLines(ctx, repoDir, "diff", "--numstat")
	if err != nil {
		return nil, &evaluators.MetricError{
			Metric: "edit_distance_patch",
			Grader: graderName,
			Reason: "git diff --numstat failed: " + err.Error(),
		}
	}
	total := 0
	for _, l := range lines {
		// numstat format: "<added>\t<deleted>\t<path>"; binary → "-\t-\t<path>".
		fields := strings.SplitN(l, "\t", 3)
		if len(fields) < 3 {
			continue
		}
		added, aerr := strconv.Atoi(fields[0])
		deleted, derr := strconv.Atoi(fields[1])
		if aerr != nil || derr != nil {
			continue // binary file ("-") or malformed line
		}
		total += added + deleted
	}
	return &total, nil
}

// underRepo reports whether the (repo-relative) path p resolves to a child of
// repoDir, mirroring the path-prefix invariant in merge.go:122-139.
func underRepo(repoDir, p string) bool {
	abs := p
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(repoDir, p)
	}
	rel, err := filepath.Rel(repoDir, abs)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// gitLines runs `git <args...>` with cmd.Dir = repoDir (fixed argv, no shell) and
// returns the non-empty stdout lines. A ctx cancellation/timeout is surfaced as an
// error (infra); a non-zero git exit on a valid repo returns the (possibly empty)
// captured output without erroring, so a clean working tree yields zero lines.
func gitLines(ctx context.Context, repoDir string, args ...string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		// ls-files / diff / numstat all exit 0 on a valid repo with no changes.
		// Any error here (git not on PATH, repo unreadable, or a non-zero exit on
		// a broken repo) is infra: surface it as a MetricError rather than
		// silently undercount tracked/changed files.
		return nil, err
	}
	var lines []string
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimRight(l, "\r"); l != "" {
			lines = append(lines, l)
		}
	}
	return lines, nil
}
