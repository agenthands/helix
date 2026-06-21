//go:build !windows
// +build !windows

package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agenthands/helix/bench/runners"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resolveRAGServerBinForTest builds (if needed) and returns the helix-bench-rag
// binary path. It prefers HELIX_BENCH_RAG_BIN, then a sibling of HELIX_BIN named
// "helix-bench-rag". When neither resolves it returns "" so the caller SKIPs —
// these are HELIX_BIN-gated cell tests (per MEMORY: go test ./... is FALSE-GREEN
// without HELIX_BIN; the baseline_rag cell tests must actually RUN).
func resolveRAGServerBinForTest(t *testing.T, helixBin string) string {
	t.Helper()
	if env := os.Getenv("HELIX_BENCH_RAG_BIN"); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env
		}
	}
	if helixBin != "" {
		sibling := filepath.Join(filepath.Dir(helixBin), "helix-bench-rag")
		if _, err := os.Stat(sibling); err == nil {
			return sibling
		}
	}
	return ""
}

// baselineRagCellConfig builds a CellConfig for the baseline_rag arm on the seed
// IT-go-patch-apply-1 task, pointing HELIX_BENCH_RAG_BIN at the resolved server
// binary so subprocess.StartRAGServer can spawn it.
func baselineRagCellConfig(t *testing.T, outDir string) (CellConfig, func()) {
	t.Helper()
	helixBin := resolveHelixBin()
	if helixBin == "" {
		t.Skip("helix binary not resolvable (set HELIX_BIN or 'go build ./cmd/helix'); skipping baseline_rag cell test")
	}
	ragBin := resolveRAGServerBinForTest(t, helixBin)
	if ragBin == "" {
		t.Skip("helix-bench-rag binary not resolvable (build it next to helix or set HELIX_BENCH_RAG_BIN); skipping baseline_rag cell test")
	}
	// HELIX_BIN-gated but HERMETIC: force the deterministic stub embedder (Open Q3)
	// via HELIX_RAG_FORCE_STUB so selectEmbedder takes the no-network stub path
	// regardless of OPENAI_API_KEY / Ollama reachability. A CI row must never depend
	// on the network and must be recorded as embedder_id=="stub-deterministic", not
	// a real model. Also pin HELIX_CACHE_DIR to a per-test temp dir so the corpus
	// index is built fresh and never collides with a developer's warm cache.
	var saved []func()
	setEnv := func(k, v string) {
		prev, had := os.LookupEnv(k)
		require.NoError(t, os.Setenv(k, v))
		saved = append(saved, func() {
			if had {
				_ = os.Setenv(k, prev)
			} else {
				_ = os.Unsetenv(k)
			}
		})
	}
	setEnv("HELIX_BENCH_RAG_BIN", ragBin)
	setEnv("HELIX_CACHE_DIR", t.TempDir())
	setEnv("HELIX_RAG_FORCE_STUB", "1")
	restore := func() {
		for i := len(saved) - 1; i >= 0; i-- {
			saved[i]()
		}
	}

	return CellConfig{
		RunID:       "20060102T150405Z",
		Benchmark:   "internal-toolbench",
		Language:    "go",
		Task:        "IT-go-patch-apply-1",
		Mode:        "baseline_rag",
		HelixBin:    helixBin,
		SeedDir:     seedDirForTest(t),
		OutDir:      outDir,
		RunnersRoot: benchRunnersRootForTest(t),
		Agent:       "scripted",
	}, restore
}

// TestBaselineRagEmitsRow (Phase 83 Task 2, ABLATE-04 #3): a baseline_rag cell
// now drives the standalone cmd/helix-bench-rag MCP server (out-of-band index +
// stdio spawn), emits a schema-valid result.v2.json on disk, and records a
// non-empty embedder_id in that row. This REPLACES the Phase-80 fail-close that
// asserted Deferred==true / NO row. HELIX_BIN-gated.
func TestBaselineRagEmitsRow(t *testing.T) {
	outDir := t.TempDir()
	cfg, restore := baselineRagCellConfig(t, outDir)
	defer restore()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	res, err := RunCell(ctx, cfg)
	if res.ScratchPreserved {
		t.Cleanup(func() { _ = removeAllForTest(res.ScratchDir) })
	}
	require.NoError(t, err, "baseline_rag must run without an infra error")
	assert.False(t, res.Deferred, "baseline_rag is no longer deferred — it is a real arm")
	assert.True(t, res.ResultValid, "baseline_rag must emit a schema-valid result.v2")

	// A real schema-valid row exists on disk at the layout path.
	require.NotEmpty(t, res.ResultPath)
	b, statErr := os.ReadFile(res.ResultPath)
	require.NoError(t, statErr, "a baseline_rag cell must WRITE a result.v2.json (no longer fail-closed)")
	require.NoError(t, Validate(b), "the on-disk baseline_rag row must be schema-valid")

	var doc struct {
		Mode       string `json:"mode"`
		EmbedderID string `json:"embedder_id"`
		ModelID    string `json:"model_id"`
	}
	require.NoError(t, json.Unmarshal(b, &doc))
	assert.Equal(t, "baseline_rag", doc.Mode)
	assert.NotEmpty(t, doc.EmbedderID,
		"every baseline_rag row must record a non-empty embedder_id (T-83-03-01)")
}

// TestBaselineRagSameContractBudget (Phase 83 Task 2, ABLATE-04 #4): the
// baseline_rag row reuses runners.DefaultContract VERBATIM — its model_id equals
// DefaultContract.ModelID (identical to your_agent_full) — and the embedding
// index build is excluded from the timed/budgeted span (token metrics are explicit
// null, never inflated by embedding-API tokens). HELIX_BIN-gated.
func TestBaselineRagSameContractBudget(t *testing.T) {
	outDir := t.TempDir()
	cfg, restore := baselineRagCellConfig(t, outDir)
	defer restore()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	res, err := RunCell(ctx, cfg)
	if res.ScratchPreserved {
		t.Cleanup(func() { _ = removeAllForTest(res.ScratchDir) })
	}
	require.NoError(t, err)
	require.NotEmpty(t, res.ResultPath)
	b, rerr := os.ReadFile(res.ResultPath)
	require.NoError(t, rerr)

	var doc struct {
		ModelID string `json:"model_id"`
		Metrics struct {
			TokensInput  *int `json:"tokens_input"`
			TokensOutput *int `json:"tokens_output"`
		} `json:"metrics"`
	}
	require.NoError(t, json.Unmarshal(b, &doc))

	// Criterion #4 identity: same model snapshot as your_agent_full (DefaultContract).
	assert.Equal(t, runners.DefaultContract.ModelID, doc.ModelID,
		"baseline_rag must reuse DefaultContract.ModelID verbatim (same model as your_agent_full)")

	// Budget exclusion: the scripted RAG drive carries no provider usage block, so
	// the token metrics are explicit null — and crucially the out-of-band embedding
	// build never enters this span, so no embedding-API tokens inflate the row.
	assert.Nil(t, doc.Metrics.TokensInput,
		"embedding-build tokens must NOT be charged to tokens_input (criterion #4 / Pitfall 3)")
	assert.Nil(t, doc.Metrics.TokensOutput,
		"embedding-build tokens must NOT be charged to tokens_output (criterion #4 / Pitfall 3)")
}
