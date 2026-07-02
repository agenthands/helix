//go:build !windows
// +build !windows

package cli_test

// Phase 140 (v2.13) — all-11-language real-binary in-body DATA_FLOWS E2E +
// function-seeded multi-hop oracle.
//
// This is the milestone headline CONFIRMATION: prove, through a REAL `helix`
// binary driven as a subprocess against a live daemon, that an in-body-origin
// DATA_FLOWS edge (producer.function -> consumer.param, Source "def_use_inbody")
// surfaces via `helix explain-symbol-deep` for all 11 languages, and that a
// producer->transform->sink multi-hop is reachable via `helix trace-data-flow`
// seeded at the producer FUNCTION node.
//
// Phase 138 proved the flow engine records the in-body flow across all 11
// grammars (the all-11 unit matrix, dataflow_matrix_test.go). Phase 139 proved
// emission + read surface + Go multi-hop IN-PROCESS (factsFromExtracted ->
// snapshot -> walk). This file proves the SAME end-to-end through the shipped
// CLI, where DATA_FLOWS surfaces to agents as `data_flows`
// (MapInternalKind, edge_kind_surface.go:66).
//
// It reuses the v2.12 HELIX_BIN-gated harness (cli_e2e_test.go /
// cli_type_resolution_e2e_test.go): newE2EFixture stands up an isolated sandbox
// + real daemon and t.Skips when no binary is available; switchModeReview flips
// the shared session to review (index-semantic-graph is review+); indexFull runs
// the REAL `helix index-semantic-graph --mode-arg=full`; explainSymbol runs the
// REAL `helix explain-symbol-deep --seed-json=...` and parses the verbatim JSON.
//
// SEED-TO-SURFACE MECHANISM (grounded — 140-CONTEXT): a def_use_inbody edge is
// producer.function -> consumer.param. Seeding explain-symbol-deep on the
// CONSUMER PARAMETER makes the edge appear as an INCOMING data_flows edge
// (IncomingEdgesOf/OutgoingEdgesOf apply no kind filter). The endpoint shape
// (From = the producer function, To = the seed param) distinguishes the in-body
// edge from a v2.9 param->param edge without relying on the Source marker (which
// is not on the surface envelope).

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// dataFlowEdgesOf returns the `data_flows` edges (surfaced DATA_FLOWS) present in
// the union of the symbol's incoming + outgoing edges. Mirrors hasTypeEdges: the
// in-body emit direction is producer.function -> consumer.param, so seeding the
// consumer param yields an INCOMING edge — but the helper scans both directions
// so an assertion does not silently pass on a direction flip.
func dataFlowEdgesOf(res explainDeepResult) []explainEdge {
	var out []explainEdge
	for _, e := range append(append([]explainEdge{}, res.EdgesOutgoing...), res.EdgesIncoming...) {
		if e.EdgeKind == "data_flows" {
			out = append(out, e)
		}
	}
	return out
}

// inBodyEdgesInto returns the data_flows edges whose To endpoint is the seed
// symbol and whose From endpoint carries the producer's name token — i.e. the
// in-body producer.function -> consumer.param edge, as opposed to a v2.9
// param->param edge (From would be another parameter) or a return-bridge
// param->function edge (To would be a function, not the seed param). The
// producer function's QualifiedName embeds the token "producer" in its stable
// key regardless of language / class wrapping, so a substring check identifies
// the in-body source without coupling to the exact stable-key layout.
func inBodyEdgesInto(res explainDeepResult, seedID, producerToken string) []explainEdge {
	var out []explainEdge
	for _, e := range dataFlowEdgesOf(res) {
		if e.To == seedID && containsToken(e.From, producerToken) {
			out = append(out, e)
		}
	}
	return out
}

