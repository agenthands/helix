package adopt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPythonGoParityCorpus pins the SHARED golden corpus
// (tools/dspy-tune/golden/parity_cases.json) to the Go classifier truth in
// scorecard.go. The same JSON file is read by the Python pytest in Plan 02, so
// this test is the Go half of the cross-language parity contract: the corpus is
// asserted against FirstCommand + ClassifyChoice for EVERY case, never
// duplicated under test/oracle/adopt/testdata.
//
// This is the non-vacuity proof for the corpus (T-106-03): the corpus IS the
// assertion. A deliberately wrong expected field on any case — e.g. claiming the
// substring-trap case (prose mentioning "helix", then a grep) is a choice —
// would make this test FAIL, exactly mirroring TestFirstCommandNotSubstring. The
// >= 8 floor mirrors loadFixtureBucket's MinTasks floor so a future corpus
// shrink that drops a branch-covering case turns RED.
func TestPythonGoParityCorpus(t *testing.T) {
	// test/oracle/adopt is three dirs deep from the repo root, so the single
	// committed corpus lives at ../../../tools/dspy-tune/golden/parity_cases.json.
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "tools", "dspy-tune", "golden", "parity_cases.json"))
	require.NoError(t, err, "reading the shared golden parity corpus")

	var cases []struct {
		Response     string `json:"response"`
		FirstCommand string `json:"first_command"`
		Chose        bool   `json:"chose"`
		FellBack     bool   `json:"fell_back"`
	}
	require.NoError(t, json.Unmarshal(data, &cases), "unmarshaling parity_cases.json")
	require.GreaterOrEqual(t, len(cases), 8,
		"corpus must cover every classifier branch (helix-choice, 6 fallback prefixes, prose, fence+prompt, substring-trap, lsp-lookalike)")

	for _, c := range cases {
		require.Equalf(t, c.FirstCommand, FirstCommand(c.Response),
			"FirstCommand mismatch on response %q", c.Response)
		chose, fell := ClassifyChoice(c.Response)
		require.Equalf(t, c.Chose, chose, "choice mismatch on response %q", c.Response)
		require.Equalf(t, c.FellBack, fell, "fallback mismatch on response %q", c.Response)
	}
}
