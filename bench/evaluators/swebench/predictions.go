package swebench

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

// Prediction is one row of the SWE-bench harness predictions.jsonl input
// contract (CITED: github.com/SWE-bench/SWE-bench docs/guides/evaluation.md): a
// single JSON object per line carrying the agent's patch for one instance. The
// json tags are the EXACT upstream snake_case keys the harness unmarshals
// (instance_id / model_name_or_path / model_patch); field ORDER here is the
// emitted key order (encoding/json marshals struct fields in declaration order),
// so it is part of the byte-stable golden contract — do not reorder.
type Prediction struct {
	InstanceID      string `json:"instance_id"`
	ModelNameOrPath string `json:"model_name_or_path"`
	ModelPatch      string `json:"model_patch"`
}

// WritePredictions emits rows as the harness predictions.jsonl: one compact JSON
// object per line, terminated by '\n'. It is a PURE, DETERMINISTIC transform —
// the load-bearing reproducibility property (Pitfall 4 / SC#3): the rows are
// SORTED by InstanceID before emit, so two calls over the same set in any input
// order produce byte-identical output. An empty/nil slice emits zero bytes (no
// trailing-newline ambiguity). HTML escaping is disabled so patch bytes (which
// routinely contain <, >, &) survive verbatim into the file the Python harness
// reads back.
//
// WritePredictions copies rows before sorting so it never mutates the caller's
// slice order (a sort-in-place would be a surprising side effect on shared data).
func WritePredictions(w io.Writer, rows []Prediction) error {
	if len(rows) == 0 {
		return nil
	}

	sorted := make([]Prediction, len(rows))
	copy(sorted, rows)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].InstanceID < sorted[j].InstanceID })

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for _, r := range sorted {
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("bench/evaluators/swebench: encode prediction %q: %w", r.InstanceID, err)
		}
	}
	return nil
}
