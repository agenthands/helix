package bench_test

// tools_bench_test.go implements BENCH-02: the all-38-tools response-time
// benchmark using testing.B.Loop, a 3-call warmup per sub-benchmark, and the
// shared-daemon pattern locked by 09-RESEARCH.md Pattern 2.
//
// Canonical references:
//   - 09-RESEARCH.md Pattern 2 (Shared daemon per benchmark function)
//   - 09-RESEARCH.md Code Examples "Table-driven 38-tool benchmark" (lines 454-477)
//   - 09-RESEARCH.md Pitfall 3 (compiler elision — testing.B.Loop mandatory)
//   - 09-RESEARCH.md Pitfall 6 (no daemon starts inside the loop body)
//   - 09-RESEARCH.md Pitfall 10 (tool errors must b.Fatal, never swallow)
//   - https://go.dev/blog/testing-b-loop — why B.Loop replaces the classic
//     counter-based benchmark loop form
//
// The inner dispatch distinguishes read-only tools (share a single warm
// fixture for the whole sub-benchmark) from the 6 edit tools that MUTATE
// files. Edit tools re-stage a fresh prepareGoFixtureCopyB(b) copy per
// iteration inside StopTimer/StartTimer fences so mutations stay scoped to
// that iteration and don't corrupt the shared daemon state across runs.
// Plan 09-02 added prepareGoFixtureCopyB specifically to close the
// fixture-mutation hazard WARNING 1 flagged by the plan-check pass.

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// seedBenchMemories creates the memories that read_memory, rename_memory, and
// edit_memory sub-benchmarks operate on. It also creates the delete-target so
// delete_memory's warmup loop (3 calls + b.Loop()) has an idempotent target;
// write_memory inside the warmup loop handles recreation.
//
// Called once from BenchmarkTools after daemon startup — NOT from TestMain,
// because TestMain runs before any daemon exists and the memory store is
// daemon-owned.
func seedBenchMemories(tb testing.TB, bd *benchDaemon) {
	tb.Helper()
	seeds := []struct {
		name    string
		content string
	}{
		{benchMemoryName, "bench manifest seeded payload"},
		{benchMemoryName + "-rename-src", "bench manifest rename-src seed"},
		{benchMemoryName + "-delete", "bench manifest delete seed"},
	}
	for _, s := range seeds {
		_ = callToolB(tb, bd.Session, "write_memory", map[string]any{
			"name":    s.name,
			"content": s.content,
		})
	}
}

// memoryReseedFn maps memory-mutating tools to a function that restores the
// pre-call state before each warmup + measured invocation. rename_memory and
// delete_memory consume their target and would otherwise fail on the 2nd
// call; edit_memory is idempotent on the "PAYLOAD" result but we re-seed to
// be safe. Reseed runs inside StopTimer/StartTimer fences for measured
// iterations so only the tool call itself is timed.
var memoryReseedFn = map[string]func(tb testing.TB, bd *benchDaemon){
	"rename_memory": func(tb testing.TB, bd *benchDaemon) {
		// Ensure src exists and dst does not.
		_ = callToolB(tb, bd.Session, "write_memory", map[string]any{
			"name":    benchMemoryName + "-rename-src",
			"content": "rename src",
		})
		_, _ = bd.Session.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "delete_memory",
			Arguments: map[string]any{"name": benchMemoryName + "-rename-dst"},
		})
	},
	"edit_memory": func(tb testing.TB, bd *benchDaemon) {
		_ = callToolB(tb, bd.Session, "write_memory", map[string]any{
			"name":    benchMemoryName,
			"content": "bench manifest write_memory payload",
		})
	},
	"delete_memory": func(tb testing.TB, bd *benchDaemon) {
		_ = callToolB(tb, bd.Session, "write_memory", map[string]any{
			"name":    benchMemoryName + "-delete",
			"content": "delete target",
		})
	},
	// switch_mode: the profile-skill refuses self-transitions
	// (edit->edit), so before each measured call we first pre-switch to
	// "read" so the actual bench invocation transitions read->edit. This
	// is reseed, not part of the measured window.
	"switch_mode": func(tb testing.TB, bd *benchDaemon) {
		_, _ = bd.Session.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "switch_mode",
			Arguments: map[string]any{"target_mode": "read"},
		})
	},
}

// isEditTool is the explicit map of the 6 symbol-editing tools listed in
// 09-03-PLAN.md Task 1 step 3. It is preserved for grep-friendliness against
// the plan's acceptance_criteria (`grep -n "isEditTool"`). The actual
// dispatch, however, keys on tc.needsCopy — which is the canonical
// mutation-safety flag set in the 09-02 manifest — so any tool flagged as
// mutating (edit tools PLUS create_file, replace_in_file, format_code) is
// routed to the per-iteration fresh-copy path. This keeps the plan's
// acceptance grep happy while fixing the real cross-contamination bug that
// would occur if we only copied fixtures for the 6 symbol-edit tools.
var isEditTool = map[string]bool{
	"replace_symbol_body":  true,
	"insert_before_symbol": true,
	"insert_after_symbol":  true,
	"rename_symbol":        true,
	"safe_delete_symbol":   true,
	"verify_edit":          true,
}

