package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/agenthands/helix/internal/eval/runner"
)

// driveDeadline bounds how long the driver waits for a single tools/call
// response before recording a timeout error for that step. It mirrors the
// integration test's 10s per-call read deadline (daemon_tap_integration_test.go).
const driveDeadline = 10 * time.Second

// driveScript replays a scripted agent's steps through the helix stdio forwarder
// over the per-cell Unix socket (D-06 transport). It shells
// `helix --mode=stdio --socket=<sockPath>` ONCE, sends an `initialize` frame,
// then an `activate_project` frame pointing the daemon's workspace at
// workspaceRoot (so the scripted edit's relative paths resolve — mirrors
// internal/eval/runner/inprocess.go:276-280), then one `tools/call` frame per
// scripted step, and records each scripted step's outcome as a
// runner.StepResult{Tool, AtTime, Response, Err}.
//
// The activate_project call is harness setup (not a scripted-task step), so it is
// NOT recorded in the returned []StepResult — only the script.Steps are, keeping
// the synthesized CC leg faithful to the task's tool-call sequence. The daemon
// still emits a tool_call for it, so it appears in the daemon-tap (a real,
// harness-issued call — not foreign-cell leakage).
//
// This mirrors driveSingleToolCall in
// internal/eval/runner/daemon_tap_integration_test.go, generalized from a single
// hard-coded call to the scripted step list, with two divergences (77-PATTERNS):
//
//  1. each step passes its own Args map as the tools/call `arguments` (the
//     integration test hard-codes `{}`); and
//  2. every dispatch is recorded as a StepResult (with a distinct AtTime dispatch
//     instant) so SynthCCTap can build the CC (agent-tap) leg with faithful
//     per-step timestamps (Pitfall 5).
//
// CRITICAL: stdin is NOT closed between frames or before responses arrive.
// Closing stdin makes the forwarder's stdin->gRPC goroutine reach EOF and
// CloseSend the gRPC stream, which races response delivery and can discard the
// daemon's reply before it is read (integration_test:139-143). stdin is closed
// only after all responses are collected (or the overall context is done).
//
// The returned []StepResult is ordered to match script.Steps. A transport-level
// failure (cannot start the forwarder, cannot open pipes) is returned as the
// error; per-step tool/timeout errors are recorded in StepResult.Err and do NOT
// abort the drive — the cell still taps the daemon log and merges whatever ran.
func driveScript(ctx context.Context, helixBin, sockPath, workspaceRoot string, script runner.Script) ([]runner.StepResult, error) {
	fwdCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd := exec.CommandContext(fwdCtx, helixBin, "--mode=stdio", "--socket="+sockPath)
	cmd.Env = append(os.Environ(), "HELIX_LOG_LEVEL=info")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("bench/runtime: drive stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("bench/runtime: drive stdout pipe: %w", err)
	}
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("bench/runtime: start forwarder: %w", err)
	}
	// done signals the background reader to stop pushing responses once the drive
	// loop has returned (WR-02). The reader selects on respCh-send vs <-done so a
	// late / unexpected id-bearing line can never block it forever on a full
	// channel after the drive has stopped draining.
	done := make(chan struct{})
	defer func() {
		// Signal the reader to stop, then close stdin only now (after all reads)
		// so the CloseSend cannot race response delivery, then kill+reap the
		// forwarder.
		close(done)
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// Background reader: parse NDJSON responses keyed by JSON-RPC id. id=1 is
	// initialize, id=2 is activate_project (harness setup), scripted steps start
	// at id=3. disp owns the id-keyed pending buffer so a response for a
	// not-yet-awaited id is RETAINED rather than discarded (WR-03), making the
	// dispatch order non-load-bearing.
	const idInit = 1
	const idActivate = 2
	const idStepBase = 3
	respCh := make(chan jsonrpcResp, len(script.Steps)+2)
	go readResponses(stdout, respCh, done)
	disp := &respDispatcher{ch: respCh, pending: map[int]jsonrpcResp{}}

	// initialize frame (id=1). Do NOT wait for its response before sending the
	// next call; the forwarder pipelines frames.
	const initFrame = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"bench","version":"0"}}}`
	if _, err := fmt.Fprintln(stdin, initFrame); err != nil {
		return nil, fmt.Errorf("bench/runtime: write initialize: %w", err)
	}

	// activate_project frame (id=2): point the daemon workspace at the cloned
	// repo so the scripted edit's relative paths resolve (inprocess.go:276-280).
	// Harness setup — NOT recorded as a scripted StepResult.
	wsJSON, err := json.Marshal(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("bench/runtime: marshal workspace root: %w", err)
	}
	activateFrame := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"activate_project","arguments":{"repo_path":%s}}}`,
		idActivate, string(wsJSON),
	)
	if _, err := fmt.Fprintln(stdin, activateFrame); err != nil {
		return nil, fmt.Errorf("bench/runtime: write activate_project: %w", err)
	}
	if _, err := disp.wait(ctx, idActivate); err != nil {
		// Activation failure is fatal to the edit — surface it.
		return nil, fmt.Errorf("bench/runtime: activate_project: %w", err)
	}
	// IN-04: the initialize reply (id=1) is intentionally not awaited explicitly.
	// With the id-keyed pending buffer (WR-03), if init's response arrives before
	// activate's it is STASHED (not discarded) and simply never read — the dispatch
	// order is no longer load-bearing. idInit is retained for documentation of the
	// frame id allocation.
	_ = idInit

	results := make([]runner.StepResult, 0, len(script.Steps))
	for i, step := range script.Steps {
		id := i + idStepBase

		argsJSON, marshalErr := json.Marshal(orEmptyArgs(step.Args))
		sr := runner.StepResult{
			Tool:   step.Tool,
			AtTime: time.Now(), // dispatch instant (Pitfall 5)
		}
		if marshalErr != nil {
			sr.Err = fmt.Errorf("marshal args for tool %q: %w", step.Tool, marshalErr)
			results = append(results, sr)
			continue
		}

		callFrame := fmt.Sprintf(
			`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":%q,"arguments":%s}}`,
			id, step.Tool, string(argsJSON),
		)
		if _, err := fmt.Fprintln(stdin, callFrame); err != nil {
			sr.Err = fmt.Errorf("write tools/call for tool %q: %w", step.Tool, err)
			results = append(results, sr)
			continue
		}

		// Wait for this step's response (matched by id) with a per-call deadline.
		resp, waitErr := disp.wait(ctx, id)
		if waitErr != nil {
			sr.Err = waitErr
			results = append(results, sr)
			continue
		}
		sr.Response = resp.raw
		if resp.rpcErr != "" {
			sr.Err = fmt.Errorf("tool %q returned error: %s", step.Tool, resp.rpcErr)
		}
		results = append(results, sr)
	}

	return results, nil
}

