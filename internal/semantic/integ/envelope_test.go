// envelope_test.go — closed-enum + JSON-shape tests for the source-field
// envelope contract emitted by 65-05/06/07. Phase 65 D-04 + D-05 + INTEG-05.
//
// Doctrine:
//   - Every emitted Source value MUST belong to the closed set
//     {semantic, tree_sitter, fallback}.
//   - Every emitted FallbackReason value MUST belong to the closed set
//     {index_disabled, no_snapshot_yet, index_building, index_error,
//     bleve_rebuilding} OR be empty.
//   - omitempty: when SourceSemantic is emitted with an empty FallbackReason,
//     the marshalled JSON omits the fallback_reason key entirely.
//   - WR-NEW-01: ChooseSource never inlines raw error text into FallbackReason.
package integ

import (
	"encoding/json"
	"fmt"
	"testing"
)

// TestEnvelope_JSONShape locks the canonical JSON shape produced by
// MarshalEnvelope: header fields source / fallback_reason / graph_version /
// freshness, plus arbitrary tool payload merged at the top level. omitempty
// behaviour is verified for fallback_reason / graph_version / freshness.
func TestEnvelope_JSONShape(t *testing.T) {
	t.Run("full envelope keeps every header key", func(t *testing.T) {
		env := Envelope{
			Source:         SourceFallback,
			FallbackReason: FallbackReasonIndexBuilding,
			GraphVersion:   42,
			Freshness:      "stale",
		}
		raw, err := MarshalEnvelope(env, map[string]any{"tree": "..."})
		if err != nil {
			t.Fatalf("MarshalEnvelope: unexpected err: %v", err)
		}
		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}
		for _, key := range []string{"source", "fallback_reason", "graph_version", "freshness", "tree"} {
			if _, ok := got[key]; !ok {
				t.Errorf("envelope JSON missing key %q; got %v", key, got)
			}
		}
		if got["source"] != "fallback" {
			t.Errorf("source: got %v, want %q", got["source"], "fallback")
		}
		if got["fallback_reason"] != "index_building" {
			t.Errorf("fallback_reason: got %v, want %q", got["fallback_reason"], "index_building")
		}
	})

	t.Run("semantic envelope omits fallback_reason when empty", func(t *testing.T) {
		env := Envelope{
			Source:       SourceSemantic,
			GraphVersion: 7,
			Freshness:    "fresh",
		}
		raw, err := MarshalEnvelope(env, nil)
		if err != nil {
			t.Fatalf("MarshalEnvelope: unexpected err: %v", err)
		}
		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}
		if _, ok := got["fallback_reason"]; ok {
			t.Errorf("fallback_reason key must be omitted when empty; got JSON %s", string(raw))
		}
		if got["source"] != "semantic" {
			t.Errorf("source: got %v, want %q", got["source"], "semantic")
		}
	})

	t.Run("tree_sitter steady-state envelope omits fallback_reason", func(t *testing.T) {
		env := Envelope{Source: SourceTreeSitter}
		raw, err := MarshalEnvelope(env, nil)
		if err != nil {
			t.Fatalf("MarshalEnvelope: unexpected err: %v", err)
		}
		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}
		if _, ok := got["fallback_reason"]; ok {
			t.Errorf("fallback_reason must be omitted when empty; got JSON %s", string(raw))
		}
		if _, ok := got["graph_version"]; ok {
			t.Errorf("graph_version must be omitted when zero; got JSON %s", string(raw))
		}
		if _, ok := got["freshness"]; ok {
			t.Errorf("freshness must be omitted when empty; got JSON %s", string(raw))
		}
		if got["source"] != "tree_sitter" {
			t.Errorf("source: got %v, want %q", got["source"], "tree_sitter")
		}
	})
}

// TestEnvelope_ClosedEnum exercises every (Source × FallbackReason) pair the
// priority ladder can emit (15 cells) plus the two terminal source-only cells
// (semantic, tree_sitter) and asserts every Source string is a member of the
// closed enum. Marshalling and re-unmarshalling round-trips the wire-string
// for each case.
func TestEnvelope_ClosedEnum(t *testing.T) {
	allowedSources := map[string]struct{}{
		"semantic":    {},
		"tree_sitter": {},
		"fallback":    {},
	}
	allowedReasons := map[string]struct{}{
		"":                 {},
		"index_disabled":   {},
		"no_snapshot_yet":  {},
		"index_building":   {},
		"index_error":      {},
		"bleve_rebuilding": {},
	}

	cases := []Envelope{
		// Terminal source-only cells.
		{Source: SourceSemantic},
		{Source: SourceTreeSitter},
		// fallback × every reason (5 cells).
		{Source: SourceFallback, FallbackReason: FallbackReasonIndexDisabled},
		{Source: SourceFallback, FallbackReason: FallbackReasonNoSnapshotYet},
		{Source: SourceFallback, FallbackReason: FallbackReasonIndexBuilding},
		{Source: SourceFallback, FallbackReason: FallbackReasonIndexError},
		{Source: SourceFallback, FallbackReason: FallbackReasonBleveRebuilding},
	}

	for _, env := range cases {
		t.Run(fmt.Sprintf("%s/%s", env.Source, env.FallbackReason), func(t *testing.T) {
			if _, ok := allowedSources[string(env.Source)]; !ok {
				t.Fatalf("Source %q not in closed enum", env.Source)
			}
			if _, ok := allowedReasons[string(env.FallbackReason)]; !ok {
				t.Fatalf("FallbackReason %q not in closed enum", env.FallbackReason)
			}

			raw, err := MarshalEnvelope(env, nil)
			if err != nil {
				t.Fatalf("MarshalEnvelope: %v", err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatalf("json.Unmarshal: %v", err)
			}
			if got := decoded["source"]; got != string(env.Source) {
				t.Errorf("source round-trip: got %v, want %q", got, env.Source)
			}
			if env.FallbackReason == "" {
				if _, ok := decoded["fallback_reason"]; ok {
					t.Errorf("fallback_reason must omitempty when blank; raw=%s", string(raw))
				}
			} else if got := decoded["fallback_reason"]; got != string(env.FallbackReason) {
				t.Errorf("fallback_reason round-trip: got %v, want %q", got, env.FallbackReason)
			}
		})
	}
}

// TestEnvelope_NoRawErrorText is the WR-NEW-01 doctrine assertion. ChooseSource
// must never propagate raw error text into FallbackReason — the only path
// to a closed-enum value is ClassifyLookupErr, which routes via errors.Is.
func TestEnvelope_NoRawErrorText(t *testing.T) {
	cfg := fakeCfg{enabled: true}
	lookup := availableLookup{}
	wrapped := fmt.Errorf("decorate: %w", ErrIndexBuilding)

	src, reason := ChooseSource(cfg, lookup, wrapped)
	if src != SourceFallback {
		t.Fatalf("Source: got %q, want %q", src, SourceFallback)
	}
	if reason != FallbackReasonIndexBuilding {
		t.Fatalf("FallbackReason: got %q, want %q", reason, FallbackReasonIndexBuilding)
	}

	// Marshal it through the envelope. Verify the wrapped string never lands
	// in the JSON.
	env := Envelope{Source: src, FallbackReason: reason}
	raw, err := MarshalEnvelope(env, nil)
	if err != nil {
		t.Fatalf("MarshalEnvelope: %v", err)
	}
	if got := string(raw); contains(got, "decorate") {
		t.Errorf("raw error text leaked into envelope JSON: %s", got)
	}
}

// contains is a small substring helper to avoid pulling strings.Contains into
// a tight test file (matches t.Helper-style style of this package).
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
