---
phase: 02-code-intelligence-kernel
plan: 03
subsystem: kernel
tags: [lsp, worker-pool, circuit-breaker, concurrency, ttl, pressure-eviction, language-server]

requires:
  - phase: 02-01
    provides: "JSON-RPC codec and LSP protocol types for LS communication"
  - phase: 01-02
    provides: "Daemon lifecycle, workspace registry, config system"
provides:
  - "LS worker pool with share-until-dirty policy and adaptive TTL"
  - "ProcessHandle for safe child process pipe I/O"
  - "Worker 5-state machine with LSP initialize handshake"
  - "CircuitBreaker with exponential backoff"
  - "LSAdapter typed LSP methods with capability checking"
  - "WorkerLease with mutation serialization and parallel reads"
  - "Platform-specific memory pressure detection (Linux PSI, macOS vm_stat)"
  - "WorkspaceRuntime with language auto-detection"
  - "Kernel coordinating multiple workspaces"
  - "WorkerPoolConfig in SerenaConfig"
affects: [02-04, 02-05, 02-06]

tech-stack:
  added: []
  patterns: [share-until-dirty worker sharing, 5-state worker machine, adaptive TTL with decaying reuse score, circuit breaker with exponential backoff, RWMutex mutation serialization, platform-specific build tags]

key-files:
  created:
    - internal/kernel/lspool/process.go
    - internal/kernel/lspool/worker.go
    - internal/kernel/lspool/circuit.go
    - internal/kernel/lspool/adapter.go
    - internal/kernel/lspool/quirks.go
    - internal/kernel/lspool/util.go
    - internal/kernel/lspool/pool.go
    - internal/kernel/lspool/lease.go
    - internal/kernel/lspool/pressure.go
    - internal/kernel/lspool/pressure_linux.go
    - internal/kernel/lspool/pressure_darwin.go
    - internal/kernel/lspool/pool_test.go
    - internal/kernel/lspool/testutil_test.go
    - internal/kernel/workspace.go
    - internal/kernel/kernel.go
  modified:
    - internal/config/config.go

key-decisions:
  - "Used interface-based MemoryPressure with platform build tags for Linux/macOS"
  - "Worker metrics use pointer return to avoid sync.Mutex copy"
  - "CGO-free macOS pressure detection via vm_stat and ps commands"
  - "Pool releases lock during worker start to avoid blocking other operations"

patterns-established:
  - "5-state worker lifecycle: Starting -> Initializing -> Ready -> ShuttingDown -> Stopped"
  - "Share-until-dirty: clean sessions share warm workers, dirty sessions get dedicated workers"
  - "Adaptive TTL with decaying reuse score: score * exp(-elapsed/300) + 1"
  - "Mutation serialization via RWMutex on WorkerLease"
  - "Platform build tags for OS-specific memory pressure detection"

requirements-completed: [DMN-07, DMN-08, DMN-09, DMN-10, DMN-11, WRK-02, WRK-03]

duration: 11min
completed: 2026-04-07
---

# Phase 2 Plan 3: LS Worker Pool Summary

**Multi-LS worker pool with share-until-dirty policy, adaptive TTL, circuit breaking, pressure eviction, and workspace-scoped language detection**

## Performance

- **Duration:** 11 min
- **Started:** 2026-04-07T20:15:50Z
- **Completed:** 2026-04-07T20:26:58Z
- **Tasks:** 2
- **Files modified:** 16

