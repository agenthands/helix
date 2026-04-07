# Phase 2: Code Intelligence Kernel - Research

**Researched:** 2026-04-07
**Domain:** LSP worker pool management, symbol retrieval/editing tools, tree-sitter body surgery, LSP type codegen
**Confidence:** HIGH

## Summary

Phase 2 builds the code intelligence kernel: LS worker pool with lifecycle management, all symbol retrieval and editing tools, file operations, diagnostics, and LSP type generation from the metamodel. The core technical challenges are (1) managing heterogeneous language server child processes with warm caching and circuit breaking, (2) generating Go types from the LSP 3.17 metamodel JSON, (3) mapping LSP DocumentSymbol ranges to tree-sitter AST nodes for precise body surgery, and (4) implementing concurrency control that serializes writes while allowing parallel reads.

The ecosystem is clear on library choices. Use `tree-sitter/go-tree-sitter` (official, v0.25.0) with per-language grammar packages. For LSP codegen, adapt the gopls generator pattern (8-file generator producing 4 output files from metaModel.json). For JSON-RPC over stdio, build a thin custom implementation since `go.lsp.dev/jsonrpc2` is unmaintained (2022). The `mcp-language-server` project by isaacphi provides a validated reference for the gopls-derived LSP types approach.

**Primary recommendation:** Build bottom-up: LSP codegen first (unlocks all protocol types), then LS worker process management, then symbol retrieval tools, then tree-sitter integration for editing tools, then file operations and diagnostics last.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Standalone engine service model -- daemon owns the pool, not embedded in client lifetime. Three core objects: WorkspaceRuntime, SessionView, WorkerLease.
- **D-02:** Share-until-dirty policy: all clean sessions share warm LS workers. First unsaved edit promotes to own worker (gopls pattern).
- **D-03:** Adaptive TTL with decaying reuse score: `score = decay(score, now-last_used_at) + 1` on reuse. `ttl = clamp(base_ttl * f(score), base_ttl, ceiling)`. Avoids stale privilege from yesterday's hot workers.
- **D-04:** Auto-tuned TTL from observed reuse gaps: `warm_penalty = median(cold_start + first_query_after_start)`, `reuse_gap = p75(now - last_used_at on successful reuse)`, `ttl_target = clamp(max(base_ttl, reuse_gap * 1.5), base_ttl, ceiling)`, biased upward if warm_penalty is high.
- **D-05:** Platform-aware pressure eviction: Linux PSI, macOS os_proc_available_memory for headroom, task_info for per-worker resident_size. Eviction order: (1) RSS > hard cap, (2) unhealthy/stuck, (3) LRU with score==0, (4) lowest score/highest RSS ratio, (5) oldest idle.
- **D-06:** Circuit breaking with exponential backoff (1s, 2s, 4s, 8s...). Never fully breaks -- always retries with increasing delay.
- **D-07:** "TTL=0" (always-on mode) means no idle timeout, NOT immortal. Pressure eviction always active.
- **D-08:** Full LSP 3.17 metamodel codegen -- generate all protocol types and method bindings. Implement only the needed subset of methods.
- **D-09:** Package layout: `cmd/lspgen/` (custom generator binary), `protocol/metaModel.json` (pinned upstream input), `protocol/gen/` (generated .go), `protocol/patch/` (generator fixups/compatibility shims), `protocol/generate.go` (`//go:generate go run ../../cmd/lspgen`)
- **D-10:** Union representation: gopls-style pragmatic hybrid -- named tagged wrapper types by default (Or_X_Y with custom JSON marshal/unmarshal + typed accessors like AsTextEdit(), SetTextEdit(...)). High-frequency unions promoted to explicit one-of structs (like DocumentChange).
- **D-11:** Patch layer for metamodel quirks (e.g. RenameParams mismatch). Generated core + controlled fixes, not hand-curated types.
- **D-12:** Hybrid approach: LSP for symbol identity and discovery, tree-sitter for precise body boundary surgery.
- **D-13:** insert-before/insert-after: LSP-first (DocumentSymbol.range boundaries sufficient). Tree-sitter optional refinement.
- **D-14:** replace-body: tree-sitter-first (parse file, map symbol to AST node, derive precise body node). LSP-assisted for symbol discovery.
- **D-15:** Fallback for languages without tree-sitter grammar: insert-before/after from LSP range; replace-body degrades to best-effort or full-symbol replacement.
- **D-16:** Phase 2 ships with 3-4 languages: gopls (Go), pyright (Python), typescript-language-server (TypeScript), rust-analyzer (Rust). Proves multi-LS worker pool. Phase 3 expands to 40+.

