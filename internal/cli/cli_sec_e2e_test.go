//go:build !windows
// +build !windows

package cli_test

// HELIX_BIN-gated, !windows real-subprocess E2E proof of the SEC-01 tools/call
// profile/mode enforcement boundary (Phase 91). This is the LIVE counterpart to
// the in-process middleware unit test in 91-02: it starts a REAL daemon under a
// read-restricted profile via the v1.12 internal/eval/sandbox harness, then runs
// a REAL `helix replace-symbol-body` subprocess (a destructive edit verb) and
// asserts the typed serr.PermissionDenied survives the full
// CLI -> gRPC -> daemon -> back round trip as a NON-ZERO exit with the typed
// refusal on stderr.
//
// Two complementary sub-tests (one per threat in the 91-04 register):
//   - TestCLI_SecRefusal_ReadMode (T-91-13): under the read-only ci-bot profile,
//     whose AllowedTools omits replace_symbol_body, the destructive verb is
//     refused — non-zero exit + typed permission_denied on stderr. Proves the
//     refusal is not swallowed CLI-side.
//   - TestCLI_SecAllow_EditMode   (T-91-14): under the default "full" profile the
//     SAME verb is NOT refused by the enforcement gate (it may still fail on its
//     own tool semantics — missing symbol, missing receipts — but NEVER
//     permission_denied). Proves the refusal is profile-conditioned, not
//     unconditional.
//
// Both SKIP cleanly when HELIX_BIN is unresolvable, exactly like the dial oracle
// in cli_e2e_test.go (T-91-15: the phase gate requires this test to RUN, not
// skip, which is why the verify command builds the binary and points HELIX_BIN at
// it).

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/eval/sandbox"
)

// readRestrictedProfile is the profile whose resolved AllowedTools omits the
// destructive edit verbs (internal/profile/profiles/ci-bot.yaml lists
// replace_symbol_body in exclude_tools), so 91-02's ProfileEnforcementMiddleware
// refuses replace_symbol_body for a session under this profile.
const readRestrictedProfile = "ci-bot"

// destructiveVerb is the flat root-attached verb (91-01) whose tool
// (replace_symbol_body) the read-restricted profile excludes. Its three required
// flags satisfy the CLI-side pre-dial validation; the enforcement refusal happens
// server-side BEFORE the handler, so the verb never has to produce a real edit to
// prove the refusal.
const (
	destructiveVerb      = "replace-symbol-body"
	destructiveVerbTool  = "replace_symbol_body"
	destructiveVerbFlags = "(--path --symbol-name --new-body)"
)

// permissionDeniedSignals are the substrings that identify the typed
// permission_denied refusal once it has traversed the gRPC wire back to the CLI
// subprocess's combined output. We accept EITHER the typed Kind prefix
// (serr.Error.Error() renders "permission_denied: <msg>") OR the middleware's
// human message ("is not available in profile") so the assertion is robust to how
// the SDK/gRPC layer stringifies the error en route to stderr.
var permissionDeniedSignals = []string{
	"permission_denied",
	"is not available in profile",
}

// containsPermissionDenied reports whether the combined CLI output carries the
// typed permission-denied refusal signal.
func containsPermissionDenied(out string) bool {
	for _, sig := range permissionDeniedSignals {
		if strings.Contains(out, sig) {
			return true
		}
	}
	return false
}

