package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	aiderpolyglot "github.com/agenthands/helix/bench/datasets/aider-polyglot"
	"github.com/agenthands/helix/internal/forwarder"
)

// aiderEditMode is the bench-mode name of the polyglot-edit arm. The arm is
// detected BY MODE NAME in RunCell (NOT a MODE.md frontmatter key — the resolver
// is strict two-key with KnownFields(true)), mirroring baselineRagMode (rag.go:37).
// It is declared here so both this agent file and the Plan 02 cell file share one
// const and can never drift on the spelling.
const aiderEditMode = "aider_edit"

// applyEditFn is the per-stub edit seam. The LIVE path (newDeterministicEditAgent)
// fills it with a daemon replace_in_file tools/call over the open session; the
// hermetic test fills it with an in-process file write. Keeping the apply step
// behind this minimal seam lets the SOLE authoritative hermetic proof (Pitfall 1)
// drive RunExercise with NO daemon / HELIX_BIN / network while the live path
// exercises the real gRPC StreamMCP wire end-to-end in Plan 02 under HELIX_BIN.
type applyEditFn func(ctx context.Context, workDir, stub, body string) error

// newEditAgentWithApply is the testable core of the deterministic EDIT agent. It
// returns an aiderpolyglot.AgentFn that, for each ex.Config.Files.Solution[i]
// stub, reads the reference body from ex.SrcDir/ex.Config.Files.Example[i] and
// applies it to the work-dir stub through the injected apply seam. On success of
// every stub edit it sets *applied = true and returns nil; on any apply error it
// sets *applied = false and returns a wrapped error (fail-closed).
//
// Path safety (T-100-01): the stub and example path segments are taken ONLY from
// the validated Config.Files.* lists (already V5-checked by the loader's
// validatePathSegment + copyFile guard) — never a raw untrusted relpath.
//
// Anti-tamper discipline: this agent edits ONLY files.solution stubs, never
// files.test; the WR-01 pristine-test restore stays in RunExercise (loader),
// never moved here.
func newEditAgentWithApply(applied *bool, apply applyEditFn) aiderpolyglot.AgentFn {
	return func(ctx context.Context, ex *aiderpolyglot.Exercise, workDir, _ string) error {
		sol := ex.Config.Files.Solution
		exm := ex.Config.Files.Example
		if len(sol) == 0 {
			*applied = false
			return fmt.Errorf("bench/runtime: aider_edit agent: exercise %q has no solution stub", ex.Name)
		}
		if len(exm) < len(sol) {
			*applied = false
			return fmt.Errorf("bench/runtime: aider_edit agent: exercise %q has fewer example files than solution stubs", ex.Name)
		}
		for i, stub := range sol {
			// Reference body from the .meta/example.<ext> under the pristine SrcDir.
			refBody, err := os.ReadFile(filepath.Join(ex.SrcDir, exm[i]))
			if err != nil {
				*applied = false
				return fmt.Errorf("bench/runtime: aider_edit agent: read reference %q: %w", exm[i], err)
			}
			if err := apply(ctx, workDir, stub, string(refBody)); err != nil {
				*applied = false
				return fmt.Errorf("bench/runtime: aider_edit agent: apply %q: %w", stub, err)
			}
		}
		*applied = true
		return nil
	}
}

// newDeterministicEditAgent is the LIVE constructor. It returns an AgentFn that
// dials the warm daemon IN-PROCESS over the gRPC StreamMCP wire (forwarder.OpenSession
// — NOT shelling the helix binary), points the daemon workspace at the cloned
// exercise repo via activate_project (drive.go:87 uses repo_path), and applies
// each reference body to its stub through the REAL replace_in_file EDIT verb.
//
// replace_in_file does pattern→replacement substitution (it has NO whole-file
// content key; internal/kernel/fileops/tools.go:52-59). To get a deterministic
// whole-stub replacement, the live apply seam reads the CURRENT stub body and
// passes it verbatim as the literal `pattern` (is_regex:false) with the reference
// body as `replacement`, so the single literal occurrence (the entire stub) is
// replaced.
func newDeterministicEditAgent(sockPath string, applied *bool) aiderpolyglot.AgentFn {
	// A bench driver needs no structured logging surfaced; discard it (drive.go:71).
	logger := slog.New(slog.NewTextHandler(nopWriter{}, nil))

	return func(ctx context.Context, ex *aiderpolyglot.Exercise, workDir, prompt string) error {
		sess, err := forwarder.OpenSession(ctx, sockPath, "", logger, "aider-edit")
		if err != nil {
			*applied = false
			return fmt.Errorf("bench/runtime: aider_edit open daemon session: %w", err)
		}
		// Ordered teardown (forwarder/session.go Close()): SDK shutdown flush, then
		// stream CloseSend (clean EOF), then conn close — after the apply loop.
		defer sess.Close()

		// activate_project: point the daemon workspace at the cloned exercise repo
		// so the EDIT verb's relative stub path resolves (drive.go:87). Harness setup.
		activateCtx, activateCancel := context.WithTimeout(ctx, driveDeadline)
		res, err := sess.CallTool(activateCtx, "activate_project", map[string]any{"repo_path": workDir})
		activateCancel()
		if err != nil {
			*applied = false
			return fmt.Errorf("bench/runtime: aider_edit activate_project: %w", err)
		}
		if res != nil && res.IsError {
			*applied = false
			return fmt.Errorf("bench/runtime: aider_edit activate_project returned error: %s", toolErrText(res))
		}

		// Live apply seam: deterministic whole-stub replace_in_file (pattern =
		// current stub body verbatim, replacement = reference body, is_regex=false).
		apply := func(ctx context.Context, workDir, stub, body string) error {
			cur, err := os.ReadFile(filepath.Join(workDir, stub))
			if err != nil {
				return fmt.Errorf("read current stub %q: %w", stub, err)
			}
			callCtx, cancel := context.WithTimeout(ctx, driveDeadline)
			defer cancel()
			callRes, callErr := sess.CallTool(callCtx, "replace_in_file", map[string]any{
				"path":        stub,
				"pattern":     string(cur),
				"replacement": body,
				"is_regex":    false,
			})
			if callErr != nil {
				return fmt.Errorf("replace_in_file transport: %w", callErr)
			}
			if callRes != nil && callRes.IsError {
				return fmt.Errorf("replace_in_file returned error: %s", toolErrText(callRes))
			}
			return nil
		}

		// Delegate the read-reference + per-stub apply + bookkeeping to the shared core.
		return newEditAgentWithApply(applied, apply)(ctx, ex, workDir, prompt)
	}
}

// newNativeTestFn returns the live TestFn that grades an exercise by running the
// dataset's NATIVE per-language test argv (aiderpolyglot.NativeTestCommand) in the
// work dir. An unknown language fail-closes (TestResult{Passed:false} carrying the
// error). Thin glue per PATTERNS.md — the per-attempt timeout is the parent ctx's
// (RunExercise wraps each attempt in an attemptTimeout child context).
func newNativeTestFn() aiderpolyglot.TestFn {
	return func(ctx context.Context, ex *aiderpolyglot.Exercise, workDir string) aiderpolyglot.TestResult {
		argv, err := aiderpolyglot.NativeTestCommand(ex.Language)
		if err != nil {
			return aiderpolyglot.TestResult{Passed: false, Output: err.Error()}
		}
		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.Dir = workDir
		out, runErr := cmd.CombinedOutput()
		return aiderpolyglot.TestResult{Passed: runErr == nil, Output: string(out)}
	}
}