// newInBodyDataFlowFixture mirrors newCTypeE2EFixture (shared sandbox +
// real-daemon bringup, HELIX_BIN-gated) but seeds a single per-language file
// carrying the in-body flow fixture instead of the default main.go. It returns
// the live fixture plus the ABSOLUTE seed path — the daemon stores file paths
// verbatim from the full walk (semantic_files.path is absolute), so the
// explain-symbol-deep / trace-data-flow seed's file_path must be absolute.
//
// The default main.go is removed unless the fixture itself is named main.go
// (the Go case), so only the seeded language file is present when the workspace
// is walked.
func newInBodyDataFlowFixture(t *testing.T, runID, filename, content string) (*e2eFixture, string) {
	t.Helper()
	f := newE2EFixture(t, runID)
	seed := filepath.Join(f.repoDir, filename)
	if err := os.WriteFile(seed, []byte(content), 0o600); err != nil {
		t.Fatalf("seed in-body fixture: %v", err)
	}
	if filename != "main.go" {
		_ = os.Remove(filepath.Join(f.repoDir, "main.go"))
	}
	return f, seed
}

// inBodyCase is one per-language fixture entry.
//   - src defines top-level producer() (returns a value), sink(consumerParam)
//     (takes one), pure(negParam) (takes one, empty body), and a caller whose
//     body is `local := producer(); sink(local)` PLUS a negative arm
//     `pure(<literal>)` — the literal carries NO in-body origin, so pure's param
//     yields ZERO in-body data_flows edges (differential anti-vacuity).
//   - Idiomatic per language; Java/C#/Kotlin are class-wrapped so producer/sink/
//     pure/caller are the class's methods (name-resolvable: nameToNode keys on
//     the bare method name, semantic_wiring.go:2468).
//   - skipReason (non-empty) => t.Skip inside the subtest with a recorded reason
//     (honest per-language ledger, FLOW-06b). All 11 languages are proven E2E in
//     v2.13 (Phase 140 D-BREADTH wired Rust/Kotlin/PHP/Ruby into langFromExt), so
//     no case sets skipReason today; the field + guard are retained so a future
//     genuinely-unresolvable language is a recorded skip, not a silent omission.
type inBodyCase struct {
	lang          string
	filename      string
	src           string
	consumerParam string // seed for the POSITIVE arm (the in-body consumer's param)
	negParam      string // seed for the NEGATIVE arm (a param with no in-body inflow)
	skipReason    string
}

// inBodyCases is the all-11-language fixture table. Consumer param is "s"
// (sink's param), negative param is "q" (pure's param, fed a literal). Producer
// name token is "producer" everywhere.
var inBodyCases = []inBodyCase{
	{lang: "go", filename: "main.go", consumerParam: "s", negParam: "q", src: `package main

func producer() int { return 1 }

func sink(s int) {}

func pure(q int) {}

func caller() {
	local := producer()
	sink(local)
	pure(0)
}
`},
	{lang: "typescript", filename: "main.ts", consumerParam: "s", negParam: "q", src: `function producer(): number { return 1; }

function sink(s: number): void {}

function pure(q: number): void {}

function caller(): void {
	const local = producer();
	sink(local);
	pure(0);
}
`},
	{lang: "java", filename: "main.java", consumerParam: "s", negParam: "q", src: `class C {
	int producer() { return 1; }

	void sink(int s) {}

	void pure(int q) {}

	void caller() {
		var local = producer();
		sink(local);
		pure(0);
	}
}
`},
	{lang: "csharp", filename: "main.cs", consumerParam: "s", negParam: "q", src: `class C {
	int producer() { return 1; }

	void sink(int s) {}

	void pure(int q) {}

	void caller() {
		var local = producer();
		sink(local);
		pure(0);
	}
}
`},
	{lang: "kotlin", filename: "main.kt", consumerParam: "s", negParam: "q", src: `class C {
	fun producer(): Int { return 1 }

	fun sink(s: Int) {}

	fun pure(q: Int) {}

	fun caller() {
		val local = producer()
		sink(local)
		pure(0)
	}
}
`},
	{lang: "php", filename: "main.php", consumerParam: "s", negParam: "q", src: `<?php
function producer() { return 1; }

function sink($s) {}

function pure($q) {}

function caller() {
	$local = producer();
	sink($local);
	pure(0);
}
`},
	{lang: "python", filename: "main.py", consumerParam: "s", negParam: "q", src: `def producer():
    return 1

def sink(s):
    pass

def pure(q):
    pass

def caller():
    local = producer()
    sink(local)
    pure(0)
`},
	{lang: "ruby", filename: "main.rb", consumerParam: "s", negParam: "q", src: `def producer
  1
end

def sink(s)
end

def pure(q)
end

def caller
  local = producer()
  sink(local)
  pure(0)
end
`},
	{lang: "rust", filename: "main.rs", consumerParam: "s", negParam: "q", src: `fn producer() -> i32 { 1 }

fn sink(s: i32) {}

fn pure(q: i32) {}

fn caller() {
	let local = producer();
	sink(local);
	pure(0);
}
`},
	{lang: "c", filename: "main.c", consumerParam: "s", negParam: "q", src: `int producer() { return 1; }

void sink(int s) {}

void pure(int q) {}

void caller() {
	int local = producer();
	sink(local);
	pure(0);
}
`},
	// C++ (unlike C) captures a function name only when it is a field_identifier
	// — i.e. an in-class method — never a top-level free-function identifier (the
	// cpp extractor's @definition.function rule; confirmed by the
	// function_basic golden `int hello(){...}` -> zero symbols). So cpp is
	// class-wrapped, like Java/C#/Kotlin, to make producer/sink/pure/caller
	// resolvable method symbols.
	{lang: "cpp", filename: "main.cpp", consumerParam: "s", negParam: "q", src: `class C {
	int producer() { return 1; }

	void sink(int s) {}

	void pure(int q) {}

	void caller() {
		int local = producer();
		sink(local);
		pure(0);
	}
};
`},
}

