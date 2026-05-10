// Package eval is the top-level runner library for the Phase 67 evaluation
// harness. It re-exports the phasegraph EvalPhases declaration so later waves
// can import from this package and fill in real Run bodies without touching
// the phasegraph/pipelines package directly.
package eval

import (
	"github.com/agenthands/helix/internal/phasegraph"
	"github.com/agenthands/helix/internal/phasegraph/pipelines"
)

// Phases is the canonical 10-phase eval pipeline DAG declared in
// internal/phasegraph/pipelines/eval.go. Wave 1+ plans replace the noopRun
// placeholders with real implementations per the DAG-02 contract: fill
// noopRun bodies, do not invent a parallel orchestrator.
var Phases []phasegraph.PhaseSpec = pipelines.EvalPhases