// orEmptyArgs returns a non-nil map so a nil Args marshals as `{}` (a valid
// empty arguments object) rather than `null`.
func orEmptyArgs(args map[string]any) map[string]any {
	if args == nil {
		return map[string]any{}
	}
	return args
}

// jsonrpcResp is a minimally-parsed JSON-RPC response line from the forwarder.
type jsonrpcResp struct {
	id     int
	raw    json.RawMessage // the full response line (for StepResult.Response)
	rpcErr string          // non-empty when the response carried a JSON-RPC error
}

// readResponses scans the forwarder's stdout for NDJSON JSON-RPC responses and
// publishes each one (keyed by id) onto respCh. It returns when stdout reaches
// EOF or errors, or when done is closed; the caller treats a missing id as a
// per-step timeout.
//
// WR-02: the send is non-blocking against done — once the drive loop returns and
// closes done, no one drains respCh, so a blocking `respCh <- parsed` on a full
// buffer (a duplicate response, a server->client request carrying a numeric id, a
// retried frame) would park this goroutine forever. Selecting on done lets the
// reader exit instead of leaking.
func readResponses(stdout io.Reader, respCh chan<- jsonrpcResp, done <-chan struct{}) {
	reader := bufio.NewReaderSize(stdout, 256*1024)
	for {
		line, err := reader.ReadString('\n')
		if s := strings.TrimSpace(line); s != "" {
			if parsed, ok := parseRPCLine([]byte(s)); ok {
				select {
				case respCh <- parsed:
				case <-done:
					// Drive returned; no drainer remains. Drop rather than leak.
					return
				}
			}
		}
		if err != nil {
			return
		}
	}
}

