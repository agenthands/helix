# Phase 51.1 — Deferred Items (Out-of-Scope Discoveries)

## DEF-51.1-OBS-01 — `process_resident_memory_bytes` metric missing on macOS hosts

**Discovered during:** Task 2 verification (`CGO_ENABLED=0 go test ./...`).

**Symptom:**
- `internal/obs.TestMetrics_RegisteredFamilies` fails: `registry missing metric family "process_resident_memory_bytes"`.
- `internal/daemon.TestTelemetryMetricsHTTPExposes...` fails the same way.

**Root cause (pre-existing, NOT introduced by this phase):**
`prometheus/client_golang`'s `collectors.NewProcessCollector` relies on procfs which is unavailable on macOS. Reproduced on `main` at the wave-base commit (`dab91d30`) with `CGO_ENABLED=0 go test ./internal/obs/` — same failure. CGO_ENABLED=1 happens to register the metric via different code paths in the prometheus library on macOS, masking the issue locally during CGO=1 runs.

**Why deferred:** Out-of-scope per executor scope rules — pre-existing failure unrelated to tree-sitter gating. The metric assertion was already broken on macOS before phase 51.1.

**Suggested fix (future phase):** Either gate the assertion behind `runtime.GOOS == "linux"`, or drop `process_resident_memory_bytes` from the `want` list and rely solely on `go_goroutines` for runtime-collector presence.

**Status:** OPEN
