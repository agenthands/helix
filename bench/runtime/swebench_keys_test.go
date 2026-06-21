package runtime

import (
	"encoding/json"
	"testing"

	"github.com/agenthands/helix/bench/runners"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContainerIDExitCodeBackwardCompat (Phase 87 Task 1, ADAPTER-SWE-01 — the
// 87-VALIDATION Wave 0 additive-key proof, Case A): a build that leaves
// ContainerID=="" and ExitCode==nil (every pre-SWE-bench row) emits NEITHER the
// container_id NOR the exit_code key (both omitempty) and STILL validates against
// the committed result.v2 schema. This proves the two new keys are additive-minor
// open keys: a row written before they existed stays byte-compatible (schema_version
// stays "v2"; neither key is in `required`; additionalProperties is OPEN).
func TestContainerIDExitCodeBackwardCompat(t *testing.T) {
	in := ResultInput{
		TaskID:    "internal-toolbench/IT-go-patch-apply-1",
		Mode:      "your_agent_full",
		Benchmark: "internal-toolbench",
		RunIndex:  0,
		Outcome:   "success",
		TraceRef:  "bench/reports/x/trace.json",
		Fairness:  runners.DefaultContract,
		// ContainerID intentionally "" and ExitCode intentionally nil — a
		// pre-SWE-bench row that never touched the harness ingestion path.
	}

	doc, err := BuildResult(in)
	require.NoError(t, err)
	require.NoError(t, Validate(doc),
		"a pre-SWE-bench row (ContainerID=='' and ExitCode==nil) must still validate (additive-minor open keys)")

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(doc, &raw))
	_, hasContainer := raw["container_id"]
	assert.False(t, hasContainer,
		"a ContainerID=='' build must OMIT the container_id key (omitempty)")
	_, hasExit := raw["exit_code"]
	assert.False(t, hasExit,
		"an ExitCode==nil build must OMIT the exit_code key (omitempty)")
}

// TestExitCodeCleanZeroPreserved (Phase 87 Task 1, ADAPTER-SWE-01 — additive-key
// proof Case B / Pitfall 2): a build with ExitCode=ptr(0) and a populated
// ContainerID emits exit_code: 0 (the clean-exit code, NOT dropped) and the
// container_id, and still validates. This is the *int-not-int contract: a
// value-type omitempty would wrongly drop a literal 0, erasing the difference
// between "harness ran clean (exit 0)" and "no exit code captured (nil)". The
// assertion parses the JSON back and confirms the key is PRESENT with value 0.
func TestExitCodeCleanZeroPreserved(t *testing.T) {
	in := ResultInput{
		TaskID:      "swe-bench-verified/sympy__sympy-20590",
		Mode:        "your_agent_full",
		Benchmark:   "swe-bench-verified",
		RunIndex:    0,
		Outcome:     "success",
		TraceRef:    "bench/reports/x/trace.json",
		Fairness:    runners.DefaultContract,
		ContainerID: "sha256:abc",
		ExitCode:    iPtr(0),
	}

	doc, err := BuildResult(in)
	require.NoError(t, err)
	require.NoError(t, Validate(doc),
		"a SWE-bench row carrying container_id + a clean exit_code 0 must validate")

	// Parse back: exit_code must be PRESENT with value 0 (a value-type omitempty
	// would have dropped this clean zero — the bug this test guards).
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(doc, &raw))

	exitRaw, hasExit := raw["exit_code"]
	require.True(t, hasExit,
		"a *int ExitCode of 0 must be PRESENT, never dropped by omitempty (Pitfall 2)")
	var exit int
	require.NoError(t, json.Unmarshal(exitRaw, &exit))
	assert.Equal(t, 0, exit, "the preserved exit_code must be the literal clean-exit 0")

	containerRaw, hasContainer := raw["container_id"]
	require.True(t, hasContainer, "a non-empty ContainerID must emit the container_id key")
	var container string
	require.NoError(t, json.Unmarshal(containerRaw, &container))
	assert.Equal(t, "sha256:abc", container, "container_id must carry the recorded container identity")
}