### Claude's Discretion
- File operations implementation (FIL-01 through FIL-06) -- straightforward Go stdlib, no gray areas
- Diagnostic subscription and code action forwarding (DGN-01 through DGN-03) -- standard LSP protocol handling
- Multi-project workspace routing (WRK-02, WRK-03) -- builds on workspace registry from Phase 1

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DMN-07 | Clean sessions attach to existing warm LS workers | WorkerLease object in pool; share-until-dirty policy (D-02) |
| DMN-08 | Sessions with divergent unsaved buffers promote to own LS view | SessionView with overlay promotion; textDocument/didChange version tracking |
| DMN-09 | Idle LS workers stay warm for configurable TTL, then retire | Adaptive TTL with decaying reuse score (D-03/D-04); platform pressure eviction (D-05) |
| DMN-10 | Crashy LS workers circuit-broken with backoff and restarted | Circuit breaker pattern (D-06); ProcessReaper goroutine from PITFALLS.md |
| DMN-11 | Mutations serialized per session/view; reads parallel where safe | File-level RWMutex with striped lock map; two-phase edit (apply fast, re-index async) |
| SYM-01 | Go to definition | textDocument/definition via LS adapter |
| SYM-02 | Find all references | textDocument/references via LS adapter |
| SYM-03 | Symbol overview (file outline) | textDocument/documentSymbol via LS adapter |
| SYM-04 | Search symbols across workspace | workspace/symbol via LS adapter |
| SYM-05 | Hover/type information | textDocument/hover via LS adapter |
| SYM-06 | Find implementations | textDocument/implementation via LS adapter |
| SYM-07 | Call hierarchy | callHierarchy/incomingCalls + outgoingCalls |
| SYM-08 | Type hierarchy | typeHierarchy/subtypes + supertypes |
| SYM-09 | Blast radius analysis | Combined references + call/type hierarchy |
| EDT-01 | Replace symbol body | tree-sitter body extraction (D-14) + textDocument/didChange |
| EDT-02 | Insert before symbol | LSP DocumentSymbol.range start (D-13) |
| EDT-03 | Insert after symbol | LSP DocumentSymbol.range end (D-13) |
| EDT-04 | Rename symbol | textDocument/rename via LS adapter |
| EDT-05 | Safe delete symbol | Reference check (SYM-02) + delete if zero refs |
| EDT-06 | Post-edit diagnostic verification | textDocument/publishDiagnostics subscription |
| FIL-01 | Read file/range | Go os.ReadFile + byte offset slicing |
| FIL-02 | Create/overwrite file | Go os.WriteFile with atomic write pattern |
| FIL-03 | List directory | Go os.ReadDir |
| FIL-04 | Find files by glob | Go filepath.Glob or doublestar library |
| FIL-05 | Search regex patterns | Go regexp + filepath.Walk or ripgrep subprocess |
| FIL-06 | Replace via regex/literal | Go regexp.ReplaceAll + atomic file write |
| DGN-01 | LSP diagnostics after edits | textDocument/publishDiagnostics notification handler |
| DGN-02 | Code actions / quick fixes | textDocument/codeAction request |
| DGN-03 | Format code via LSP | textDocument/formatting request |
| WRK-02 | Auto-detect project languages | Scan for go.mod, pyproject.toml, tsconfig.json, Cargo.toml |
| WRK-03 | Multi-project support | Workspace registry extension with per-project LS workers |
| LNG-04 | LSP types generated from official metamodel JSON | Custom lspgen binary from metaModel.json (D-08/D-09) |
</phase_requirements>

## Standard Stack