// newE2EFixtureWithProfile is the profile-parameterized variant of
// newE2EFixture: it stands up a sandbox, seeds a workspace, and starts a REAL
// daemon under the given profile (passed as StartDaemon's 4th arg, which the
// daemon resolves into the session's AllowedTools whitelist). Passing "" yields
// the default "full" profile, matching newE2EFixture. The daemon is reaped and
// the sandbox removed via t.Cleanup (T-90-13).
func newE2EFixtureWithProfile(t *testing.T, runID, profileName string) *e2eFixture {
	t.Helper()

	helixBin := resolveHelixBin()
	if helixBin == "" {
		t.Skip("helix binary not resolvable (set HELIX_BIN or 'go build -o helix ./cmd/helix'); skipping SEC-01 E2E")
	}

	const (
		taskID = "cli-sec-e2e"
		mode   = "full"
	)

	sb, err := sandbox.NewSandbox(runID, helixBin)
	if err != nil {
		t.Fatalf("sandbox.NewSandbox: %v", err)
	}
	t.Cleanup(func() { _ = sb.Cleanup() })

	if err := sb.Prepare(taskID, mode); err != nil {
		t.Fatalf("sandbox.Prepare: %v", err)
	}

	// Seed a real Go symbol so the verb's required --path/--symbol-name resolve to
	// a genuine file+symbol — the refusal must precede the handler regardless, but
	// a real target keeps the allow-path test honest (the full-profile call gets to
	// the handler and fails only on its own semantics, never permission_denied).
	repoDir := sb.RepoFor(taskID, mode)
	seed := filepath.Join(repoDir, "main.go")
	content := "package main\n\nfunc Target() string {\n\treturn \"old\"\n}\n\nfunc main() {\n\tprintln(Target())\n}\n"
	if err := os.WriteFile(seed, []byte(content), 0o600); err != nil {
		t.Fatalf("seed fixture: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	// Real daemon under the requested profile. The 4th arg is profileName; the
	// daemon resolves it to the session AllowedTools the enforcement middleware
	// gates on (RESEARCH Pitfall 6: profile/mode reaches the daemon via --profile,
	// NOT per-call from the CLI).
	h, err := sb.StartDaemon(ctx, taskID, mode, profileName, "")
	if err != nil {
		t.Fatalf("sandbox.StartDaemon(profile=%q): %v", profileName, err)
	}
	t.Cleanup(func() { _ = h.Kill() })

	return &e2eFixture{
		helixBin: helixBin,
		socket:   sb.SocketFor(taskID, mode),
		repoDir:  repoDir,
		sb:       sb,
		handle:   h,
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// runDestructiveVerb runs the flat destructive verb subprocess with its three
// required flags satisfied against the active workspace's seeded symbol.
func (f *e2eFixture) runDestructiveVerb(ctx context.Context) (string, error) {
	return f.runCLIVerb(ctx,
		destructiveVerb,
		"--path="+filepath.Join(f.repoDir, "main.go"),
		"--symbol-name=Target",
		"--new-body=return \"new\"",
	)
}

// TestCLI_SecRefusal_ReadMode is the SEC-01 live refusal proof (T-91-13). Under
// the read-restricted ci-bot profile a REAL `helix replace-symbol-body`
// subprocess MUST exit non-zero and surface the typed permission_denied on its
// combined output — the deny survived the CLI -> gRPC -> daemon -> back trip.
func TestCLI_SecRefusal_ReadMode(t *testing.T) {
	f := newE2EFixtureWithProfile(t, "sec-refuse", readRestrictedProfile)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	f.mcpActivate(t, ctx)

	out, err := f.runDestructiveVerb(ctx)

	// Non-zero exit is the load-bearing CLI contract: a refused destructive verb
	// must NOT exit 0 (T-91-13 — a refusal that succeeds CLI-side is a privilege
	// escalation).
	if err == nil {
		t.Fatalf("expected %s under profile=%s to FAIL (non-zero exit), but it exited 0.\noutput:\n%s",
			destructiveVerb, readRestrictedProfile, out)
	}

	// And the failure must be the TYPED permission_denied, not some incidental
	// error (e.g. a dial failure or unknown-flag). This is the proof the typed
	// refusal round-tripped.
	if !containsPermissionDenied(out) {
		t.Fatalf("expected %s %s under profile=%s to be refused with a typed permission_denied "+
			"signal (one of %v), got exit=%v.\noutput:\n%s",
			destructiveVerb, destructiveVerbFlags, readRestrictedProfile,
			permissionDeniedSignals, err, out)
	}

	t.Logf("SEC-01 live refusal: profile=%s verb=%s tool=%s -> non-zero exit + typed permission_denied (round trip proven)\noutput:\n%s",
		readRestrictedProfile, destructiveVerb, destructiveVerbTool, out)
}

// TestCLI_SecAllow_EditMode is the false-positive guard (T-91-14). Under the
// default "full" profile the SAME destructive verb MUST NOT be refused by the
// enforcement gate: the output must NOT contain a permission_denied signal. The
// call may still fail on the tool's own semantics (e.g. a guardrail receipt
// requirement), but never on profile enforcement — so the refusal is proven
// profile-conditioned, not unconditional.
func TestCLI_SecAllow_EditMode(t *testing.T) {
	// "" profile == the default "full" profile whose AllowedTools includes the
	// edit verbs (mirrors newE2EFixture's default).
	f := newE2EFixtureWithProfile(t, "sec-allow", "")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	f.mcpActivate(t, ctx)

	out, _ := f.runDestructiveVerb(ctx)

	// The ONLY assertion: the enforcement gate did NOT refuse. We deliberately do
	// not assert on exit code — under the full profile the call reaches the handler
	// and may legitimately fail on its own semantics (guardrail receipts, etc.).
	// What MUST hold is the absence of the permission_denied signal.
	if containsPermissionDenied(out) {
		t.Fatalf("expected %s under the full profile to NOT be gate-refused, but the output "+
			"carried a permission_denied signal (the gate is refusing unconditionally — T-91-14).\noutput:\n%s",
			destructiveVerb, out)
	}

	t.Logf("SEC-01 allow-path: full profile verb=%s -> NOT gate-refused (refusal is profile-conditioned)\noutput:\n%s",
		destructiveVerb, out)
}