// TestCLI_E2E_InBodyDataFlow is the Phase-140 headline oracle (FLOW-06a/06b/06d):
// for each of the 11 languages, the in-body producer.function -> consumer.param
// data_flows edge surfaces via the REAL `helix explain-symbol-deep`, a param
// with no in-body inflow surfaces none (differential anti-vacuity), and the Go
// subtest re-indexes to prove determinism (FLOW-06d).
func TestCLI_E2E_InBodyDataFlow(t *testing.T) {
	for _, c := range inBodyCases {
		c := c
		t.Run(c.lang, func(t *testing.T) {
			if c.skipReason != "" {
				t.Skipf("breadth-ledger skip: %s", c.skipReason)
			}

			f, seed := newInBodyDataFlowFixture(t, "inbody-"+c.lang, c.filename, c.src)

			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()

			f.mcpActivate(t, ctx)
			f.switchModeReview(t, ctx)
			f.indexFull(t, ctx)

			// --- HEADLINE (assert FIRST): the consumer param carries an in-body
			//     data_flows edge from the producer FUNCTION via the real
			//     `helix explain-symbol-deep` (FLOW-06a). ---
			pos := f.explainSymbol(t, ctx, seed, c.consumerParam)
			if pos.Seed.Resolution != "exact" {
				t.Fatalf("positive seed %q: resolution=%q fallback=%q, want exact (indexed param)",
					c.consumerParam, pos.Seed.Resolution, pos.FallbackReason)
			}
			inBody := inBodyEdgesInto(pos, pos.Seed.SymbolID, "producer")
			if len(inBody) == 0 {
				t.Fatalf("HEADLINE: real `helix explain-symbol-deep` on %s.%s surfaced NO in-body data_flows edge (producer.function -> %s param); all data_flows=%+v\n incoming=%+v\n outgoing=%+v",
					c.lang, c.consumerParam, c.consumerParam, dataFlowEdgesOf(pos), pos.EdgesIncoming, pos.EdgesOutgoing)
			}
			for _, e := range inBody {
				if e.InternalKind != "DATA_FLOWS" {
					t.Errorf("in-body edge internal_kind=%q, want DATA_FLOWS", e.InternalKind)
				}
				if e.To != pos.Seed.SymbolID {
					t.Errorf("in-body edge to=%q, want the seed param %q (%q) — production direction is producer.function -> consumer.param",
						e.To, c.consumerParam, pos.Seed.SymbolID)
				}
				if !containsToken(e.From, "producer") {
					t.Errorf("in-body edge from=%q, want the producer function symbol", e.From)
				}
			}

			// --- DIFFERENTIAL anti-vacuity (FLOW-06a): the negative param (pure's
			//     param, fed a literal) surfaces NO data_flows edge while still
			//     resolving exact (the zero is a real absence, not a missing
			//     symbol). ---
			neg := f.explainSymbol(t, ctx, seed, c.negParam)
			if neg.Seed.Resolution != "exact" {
				t.Fatalf("negative seed %q: resolution=%q fallback=%q, want exact (the param IS indexed — the zero must be non-vacuous)",
					c.negParam, neg.Seed.Resolution, neg.FallbackReason)
			}
			if negEdges := dataFlowEdgesOf(neg); len(negEdges) != 0 {
				t.Fatalf("differential anti-vacuity: no-inflow param %q returned %d data_flows edges, want 0: %+v",
					c.negParam, len(negEdges), negEdges)
			}

			// --- DETERMINISM (FLOW-06d): re-index the same fixture via a second
			//     REAL index subprocess and assert the in-body data_flows edge set
			//     for the consumer param is identical (same count, same
			//     from/to endpoints). Run for Go (a stable lang per the 138
			//     ledger) at minimum; running per-language is free here. ---
			if c.lang == "go" {
				f.indexFull(t, ctx)
				pos2 := f.explainSymbol(t, ctx, seed, c.consumerParam)
				inBody2 := inBodyEdgesInto(pos2, pos2.Seed.SymbolID, "producer")
				if len(inBody2) != len(inBody) {
					t.Fatalf("determinism: in-body data_flows edge count changed across re-index: %d vs %d\n first=%+v\n second=%+v",
						len(inBody), len(inBody2), inBody, inBody2)
				}
				if !sameEdgeSet(inBody, inBody2) {
					t.Fatalf("determinism: in-body data_flows edge set changed across re-index:\n first=%+v\n second=%+v", inBody, inBody2)
				}
			}
		})
	}
}

