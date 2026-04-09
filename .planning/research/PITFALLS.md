# Domain Pitfalls: v1.2 Performance & Production Hardening

**Domain:** Adding benchmarks, observability, graceful degradation, and user docs to an existing Go daemon (Serena)
**Researched:** 2026-04-08
**Overall confidence:** MEDIUM-HIGH (mix of authoritative Go docs/Prometheus guidance and community WebSearch findings)

## Scope Note

These are pitfalls specific to **adding** these features to an already-shipped daemon with:
- 38+ MCP tools on hot paths
- Worker pool with existing circuit breaker, adaptive TTL, pressure eviction
- gRPC forwarder + daemon split
- Signal-first graceful shutdown (errgroup orchestration)
- Existing ad-hoc logging (not yet structured)

The risk profile is **different from greenfield**: instrumentation added after the fact tends to expose races, allocate in hot paths that were previously alloc-free, and change shutdown timing in ways that break existing lifecycle tests.

---

## Critical Pitfalls

### Pitfall 1: Benchmark Compiler Optimization Elision
**What goes wrong:** `testing.B` benchmark loops get dead-code-eliminated because the compiler notices the result is unused. You end up measuring "how long it takes to do nothing" and report wildly optimistic ns/op. Regression CI gate becomes meaningless — real slowdowns look like no-ops.
**Why it happens:** Go compiler inlines small functions and eliminates calls whose results aren't observed. Classic anti-patterns: `_ = Foo()` (compiler still elides), benchmarking pure functions without sinks, reusing the same input so the compiler hoists it out of the loop.
**Consequences:** False baseline; real regressions pass the gate; LOC/sec numbers in docs are fiction.
**Prevention:**
- Use **Go 1.24+ `testing.B.Loop`** instead of `for i := 0; i < b.N; i++`. It prevents unwanted optimization, auto-excludes setup, and is the canonical form going forward.
- Assign results to package-level `var sink` or `b.Keep(...)`.
- Vary inputs per iteration to defeat CPU-cache-hoisting artifacts.
- Compare benchmarks with `benchstat` — **never eyeball ns/op**. Require `-count=10` minimum for CI gate; reject deltas <5% as noise.
**Detection:** Absurdly fast numbers (sub-nanosecond ops on a real function = elided). Run `go test -bench=. -gcflags='-m'` to confirm inlining behavior.
**Phase:** Phase that introduces benchmark harness. **Must land before the CI regression gate phase.**

### Pitfall 2: Prometheus Cardinality Explosion from Tool/Workspace Labels
**What goes wrong:** Exposing metrics like `serena_tool_duration_seconds{tool, workspace, file_path, symbol_name, session_id}` looks harmless — each label bounds a series. File paths and session IDs are **unbounded**. After a week of use the daemon's in-process metric map has hundreds of thousands of series; memory balloons, Prometheus scrape times spike, the TSDB OOMs on the scraping side.
**Why it happens:** Serena's natural label candidates (file path, symbol name, workspace root, session ID, request ID) are exactly the "unbounded user-generated" shape that Prometheus docs warn against. The relationship between unique label values and memory is roughly linear.
**Consequences:** Daemon RSS grows unbounded; scrape latency climbs until Prometheus gives up; dashboards break retroactively.
**Prevention:**
- **Bound every label set at design time.** Allowed labels: `tool_name` (38 fixed), `profile` (5 fixed), `mode` (4 fixed), `language` (52 fixed), `outcome` (success/error/timeout). Total max cardinality is large but **bounded and known**.
- **Forbidden labels:** file path, symbol name, workspace path, session ID, request ID, LS worker PID, error message.
- Use `trace_id` / `request_id` as **exemplars** on histograms, not labels (Prom ≥2.26 supports exemplars for high-cardinality correlation without series explosion).
- Add a CI lint that greps metric registration for the forbidden label names.
- Pre-initialize known label combinations at startup (avoids dashboard gaps, forces cardinality accounting upfront).
**Detection:** Track `prometheus_tsdb_head_series` in staging. Alert if series count grows unbounded over 24h steady state. Also expose `serena_metric_series_total` inventory gauge.
**Phase:** Metrics phase — **cardinality contract must land in the same PR as the first metric**.