### Core (Phase 2 additions)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| tree-sitter/go-tree-sitter | v0.25.0 | Tree-sitter Go bindings for AST parsing | Official bindings from tree-sitter org. Modular grammar loading. 226 stars, 172 importers. Requires CGO (Apple Clang available). |
| tree-sitter/tree-sitter-go/bindings/go | v0.0.0-20240817 | Go grammar for tree-sitter | Official grammar from tree-sitter org |
| tree-sitter/tree-sitter-python/bindings/go | v0.0.0-20240501 | Python grammar for tree-sitter | Official grammar from tree-sitter org |
| tree-sitter/tree-sitter-typescript/bindings/go | v0.0.0-20240720 | TypeScript grammar for tree-sitter | Official grammar from tree-sitter org |
| tree-sitter/tree-sitter-rust/bindings/go | v0.0.0-20240505 | Rust grammar for tree-sitter | Official grammar from tree-sitter org |
| dgraph-io/ristretto/v2 | v2.4.0 | In-memory TinyLFU cache for symbol/LS response caching | Generics-aware, concurrent-safe, cost-based eviction. Best Go cache library. |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| golang.org/x/sync/semaphore | (already in go.mod) | Concurrency limits on LS worker pool | Bound concurrent requests to a single LS |
| golang.org/x/sync/errgroup | (already in go.mod) | Parallel LS reads with error propagation | Fan-out symbol lookups across files |
| golang.org/x/sys | (already in go.mod) | Platform-specific memory pressure APIs | macOS syscall for os_proc_available_memory |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| tree-sitter/go-tree-sitter | smacker/go-tree-sitter | smacker bundles 30+ grammars (convenient) but last commit Aug 2024, 546 stars. Official bindings are actively maintained by tree-sitter org and more modular. |
| Custom JSON-RPC | go.lsp.dev/jsonrpc2 v0.10.0 | jsonrpc2 last updated 2022, Go 1.17 era. Small and correct but unmaintained. A thin custom implementation (~300 lines) is safer for a long-lived daemon with multiplexing needs. |
| Custom lspgen | bytebase/lsp-protocol | Bytebase extracts gopls types automatically. Could save initial effort but adds external dependency for core types. Own generator gives full control over union representation and patch layer. |

**Installation:**
```bash
go get github.com/tree-sitter/go-tree-sitter@v0.25.0
go get github.com/tree-sitter/tree-sitter-go/bindings/go@latest
go get github.com/tree-sitter/tree-sitter-python/bindings/go@latest
go get github.com/tree-sitter/tree-sitter-typescript/bindings/go@latest
go get github.com/tree-sitter/tree-sitter-rust/bindings/go@latest
go get github.com/dgraph-io/ristretto/v2@v2.4.0
```

## Architecture Patterns

### Recommended Package Structure (Phase 2 additions)

