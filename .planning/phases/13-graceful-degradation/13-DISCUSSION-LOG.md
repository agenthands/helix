# Phase 13: Graceful Degradation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-10
**Phase:** 13-graceful-degradation
**Areas discussed:** Timeout budgets, Circuit breaker tuning, Error surface design, Memory pressure + shutdown

---

## Timeout Budgets

### Q1: Where should timeout deadlines originate?

| Option | Description | Selected |
|--------|-------------|----------|
| Daemon middleware | TelemetryMiddleware wraps context.WithTimeout per tool class BEFORE calling the kernel handler. Single enforcement point. | ✓ |
| Forwarder propagation | Forwarder sets deadline, propagates via gRPC metadata. More complex but allows client-side control. | |
| Kernel-level only | Each kernel tool handler sets its own timeout internally. Simpler but scattered enforcement. | |

**User's choice:** Daemon middleware
**Notes:** Single enforcement point preferred.

### Q2: Should per-class timeout budgets be configurable?

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, with defaults | Ship hardcoded defaults but allow project.yml overrides for slow LS environments. | ✓ |
| Hardcoded only | No config surface. Budgets are compile-time constants. | |
| Fully configurable per-tool | Each of 28 tools gets its own timeout. Maximum flexibility but config explosion. | |

**User's choice:** Yes, with defaults

---

## Circuit Breaker Tuning

### Q3: How should the backoff jitter work?

| Option | Description | Selected |
|--------|-------------|----------|
| Decorrelated jitter | sleep = min(cap, random_between(base, sleep*3)). AWS pattern. Prevents thundering herd. | ✓ |
| Full jitter | sleep = random_between(0, min(cap, base*2^attempt)). Wider spread. | |
| Equal jitter | Guaranteed minimum wait with random spread. Compromise. | |

**User's choice:** Decorrelated jitter

### Q4: What restart budget for crashy LS?

| Option | Description | Selected |
|--------|-------------|----------|
| 3 restarts then hold | After 3 crashes, circuit stays open until TTL or manual intervention. | ✓ |
| 5 restarts with cooldown | More lenient, gives LS more chances. | |
| Unlimited with backoff | Never fully give up. Current behavior extended. | |

**User's choice:** 3 restarts then hold

---

## Error Surface Design

### Q5: What should ErrCircuitOpen carry?

| Option | Description | Selected |
|--------|-------------|----------|
| Language + backoff + failures | Structured info with RetryAfter hint. Client gets actionable data. | ✓ |
| Language + message only | Minimal. Client retries on own schedule. | |
| Full diagnostic envelope | Maximum debugging info but potentially leaks LS internals. | |

**User's choice:** Language + backoff + failures

---

## Memory Pressure + Shutdown

### Q6: How should GOMEMLIMIT interact with the worker pool?

| Option | Description | Selected |
|--------|-------------|----------|
| Config-driven GOMEMLIMIT + existing pressure eviction | Two layers: Go GC budget + pool eviction operate independently. | ✓ |
| GOMEMLIMIT only | Trust Go runtime. Remove platform-specific checks. | |
| No GOMEMLIMIT, enhance eviction | Don't set GOMEMLIMIT. Improve existing pressure-based eviction. | |

**User's choice:** Config-driven GOMEMLIMIT + existing pressure eviction

### Q7: How should SIGTERM drain work?

| Option | Description | Selected |
|--------|-------------|----------|
| Context cancel + configurable drain timeout | SIGTERM cancels errgroup ctx. ShutdownTimeout bounds drain. | ✓ |
| Immediate kernel shutdown | Force kill workers. Fastest but responses lost. | |
| Two-stage: soft then hard | First SIGTERM soft, second hard kill. | |

**User's choice:** Context cancel + configurable drain timeout

---

## Claude's Discretion

- Tool-to-class mapping assignments
- Specific jitter algorithm parameters
- Config key naming
- Integration test design

## Deferred Ideas

None.
