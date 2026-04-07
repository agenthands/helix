# Phase 1: Foundation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-07
**Phase:** 01-foundation
**Areas discussed:** CLI & invocation, Legacy migration, IPC protocol, Project config, Go module layout, Socket location, Logging & observability, Error model

---

## CLI & Invocation

| Option | Description | Selected |
|--------|-------------|----------|
| Auto-start from forwarder | Forwarder auto-launches daemon if not running (gopls pattern) | ✓ |
| Explicit daemon command | `serena daemon start/stop/status` | |
| Both modes | Auto-start by default, explicit commands for power users | |

**User's choice:** Auto-start from forwarder
**Notes:** Zero manual daemon management — gopls pattern

| Option | Description | Selected |
|--------|-------------|----------|
| Subcommand style | `serena serve`, `serena daemon`, `serena init` | |
| Flat with flags | `serena --mode=stdio`, `serena --mode=http` | ✓ |

**User's choice:** Flat with flags

| Option | Description | Selected |
|--------|-------------|----------|
| serena | Same name as current Python CLI | ✓ |
| serena-go | Distinguish from Python version | |

**User's choice:** serena

---

## Legacy Migration

| Option | Description | Selected |
|--------|-------------|----------|
| Everything Python | src/, test/, scripts/, pyproject.toml, docs/ — all into legacy/ | ✓ |
| Source only | Only src/ and test/ move | |

**User's choice:** All Python-related code moves to legacy/ as reference
**Notes:** "We need to move all python related code to legacy folder and use as reference"

| Option | Description | Selected |
|--------|-------------|----------|
| Preserve in legacy/ | CI can still run Python tests from legacy/ | ✓ |
| Drop Python CI | Go tests only going forward | |

**User's choice:** Preserve in legacy/

| Option | Description | Selected |
|--------|-------------|----------|
| Keep .serena/ | Brand continuity | ✓ |
| New name | Fresh start | |

**User's choice:** Keep .serena/

---

## IPC Protocol

| Option | Description | Selected |
|--------|-------------|----------|
| Raw MCP passthrough | Relay MCP JSON-RPC bytes directly | |
| gRPC internal | Internal gRPC between forwarder and daemon | ✓ |
| Custom framing | Length-prefixed JSON | |

**User's choice:** gRPC internal

---

## Project Config

| Option | Description | Selected |
|--------|-------------|----------|
| YAML | Current Serena uses YAML | ✓ |
| TOML | Go ecosystem standard | |
| JSON | Universal | |

**User's choice:** YAML

| Option | Description | Selected |
|--------|-------------|----------|
| Minimal: project root + languages | Just enough to bootstrap | |
| Full: matching current Serena | Contexts, modes, tool overrides, memory config, LS-specific settings | ✓ |

**User's choice:** Full config matching current Serena

---

## Go Module Layout

| Option | Description | Selected |
|--------|-------------|----------|
| Standard Go layout | cmd/, internal/, pkg/ | ✓ |
| Domain-driven | Organized by layer: daemon/, kernel/, skills/ | |

**User's choice:** Standard Go layout

| Option | Description | Selected |
|--------|-------------|----------|
| api/proto/ | Top-level (Google convention) | ✓ |
| internal/proto/ | Private internal IPC only | |

**User's choice:** api/proto/

---

## Socket Location

| Option | Description | Selected |
|--------|-------------|----------|
| XDG runtime dir | $XDG_RUNTIME_DIR/serena/daemon.sock | |
| User home | ~/.serena/daemon.sock | |
| System temp | /tmp/serena-$UID/daemon.sock | ✓ |

**User's choice:** System temp (gopls-similar pattern)

| Option | Description | Selected |
|--------|-------------|----------|
| Named pipes | Windows named pipes as Unix socket equivalent | ✓ |
| TCP localhost | Fall back to localhost TCP on Windows | |
| Skip Windows v1 | macOS + Linux only | |

**User's choice:** Named pipes (full Windows support)

---

## Logging & Observability

| Option | Description | Selected |
|--------|-------------|----------|
| Structured JSON | Machine-parseable (slog JSON handler) | |
| Human-readable | Text format for terminal | |
| Both (configurable) | Default text, --json for structured | ✓ |

**User's choice:** Both configurable

| Option | Description | Selected |
|--------|-------------|----------|
| stderr + file | stderr for forwarder, rotated file for daemon | ✓ |
| stderr only | Let systemd/launchd handle logs | |

**User's choice:** stderr + file

---

## Error Model

| Option | Description | Selected |
|--------|-------------|----------|
| MCP error codes + detail | Structured JSON detail (cause, suggestion) | ✓ |
| Simple error strings | MCP error code + message string only | |

**User's choice:** MCP error codes with structured detail

| Option | Description | Selected |
|--------|-------------|----------|
| Sentinel errors + wrapping | errors.Is/As with domain-specific sentinels | ✓ |
| Error types (structs) | Custom error structs with fields | |

**User's choice:** Sentinel errors + wrapping

---

## Claude's Discretion

No areas deferred to Claude's discretion.

## Deferred Ideas

None — discussion stayed within phase scope.