```
serena/
  cmd/
    lspgen/                  # LSP type generator binary (D-09)
      main.go                # Reads metaModel.json, generates Go types
      tables.go              # Type name fixups, field patches
      typenames.go           # Unnamed type naming heuristics
      output.go              # gofmt + file writing
  protocol/                  # LSP protocol types (D-09)
    metaModel.json           # Pinned LSP 3.17 metamodel input
    generate.go              # //go:generate go run ../../cmd/lspgen
    gen/                     # Generated output (4 files)
      tsprotocol.go          # All type + const definitions
      tsclient.go            # Client method stubs
      tsserver.go            # Server dispatch (for notification handling)
      tsjson.go              # Custom JSON marshal/unmarshal
    patch/                   # Generator fixups (D-11)
      rename_params.go       # RenameParams mismatch fix
      compatibility.go       # Shims for quirky types
  internal/
    kernel/                  # Layer 1: Code Intelligence Kernel
      kernel.go              # Kernel interface implementation
      workspace.go           # WorkspaceRuntime (D-01) extending Phase 1 workspace
    kernel/lspool/           # LS worker pool
      pool.go                # Pool manager: TTL, pressure eviction, scoring
      worker.go              # Single LS worker: child process + state machine
      lease.go               # WorkerLease (D-01): session-to-worker binding
      process.go             # ProcessHandle: pipe I/O goroutines, reaper
      circuit.go             # Circuit breaker with exponential backoff (D-06)
      pressure_linux.go      # Linux PSI pressure detection (D-05)
      pressure_darwin.go     # macOS memory headroom detection (D-05)
      adapter.go             # Generic LSP client: JSON-RPC codec over stdio
      quirks.go              # Per-language initialization quirk registry
    kernel/symbols/          # Symbol operations
      retrieval.go           # Definition, references, hover, implementations
      hierarchy.go           # Call hierarchy, type hierarchy
      overview.go            # DocumentSymbol file outline
      search.go              # Workspace symbol search
      blast.go               # Blast radius (combined refs + hierarchy)
    kernel/edit/             # Symbol editing
      planner.go             # Edit planning: resolve symbol, compute edit
      treesitter.go          # Tree-sitter body extraction engine
      queries/               # Per-language tree-sitter queries
        go.scm               # Go function/method body queries
        python.scm            # Python function/class body queries
        typescript.scm        # TypeScript function/class body queries
        rust.scm              # Rust fn/impl body queries
      replace.go             # Replace body implementation (D-14)
      insert.go              # Insert before/after (D-13)
      rename.go              # LSP rename forwarding (EDT-04)
      delete.go              # Safe delete with ref check (EDT-05)
      verify.go              # Post-edit diagnostic verification (EDT-06)
    kernel/fileops/          # File operations
      read.go                # Read file/range (FIL-01)
      write.go               # Create/overwrite with atomic writes (FIL-02)
      list.go                # Directory listing (FIL-03)
      find.go                # Glob file finding (FIL-04)
      search.go              # Regex search across codebase (FIL-05)
      replace.go             # Regex/literal replace (FIL-06)
    kernel/diag/             # Diagnostics
      subscriber.go          # publishDiagnostics handler (DGN-01)
      actions.go             # Code action forwarding (DGN-02)
      format.go              # Format via LSP (DGN-03)
    kernel/jsonrpc/          # Custom JSON-RPC 2.0 codec
      codec.go               # Content-Length framed reader/writer
      conn.go                # Bidirectional connection with request routing
      message.go             # Request, Response, Notification types
```

### Pattern 1: ProcessHandle for Safe Child Process I/O

**What:** A structural type that enforces safe pipe management for LS child processes. Separate goroutines for stdin writes and stdout reads. The goroutine calling `cmd.Wait()` never touches pipes directly.

**When to use:** Every LS worker child process.