### Pitfall 3: Structured Logging Allocations on Hot Paths
**What goes wrong:** Swapping ad-hoc `log.Printf` for `slog.Info(...)` on every MCP tool dispatch adds allocations where there were none. Tools like `find_references` and `search_for_pattern` previously alloc-cheap now show 2-3x heap pressure under load. GC frequency rises, tail latency regresses.
**Why it happens:** The default `slog` API (`slog.Info("msg", "key", value, ...)`) boxes every value into `any`, causing an allocation per field. Reflection-based encoders (stdlib default JSON) compound this. Even structured loggers are expensive on truly hot paths.
**Consequences:** p99 tool response time regresses precisely while you're trying to benchmark it — you can't tell which regression is "real" vs observability-induced.
**Prevention:**
- **Use `slog.LogAttrs` with typed `slog.Attr` values** on hot paths: `slog.LogAttrs(ctx, slog.LevelInfo, "tool finished", slog.String("tool", name), slog.Duration("elapsed", d))`. This avoids boxing.
- Gate DEBUG-level logs behind `logger.Enabled(ctx, slog.LevelDebug)` before constructing arguments.
- **Benchmark before and after** adding logging to a representative tool (e.g., `find_references`). Bake an alloc budget into the CI gate: `b.ReportAllocs()` + `benchstat` delta ≤ +1 alloc/op.
- Default to `log/slog` for stdlib alignment; only escalate to `zerolog`/`zap` if `slog.LogAttrs` benchmarks prove unacceptable.
**Detection:** Alloc-per-op diff on benchmark suite before/after logging rollout. Flame graph with `pprof` should not show `slog.Record.AddAttrs` as hot.
**Phase:** Observability phase — structured logging rollout **must** include a benchmark diff against baseline.

### Pitfall 4: Metric/Trace Flush Lost on SIGTERM (Graceful Shutdown Race)
**What goes wrong:** Daemon receives SIGTERM, errgroup cancels context, Prometheus HTTP handler shuts down, OTel span exporter's batch buffer is dropped. The final minute of traces/metrics before shutdown — including the shutdown sequence itself and any crash-adjacent signals — is silently lost. Debugging the "why did it die" story becomes impossible.
**Why it happens:** OTel SDKs default to **async batch exporters** that queue spans in memory and flush on a timer. If the exporter's `Shutdown(ctx)` isn't explicitly called (or is called with an already-canceled context), in-flight batches are dropped. Prometheus pull-model has the inverse problem: if the scrape endpoint dies before the final scrape, last metrics window is lost.
**Consequences:** Observability goes dark exactly when you need it most (during shutdown/crash). Undermines the "production hardening" goal.
**Prevention:**
- **Kernel-first shutdown order is already correct** for Serena. Extend the sequence: (1) stop accepting new requests, (2) drain in-flight tools with timeout budget, (3) `tracerProvider.Shutdown(ctx)` with **fresh 5s context** (not the canceled errgroup ctx), (4) `meterProvider.Shutdown(ctx)`, (5) flush structured log buffer, (6) close listeners.
- Use a **separate `context.WithTimeout(context.Background(), 5*time.Second)`** for flush — do NOT pass the errgroup's canceled context.
- Push final "shutdown reason" metric/log **before** exporter shutdown, not after.
- For Prometheus pull model: expose a `/metrics/final` endpoint that supervisors can scrape post-SIGTERM, or emit critical shutdown counters to stderr as a fallback.
- Add a shutdown integration test: send SIGTERM mid-request, assert span export happened in the test collector.
**Detection:** Chaos-style test — kill daemon, verify N spans arrived in collector. Count of "shutdown completed" spans should equal daemon restart count.
**Phase:** Graceful degradation phase. Touches existing errgroup orchestration — **high risk of regressing v1.0 shutdown correctness**. Pair with the integration test harness from v1.1.

---

## Moderate Pitfalls

