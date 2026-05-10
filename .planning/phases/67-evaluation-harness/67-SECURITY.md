# 67-SECURITY.md — Phase 67 Threat Model Verification

**Date:** 2026-05-10
**Phase:** 67 — evaluation-harness
**Threat model source:** 67-RESEARCH.md §Threat Model Seeds (lines 1149-1162)
**Auditor:** security audit pass, HEAD c2563c9d..cb7409d5 (post code-review fixes)
**Verdict:** PASS — 10 threats mitigated, both flags closed (see Flag Closure Log), 0 unmitigated blocking
**Flag closure update (2026-05-10):** FLAG-1 closed by `3aabbb7b` (PID-gate fail-safe — skip tap when daemonHandle is nil). FLAG-2 closed by `e46aecd4` (path-traversal task-ID validation in RunTask + BuildEvalPhases).

---

## Mitigation Verification Matrix

| # | Threat ID | Asset | STRIDE | Mitigation Claimed | Mitigation Found | Status |
|---|-----------|-------|--------|--------------------|------------------|--------|
| T1 | T-67-01 | Eval workspace tmpdirs | Tampering | `os.MkdirTemp` 0700; reject symlink-targeting tmpdir | `sandbox.go:40` MkdirTemp `/tmp`; `sandbox.go:53` `os.Chmod(root, 0700)`; `sandbox.go:59-62` `checkNotSymlink`; `sandbox.go:72-80` `os.Lstat` + ModeSymlink check | CLOSED |
| T2 | T-67-02 | Per-mode HOME dir | Tampering | Daemon reads config once at start; subsequent writes ignored | `config/loader.go:23-69` uses koanf `file.Provider` with no `.Watch()` call; `daemon.go` calls `Load()` once at startup (line 1045 context). No hot-reload goroutine present. | CLOSED |
| T3 | T-67-03 | `claude` stdout/stderr files | Info Disclosure | Files mode 0600; tmpdir 0700 | `claude.go:125` `os.OpenFile(stdoutPath, ..., 0600)`; `claude.go:131` `os.OpenFile(stderrPath, ..., 0600)`; parent dir created via `sandbox.Prepare` at `sandbox.go:117` `os.MkdirAll(d, 0700)` | CLOSED |
| T4 | T-67-03b | `daemon.log` | Info Disclosure | Files mode 0600; tmpdir 0700 | `sandbox.go:261` `os.OpenFile(logPath, ..., 0600)` | CLOSED |
| T5 | T-67-Pitfall-1 | `ANTHROPIC_API_KEY` env var | Info Disclosure | `cleanEnv` allowlist; `--bare` skips hooks | `claude.go:62-71` `cleanEnv`: allowlist is PATH, HOME (isolated), ANTHROPIC_API_KEY, HELIX_LOG_LEVEL. `buildArgv` passes `--bare` at `claude.go:82`. Daemon env allowlist at `sandbox.go:247-254`: PATH, HOME (isolated), HELIX_LOG_LEVEL only (no ANTHROPIC_API_KEY leak to daemon subprocess). | CLOSED |
| T6 | T-67-06 | Eval corpus ZDR gate | Info Disclosure | `HELIX_EVAL_ZDR_VERIFIED=1` attestation gate; allowlist on `source:` values | `zdr_gate.go:86-91` blocks external tasks without env-var; `zdr_gate.go:122-127` allowlist rejects unknown source values (CR-04 fix at commit `b06556b1`) | CLOSED |
| T7 | T-67-MCP | MCP config file `mcp-config.json` | Tampering | File 0600; absolute helixBin path; tmpdir 0700 | `mcpconfig.go:33-34` rejects non-absolute helixBin; `mcpconfig.go:53` `os.WriteFile(path, data, 0600)`; file lives under 0700 modeDir | CLOSED |
| T8 | T-67-Spoofing | Daemon Unix socket | Spoofing | Per-mode tmpdir 0700; fail if socket exists | Socket path is `<sandbox.Root>/<task>/<mode>/daemon.sock`; `sandbox.Root` is freshly created 0700 tmpdir via `MkdirTemp` — no other user can create files inside it. Daemon's own `socket.go:14-28` `ensureSocket` additionally checks for live daemon before allowing socket re-use. Pre-existence check is layered: (a) 0700 dir makes pre-planting impossible, (b) `ensureSocket` handles stale-socket cleanup. | CLOSED |
| T9 | T-67-05 | LLM judge prompt | Tampering | Judge sees merged trace + task title only; rubric is source-controlled | `judge.go:103-118` `BuildPrompt` uses `inp.TaskDescription` (title only, comment "TITLE ONLY — bias mitigation m4") and `inp.Trace.Events`, NOT raw `task.md` body. Rubric at `client.go:21` `//go:embed prompts/rubric.md` — compiled into binary. | CLOSED |
| T10 | T-67-04 | Daemon PID gate | Tampering/Spoofing | TapDaemonLog gated by PID to reject foreign-process log lines | `tap.go:40` `TapDaemonLog(path, expectedPid)`; `tap.go:69-73` rejects lines where `raw.Pid != expectedPid`. `DaemonHandle.Pid()` accessor at `sandbox.go:185-190`. | CLOSED (see FLAG-1) |
| T11 | T-67-02b | Path-prefix invariant (PatchPaths) | Tampering | Paths outside RepoRoot cause trace failure | `merge.go:118-135`: `filepath.Rel(RepoRoot, abs)` with `strings.HasPrefix(rel, "..")` check; tested by `TestMergePathPrefixInvariant` (pass confirmed) | CLOSED |
| T12 | T-67-EoP | `verify.sh` execution | Elevation of Privilege | verify.sh runs in cloned tmpdir, not user HOME; CI does not pull external corpora | `runner.go:194-195` `runVerify` with `cmd.Dir = repoDir` (sandbox clone). Declared out-of-scope (no container isolation). | ACCEPTED (documented out-of-scope in 67-RESEARCH.md:1170-1172) |

