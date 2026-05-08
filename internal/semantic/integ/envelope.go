// envelope.go — JSON wire shape for the four MCP tool envelopes emitted by
// 65-05 (get_repo_map), 65-06 (analyze_blast_radius), 65-07 (get_context),
// and the get_health bridge. Phase 65 D-04 + D-05 + INTEG-05.
//
// Wire-shape doctrine:
//
//   - "source" is mandatory on every envelope and MUST be one of the closed
//     enum values declared in source.go: "semantic" | "tree_sitter" |
//     "fallback".
//   - "fallback_reason" is omitted when the envelope reports SourceSemantic
//     or SourceTreeSitter (steady state). When present it MUST be one of the
//     closed enum values from source.go.
//   - "graph_version" is omitted when zero (caller hasn't computed a version
//     yet — typical for the tree-sitter and pre-snapshot fallback paths).
//   - "freshness" is omitted when empty; closed-enum mirror of Phase 64
//     SPEC §26.2 (semantic/envelope.go declares the constants).
//   - Tool-specific payload (the get_repo_map tree, the blast-radius impacts,
//     the context candidates, etc.) is merged at the top level by
//     MarshalEnvelope so consumers see a flat object — not a nested
//     "payload": {} wrapper. This matches the SPEC §24 envelope contract.
package integ

import "encoding/json"

// Envelope carries the per-call envelope header fields that every Phase 65
// MCP tool emits. Tool-specific payload travels via the second argument to
// MarshalEnvelope; the Payload field on Envelope is reserved for future use
// (or in-memory composition by callers that want a strongly-typed handle).
type Envelope struct {
	// Source is the closed-enum source classification (D-04).
	Source Source `json:"source"`
	// FallbackReason explains why Source==SourceFallback. Omitted when the
	// envelope reports SourceSemantic or SourceTreeSitter.
	FallbackReason FallbackReason `json:"fallback_reason,omitempty"`
	// GraphVersion is the (repo_id, projection)-stamped Phase 62 D-07
	// version the payload was computed against. Omitted when zero.
	GraphVersion uint64 `json:"graph_version,omitempty"`
	// Freshness is the closed-enum freshness marker; mirrors Phase 64
	// SPEC §26.2. Omitted when empty.
	Freshness string `json:"freshness,omitempty"`
	// Payload is the tool-specific in-memory handle. NOT serialized
	// directly — MarshalEnvelope merges a separate payload map at the top
	// level so the wire shape stays flat.
	Payload any `json:"-"`
}

// MarshalEnvelope produces the canonical JSON for an Envelope plus an
// optional tool-specific payload. The payload's keys are merged at the top
// level — collisions with envelope keys ("source", "fallback_reason",
// "graph_version", "freshness") are rejected so a tool can't accidentally
// shadow the closed-enum source field.
//
// Omitempty semantics:
//   - FallbackReason is emitted only when non-empty.
//   - GraphVersion is emitted only when non-zero.
//   - Freshness is emitted only when non-empty.
//
// Source is always emitted (the field is non-optional in the SPEC §24
// envelope contract).
func MarshalEnvelope(env Envelope, payload map[string]any) ([]byte, error) {
	out := make(map[string]any, 4+len(payload))
	out["source"] = string(env.Source)
	if env.FallbackReason != "" {
		out["fallback_reason"] = string(env.FallbackReason)
	}
	if env.GraphVersion != 0 {
		out["graph_version"] = env.GraphVersion
	}
	if env.Freshness != "" {
		out["freshness"] = env.Freshness
	}
	for k, v := range payload {
		// Reserved keys take precedence; payload may not shadow them.
		if _, reserved := reservedEnvelopeKeys[k]; reserved {
			continue
		}
		out[k] = v
	}
	return json.Marshal(out)
}

// reservedEnvelopeKeys names the JSON keys that MarshalEnvelope owns. A
// tool payload that includes one of these keys is silently dropped (the
// envelope's value wins). This prevents per-tool drift from the closed-enum
// contract — a 65-05 implementer cannot, for example, override the source
// field by injecting "source": "anything" into their payload.
var reservedEnvelopeKeys = map[string]struct{}{
	"source":          {},
	"fallback_reason": {},
	"graph_version":   {},
	"freshness":       {},
}
