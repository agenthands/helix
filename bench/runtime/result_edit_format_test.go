package runtime

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/agenthands/helix/bench/runners"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEditFormatApplied (EDITBENCH-03) proves the additive `edit_format_applied`
// *bool open key mirrors the SwebenchRawResolved discipline EXACTLY: a literal
// false survives marshalling (NOT dropped by omitempty), a literal true marshals,
// a nil drops the key, and a row carrying it still validates with schema_version
// "v2" (no v3 bump; the key is NOT in the schema `required` set).
func TestEditFormatApplied(t *testing.T) {
	base := func() ResultInput {
		return ResultInput{
			TaskID:    "aider-polyglot/go-wordy",
			Mode:      "aider_edit",
			Benchmark: "aider-polyglot",
			RunIndex:  0,
			Outcome:   "success",
			TraceRef:  "bench/reports/x/trace.json",
			Fairness:  runners.DefaultContract,
		}
	}

	// (1) Round-trip false: a literal false is PRESERVED (the load-bearing
	// "edit format NOT applied" verdict), not dropped by omitempty.
	t.Run("false-preserved", func(t *testing.T) {
		in := base()
		in.EditFormatApplied = bPtr(false)

		doc, err := BuildResult(in)
		require.NoError(t, err)
		assert.True(t, bytes.Contains(doc, []byte(`"edit_format_applied": false`)),
			"a literal false must be PRESERVED (not dropped by omitempty): %s", doc)
	})

	// (2) Round-trip true.
	t.Run("true-marshals", func(t *testing.T) {
		in := base()
		in.EditFormatApplied = bPtr(true)

		doc, err := BuildResult(in)
		require.NoError(t, err)
		assert.True(t, bytes.Contains(doc, []byte(`"edit_format_applied": true`)),
			"a literal true must marshal: %s", doc)
	})

	// (3) Nil drops the key entirely.
	t.Run("nil-drops-key", func(t *testing.T) {
		in := base()
		in.EditFormatApplied = nil

		doc, err := BuildResult(in)
		require.NoError(t, err)
		assert.False(t, bytes.Contains(doc, []byte("edit_format_applied")),
			"a nil EditFormatApplied must omit the key entirely: %s", doc)
	})

	// (4) Schema still valid, schema_version still "v2" (no v3 bump), and the key
	// is NOT in the schema required set (the doc validates while carrying it).
	t.Run("schema-v2-still-valid", func(t *testing.T) {
		in := base()
		in.EditFormatApplied = bPtr(false)

		doc, err := BuildResult(in)
		require.NoError(t, err)
		require.NoError(t, Validate(doc),
			"a row carrying edit_format_applied must pass schema validation (additive open key)")

		var m map[string]any
		require.NoError(t, json.Unmarshal(doc, &m))
		assert.Equal(t, "v2", m["schema_version"],
			"schema_version must stay v2 — the open key is additive-minor, no v3 bump")
		// The key rode through as an additional property, not a named required prop.
		_, present := m["edit_format_applied"]
		assert.True(t, present, "edit_format_applied must be present in the marshalled doc")
	})
}