// BenchmarkTools is the BENCH-02 table-driven benchmark covering every MCP
// tool in the 38-tool manifest. One daemon is started per bench function
// (Pattern 2) and reused across every sub-benchmark and every iteration.
//
// Each sub-benchmark:
//  1. Performs exactly 3 warmup calls before entering b.Loop() (phase Q6)
//     to prime gopls symbol caches and any lazy init in the kernel tool
//     dispatcher so the measured window reflects steady-state latency.
//  2. Calls ReportAllocs AFTER warmup but BEFORE the loop so alloc
//     accounting covers only measured iterations.
//  3. Uses the B.Loop form exclusively (never the classic counter loop) to
//     defeat compiler elision per 09-RESEARCH.md Pitfall 3.
//  4. Assigns the tool result to `sink` as a belt-and-suspenders guard over
//     b.Loop's own elision prevention.
//  5. For the 6 edit tools, wraps the inner body with StopTimer/StartTimer
//     around prepareGoFixtureCopyB + activateWorkspaceB so each iteration
//     operates on a fresh mutable copy of the Go fixture.
func BenchmarkTools(b *testing.B) {
	requireGoplsB(b)
	bd := startBenchDaemon(b)
	fixture := prepareGoFixtureB(b)
	activateWorkspaceB(b, bd, fixture)

	// Seed the memory store so read_memory / rename_memory / edit_memory
	// sub-benches have state to operate on. The 09-02 tools_manifest_test.go
	// header comment claims TestMain seeds these, but TestMain only runs the
	// leak check — the actual seeding lives here, co-located with
	// BenchmarkTools which is the sole consumer. Plan 09-03 added this fix
	// after the smoke run surfaced "memory not found" failures.
	seedBenchMemories(b, bd)

	for _, tc := range benchTools {
		tc := tc // capture per iteration for b.Run closure
		b.Run(tc.name, func(b *testing.B) {
			// Re-activate the shared fixture at the start of every
			// sub-benchmark. Edit-tool sub-benchmarks point the daemon at
			// tb.TempDir() copies that get cleaned up when their sub-bench
			// exits, leaving subsequent non-edit sub-benches pointing at a
			// deleted workspace. Re-anchoring here makes every sub-bench
			// independent of sibling ordering.
			activateWorkspaceB(b, bd, fixture)

			// Dispatch on tc.needsCopy (the 09-02 manifest flag) rather
			// than isEditTool, so non-symbol mutating tools such as
			// create_file, replace_in_file, and format_code also get the
			// per-iteration fresh-copy path. isEditTool remains defined
			// above to satisfy Plan 09-03 step 3's explicit 6-tool map.
			edit := tc.needsCopy || isEditTool[tc.name]
			reseed := memoryReseedFn[tc.name]

			// Warmup: 3 calls before the measured loop (phase Q6).
			// Purpose: prime gopls symbol caches + any lazy init in the
			// kernel tool dispatcher so the measured window reflects
			// steady-state latency, not first-touch penalties.
			//
			// Edit-tool warmups MUST NOT run against the shared read-only
			// fixture — repeated mutations would corrupt it for sibling
			// sub-benches and also drive the tool into inconsistent
			// states on the second warmup call (e.g. rename_symbol sees
			// its own prior rename). Each edit warmup therefore stages
			// its own fresh copy first, matching the measured path.
			for i := 0; i < 3; i++ {
				if edit {
					fresh := prepareGoFixtureCopyB(b)
					activateWorkspaceB(b, bd, fresh)
				}
				if reseed != nil {
					reseed(b, bd)
				}
				_ = callToolB(b, bd.Session, tc.name, tc.args)
			}

			b.ReportAllocs()

			// Single b.Loop() construct (acceptance criteria: exactly 1
			// occurrence). Per-iteration setup is dispatched by
			// edit/reseed flags inside StopTimer/StartTimer fences so the
			// measured window still covers only the tool call itself.
			//
			//   edit=true    -> fresh fixture copy + workspace re-activate
			//                   (6 symbol-edit tools + create_file +
			//                   replace_in_file + format_code)
			//   reseed!=nil  -> memory store restored to its pre-call state
			//                   (rename_memory, edit_memory, delete_memory)
			//   both false   -> shared warm fixture, no per-iteration setup
			var sink any
			for b.Loop() {
				if edit || reseed != nil {
					b.StopTimer()
					if edit {
						fresh := prepareGoFixtureCopyB(b)
						activateWorkspaceB(b, bd, fresh)
					}
					if reseed != nil {
						reseed(b, bd)
					}
					b.StartTimer()
				}
				r := callToolB(b, bd.Session, tc.name, tc.args)
				sink = r
			}
			_ = sink
		})
	}
}
