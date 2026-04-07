# Domain Pitfalls

**Domain:** Go-native MCP code intelligence platform (LSP gateway + daemon architecture)
**Researched:** 2026-04-07

## Critical Pitfalls

Mistakes that cause rewrites, data corruption, or production outages.

### Pitfall 1: Pipe Deadlock on Child Process I/O

**What goes wrong:** Go's `exec.Command` with `StdinPipe`/`StdoutPipe` deadlocks when the parent blocks on `cmd.Wait()` while the child blocks on stdin read, or when stdout pipe buffers fill because the parent isn't draining them fast enough. LSP servers are long-lived stdin/stdout processes -- this is the single most likely showstopper.

**Why it happens:** Go's `cmd.Wait()` waits for both process exit AND pipe EOF. If any goroutine still holds a reference to stdout/stderr pipes, or if stdin isn't closed before Wait, the call hangs forever. The existing Python Serena already has a multi-stage shutdown with threaded timeout to work around this exact class of bug (see `ls.py:_shutdown`).

**Consequences:** Daemon hangs on LS shutdown, zombie LS processes accumulate, entire daemon becomes unresponsive. On Linux/macOS the daemon appears alive but can't serve requests.

**Warning signs:** Tests pass locally but CI hangs. Shutdown takes >5s. `goroutine` count in pprof grows monotonically.

**Prevention:**
- Dedicate separate goroutines for stdin writes and stdout/stderr reads. Never do I/O on pipes from the goroutine that calls `Wait()`.
- Use `io.Copy` in goroutines with proper error channels, not direct `Read`/`Write` on pipes.
- Always close stdin pipe explicitly before calling `Wait()`.
- Use `context.Context` with timeout wrapping `Wait()` -- if timeout fires, send SIGTERM then SIGKILL escalation (mirror the 3-stage pattern in current Python `_shutdown`).
- Wrap all LS process management in a `ProcessHandle` type that enforces this invariant structurally.

**Detection:** Integration test that starts a real LS, sends requests, then shuts down with a 5-second hard deadline. Fails if process is still alive.

**Phase:** Layer 1 (Code Intelligence Kernel) -- must be correct from day one.