### Pitfall 5: Circuit Breaker Tuning Creates Thundering Herd on Recovery
**What goes wrong:** Tuning the existing LS worker circuit breaker for "production hardening" typically means reducing the half-open probe interval or increasing the half-open concurrency. Under real load, the moment a breaker transitions to half-open, *every* queued request attempts to probe simultaneously. LS worker gets hammered, instantly trips the breaker back open, oscillates.
**Why it happens:** Without jitter, all waiters wake at the same tick. Classic thundering herd. Made worse by the fact that multiple clients may all have been queued during the open state.
**Prevention:**
- **Add jitter to every retry/probe delay** (exponential backoff + jitter — decorrelated jitter is the current recommendation).
- Limit half-open state to **exactly one** probe request at a time; others fail fast.
- Use a token bucket for post-recovery request rate, not a binary open/closed gate.
- Benchmark the breaker under load **with a synthetic crashy LS worker** before shipping new tuning.
- Document the tuning knobs in USAGE.md; warn that aggressive values worsen the problem.
**Phase:** Graceful degradation phase. **Do not tune existing breaker without a load test fixture.**

### Pitfall 6: Timeout Budget Double-Counting Across Layers
**What goes wrong:** Adding "timeout budgets" across forwarder → gRPC → daemon → kernel → LS worker creates overlapping timeouts. Forwarder sets 30s, daemon sets 30s from its own clock (now it's 30s from when the daemon received the request, which is 200ms later). Nested `context.WithTimeout` calls each reset their own deadline if not derived from parent. LS worker cancellation fires after forwarder has already given up; daemon does expensive cleanup on an orphaned request.
**Why it happens:** Engineers reach for `context.WithTimeout` at each layer instead of `context.WithDeadline` derived from the incoming request's deadline. Each layer "helpfully" adds its own cushion.
**Prevention:**
- **Propagate deadlines, not durations.** The forwarder sets the initial deadline; every downstream layer uses `context.WithDeadline(parent, min(parent.Deadline(), local_cap))`.
- Reserve a **shrinking budget** explicitly: each layer subtracts its expected overhead from the remaining budget before passing down.
- Log the deadline at each layer boundary with the request ID as exemplar. Make deadline propagation observable.
- Integration test: set a 500ms deadline at the client, assert the LS worker sees a deadline ≤500ms minus forwarding overhead.
**Phase:** Graceful degradation phase. Touches the gRPC boundary — coordinate with forwarder code.

### Pitfall 7: OTel Tracing Overhead on Tool Dispatch
**What goes wrong:** Wrapping every MCP tool dispatch in an OTel span adds measurable CPU (~20-35% reported in production Go services) and a few ms of p99 latency even with sampling disabled. Benchmark numbers shift as observability is added; you can't tell if tool changes regressed or if tracing did.
**Why it happens:** Span creation allocates; context propagation cost is incurred even when the sampler says "don't sample"; custom samplers are called synchronously per span.
**Prevention:**
- **Trace at tool-dispatch granularity only**, not at every internal helper. The kernel is hot path; instrumenting `jsonrpc` send/recv is almost certainly too fine-grained.
- Use **parent-based + traceidratio** sampler with a low ratio (1-5%) in production defaults. Head sampling is cheaper than tail.
- **Never put network calls or complex logic in a custom Sampler.ShouldSample** — it's called synchronously per span.
- Preserve parent `tracestate` in custom samplers or distributed context propagation silently breaks.
- Gate span creation behind an explicit "tracing enabled" check at the dispatch layer so the zero-config path has zero tracing overhead.
- **Benchmark with tracing enabled and disabled** — report both numbers. Document the overhead in USAGE.md.
**Phase:** Observability phase. Include tracing-on vs tracing-off columns in the benchmark report.

### Pitfall 8: Histogram Bucket Defaults Are Wrong for Your SLO
**What goes wrong:** `prometheus.DefBuckets` goes `0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10` seconds. MCP tool responses mostly land in 10-500ms. p99 latency reported from default buckets rounds to the nearest bucket edge — you get p99 = 500ms when the true value is 180ms, making the CI regression gate noisy at best, useless at worst.
**Prevention:**
- **Tune histogram buckets to SLO targets**, not defaults. For tool response time: `0.005, 0.01, 0.025, 0.05, 0.1, 0.2, 0.5, 1, 2, 5` or use native histograms (Prometheus 2.40+) to sidestep the problem.
- Separate histograms per tool class: fast read-only symbol ops get tighter buckets than indexing operations.
- Document the bucket choice in the metrics phase design doc with the SLO rationale.
**Phase:** Metrics phase.

---

## Minor Pitfalls

### Pitfall 9: Documentation Rot — README/USAGE Go Stale Immediately
**What goes wrong:** README and USAGE ship with code snippets, CLI examples, and config fragments. Two sprints later, a flag is renamed, a profile's default toolset changes, a workflow renumbers its steps. Nothing fails CI, but new users hit "command not found" and file bugs.
**Prevention:**
- **Executable docs**: extract CLI examples into `docs/examples/*.sh` files that are run in CI as smoke tests. If the example breaks, CI breaks.
- **Generate the "38 tools" and "5 profiles" lists from code** via `go generate` — never hand-maintain in Markdown.
- Add a CI check that greps README/USAGE for hard-coded version strings and fails if they mismatch `VERSION` or `go.mod`.
- Consider AST-anchored doc linters (Drift-style) for snippets that quote source code.
- Date-stamp the USAGE file footer with last-verified commit.
**Phase:** Documentation phase — set up the generation + CI check as part of writing the doc, not after.

### Pitfall 10: Benchmark Environment Noise
**What goes wrong:** Running benchmarks on laptops, shared CI runners, or under CPU frequency scaling produces ±20% variance. Regression gate either false-positives constantly (team learns to ignore it) or false-negatives through noise tolerance set too wide.
**Prevention:**
- **Dedicated benchmark runner**: self-hosted, pinned CPU, disabled turbo, cpupower performance governor. GitHub-hosted runners are unstable for benchmarks — consider a separate cron job on dedicated hardware.
- Always `-count=10` minimum. Compare with `benchstat` and require p < 0.05 on deltas.
- Warm-up run discarded. Pin GOMAXPROCS explicitly.
- Don't gate PR merges on benchmark absolute numbers — gate on **relative delta** vs baseline from main.
**Phase:** Benchmark phase.

### Pitfall 11: Ticker and Goroutine Leaks from New Background Workers
**What goes wrong:** Adding a metrics-flush ticker, a trace-exporter goroutine, a doc-generator goroutine — each creates goroutines that never stop because the author forgot `defer ticker.Stop()` or missed wiring into the daemon's errgroup. `runtime.NumGoroutine()` climbs over daemon lifetime; eventually noticed only in v1.3.
**Prevention:**
- **Every new background goroutine goes through the daemon's errgroup**, period. No `go f()` in skill or kernel init.
- Every `time.NewTicker` has a paired `defer t.Stop()` caught in code review.
- Add a lifecycle test that asserts `runtime.NumGoroutine()` returns to baseline after daemon.Shutdown().
**Phase:** Every phase that adds background work — enforce via code review checklist.

### Pitfall 12: Logging Sensitive Data (Source Code, File Paths)
**What goes wrong:** Structured logging adds `slog.String("file", path)` and `slog.String("symbol", symbolBody)` for "debuggability." User ships logs to centralized aggregator. Proprietary source code ends up in third-party observability backends. Compliance nightmare.
**Prevention:**
- **Log redaction policy baked into logger config**: never log file contents, symbol bodies, LSP request payloads, or absolute file paths (use workspace-relative paths).
- Allowlist: tool name, duration, outcome, workspace hash, language, error category.
- Denylist guard: runtime check that rejects attrs matching forbidden key names.
- Document the policy in USAGE.md "security considerations."
**Phase:** Observability phase. **Critical for production credibility.**

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| Benchmarks | Elision, env noise, no statistical gate (#1, #10) | `testing.B.Loop` + benchstat + dedicated runner |
| Benchmarks → CI gate | Gate on absolute numbers → flaky | Gate on relative delta, require p<0.05 |
| Structured logging | Hot-path allocations, sensitive data leak (#3, #12) | `slog.LogAttrs` + allow-list labels + bench diff |
| Metrics | Cardinality explosion, wrong buckets (#2, #8) | Bounded label contract + SLO-tuned buckets + CI lint |
| Tracing | CPU overhead, custom sampler bugs (#7) | Low sampling ratio + tool-granularity only + preserve tracestate |
| Graceful shutdown | Lost final traces, shutdown race (#4) | Separate flush context + kernel-first order + integration test |
| Circuit breaker tuning | Thundering herd on recovery (#5) | Jitter + single-probe half-open + load test |
| Timeout budgets | Double-counting across layers (#6) | Propagate deadlines not durations |
| Documentation | Rot, stale snippets (#9) | Executable examples + code-generated lists + CI lint |
| Any new background work | Goroutine leaks (#11) | errgroup enforcement + lifecycle test |

## Warning Signs Matrix

| Symptom | Likely Pitfall | First Check |
|---------|---------------|-------------|
| Benchmark ns/op < 1ns for real function | #1 (elision) | `-gcflags='-m'`, convert to `b.Loop()` |
| Daemon RSS grows linearly with uptime | #2 (cardinality) or #11 (goroutine leak) | `prometheus_tsdb_head_series` or `runtime.NumGoroutine()` |
| p99 tool latency regressed after observability added | #3 (slog allocs) or #7 (OTel overhead) | Toggle observability, re-bench |
| Final shutdown logs/spans missing | #4 (flush race) | Shutdown context is canceled errgroup ctx |
| Breaker oscillates under load | #5 (thundering herd) | No jitter on half-open probes |
| Deadline exceeded errors with plenty of budget remaining | #6 (double-counting) | Log deadlines at each layer |
| Histogram p99 always lands on bucket edges | #8 (default buckets) | Check bucket definitions |
| README examples fail on fresh checkout | #9 (doc rot) | No CI execution of examples |
| Goroutine count grows over daemon lifetime | #11 (leak) | `runtime.NumGoroutine()` trace |

## Cross-Cutting Meta-Pitfall: Observing the Thing You're Benchmarking

**The worst pitfall of all for this milestone**: adding observability and benchmarks in the same release means you can't cleanly attribute regressions. If the observability overhead lands in the same PR as a new benchmark, you have no baseline.

**Recommended ordering:**
1. Benchmarks first — establish baseline without observability changes.
2. Structured logging second — measure and publish the allocation/latency delta vs baseline.
3. Metrics third — measure delta vs post-logging baseline.
4. Tracing fourth — measure delta vs post-metrics baseline.
5. Graceful degradation + circuit breaker tuning last — now you have real observability to measure the tuning.
6. Documentation in parallel, gated by executable examples.

Each step must publish a "delta from previous baseline" report so the team can see exactly what each feature cost.

## Sources

- [Common pitfalls in Go benchmarking — Eli Bendersky](https://eli.thegreenplace.net/2023/common-pitfalls-in-go-benchmarking/) — HIGH
- [More predictable benchmarking with testing.B.Loop — go.dev](https://go.dev/blog/testing-b-loop) — HIGH (authoritative)
- [Cardinality is Key — Robust Perception](https://www.robustperception.io/cardinality-is-key/) — HIGH (Prometheus authors)
- [How to Manage High Cardinality Metrics — Grafana Labs](https://grafana.com/blog/2022/10/20/how-to-manage-high-cardinality-metrics-in-prometheus-and-kubernetes/) — HIGH
- [Unbounded cardinality guard discussion — client_golang](https://github.com/prometheus/client_golang/discussions/970) — HIGH
- [Structured Logging with slog — go.dev](https://go.dev/blog/slog) — HIGH (authoritative)
- [Report Shows OpenTelemetry's Impact on Go Performance — InfoQ](https://www.infoq.com/news/2025/06/opentelemetry-go-performance/) — MEDIUM
- [OpenTelemetry Go Sampling — opentelemetry.io](https://opentelemetry.io/docs/languages/go/sampling/) — HIGH (authoritative)
- [Graceful Shutdown in Go: Practical Patterns — VictoriaMetrics](https://victoriametrics.com/blog/go-graceful-shutdown/) — MEDIUM
- [Thundering Herd with Circuit Breakers — Encore Blog](https://encore.dev/blog/thundering-herd-problem) — MEDIUM
- [Retries, Backoff and Jitter — CodeReliant](https://www.codereliant.io/p/retries-backoff-jitter) — MEDIUM
- [Drift documentation linter — Fiberplane](https://fiberplane.com/blog/drift-documentation-linter/) — LOW (product blog, concept validated)