// sameEdgeSet reports whether two data_flows edge slices carry the identical set
// of (from,to,edge_kind,internal_kind) tuples (order-independent).
func sameEdgeSet(a, b []explainEdge) bool {
	if len(a) != len(b) {
		return false
	}
	key := func(e explainEdge) string {
		return e.From + "\x00" + e.To + "\x00" + e.EdgeKind + "\x00" + e.InternalKind
	}
	seen := make(map[string]int, len(a))
	for _, e := range a {
		seen[key(e)]++
	}
	for _, e := range b {
		seen[key(e)]--
	}
	for _, v := range seen {
		if v != 0 {
			return false
		}
	}
	return true
}

// traceDataFlowResult is the reachability-relevant subset of the daemon's
// TraceDataFlowResult JSON (internal/skill/semantic.TraceDataFlowResult). The
// test parses the REAL subprocess stdout structurally rather than importing the
// daemon package into this external test.
type traceDataFlowResult struct {
	Reachable []struct {
		SymbolID string `json:"symbol_id"`
		Hops     int    `json:"hops"`
	} `json:"reachable"`
	NodesCount     int    `json:"nodes_count"`
	ReachedHops    int    `json:"reached_hops"`
	FallbackReason string `json:"fallback_reason"`
}

// traceDataFlow runs `helix trace-data-flow --seed-json=...` as a REAL
// subprocess (dir=repoDir) and parses the verbatim JSON stdout. The seed is a
// (file_path, symbol_name) tuple; symbol_name may be a FUNCTION (post-v2.13 the
// BFS is kind-agnostic, so a function seed mechanically reaches in-body
// targets — M2 / FLOW-06c).
func (f *e2eFixture) traceDataFlow(t *testing.T, ctx context.Context, absPath, name string) traceDataFlowResult {
	t.Helper()
	seedJSON := fmt.Sprintf(`{"file_path":%q,"symbol_name":%q}`, absPath, name)
	out, err := f.runCLIVerbInDir(ctx, f.repoDir, "trace-data-flow", "--seed-json="+seedJSON)
	if err != nil {
		t.Fatalf("real `helix trace-data-flow` (%s) failed: %v\noutput:\n%s", name, err, out)
	}
	var res traceDataFlowResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("parse trace result (%s): %v\noutput:\n%s", name, err, out)
	}
	return res
}

// reaches reports whether symID appears in the reachable set (any hop > 0).
func (r traceDataFlowResult) reaches(symID string) bool {
	for _, n := range r.Reachable {
		if n.SymbolID == symID {
			return true
		}
	}
	return false
}