**Sources:**
- [golang/go#47061: cmd.Start deadlock with cmd.Stdin](https://github.com/golang/go/issues/47061)
- [golang/go#10338: cmd.Wait does not return when stdin attached](https://github.com/golang/go/issues/10338)
- Current Python Serena `src/solidlsp/ls.py` lines 636-693

---

### Pitfall 2: LSP Initialization Race -- Messages Before `initialized`

**What goes wrong:** Client sends `textDocument/didOpen` or other requests after the `initialize` response but before sending the `initialized` notification. Some language servers silently ignore these; others crash, return errors, or enter undefined state.

**Why it happens:** In a daemon architecture with warm LS workers, a new session might immediately want to open documents on an LS that's still initializing, or an LS that just restarted. The temptation is to pipeline requests to reduce latency.

**Consequences:** Silent data loss (symbols not indexed for opened files), server crashes requiring restart, or subtle bugs where the first file opened in a session has no diagnostics/symbols.

**Warning signs:** "Works on second try" bug reports. Different behavior depending on which file is opened first. Flaky tests that pass when run individually.

**Prevention:**
- Gate ALL requests behind an `initialized` state machine per LS worker. Use a `sync.WaitGroup` or channel that blocks request dispatch until initialization handshake completes.
- The state machine should have exactly these states: `Starting -> Initializing -> Ready -> ShuttingDown -> Stopped`. Only `Ready` allows request dispatch.
- Buffer `didOpen` notifications that arrive during `Initializing` and replay them once `Ready`.
- Never reuse a connection to an LS that has been sent `shutdown` -- always create new process.

**Detection:** Test that sends `textDocument/documentSymbol` immediately after `initialize` response but before `initialized` notification. Must either queue or error cleanly, never hang.

**Phase:** Layer 1 -- part of the LS adapter design.

**Sources:**
- [eglot#100: didOpen before initialized violates spec](https://github.com/joaotavora/eglot/issues/100)
- [sublimelsp/LSP#668: shutdown between initialize and initialized](https://github.com/sublimelsp/LSP/issues/668)
- [LSP Specification 3.17 - Lifecycle](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/)

---

### Pitfall 3: Goroutine Leak via Unreaped Child Processes

**What goes wrong:** Goroutines that call `cmd.Wait()` on a child process hang forever because the LS process doesn't exit cleanly (common with Java-based servers like jdtls, or Node-based servers). These goroutines accumulate, each holding pipe file descriptors, eventually exhausting FDs or memory.

**Why it happens:** Not all language servers honor the LSP shutdown/exit sequence. Some ignore `shutdown`, some require SIGTERM, some need stdin close. Java LS processes may spawn child JVMs that outlive the parent. The daemon can't just `os.Exit()` like a short-lived process -- it must actively manage every child.

**Consequences:** File descriptor exhaustion (default 1024 on macOS), OOM from goroutine stack accumulation, daemon becomes unable to spawn new LS workers.

**Warning signs:** `runtime.NumGoroutine()` grows over time. `lsof -p <daemon-pid>` shows hundreds of pipe FDs. LS restart frequency increases.

**Prevention:**
- Implement a `ProcessReaper` goroutine per LS worker that owns the `Wait()` call and enforces a hard kill deadline (SIGTERM -> 3s -> SIGKILL -> 2s -> log error and abandon).
- Set `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` so SIGKILL can target the entire process group, catching child JVMs.
- Track all LS workers in a registry with periodic health checks (is process alive? is it responding to LSP requests?).
- Expose `/debug/goroutines` endpoint on the daemon for operational visibility.
- Set `RLIMIT_NOFILE` high at daemon startup and monitor FD count.

**Detection:** Integration test that starts and stops 50 LS workers in sequence, asserts goroutine count returns to baseline.

**Phase:** Layer 1 -- core process lifecycle.

---

### Pitfall 4: Unix Socket Stale File on Crash

**What goes wrong:** Daemon crashes (SIGKILL, OOM kill, power loss) and leaves its Unix socket file on disk. Next daemon startup fails with "address already in use." Users see a cryptic error and don't know to delete the socket file.

**Why it happens:** Unix domain sockets create filesystem entries that persist after process death. Unlike TCP ports, the OS does not automatically reclaim them. The gopls daemon had exactly this problem when different tools used different TMPDIR values, causing multiple daemons to spawn.

**Consequences:** Daemon won't start after crash. Users must manually find and delete socket file. In CI, this causes flaky pipelines.

**Warning signs:** "Works after reboot" bug reports. Different behavior in Docker vs host.

**Prevention:**
- On startup, check if socket file exists. If it does, attempt to connect -- if connection refused, the previous daemon is dead, so unlink and proceed. If connection succeeds, the daemon is already running, so become a client or exit with a helpful message.
- Write a PID file alongside the socket. On startup, check if PID is alive (via `kill(pid, 0)`).
- Register signal handlers for SIGTERM and SIGINT that clean up the socket file. Accept that SIGKILL can't be caught -- the startup probe handles this case.
- Use `SO_REUSEADDR` equivalent for Unix sockets: `os.Remove(path)` before `net.Listen("unix", path)` only after confirming no live daemon.
- Place socket in `$XDG_RUNTIME_DIR` (Linux) or `$TMPDIR` (macOS) with a deterministic name based on workspace key.

**Detection:** Test that creates socket file, starts daemon (should clean up stale file and start), kills daemon with SIGKILL, starts daemon again (should recover).

**Phase:** Layer 0 (MCP Runtime) -- daemon lifecycle.

**Sources:**
- [gopls daemon socket issue](https://groups.google.com/g/golang-tools/c/y3OQNIudLzQ)
- [Go gopls daemon docs](https://go.dev/gopls/daemon)
- [Unix domain socket cleanup guide](https://copyprogramming.com/howto/unix-domain-socket-not-closed-after-close)

---

### Pitfall 5: MCP Session State vs Transport Confusion

**What goes wrong:** Treating MCP sessions as equivalent to transport connections. A stdio connection dies, the session state is lost, and the agent must re-initialize everything (LS warmup, file opens, context). This is the exact pain point Serena 1.0 suffers -- per-session LS startup cost.

**Why it happens:** The MCP spec (2025-11-25) ties sessions to transport lifecycle. Stdio sessions are implicit in process lifetime. Streamable HTTP sessions are created at initialization. The protocol is actively evolving to decouple sessions from transport (2026 roadmap mentions cookie-like mechanism). Building too tightly to current spec means rework.

**Consequences:** Client disconnect kills warm caches. Agent must re-pay initialization cost. Multi-client scenarios (two editors on same project) can't share LS workers. Stateful sessions fight load balancers for HTTP transport.

**Warning signs:** Users complaining about slow reconnection. Memory usage spikes on reconnect as caches rebuild.

**Prevention:**
- Separate three concerns from day one: transport connection, MCP session, workspace state. A workspace lives independently of any session.
- The daemon owns workspace state (warm LS workers, caches, file watchers). Sessions are lightweight views into workspaces with their own dirty buffers and mode/capability profiles.
- When a transport dies, the session can be tombstoned (kept for TTL) and resumed if the client reconnects with the same session ID.
- Design the internal API so sessions never directly hold LS process handles -- they go through the workspace registry.
- This is explicitly called out in PROJECT.md as the target architecture. The pitfall is implementing it halfway (e.g., session holds a direct LS reference "for performance").

**Detection:** Test that connects via stdio, opens files, disconnects, reconnects, verifies LS is still warm and symbols are available without re-initialization.

**Phase:** Layer 0 (MCP Runtime) -- foundational session/workspace separation.

**Sources:**
- [MCP Specification 2025-11-25](https://modelcontextprotocol.io/specification/2025-11-25)
- [2026 MCP Roadmap - Session evolution](https://blog.modelcontextprotocol.io/posts/2026-mcp-roadmap/)
- [MCP Transport Future](https://blog.modelcontextprotocol.io/posts/2025-12-19-mcp-transport-future/)

---

### Pitfall 6: Cache Invalidation Races with File Watchers

**What goes wrong:** File watcher reports a change, daemon invalidates cache and re-indexes, but the LS hasn't processed the `didChange`/`didSave` notification yet. Queries during this window return stale or inconsistent results. Alternatively, rapid saves (e.g., `git checkout` changing 500 files) overwhelm the watcher and invalidation queue.

**Why it happens:** Three independent systems must stay in sync: file watcher (OS-level), LS document state (via didOpen/didChange/didSave), and daemon cache (symbol graph, indexes). Each has different latency characteristics. fsnotify on macOS uses kqueue which can coalesce events; on Linux inotify delivers per-file events that can flood.

**Consequences:** Agent gets stale symbol information, edits based on outdated positions, corrupts files. Or: daemon spends 100% CPU processing file watcher events after `git checkout`.

**Warning signs:** Symbol positions are off by a few lines after edits. Tests pass on single-file changes but fail on bulk operations.

**Prevention:**
- Debounce file watcher events (200-500ms window). Batch events per directory.
- Use a versioned cache: every cache entry has a version number. When a file changes, increment the version. Reads check version before returning.
- For bulk operations (`git checkout`, `git pull`), detect the pattern (>N changes in <M seconds) and pause indexing, then do a single full re-index.
- Watch parent directories, not individual files. fsnotify loses watches when files are atomically replaced (write-to-temp-then-rename pattern).
- The LS is the source of truth for open files. Only use file watchers for files not currently open in any session.

**Detection:** Test that modifies a file on disk, immediately queries for symbols, verifies results are eventually consistent within 1 second.

**Phase:** Layer 1 -- cache/index subsystem. Should be designed early but can be iterated.

**Sources:**
- [fsnotify#666: race/deadlock on recursive watcher](https://github.com/fsnotify/fsnotify/issues/666)
- [fsnotify: atomic rename loses watch](https://pkg.go.dev/github.com/fsnotify/fsnotify)

## Moderate Pitfalls

### Pitfall 7: Python-to-Go Rewrite Trap -- Porting Instead of Redesigning

**What goes wrong:** Translating Python patterns line-by-line into Go instead of redesigning for Go's strengths. Examples: porting Python's `threading.Thread(target=self.server.shutdown)` with daemon threads into goroutines without proper cleanup; porting the single-threaded task executor into a goroutine pool that's unnecessarily complex; porting pickle-based caching into gob encoding when a simpler approach exists.

**Why it happens:** The existing Python codebase is the specification. It's tempting to maintain 1:1 correspondence for safety. But Python idioms (dynamic typing, exception chaining, context managers, asyncio task groups) don't map cleanly to Go.

**Consequences:** Unidiomatic Go code that's hard to maintain, misses Go's concurrency strengths, and carries forward Python's architectural compromises.

**Warning signs:** Go code that uses `interface{}` everywhere (mimicking Python dynamic typing). Goroutines used as threads with shared mutable state instead of channels. Error handling that wraps every call in a recovery defer.

**Prevention:**
- Use the Python code as a *feature specification*, not an *implementation guide*. Extract: what are the inputs, outputs, and invariants of each tool? Then implement idiomatically.
- Specifically redesign:
  - Task executor -> `context.Context` + `errgroup.Group` (not a custom executor)
  - Cache serialization -> protobuf or `encoding/json` (not gob, for debuggability)
  - LS lifecycle -> process supervisor pattern with channels (not thread-with-timeout)
  - Error recovery -> typed errors with `errors.Is`/`errors.As` chains
- The current Python `tools_base.py` retry logic (catch LSP exception, restart LS, retry once) should become a middleware/decorator pattern in Go, not inline logic.

**Detection:** Code review checklist: "Is this pattern here because Go needs it, or because Python had it?"

**Phase:** All phases -- establish Go idioms in Phase 1 and enforce via review.

**Sources:**
- Current Serena `src/serena/tools/tools_base.py` lines 316-332 (retry pattern)
- Current Serena `src/solidlsp/ls.py` lines 636-693 (thread-based shutdown)

---

### Pitfall 8: Feature Parity Obsession Delaying the Core

**What goes wrong:** Trying to rewrite all 40+ tools before shipping anything. The rewrite stalls at 80% completion, never ships, and the Python version continues accumulating fixes that the Go version must also absorb.

**Why it happens:** The Python Serena has a large tool surface (symbol tools, file tools, memory tools, config tools, workflow tools). There's pressure to reach feature parity before switching users over. But the rewrite's value proposition is the runtime model (daemon, warm caches, proper lifecycle), not the tool count.

**Consequences:** 6+ month rewrite with no users. Divergence between Python and Go versions. Team demoralization.

**Warning signs:** Roadmap has "rewrite tool X" as a milestone. No user testing before Phase 3.

**Prevention:**
- Ship a working daemon with 5-8 core tools (find_symbol, get_symbols_overview, read_symbol_body, find_references, get_definition, search_for_pattern, read_file, replace_symbol_body) as the MVP.
- These tools cover >80% of agent usage based on typical coding workflows.
- Use the same MCP tool names and parameter schemas so agents can switch transparently.
- Run both Python and Go versions in parallel during migration -- agents can fall back to Python for tools not yet ported.
- Track "which tools do agents actually call?" in production before prioritizing ports.

**Detection:** If a milestone has more than 10 tools, it's too big. If no one uses the Go version by end of Phase 2, scope is wrong.

**Phase:** Roadmap structure -- affects phase boundaries.

---

### Pitfall 9: LSP Server Quirks -- The Long Tail of Language-Specific Bugs

**What goes wrong:** Building a generic LSP client that works perfectly with gopls but fails with pyright, jdtls, typescript-language-server, rust-analyzer, etc. Each server interprets the spec differently, has different capabilities, and has different timing requirements.

**Why it happens:** The LSP spec is large and ambiguous in places. Servers implement subsets. Some servers require specific initialization options. jdtls needs workspace folders set during initialization. Pyright needs `pythonPath` configuration. TypeScript server needs `tsserver.path`. Some servers send `$/progress` notifications that must be handled to avoid blocked request queues.

**Consequences:** "Works with Go, fails with Python" bugs. Each new language requires debugging a unique set of server behaviors. Test matrix explodes.

**Warning signs:** Each language server requires its own initialization config struct. Bug reports cluster by language, not by feature.

**Prevention:**
- Create a `LanguageServerProfile` type that encodes per-server quirks: initialization options, required capabilities, shutdown behavior, known protocol deviations.
- Port these profiles from the existing Python `src/solidlsp/language_servers/` directory -- this is the most valuable knowledge in the current codebase.
- Start with 3-4 servers (gopls, pyright, typescript-language-server, rust-analyzer) and get them perfect before adding more.
- Integration test each supported server in CI with a real project. The existing `test/resources/repos/` are a goldmine.
- Handle `$/progress` and `window/workDoneProgress` generically -- many servers block if these aren't acknowledged.

**Detection:** Per-language integration test suite. If adding a new language takes >1 day, the abstraction is wrong.

**Phase:** Layer 1 -- LS adapter layer. Start narrow (3-4 languages), widen in later phases.

---

### Pitfall 10: Serialized Mutations Creating a Bottleneck

**What goes wrong:** The design calls for "serialized mutations, parallel reads." Under load, a mutation (e.g., `replace_symbol_body`) holds a write lock, blocking all reads across all sessions on that workspace. If the mutation involves an LS round-trip (send edit, wait for diagnostics/re-index), the lock is held for seconds.

**Why it happens:** Write serialization is correct for safety, but the granularity matters. A workspace-level write lock is too coarse. A file-level write lock may be too fine (renames affect multiple files).

**Consequences:** Multi-agent scenarios (two Claude Code sessions on the same repo) degrade to serial execution. Tool response times spike during edits.

**Warning signs:** P99 latency spikes correlated with edit operations. Agents time out waiting for read operations during another session's edit.

**Prevention:**
- Use file-level read-write locks, not workspace-level. Group multi-file mutations (rename) into atomic lock sets acquired in sorted order (prevents deadlock).
- Separate the mutation into two phases: (1) acquire lock + apply edit (fast, <100ms), (2) release lock + wait for LS to re-index (async, can run in background).
- Reads during re-indexing return results from the pre-edit cache with a staleness flag, rather than blocking.
- Use Go's `sync.RWMutex` per file path, managed by a striped lock map to avoid unbounded lock allocation.

**Detection:** Load test with 3 concurrent sessions: 2 reading, 1 writing. Read latency should not degrade >2x during writes.

**Phase:** Layer 1 -- concurrency model. Design early, load test in Phase 2.

---

### Pitfall 11: Daemon Health Check and Liveness Confusion

**What goes wrong:** The stdio forwarder connects to the daemon's Unix socket but the daemon is in a degraded state (e.g., all LS workers crashed, GC pause, deadlocked goroutine). The forwarder considers the daemon "alive" because the socket accepted the connection, but requests hang.

**Why it happens:** TCP/Unix socket accept != application health. Without application-level health checks, the forwarder can't distinguish "daemon is alive and healthy" from "daemon is alive but broken."

**Consequences:** Agent hangs waiting for tool responses. User must manually kill and restart daemon. In CI, builds time out.

**Warning signs:** Socket connect succeeds but first request times out. Daemon process exists but CPU is 0% (deadlocked).

**Prevention:**
- Implement a lightweight health check endpoint on the daemon (e.g., `{"method":"health/check"}` on the control socket) that verifies: (a) event loop is responsive, (b) at least one LS worker is healthy, (c) goroutine count is within bounds.
- The stdio forwarder should health-check the daemon before forwarding the first request. If unhealthy, restart the daemon.
- Set a global request timeout in the forwarder (e.g., 120s). If any request exceeds this, assume daemon is degraded and attempt reconnection.
- Expose pprof endpoints (`/debug/pprof/`) on a separate port for operational debugging.

**Detection:** Test that starts daemon, kills all LS workers, sends a tool request via forwarder. Should get an error response within 5s, not a hang.

**Phase:** Layer 0 -- daemon infrastructure.

## Minor Pitfalls

### Pitfall 12: Signal Forwarding to Child Process Groups

**What goes wrong:** Daemon receives SIGTERM, shuts down its own goroutines, but forgets to forward the signal to LS child processes. Orphaned LS processes continue running, holding file locks and ports.

**Prevention:** Use process groups (`Setpgid: true`) and send signals to the negative PID (entire group). Register a shutdown hook that iterates all tracked child PIDs and sends SIGTERM before the daemon exits.

**Phase:** Layer 1 -- process manager.

---

### Pitfall 13: JSON-RPC ID Collision Between Sessions

**What goes wrong:** Two MCP sessions share an LS worker. Both send requests. If the daemon uses sequential integer IDs for JSON-RPC requests to the LS, responses can't be routed back to the correct session.

**Prevention:** Use session-prefixed request IDs (e.g., `"session-abc:42"`) or maintain a per-LS response routing table keyed by request ID with session callback. The LS doesn't care about ID format -- it echoes whatever it receives.

**Phase:** Layer 1 -- LS multiplexing.

---

### Pitfall 14: `textDocument/didOpen` Version Tracking

**What goes wrong:** LSP requires a monotonically increasing version number on `didChange` notifications. If two sessions open the same file and make edits, version numbers must be consistent from the LS's perspective. Sending version 3 then version 2 causes undefined behavior in some servers.

**Prevention:** The workspace (not the session) owns the document version counter. All edits go through the workspace which assigns the next version atomically. Sessions submit edit intents; the workspace serializes them.

**Phase:** Layer 1 -- document sync subsystem.

---

### Pitfall 15: Oversized Tool Responses Blowing Agent Context

**What goes wrong:** A tool returns the entire symbol tree of a large file (thousands of symbols) or a full file read of a 10K-line file. The agent's context window fills up, degrading reasoning quality.

**Prevention:** Implement response size limits at the tool layer. Truncate with a message indicating truncation. The current Python Serena already handles this -- port the truncation logic. Add pagination support for symbol overview and search results.

**Phase:** Layer 2 -- skill/tool layer.

---

### Pitfall 16: Test Environment Assumes Installed Language Servers

**What goes wrong:** Integration tests require gopls, pyright, jdtls, etc. to be installed. CI environments don't have them. Tests pass locally, fail in CI.

**Prevention:** CI must install all tested language servers as an explicit setup step. Document exact versions. Use the existing `test/resources/repos/` pattern. Consider a Docker-based test environment with all servers pre-installed. Pin LS versions in CI to avoid flaky tests from upstream LS updates.

**Phase:** Phase 1 -- CI setup, before any integration tests.

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| Layer 0: Daemon bootstrap | Stale socket file (#4), signal handling | Implement startup probe + socket cleanup first |
| Layer 0: MCP runtime | Session/transport confusion (#5) | Separate session, transport, workspace from day one |
| Layer 1: LS process lifecycle | Pipe deadlock (#1), goroutine leak (#3) | ProcessHandle type with enforced I/O goroutine pattern |
| Layer 1: LS initialization | Race condition (#2), server quirks (#9) | State machine per LS, start with 3-4 servers only |
| Layer 1: Concurrency model | Write lock bottleneck (#10) | File-level locks, async re-index after edit |
| Layer 1: Cache/index | File watcher races (#6) | Debounce, version stamps, bulk detection |
| Layer 1: LS multiplexing | ID collision (#13), version tracking (#14) | Session-prefixed IDs, workspace-owned version counter |
| Layer 2: Tool implementation | Feature parity obsession (#8), porting idioms (#7) | MVP with 5-8 core tools, Go-idiomatic design |
| All phases: Testing | Missing LS in CI (#16) | Docker test environment, pinned LS versions |

## Sources

- [golang/go#47061: exec.Command stdin deadlock](https://github.com/golang/go/issues/47061)
- [golang/go#10338: cmd.Wait hangs with stdin](https://github.com/golang/go/issues/10338)
- [eglot#100: didOpen before initialized](https://github.com/joaotavora/eglot/issues/100)
- [gopls daemon documentation](https://go.dev/gopls/daemon)
- [gopls design documentation](https://go.dev/gopls/design/design)
- [gopls daemon socket issue](https://groups.google.com/g/golang-tools/c/y3OQNIudLzQ)
- [MCP Specification 2025-11-25](https://modelcontextprotocol.io/specification/2025-11-25)
- [2026 MCP Roadmap](https://blog.modelcontextprotocol.io/posts/2026-mcp-roadmap/)
- [MCP Transport Future](https://blog.modelcontextprotocol.io/posts/2025-12-19-mcp-transport-future/)
- [fsnotify#666: recursive watcher race](https://github.com/fsnotify/fsnotify/issues/666)
- [fsnotify package docs](https://pkg.go.dev/github.com/fsnotify/fsnotify)
- [LSP Specification 3.17](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/)
- [Six Fatal Flaws of MCP](https://www.scalifiai.com/blog/model-context-protocol-flaws-2025)
- Current Serena Python codebase: `src/solidlsp/ls.py`, `src/serena/ls_manager.py`, `src/serena/tools/tools_base.py`