---

## Findings

### BLOCKING — Unmitigated Threats

None.

---

### FLAG-1 (WARNING) — PID Gate Is Wired but Effectively Inactive (Wave-2 Gap)

**Threat:** T-67-04 — daemon log injection from a foreign process.

**Evidence:** `runner.go:152` declares `var daemonHandle *sandbox.DaemonHandle` (nil). `runner.go:156` calls `trace.TapDaemonLog(daemonLogPath, daemonHandle.Pid())`. `DaemonHandle.Pid()` is nil-safe and returns `0` when the handle is nil (`sandbox.go:186-189`). `TapDaemonLog` will therefore only accept log lines whose `pid` JSON field equals `0`. Real daemon processes have non-zero PIDs, so all real daemon log lines are rejected — the gate is vacuously "secure" but functionally broken.

**Impact:** In wave-1 (current), the daemon is not actually spawned by `RunTask` (no `StartDaemon` call), so `daemonLogPath` does not exist and `TapDaemonLog` is never reached. The threat is dormant, not active. When wave-2 wires real subprocess orchestration, the TODO at `runner.go:151` must populate `daemonHandle` from the real `StartDaemon` call, or all daemon log telemetry will be silently dropped and the PID gate bypass risk reactivates.

**Action required before wave-2 ships:** Resolve the `TODO(wave-2)` in `runner.go:151`. Assign the `*sandbox.DaemonHandle` returned by `StartDaemon` to `daemonHandle`.

---

### FLAG-2 (WARNING) — Report Path Traversal via Corpus TaskID Not Sanitized

**Threat:** Unregistered — not in 67-RESEARCH.md threat model.

**Evidence:** `runner.go:58` `taskOutDir` builds: `filepath.Join(r.cfg.OutDir, r.cfg.RunID, "tasks", taskID, mode)`. `runner.go:82` builds `filepath.Join(r.cfg.CorpusDir, ts.ID)`. Neither `taskID` nor `mode` is validated against path-traversal characters (e.g., `../../etc`). A malicious task directory named `../../` in `corpusDir` would cause output artifacts to land outside `OutDir`. The corpus is PR-reviewed (trusted), but the threat model does not document this trust dependency.

