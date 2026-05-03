package phasegraph

import (
	"fmt"
	"io"
	"sort"
)

// WriteDOT emits a Graphviz `digraph` describing `phases`: one node per phase,
// one directed edge per Requires entry (drawn from dependent → dependency).
// Output is deterministic — nodes and edges are sorted by PhaseID.
//
// SPEC §39.9 describes the artifact convention
// (`.helix/debug/phasegraph-*.dot`); the path is the caller's choice — this
// function only writes to the supplied [io.Writer].
//
// Threat note (T-57-01-03): output contains only phase IDs and edges, both
// caller-defined static constants. No file paths, env vars, or runtime data.
func WriteDOT(w io.Writer, phases []PhaseSpec) error {
	// Sort phases by ID for deterministic node lines.
	ids := make([]PhaseID, len(phases))
	byID := make(map[PhaseID]PhaseSpec, len(phases))
	for i, p := range phases {
		ids[i] = p.ID
		byID[p.ID] = p
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	if _, err := fmt.Fprintln(w, "digraph phasegraph {"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "  rankdir=LR;"); err != nil {
		return err
	}
	// Nodes.
	for _, id := range ids {
		if _, err := fmt.Fprintf(w, "  %q;\n", string(id)); err != nil {
			return err
		}
	}
	// Edges (dependent → dependency), Requires sorted within each node.
	for _, id := range ids {
		p := byID[id]
		deps := append([]PhaseID(nil), p.Requires...)
		sort.Slice(deps, func(i, j int) bool { return deps[i] < deps[j] })
		for _, dep := range deps {
			if _, err := fmt.Fprintf(w, "  %q -> %q;\n", string(p.ID), string(dep)); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintln(w, "}"); err != nil {
		return err
	}
	return nil
}
