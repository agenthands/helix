// Package runtime — rag.go owns the Phase 83 baseline_rag drive leg (ABLATE-04
// #3/#4). It REPLACES the Phase-80 fail-close at cell.go:429-433 with a real run:
// the embedding index is built OUT-OF-BAND (before the timed agent span), the
// standalone cmd/helix-bench-rag MCP server is spawned (subprocess.StartRAGServer)
// in place of the Helix daemon, the agent is driven over the server's stdio
// (no forwarder — the bench-rag server IS the MCP endpoint), and a schema-valid
// result.v2 row is emitted carrying the selected embedder_id under the SAME
// runners.DefaultContract model snapshot + budget as your_agent_full.
//
// Budget exclusion (criterion #4 / Pitfall 3): ragindex.Open(repoDir) runs BEFORE
// the `start := time.Now()` span anchor, so embedding-API tokens never enter the
// timed/budgeted agent span. The server's own RunE re-opens the now-warm per-corpus
// cache (zero re-embeds), so the spawned server adds no embedding cost either.
package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/agenthands/helix/bench/evaluators/coordinator"
	"github.com/agenthands/helix/bench/languages"
	"github.com/agenthands/helix/bench/ragindex"
	"github.com/agenthands/helix/bench/runners"
	benchsandbox "github.com/agenthands/helix/bench/runtime/sandbox"
	"github.com/agenthands/helix/bench/runtime/subprocess"
	"github.com/agenthands/helix/internal/eval/runner"
	"github.com/agenthands/helix/internal/eval/trace"
)

// baselineRagMode is the bench-mode name of the retrieval-only control arm. The
// arm is detected BY MODE NAME in RunCell (NOT a MODE.md frontmatter key — the
// resolver is strict two-key with KnownFields(true)).
const baselineRagMode = "baseline_rag"

// ragSearchK is the k for the harness-issued probe rag_search call. The hermetic
// scripted gate drives a single rag_search so the CC (agent-tap) leg carries a
// real tool event; a live run would drive the full task interaction.
const ragSearchK = 5