// multiHopSrc is the FLOW-06c multi-hop fixture (Go — the safe proven language):
//
//	caller(){ a := src(); b := transform(a); sink(b) }
//
// yields in-body(src -> transform.x) + in-body(transform -> sink.v). transform
// returns its param, so the return-bridge(transform.x -> transform) connects the
// two in-body hops:  src.fn -> transform.x -> transform.fn -> sink.v. Seeding
// trace-data-flow at the producer FUNCTION `src` reaches sink's param v.
const multiHopSrc = `package main

func src() int { return 1 }

func transform(x int) int { return x }

func sink(v int) {}

func caller() {
	a := src()
	b := transform(a)
	sink(b)
}
`

// multiHopBrokenSrc severs the return-bridge hop: transform no longer returns
// its param, so transform.x is a dead param (no ParamFlow -> no
// return-bridge edge). The chain breaks at transform.x and sink's param v is
// NOT reachable from src's function node (revert-and-fail RED).
const multiHopBrokenSrc = `package main

func src() int { return 1 }

func transform(x int) int { return 0 }

func sink(v int) {}

func caller() {
	a := src()
	b := transform(a)
	sink(b)
}
`

// TestCLI_E2E_InBodyMultiHop is the Phase-140 function-seeded multi-hop oracle
// (FLOW-06c, M2), all through the REAL `helix` binary. A producer FUNCTION seed
// (`src`) reaches the sink's param over in-body -> return-bridge -> in-body; a
// broken-hop variant (transform drops the return) makes the sink unreachable.
func TestCLI_E2E_InBodyMultiHop(t *testing.T) {
	f, seed := newInBodyDataFlowFixture(t, "inbody-multihop", "main.go", multiHopSrc)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	f.mcpActivate(t, ctx)
	f.switchModeReview(t, ctx)
	f.indexFull(t, ctx)

	// The reachability target: sink's param v. Its symbol_id (stable key) is the
	// same namespace the trace reachable set reports (QueryStableKeyByNodeID),
	// so an exact symbol_id match is the non-fragile reachability assertion.
	sinkParam := f.explainSymbol(t, ctx, seed, "v")
	if sinkParam.Seed.Resolution != "exact" {
		t.Fatalf("sink param v: resolution=%q fallback=%q, want exact", sinkParam.Seed.Resolution, sinkParam.FallbackReason)
	}
	sinkID := sinkParam.Seed.SymbolID

	// --- HEADLINE (assert FIRST): seed the producer FUNCTION `src`; sink.v is
	//     reachable over the multi-hop chain (FLOW-06c). ---
	tr := f.traceDataFlow(t, ctx, seed, "src")
	if tr.FallbackReason != "" {
		t.Fatalf("trace-data-flow(src) degraded: fallback_reason=%q (want a real walk)", tr.FallbackReason)
	}
	if !tr.reaches(sinkID) {
		t.Fatalf("FLOW-06c: function-seeded multi-hop failed — sink.v (%q) NOT reachable from producer function `src` via the real `helix trace-data-flow`; reachable=%+v",
			sinkID, tr.Reachable)
	}

	// --- BROKEN-HOP (revert-and-fail RED): sever the return-bridge hop
	//     (transform drops the return). Re-index the SAME workspace and re-trace;
	//     sink.v must NO LONGER be reachable from src. ---
	if err := os.WriteFile(seed, []byte(multiHopBrokenSrc), 0o600); err != nil {
		t.Fatalf("overwrite with broken-hop fixture: %v", err)
	}
	f.mcpActivate(t, ctx) // re-read the mutated source
	f.indexFull(t, ctx)

	// sink.v is still an indexed symbol after re-index (its signature is
	// unchanged), so re-resolve its id to compare against the fresh snapshot.
	sinkParam2 := f.explainSymbol(t, ctx, seed, "v")
	if sinkParam2.Seed.Resolution != "exact" {
		t.Fatalf("broken-hop: sink param v resolution=%q, want exact", sinkParam2.Seed.Resolution)
	}
	sinkID2 := sinkParam2.Seed.SymbolID

	trBroken := f.traceDataFlow(t, ctx, seed, "src")
	if trBroken.reaches(sinkID2) {
		t.Fatalf("broken-hop revert-and-fail: sink.v (%q) STILL reachable from `src` despite the severed return-bridge hop; reachable=%+v",
			sinkID2, trBroken.Reachable)
	}
}