**Impact:** Low — corpus is treated as trusted (PR-reviewed per 67-RESEARCH.md:1156). No privilege boundary crossed. Not a blocker.

**Recommendation:** Document the trust assumption that `corpusDir` task IDs are PR-reviewed and must not contain path separator characters. Optionally add `filepath.Clean` + prefix check on `taskID` in `taskOutDir`.

---

### Notes & Defense-in-Depth Observations

- **API key never logged:** `client.go:55` comment "Never logged"; log calls at `client.go:171-175` use `error` field only, not key field. Confirmed.
- **TLS:** `client.go:97` uses `&http.Client{}` with default transport — Go's default transport enforces system CA verification. No custom `InsecureSkipVerify`. CLOSED.
- **EVAL-07 structural enforcement:** `judge.go:138` signature `Run(...) Output` (no error return). Judge failures structurally cannot propagate to `helix-eval` exit code.
- **Budget watchdog:** `runner.go:131-137` checks `ctx.Err()` before agent phase. Context cancellation propagates to subprocess via `exec.CommandContext`. No goroutine leak path observed; `cmd.Wait()` always called.
- **Zombie reaping (CR-02):** `sandbox.go:198-201` handles `ErrProcessDone` and calls `cmd.Wait()`. CLOSED.
- **SIGKILL to process group on timeout (WR-03):** `sandbox.go:216` `syscall.Kill(-pid, SIGKILL)`. CLOSED.
- **Log fd lifecycle (CR-01):** `sandbox.go:262-265` opens log file; `defer logFile.Close()` is inside `StartDaemon`, so file stays open until `waitSocket` returns. CLOSED.
- **Concurrent run isolation:** Each `RunMatrix` iteration creates a fresh `sandbox.NewSandbox` with run+task namespaced ID at `runner.go:286`. No shared mutable state between (task, mode) goroutines outside `mu`-protected `results` slice.

---

## Out-of-Model Threats Discovered

| # | Threat | Asset | Finding | Severity |
|---|--------|-------|---------|----------|
| OT-1 | Path traversal via task ID | Report output directory | TaskID used as path component; **closed by `e46aecd4`** — `validateTaskID` in runner.go + inline guard in pipeline.go reject IDs with path separators, parent refs, or leading dots | CLOSED |
| OT-2 | ZDR bypass via unknown `source:` value | Corpus source.yaml | Pre-CR-04: any unknown source value silently defaulted to `synthetic`. Post-CR-04 (`b06556b1`): `readTaskSource` allowlist rejects unknown values with error. Now CLOSED. | Closed by CR-04 |

---

## Test Coverage of Mitigated Threats

All relevant tests passed on HEAD:

| Test | Package | Covers |
|------|---------|--------|
| `TestSandboxIsolation` | `internal/eval/sandbox` | T-67-01 tmpdir isolation |
| `TestSandboxRefusesSymlinkedTmpdir` | `internal/eval/sandbox` | T-67-01 symlink rejection |
| `TestZDRGate_NonSyntheticBlocked` | `internal/eval/runner` | T-67-06 ZDR gate enforcement |
| `TestZDRGate_NonSyntheticAllowedWithEnvVar` | `internal/eval/runner` | T-67-06 attestation bypass |
| `TestZDRGate_HelixOSSAllowed` | `internal/eval/runner` | T-67-06 OSS corpus exemption |
| `TestTapDaemonLog_PidGate` | `internal/eval/trace` | T-67-04 PID gate (unit-level) |
| `TestMergePathPrefixInvariant` | `internal/eval/trace` | T-67-02b PatchPath invariant |

```
ok  internal/eval/sandbox   0.394s
ok  internal/eval/runner    2.550s
ok  internal/eval/trace     0.208s
ok  internal/eval/judge     0.476s
```
