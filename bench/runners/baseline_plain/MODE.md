---
mode: baseline_plain
profile: baseline
---

# baseline_plain

The zero-Helix-tools control arm. Resolves to the `baseline` profile
(internal/profile/profiles/baseline.yaml) — NOT a new `bench-*` YAML (D-01,
ABLATE-03). This arm reuses the existing eval-harness control profile so the
five-of-six ablation matrix has a true "no Helix code intelligence" floor to
measure every other arm against.

The Helix tool inventory exposed to the agent in this mode is **empty**: the
agent sees only the shell / grep / read / edit / test capabilities its own
runtime exposes natively. The empty inventory is enforced by the profile
filter (the `baseline` profile's empty tool lists), not by any bench-side
code. The daemon still spawns for trace symmetry — every arm produces a
two-leg merged trace, so baseline_plain's run shape matches the other arms
even though it offers no Helix tools.