## Accomplishments
- ProcessHandle with safe pipe I/O goroutine separation (separate goroutines for reading, writing, waiting per Pitfall 1)
- Worker 5-state machine with LSP initialize handshake, didOpen buffering during Initializing, and capability storage
- CircuitBreaker with exponential backoff that never fully breaks (per D-06)
- LSAdapter providing 10 typed LSP methods (Definition, References, DocumentSymbol, Hover, Implementation, TypeDefinition, WorkspaceSymbol, Formatting, Rename, Initialize) with capability checking
- Language quirks registry for gopls, pyright, typescript-language-server, rust-analyzer (per D-16)
- Pool with share-until-dirty policy, adaptive TTL with decaying reuse score, and PromoteToDirty for buffer promotion
- WorkerLease with RWMutex for mutation serialization and parallel reads (per D-11)
- Platform-specific memory pressure: Linux PSI (/proc/pressure/memory), macOS vm_stat/ps (CGO-free)
- WorkspaceRuntime detecting Go, Python, TypeScript, Rust from marker files (per WRK-02)
- Kernel coordinating multiple workspaces with pool lifecycle (per WRK-03)
- WorkerPoolConfig added to SerenaConfig with BaseTTL, CeilingTTL, MaxWorkers, RSSHardCapMB, PressureCheckInterval
- 15 unit tests covering circuit breaker, TTL computation, concurrency, pool lifecycle

## Task Commits

Each task was committed atomically:

1. **Task 1: LS worker process management and state machine** - `eddb6f4e` (feat)
2. **Task 2: Worker pool, workspace runtime, kernel interface, and concurrency control** - `e41a1e52` (feat)

## Files Created/Modified
- `internal/kernel/lspool/process.go` - ProcessHandle with safe pipe I/O, SIGTERM->SIGKILL escalation, stderr ring buffer
- `internal/kernel/lspool/worker.go` - Worker 5-state machine, LSP initialize handshake, didOpen buffering, adaptive metrics
- `internal/kernel/lspool/circuit.go` - CircuitBreaker with exponential backoff, never fully breaks
- `internal/kernel/lspool/adapter.go` - LSAdapter with 10 typed LSP methods and capability checking
- `internal/kernel/lspool/quirks.go` - Language quirks for gopls, pyright, typescript-language-server, rust-analyzer
- `internal/kernel/lspool/util.go` - Process ID helper
- `internal/kernel/lspool/pool.go` - Pool manager with TTL checks, pressure eviction, share-until-dirty
- `internal/kernel/lspool/lease.go` - WorkerLease with RWMutex mutation serialization
- `internal/kernel/lspool/pressure.go` - MemoryPressure interface and PressureLevel enum
- `internal/kernel/lspool/pressure_linux.go` - Linux PSI memory pressure detection
- `internal/kernel/lspool/pressure_darwin.go` - macOS vm_stat/ps pressure detection (CGO-free)
- `internal/kernel/lspool/pool_test.go` - 15 unit tests for circuit breaker, TTL, pool, concurrency
- `internal/kernel/lspool/testutil_test.go` - Test logger helper
- `internal/kernel/workspace.go` - WorkspaceRuntime with language detection and doc versioning
- `internal/kernel/kernel.go` - Kernel coordinating workspaces and pool lifecycle
- `internal/config/config.go` - Added WorkerPoolConfig to SerenaConfig

## Decisions Made
- Used interface-based MemoryPressure with platform build tags (linux/darwin) for clean OS abstraction
- Worker Metrics() returns pointer to avoid copying sync.Mutex (caught by go vet)
- CGO-free macOS pressure detection via vm_stat command parsing and ps for RSS, avoiding CGO dependency
- Pool temporarily releases its lock during worker start (which may block on LSP initialize) to avoid holding the lock

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed WorkerMetrics returning copy with sync.Mutex**
- **Found during:** Task 1 (worker.go)
- **Issue:** go vet flagged `Metrics() WorkerMetrics` as copying lock value
- **Fix:** Changed to return `*WorkerMetrics` pointer
- **Files modified:** internal/kernel/lspool/worker.go
- **Verification:** go vet passes
- **Committed in:** eddb6f4e (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Minor correctness fix required by Go's vet tool. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Worker pool ready for symbol retrieval tools (Plan 04) to acquire leases and send LSP requests
- Kernel provides ActivateWorkspace and GetRuntime for tool dispatch
- LSAdapter provides typed methods for all symbol operations
- Platform pressure detection ready for production memory management

---
*Phase: 02-code-intelligence-kernel*
*Completed: 2026-04-07*