// runRAGCell is the baseline_rag drive leg invoked from RunCell after the
// unconditional fairness gate. It mirrors the daemon spine (sandbox / clone /
// spawn / drive / verify / merge / build+validate / write) with two divergences:
//
//  1. the embedding index is built OUT-OF-BAND before the timed span (criterion
//     #4 / Pitfall 3), and
//  2. the MCP server is cmd/helix-bench-rag spawned over stdio via
//     subprocess.StartRAGServer (NOT subprocess.StartDaemon over a socket).
//
// res arrives pre-populated by RunCell with the validated path layout
// (ResultPath/MergedTracePath) and Task/Mode. The returned error is non-nil only
// for an infrastructure failure (scratch preserved by the caller's preserve path
// shape, replicated here).
func runRAGCell(ctx context.Context, cfg CellConfig, res CellResult) (CellResult, error) {
	// (2) Bench sandbox (D-07) — same ephemeral OS-temp scratch as the daemon leg.
	sb, err := benchsandbox.New(cfg.RunID, cfg.HelixBin, cfg.OutDir)
	if err != nil {
		return res, fmt.Errorf("bench/runtime: create sandbox: %w", err)
	}
	res.ScratchDir = sb.Root

	cleanedUp := false
	cleanup := func() {
		if cleanedUp {
			return
		}
		_ = sb.Cleanup()
		cleanedUp = true
	}
	preserve := func(e error) (CellResult, error) {
		res.ScratchPreserved = true
		fmt.Fprintf(os.Stderr, "bench/runtime: cell %s/%s failed; preserving scratch at %s: %v\n",
			cfg.Task, cfg.Mode, sb.Root, e)
		return res, e
	}

	if err := sb.Prepare(cfg.Task, cfg.Mode); err != nil {
		return preserve(fmt.Errorf("bench/runtime: prepare cell: %w", err))
	}
	if err := sb.CloneRepo(cfg.SeedDir, cfg.Task, cfg.Mode); err != nil {
		return preserve(fmt.Errorf("bench/runtime: clone seed repo: %w", err))
	}
	repoDir := sb.RepoFor(cfg.Task, cfg.Mode)

	// Resolve the standalone server binary BEFORE the timed span (a missing binary
	// is a clean config failure, not a budgeted cost).
	ragBin, err := subprocess.ResolveRAGServerBin(cfg.HelixBin)
	if err != nil {
		return preserve(fmt.Errorf("bench/runtime: resolve rag server binary: %w", err))
	}

	// (2b) OUT-OF-BAND index build (criterion #4 / Pitfall 3): build/load the
	// per-corpus embedding index over the cloned repo working copy BEFORE the
	// `start := time.Now()` agent-span anchor. This is the budget-exclusion seam —
	// the embedding-API tokens spent here are NOT charged to tokens_input/output.
	// The selected embedder_id is recorded on every baseline_rag row (T-83-03-01).
	// Mirrors how prePatchSnapshot runs out-of-band before driveScript.
	idx, err := ragindex.Open(ctx, repoDir)
	if err != nil {
		return preserve(fmt.Errorf("bench/runtime: build rag index: %w", err))
	}
	embedderID := idx.EmbedderID()

	// D-05 / Pitfall 6: pre-patch test snapshot (out-of-band, like the daemon leg)
	// for regression_checker. baseline_rag is retrieval-only (no edit tool), so the
	// post-patch outcome typically equals the pre-patch outcome — but the snapshot
	// keeps the grader contract identical to your_agent_full (criterion #4).
	langRunner := languages.RunnerFor(cfg.Benchmark, cfg.Language)
	prePatch, ppErr := prePatchSnapshot(ctx, langRunner, repoDir)
	if ppErr != nil {
		return preserve(fmt.Errorf("bench/runtime: pre-patch snapshot: %w", ppErr))
	}

	// Anchor the timed/budgeted span AFTER the out-of-band index build and BEFORE
	// the server spawn + agent drive (criterion #4): embedding cost is already
	// excluded; the span measures only the agent's RAG interaction.
	start := time.Now()

	// (3) Spawn cmd/helix-bench-rag over stdio (D-06: no socket/port) and capture
	// its PID before any Kill (criterion #4 PID gate).
	h, err := subprocess.StartRAGServer(ctx, sb, ragBin, cfg.Task, cfg.Mode, repoDir)
	if err != nil {
		return preserve(fmt.Errorf("bench/runtime: start rag server: %w", err))
	}
	if h.Pid() <= 0 {
		_ = h.Kill()
		return preserve(fmt.Errorf("bench/runtime: rag server returned non-positive pid"))
	}

	// (4) Drive the agent against the RAG server over its stdio. The hermetic
	// scripted gate issues the MCP initialize handshake and one rag_search probe so
	// the CC leg carries a real tool event; a live agent would drive the full task.
	steps, driveErr := driveRAGServer(ctx, h, ragSearchQueryFor(cfg.Task))
	if driveErr != nil {
		fmt.Fprintf(os.Stderr, "bench/runtime: cell %s/%s rag drive error (continuing to verify): %v\n",
			cfg.Task, cfg.Mode, driveErr)
	}

	// (5) Reap the server (no graceful-shutdown semantics needed: the bench-rag
	// server has no semantic-store reads-total line to flush — it shares no daemon
	// code by construction).
	if err := h.Kill(); err != nil {
		return preserve(fmt.Errorf("bench/runtime: kill rag server: %w", err))
	}

	// (6) Resolve the outcome via the structured runner (or verify.sh fallback) —
	// IDENTICAL to the daemon leg (criterion #4: same verification contract). A
	// retrieval-only arm with no edit will leave the bug in place, so verify
	// typically fails; that is an HONEST recorded outcome, not an infra error.
	var verifyExit int
	var postPatch *languages.TestOutcome
	if r := langRunner; r != nil {
		if serr := r.Setup(ctx, repoDir); serr != nil {
			return preserve(fmt.Errorf("bench/runtime: runner setup: %w", serr))
		}
		outcome, runErr := r.RunTests(ctx, repoDir)
		if runErr != nil {
			return preserve(fmt.Errorf("bench/runtime: run tests: %w", runErr))
		}
		oc := outcome
		postPatch = &oc
		if outcome.Passed {
			verifyExit = 0
		} else {
			verifyExit = 1
		}
	} else {
		var verifyErr error
		verifyExit, verifyErr = runVerify(ctx, filepath.Join(repoDir, "verify.sh"), repoDir)
		if verifyErr != nil {
			return preserve(fmt.Errorf("bench/runtime: verify: %w", verifyErr))
		}
	}
	res.VerifyExitCode = verifyExit

	// (7) Synthesize the CC (agent-tap) leg from the scripted RAG StepResults.
	cc := SynthCCTap(steps)

	// (8) Merge: the RAG arm has no daemon leg (the bench-rag server is not the
	// PID-gated helix daemon and emits no daemon.log tool_call lines), so the
	// merged trace is the CC leg alone. trace.Merge tolerates a zero daemon tap.
	merged, err := trace.Merge(trace.MergeInput{
		TaskID:         cfg.Task,
		Mode:           cfg.Mode,
		RunID:          cfg.RunID,
		StartedAt:      start,
		EndedAt:        time.Now(),
		CC:             cc,
		VerifyExitCode: verifyExit,
		RepoRoot:       repoDir,
	})
	if err != nil {
		return preserve(fmt.Errorf("bench/runtime: merge trace: %w", err))
	}
	res.Merged = merged
	res.ToolCallTotal = merged.ToolCallSummary.Total
	res.CCLegPresent = ccLegPresent(merged)

	// (8b) Grade the cell — same coordinator contract as the daemon leg. The
	// scripted RAG drive carries no provider usage block, so token metrics are
	// explicit null (NOT a fabricated 0); crucially the embedding-build cost is
	// already excluded from this span (criterion #4).
	var postOutcome languages.TestOutcome
	if postPatch != nil {
		postOutcome = *postPatch
	} else {
		postOutcome = languages.TestOutcome{Passed: verifyExit == 0, ExitCode: verifyExit}
	}
	var prePatchVal languages.TestOutcome
	if prePatch != nil {
		prePatchVal = *prePatch
	}
	metrics, metricErrs := coordinator.Grade(ctx, coordinator.GradeInput{
		TestOutcome:     postOutcome,
		PrePatchOutcome: prePatchVal,
		RepoDir:         repoDir,
		Merged:          merged,
		UsagePresent:    false, // scripted RAG drive: no provider usage block
		Agent:           cfg.Agent,
	})

	// (9) Build + validate result.v2 with EmbedderID set (criterion #3) and
	// Fairness=runners.DefaultContract VERBATIM (criterion #4 — same ModelID
	// snapshot + budget as your_agent_full; never a separate config).
	resultBytes, err := BuildResult(ResultInput{
		TaskID:         cfg.Task,
		Mode:           cfg.Mode,
		Benchmark:      cfg.Benchmark,
		RunIndex:       cfg.RunIndex,
		Outcome:        merged.Outcome,
		TraceRef:       res.MergedTracePath,
		Fairness:       runners.DefaultContract,
		EmbedderID:     embedderID,
		AblationStatus: "",
		Metrics:        metrics,
		MetricErrors:   metricErrs,
	})
	if err != nil {
		return preserve(fmt.Errorf("bench/runtime: build result.v2: %w", err))
	}
	if err := Validate(resultBytes); err != nil {
		return preserve(fmt.Errorf("bench/runtime: result.v2 invalid: %w", err))
	}
	res.ResultValid = true

	// (10) Write durable artifacts under <out>/<task>/<mode>/<run_index>/.
	if err := writeDurable(res.ResultPath, resultBytes); err != nil {
		return preserve(fmt.Errorf("bench/runtime: write result.v2: %w", err))
	}
	traceBytes, err := marshalTrace(merged)
	if err != nil {
		return preserve(fmt.Errorf("bench/runtime: marshal merged trace: %w", err))
	}
	if err := writeDurable(res.MergedTracePath, traceBytes); err != nil {
		return preserve(fmt.Errorf("bench/runtime: write merged trace: %w", err))
	}

	cleanup()
	return res, nil
}