**Why critical:** Pipe deadlocks (PITFALLS.md #1) are the single most likely showstopper. This pattern makes the safe approach the only approach.

```go
// ProcessHandle enforces safe I/O goroutine separation for child processes.
type ProcessHandle struct {
    cmd      *exec.Cmd
    stdin    io.WriteCloser
    stdout   io.ReadCloser
    stderr   io.ReadCloser
    requests chan *jsonrpcRequest   // writer goroutine drains this
    responses chan *jsonrpcResponse // reader goroutine feeds this
    done     chan struct{}          // closed when process exits
    exitErr  error
}

func (h *ProcessHandle) Start(ctx context.Context) error {
    // 1. Start process
    // 2. Launch reader goroutine: reads stdout, decodes JSON-RPC, sends to responses chan
    // 3. Launch writer goroutine: reads from requests chan, encodes JSON-RPC, writes to stdin
    // 4. Launch reaper goroutine: calls cmd.Wait(), closes done channel
    //    On ctx cancel: SIGTERM -> 3s -> SIGKILL escalation
}
```

### Pattern 2: LS Worker State Machine

**What:** Each LS worker has exactly 5 states: Starting -> Initializing -> Ready -> ShuttingDown -> Stopped. Only Ready allows request dispatch.

**When to use:** Every LS worker lifecycle.

```go
type WorkerState int
const (
    WorkerStarting     WorkerState = iota // Process spawning
    WorkerInitializing                     // LSP initialize sent, awaiting response
    WorkerReady                            // initialized sent, accepting requests
    WorkerShuttingDown                     // shutdown sent, draining
    WorkerStopped                          // Process exited
)
```

**Critical:** Buffer `didOpen` notifications arriving during Initializing state. Replay them once Ready. Never reuse a worker that has been sent `shutdown`.

### Pattern 3: DocumentSymbol-to-TreeSitter Node Mapping

**What:** Given an LSP DocumentSymbol (name + range), parse the file with tree-sitter, find the AST node whose range contains the symbol's range, then extract the precise body child node.

**When to use:** replace-body operations (EDT-01).

```go
func MapSymbolToBody(source []byte, lang *tree_sitter.Language, symbol DocumentSymbol) (start, end uint32, err error) {
    parser := tree_sitter.NewParser()
    defer parser.Close()
    parser.SetLanguage(lang)
    
    tree := parser.Parse(source, nil)
    defer tree.Close()
    
    root := tree.RootNode()
    // Walk tree to find declaration node containing symbol range
    node := findDeclarationAt(root, symbol.Range)
    if node == nil {
        return 0, 0, fmt.Errorf("no declaration node at range")
    }
    
    // Extract body child (language-specific field name)
    body := node.ChildByFieldName("body")
    if body == nil {
        return 0, 0, fmt.Errorf("declaration has no body field")
    }
    
    return body.StartByte(), body.EndByte(), nil
}
```

**Key insight:** The mapping works because LSP DocumentSymbol.range and tree-sitter node ranges both use (line, column) coordinates over the same source text. Find the tree-sitter declaration node whose range encompasses the DocumentSymbol range, then access its `body` field child.

### Pattern 4: Adaptive TTL with Decaying Reuse Score

**What:** Workers earn reuse credit on each use, which decays over time. TTL is a function of the score.

```go
type WorkerMetrics struct {
    score      float64
    lastUsedAt time.Time
    coldStart  time.Duration   // measured at spawn
    firstQuery time.Duration   // measured on first request after init
}

func (m *WorkerMetrics) OnReuse(now time.Time) {
    elapsed := now.Sub(m.lastUsedAt)
    m.score = m.score*math.Exp(-elapsed.Seconds()/decayHalfLife) + 1.0
    m.lastUsedAt = now
}

func (m *WorkerMetrics) TTL(baseTTL, ceiling time.Duration) time.Duration {
    factor := 1.0 + math.Log1p(m.score)
    ttl := time.Duration(float64(baseTTL) * factor)
    if ttl > ceiling { ttl = ceiling }
    if ttl < baseTTL { ttl = baseTTL }
    return ttl
}
```

### Anti-Patterns to Avoid

- **Direct LS protocol in tool handlers:** Tools must call the Kernel interface, never construct raw LSP JSON-RPC requests. The Kernel handles quirk normalization and worker routing.
- **Workspace-level write lock:** Use file-level RWMutex with striped lock map. Workspace-level locks serialize multi-agent scenarios unnecessarily.
- **Synchronous LS init in request path:** First request to a cold workspace must not block for 5-30s LS startup. Return "workspace initializing" and warm asynchronously.
- **Sequential integer JSON-RPC IDs shared across sessions:** Use session-prefixed IDs or a per-LS routing table to demultiplex responses to correct sessions.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| LSP protocol types | Hand-written Go structs for 300+ LSP types | Code generator from metaModel.json | Spec has 300+ types with complex unions; hand-writing is error-prone and unmaintainable |
| In-memory cache with eviction | Custom LRU/LFU map | ristretto v2 | TinyLFU admission policy, cost-based eviction, concurrent-safe -- thousands of edge cases |
| AST parsing for body extraction | Regex-based or line-counting body detection | tree-sitter | Handles nested braces, string literals containing braces, comments, multiline signatures |
| Content-Length framing | Manual string parsing of headers | Structured codec type | Edge cases: partial reads, multi-byte chars in Content-Length, multiple headers |
| Platform memory pressure | Polling /proc/meminfo or vm_stat | PSI (Linux) / os_proc_available_memory (macOS) | OS-provided pressure signals are more accurate than userspace polling |

**Key insight:** The LSP metamodel codegen is a one-time ~3-day investment that eliminates an entire class of type mismatch bugs. Every Go project that wraps LSP servers (gopls, mcp-language-server, opencode) generates types from the metamodel.

## Common Pitfalls

### Pitfall 1: Pipe Deadlock on LS Child Process I/O
**What goes wrong:** `cmd.Wait()` blocks forever because stdin isn't closed or stdout pipe buffer fills.
**Why it happens:** Go's Wait waits for both process exit AND pipe EOF. If any goroutine holds pipe references, Wait hangs.
**How to avoid:** ProcessHandle type with separate goroutines for stdin writes, stdout reads, and process reaping. Never do pipe I/O from the Wait goroutine.
**Warning signs:** Tests pass locally but CI hangs. Shutdown takes >5s. Goroutine count grows monotonically.

### Pitfall 2: LSP Initialization Race
**What goes wrong:** Sending textDocument/didOpen before sending `initialized` notification causes some LS servers to crash or silently drop the file.
**Why it happens:** Eager pipelining in a warm-worker model where new sessions want immediate access.
**How to avoid:** Worker state machine gates ALL requests behind Ready state. Buffer didOpen notifications during Initializing, replay on Ready.
**Warning signs:** "Works on second try" bugs. First file opened has no diagnostics.

### Pitfall 3: Tree-Sitter Body Field Names Vary by Language
**What goes wrong:** Assuming all languages use `body` as the field name for function bodies. Python uses `body` for functions but the node structure differs for classes. Rust uses `body` for fn but `body` field is absent on some declarations.
**Why it happens:** Each tree-sitter grammar has its own node type hierarchy and field names.
**How to avoid:** Per-language query files (go.scm, python.scm, etc.) that encode the correct node types and field names. Test each language's body extraction independently.
**Warning signs:** Body extraction works for Go but silently returns wrong ranges for Python.

### Pitfall 4: JSON-RPC ID Collision Between Sessions
**What goes wrong:** Two sessions sharing an LS worker send requests with overlapping integer IDs. Responses route to the wrong session.
**How to avoid:** Session-prefixed request IDs (e.g., `"sess-abc:42"`) or a per-LS routing table keyed by request ID with session callback.

### Pitfall 5: Document Version Tracking Across Shared Workers
**What goes wrong:** Two sessions open the same file, both send didChange with conflicting version numbers. LS enters undefined state.
**How to avoid:** The WorkspaceRuntime (not the session) owns the document version counter. All didChange notifications go through the workspace which assigns versions atomically.

### Pitfall 6: Memory Pressure API Differences Between Linux and macOS
**What goes wrong:** Code assumes Linux PSI is available on macOS, or vice versa.
**How to avoid:** Build-tagged files: `pressure_linux.go` and `pressure_darwin.go`. Use `//go:build linux` and `//go:build darwin` tags. Common interface `MemoryPressure` with platform-specific implementations.

## Code Examples

### LSP JSON-RPC Content-Length Codec

```go
// codec.go -- Content-Length framed JSON-RPC over stdio
package jsonrpc

import (
    "bufio"
    "encoding/json"
    "fmt"
    "io"
    "strconv"
    "strings"
)

func ReadMessage(r *bufio.Reader) (json.RawMessage, error) {
    var contentLen int
    for {
        line, err := r.ReadString('\n')
        if err != nil { return nil, err }
        line = strings.TrimSpace(line)
        if line == "" { break } // empty line separates headers from body
        if strings.HasPrefix(line, "Content-Length:") {
            val := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
            contentLen, err = strconv.Atoi(val)
            if err != nil { return nil, fmt.Errorf("invalid Content-Length: %w", err) }
        }
    }
    if contentLen == 0 { return nil, fmt.Errorf("missing Content-Length header") }
    body := make([]byte, contentLen)
    if _, err := io.ReadFull(r, body); err != nil { return nil, err }
    return json.RawMessage(body), nil
}

func WriteMessage(w io.Writer, msg json.RawMessage) error {
    header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(msg))
    if _, err := io.WriteString(w, header); err != nil { return err }
    _, err := w.Write(msg)
    return err
}
```

### Tree-Sitter Body Extraction (Go Example)

```go
// treesitter.go -- Extract function body using tree-sitter
package edit

import (
    tree_sitter "github.com/tree-sitter/go-tree-sitter"
    tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

func ExtractGoFunctionBody(source []byte, funcName string) (bodyStart, bodyEnd uint32, err error) {
    parser := tree_sitter.NewParser()
    defer parser.Close()

    lang := tree_sitter.NewLanguage(tree_sitter_go.Language())
    if err := parser.SetLanguage(lang); err != nil {
        return 0, 0, err
    }

    tree := parser.Parse(source, nil)
    defer tree.Close()

    root := tree.RootNode()
    // Walk children looking for function_declaration or method_declaration
    for i := uint32(0); i < root.ChildCount(); i++ {
        child := root.Child(i)
        if child.Kind() == "function_declaration" || child.Kind() == "method_declaration" {
            nameNode := child.ChildByFieldName("name")
            if nameNode != nil && string(source[nameNode.StartByte():nameNode.EndByte()]) == funcName {
                body := child.ChildByFieldName("body")
                if body != nil {
                    return body.StartByte(), body.EndByte(), nil
                }
            }
        }
    }
    return 0, 0, fmt.Errorf("function %q not found", funcName)
}
```

### Per-Language Body Field Mapping

| Language | Declaration Types | Body Field | Notes |
|----------|-------------------|------------|-------|
| Go | `function_declaration`, `method_declaration` | `body` (block) | Body is the `{ ... }` block |
| Python | `function_definition`, `class_definition` | `body` (block) | Indentation-based; body is everything after `:` |
| TypeScript | `function_declaration`, `method_definition`, `arrow_function` | `body` (statement_block or expression) | Arrow functions may have expression body |
| Rust | `function_item`, `impl_item` | `body` (block) | `impl_item` body contains methods |

### LSP Codegen: metaModel.json Structure

The metaModel.json has these top-level sections:
```json
{
  "metaData": { "version": "3.17.0" },
  "requests": [...],       // ~80 request/response pairs
  "notifications": [...],  // ~30 notification types  
  "structures": [...],     // ~200 named structs
  "enumerations": [...],   // ~40 enums
  "typeAliases": [...]     // ~20 type aliases
}
```

The generator must handle these type kinds: `base`, `reference`, `array`, `map`, `literal`, `stringLiteral`, `integerLiteral`, `booleanLiteral`, `tuple`, `and`, `or`. The `or` kind produces union types requiring tagged wrappers in Go.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| go.lsp.dev/protocol (LSP 3.15) | Generated from metaModel.json (LSP 3.17) | 2023 (gopls switched generators) | Full LSP 3.17 coverage including type hierarchy, inlay hints |
| smacker/go-tree-sitter (community) | tree-sitter/go-tree-sitter (official) | Feb 2025 | Official bindings with modular grammar loading |
| Manual LSP type definitions | gopls-pattern metamodel codegen | 2023 | Ecosystem consensus: gopls, mcp-language-server, opencode all generate |

**Deprecated/outdated:**
- `go.lsp.dev/protocol`: Stuck at LSP 3.15 (2022), 12 importers, effectively dead
- `sourcegraph/go-lsp`: Minimal subset, unmaintained
- `go.lsp.dev/jsonrpc2 v0.10.0`: Last updated 2022, Go 1.17 era. Functional but unmaintained.

## Open Questions

1. **Tree-sitter grammar completeness for body extraction**
   - What we know: Go, Python, TypeScript, Rust grammars all have `body` fields on function declarations
   - What's unclear: Edge cases for nested declarations, anonymous functions, decorators wrapping functions
   - Recommendation: Build comprehensive test suite per language during implementation. Start with Go (dog-food) and expand.

2. **go.lsp.dev/jsonrpc2 vs custom JSON-RPC**
   - What we know: jsonrpc2 v0.10.0 works but is unmaintained. gopls has its own internal jsonrpc2. Custom is ~300 lines.
   - What's unclear: Whether v0.10.0's Stream abstraction handles multiplexed daemon connections
   - Recommendation: Start with custom thin implementation. The Content-Length codec is simple; the routing table for multiplexed sessions is project-specific anyway.

3. **macOS memory pressure API availability**
   - What we know: `os_proc_available_memory()` is available on macOS 10.15+. `task_info` gives per-process RSS via mach_task_basic_info.
   - What's unclear: Whether Go's syscall package exposes these directly or if CGO is needed
   - Recommendation: Prototype the darwin pressure detection early. May need `unix.SysctlRaw("kern.memorystatus_level")` as alternative.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go compiler | Everything | Yes | go1.25.1 darwin/arm64 | -- |
| C compiler (CGO) | tree-sitter bindings | Yes | Apple clang 21.0.0 | -- |
| gopls | Go LS testing | Yes | v0.21.1 | -- |
| pyright | Python LS testing | No | -- | `npm install -g pyright` |
| typescript-language-server | TypeScript LS testing | Yes | (installed) | -- |
| rust-analyzer | Rust LS testing | Yes | (installed) | -- |

**Missing dependencies with no fallback:**
- None blocking

**Missing dependencies with fallback:**
- pyright: Not installed. Install via `npm install -g pyright` or `pip install pyright` before Python LS integration tests.

## Project Constraints (from CLAUDE.md)

- **Format:** `uv run poe format` (RUFF) -- only allowed formatting command (Python legacy only; Go code uses `gofmt`)
- **Type check:** `uv run poe type-check` (mypy) -- only for Python legacy
- **Test:** `uv run poe test` for Python; `go test ./...` for Go code
- **Run format, type-check, and test before completing any task** -- for Go this means `gofmt` + `go vet` + `go test`
- **Architecture:** Serena is a dual-layer coding agent toolkit with SerenaAgent, SolidLanguageServer, Tool System, Configuration System
- **Legacy reference:** `legacy/src/solidlsp/ls.py` for LS lifecycle, `legacy/src/serena/tools/symbol_tools.py` for tool behavior specification, `legacy/src/solidlsp/language_servers/` for per-language quirk handling

## Sources

### Primary (HIGH confidence)
- [gopls protocol generator](https://go.googlesource.com/tools/+/refs/heads/master/gopls/internal/protocol/generate/) -- Generator architecture, tables.go fixup pattern, 4-file output structure
- [LSP 3.17 metaModel.json](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/metaModel/metaModel.json) -- Official metamodel, ~200KB, 5 top-level sections
- [tree-sitter/go-tree-sitter](https://github.com/tree-sitter/go-tree-sitter) -- v0.25.0, official Go bindings, API documentation
- [tree-sitter/tree-sitter-go node-types.json](https://github.com/tree-sitter/tree-sitter-go/blob/master/src/node-types.json) -- Go grammar declaration node types and field names
- [mcp-language-server](https://github.com/isaacphi/mcp-language-server) -- Validated reference for gopls-derived LSP types in Go MCP server
- [bytebase/lsp-protocol](https://github.com/bytebase/lsp-protocol) -- Automated extraction of gopls protocol types, updated March 2025

### Secondary (MEDIUM confidence)
- [smacker/go-tree-sitter](https://github.com/smacker/go-tree-sitter) -- Community bindings comparison, 546 stars, 30+ bundled grammars
- [go.lsp.dev/jsonrpc2](https://pkg.go.dev/go.lsp.dev/jsonrpc2) -- v0.10.0, JSON-RPC 2.0 transport, last updated 2022
- [gopls daemon documentation](https://go.dev/gopls/daemon) -- Cache/Session/View/Snapshot hierarchy
- [golang/go#47061](https://github.com/golang/go/issues/47061) -- exec.Command stdin deadlock documentation

### Tertiary (LOW confidence)
- macOS `os_proc_available_memory` availability via Go syscall -- needs prototype validation
- Tree-sitter body extraction edge cases for decorators/nested functions -- needs per-language test suite

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all libraries verified with `go list -m`, versions confirmed against registry
- Architecture: HIGH -- patterns proven by gopls, mcp-language-server; package layout follows established conventions
- LSP codegen: HIGH -- gopls generator is well-documented with clear 8-file structure and 4-file output
- Tree-sitter integration: MEDIUM -- official bindings confirmed but body extraction patterns need per-language validation
- Pitfalls: HIGH -- documented from gopls experience, existing Python Serena codebase, and Go issue tracker

**Research date:** 2026-04-07
**Valid until:** 2026-05-07 (stable domain, tree-sitter and LSP spec move slowly)