// parseRPCLine extracts the id and any error message from one JSON-RPC response
// line. Lines without a numeric id (e.g. notifications) are ignored (ok=false).
func parseRPCLine(line []byte) (jsonrpcResp, bool) {
	var env struct {
		ID    *json.Number `json:"id"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Result *struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(line, &env); err != nil || env.ID == nil {
		return jsonrpcResp{}, false
	}
	idInt, err := env.ID.Int64()
	if err != nil {
		return jsonrpcResp{}, false
	}
	r := jsonrpcResp{id: int(idInt), raw: json.RawMessage(append([]byte(nil), line...))}
	switch {
	case env.Error != nil:
		// Transport / protocol-level JSON-RPC error.
		r.rpcErr = env.Error.Message
		if r.rpcErr == "" {
			r.rpcErr = "jsonrpc error"
		}
	case env.Result != nil && env.Result.IsError:
		// Tool-level error (MCP result.isError): surface the text content so the
		// recorded StepResult.Err reflects the failure (e.g. "no_workspace: ...").
		if len(env.Result.Content) > 0 && env.Result.Content[0].Text != "" {
			r.rpcErr = env.Result.Content[0].Text
		} else {
			r.rpcErr = "tool returned isError"
		}
	}
	return r, true
}

// respDispatcher demultiplexes the single forwarder response stream into id-keyed
// waits. It holds a pending buffer so a response for a not-yet-awaited id is
// RETAINED across wait() calls instead of being discarded (WR-03). Today one
// daemon over one socket answers in dispatch order, but nothing enforces that;
// buffering makes out-of-order delivery (id=4 before id=3) correct rather than a
// silent 10s-per-step stall plus a lost response.
//
// respDispatcher is single-consumer: all wait() calls happen on the drive
// goroutine, so pending needs no lock.
type respDispatcher struct {
	ch      <-chan jsonrpcResp
	pending map[int]jsonrpcResp
}

// wait returns the response with the given id, consulting the pending buffer
// first and otherwise reading from the channel — stashing any non-matching id
// into pending rather than dropping it — until the id arrives, the per-call
// deadline elapses, or the parent context is done.
func (d *respDispatcher) wait(ctx context.Context, id int) (jsonrpcResp, error) {
	if r, ok := d.pending[id]; ok {
		delete(d.pending, id)
		return r, nil
	}
	deadline := time.NewTimer(driveDeadline)
	defer deadline.Stop()
	for {
		select {
		case r, ok := <-d.ch:
			if !ok {
				return jsonrpcResp{}, fmt.Errorf("forwarder stdout closed before response id=%d", id)
			}
			if r.id == id {
				return r, nil
			}
			// Response for a different (possibly not-yet-awaited) id: retain it so
			// a later wait() for that id finds it instead of timing out (WR-03).
			d.pending[r.id] = r
		case <-deadline.C:
			return jsonrpcResp{}, fmt.Errorf("no tools/call response for id=%d within %s", id, driveDeadline)
		case <-ctx.Done():
			return jsonrpcResp{}, fmt.Errorf("context done waiting for response id=%d: %w", id, ctx.Err())
		}
	}
}