// ragSearchQueryFor derives the probe rag_search query for a task. The hermetic
// gate uses a stable, task-derived string so the drive is deterministic; a live
// agent would issue its own queries. It is intentionally simple — the row's
// validity does not depend on the query text.
func ragSearchQueryFor(task string) string {
	return "fix the failing test in " + task
}

// driveRAGServer performs the MCP handshake and issues one rag_search probe over
// the bench-rag server's stdio pipes, recording the call as a runner.StepResult so
// SynthCCTap can build a faithful CC leg. The bench-rag server is the MCP endpoint
// directly (StdioTransport) — there is NO forwarder, so we frame JSON-RPC straight
// onto h.Stdin and read NDJSON responses from h.Stdout, reusing the drive.go
// response reader/dispatcher.
//
// A transport-level failure (write error, server EOF before the handshake) is
// returned as the error; the cell still reaps the server and merges whatever ran.
func driveRAGServer(ctx context.Context, h *subprocess.RAGHandle, query string) ([]runner.StepResult, error) {
	const idInit = 1
	const idSearch = 2

	done := make(chan struct{})
	respCh := make(chan jsonrpcResp, 4)
	go readResponses(h.Stdout, respCh, done)
	defer close(done)
	disp := &respDispatcher{ch: respCh, pending: map[int]jsonrpcResp{}}

	// initialize frame (id=1).
	initFrame := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":%d,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"bench-rag","version":"0"}}}`,
		idInit)
	if _, err := fmt.Fprintln(h.Stdin, initFrame); err != nil {
		return nil, fmt.Errorf("bench/runtime: write rag initialize: %w", err)
	}
	if _, err := disp.wait(ctx, idInit); err != nil {
		return nil, fmt.Errorf("bench/runtime: rag initialize: %w", err)
	}

	// rag_search probe (id=2), recorded as the single scripted StepResult so the CC
	// leg carries a real tool event (ccLegPresent → true).
	argsJSON, _ := json.Marshal(map[string]any{"query": query, "k": ragSearchK})
	searchFrame := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"rag_search","arguments":%s}}`,
		idSearch, string(argsJSON))

	sr := runner.StepResult{Tool: "rag_search", AtTime: time.Now()}
	if _, err := fmt.Fprintln(h.Stdin, searchFrame); err != nil {
		sr.Err = fmt.Errorf("write rag_search: %w", err)
		return []runner.StepResult{sr}, nil
	}
	resp, waitErr := disp.wait(ctx, idSearch)
	if waitErr != nil {
		sr.Err = waitErr
		return []runner.StepResult{sr}, nil
	}
	sr.Response = resp.raw
	if resp.rpcErr != "" {
		sr.Err = fmt.Errorf("rag_search returned error: %s", resp.rpcErr)
	}
	return []runner.StepResult{sr}, nil
}
