package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/agenthands/helix/internal/eval/runner"
	"github.com/agenthands/helix/internal/forwarder"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// driveDeadline bounds how long the driver waits for a single tools/call
// response before recording a timeout error for that step. It mirrors the
// integration test's 10s per-call read deadline (daemon_tap_integration_test.go).
const driveDeadline = 10 * time.Second

// driveScript replays a scripted agent's steps against the warm daemon over the
// RETAINED gRPC StreamMCP wire (Phase 94 fix(94-02)). It opens ONE MCP session
// via forwarder.OpenSession against the per-cell Unix socket (D-06 transport),
// issues an `activate_project` tools/call pointing the daemon's workspace at
// workspaceRoot (so the scripted edit's relative paths resolve — mirrors
// internal/eval/runner/inprocess.go:276-280), then one `tools/call` per scripted
// step, and records each scripted step's outcome as a
// runner.StepResult{Tool, AtTime, Response, Err}.
//
// History (Phase 94): the original driver shelled `helix --mode=stdio
// --socket=<sockPath>` ONCE and piped newline-delimited MCP JSON-RPC frames over
// the forwarder's stdin/stdout with an id-keyed pending buffer. Phase 94 deleted
// the stdio forwarder head, so this now dials the gRPC wire directly in-process.
// The helixBin path is no longer needed to spawn a forwarder — forwarder.OpenSession
// dials the same unix socket the daemon already listens on (and cold-starts the
// daemon if needed via ConnectOrStartDaemon, the auto-start behavior the harness
// relied on). The MCP SDK client performs the initialize handshake the daemon
// requires, so there is no hand-framed `initialize` frame anymore.
//
// The activate_project call is harness setup (not a scripted-task step), so it is
// NOT recorded in the returned []StepResult — only the script.Steps are, keeping
// the synthesized CC leg faithful to the task's tool-call sequence. The daemon
// still emits a tool_call for it, so it appears in the daemon-tap (a real,
// harness-issued call — not foreign-cell leakage).
//
// Timing fidelity is preserved across the migration (Pitfall 5): every scripted
// step records a distinct AtTime dispatch instant so SynthCCTap can build the CC
// (agent-tap) leg with faithful per-step timestamps.
//
// The id-keyed pending buffer / out-of-order response demux of the old stdio
// driver is obsolete under the SDK session: each session.CallTool blocks for its
// own matching response, so responses cannot be delivered out of order to the
// wrong step. The per-call deadline is enforced via a per-step context derived
// from the parent ctx.
//
// The exact stdin-close-after-reads race the old driver guarded (closing stdin
// EOF'd the forwarder's stdin->gRPC goroutine and CloseSend'd the stream, racing
// response delivery) is MOOT under gRPC: there is no stdin pipe. The equivalent
// invariant — half-close the send direction only AFTER all sends complete — is
// satisfied by Session.Close() running after the drive loop returns (see
// forwarder/session.go Close()).
//
// The returned []StepResult is ordered to match script.Steps. A transport-level
// failure (cannot dial the daemon, cannot open the stream, activation fails) is
// returned as the error; per-step tool/timeout errors are recorded in
// StepResult.Err and do NOT abort the drive — the cell still taps the daemon log
// and merges whatever ran.
func driveScript(ctx context.Context, helixBin, sockPath, workspaceRoot string, script runner.Script) ([]runner.StepResult, error) {
	_ = helixBin // retained for signature stability; the gRPC dial needs no binary path.

	// A bench driver needs no structured logging surfaced; discard it.
	logger := slog.New(slog.NewTextHandler(nopWriter{}, nil))

	sess, err := forwarder.OpenSession(ctx, sockPath, "", logger, "bench-runtime")
	if err != nil {
		return nil, fmt.Errorf("bench/runtime: open daemon session: %w", err)
	}
	// Ordered teardown (forwarder/session.go Close()): SDK shutdown flush, then
	// stream CloseSend (clean EOF -> daemon records outcome="ended"), then conn
	// close. Runs after the drive loop returns, so no CloseSend races a pending
	// response (the gRPC analog of the old stdin-close-after-reads guard).
	defer sess.Close()

	// activate_project: point the daemon workspace at the cloned repo so the
	// scripted edit's relative paths resolve (inprocess.go:276-280). Harness
	// setup — NOT recorded as a scripted StepResult.
	activateCtx, activateCancel := context.WithTimeout(ctx, driveDeadline)
	res, err := sess.CallTool(activateCtx, "activate_project", map[string]any{"repo_path": workspaceRoot})
	activateCancel()
	if err != nil {
		// Activation transport failure is fatal to the edit — surface it.
		return nil, fmt.Errorf("bench/runtime: activate_project: %w", err)
	}
	if res != nil && res.IsError {
		return nil, fmt.Errorf("bench/runtime: activate_project returned error: %s", toolErrText(res))
	}

	results := make([]runner.StepResult, 0, len(script.Steps))
	for _, step := range script.Steps {
		sr := runner.StepResult{
			Tool:   step.Tool,
			AtTime: time.Now(), // dispatch instant (Pitfall 5)
		}

		// Per-step deadline derived from the parent ctx, mirroring the old
		// per-call read deadline. A per-step timeout is recorded in StepResult.Err
		// and does NOT abort the drive.
		stepCtx, stepCancel := context.WithTimeout(ctx, driveDeadline)
		callRes, callErr := sess.CallTool(stepCtx, step.Tool, orEmptyArgs(step.Args))
		stepCancel()

		if callErr != nil {
			sr.Err = callErr
			results = append(results, sr)
			continue
		}
		sr.Response = marshalResult(callRes)
		if callRes != nil && callRes.IsError {
			sr.Err = fmt.Errorf("tool %q returned error: %s", step.Tool, toolErrText(callRes))
		}
		results = append(results, sr)
	}

	return results, nil
}

// orEmptyArgs returns a non-nil map so a nil Args is sent as an empty arguments
// object rather than a nil map.
func orEmptyArgs(args map[string]any) map[string]any {
	if args == nil {
		return map[string]any{}
	}
	return args
}

// marshalResult serializes the SDK CallToolResult into the raw JSON the old
// driver recorded in StepResult.Response (a best-effort capture for downstream
// trace merging). A marshal failure yields nil rather than aborting the step.
func marshalResult(res *mcpsdk.CallToolResult) json.RawMessage {
	if res == nil {
		return nil
	}
	b, err := json.Marshal(res)
	if err != nil {
		return nil
	}
	return json.RawMessage(b)
}

// toolErrText extracts the first text content block from a tool-level error
// result so the recorded StepResult.Err reflects the failure (e.g.
// "no_workspace: ..."), matching the old parseRPCLine isError-content behavior.
func toolErrText(res *mcpsdk.CallToolResult) string {
	if res != nil {
		for _, c := range res.Content {
			if tc, ok := c.(*mcpsdk.TextContent); ok && tc.Text != "" {
				return tc.Text
			}
		}
	}
	return "tool returned isError"
}

// nopWriter is an io.Writer that discards all writes, used to silence the
// forwarder dial logger in the bench driver.
type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }
