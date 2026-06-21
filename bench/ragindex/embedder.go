package ragindex

import (
	"context"
	"fmt"
	"math"
	"net"
	"os"
	"time"

	chromem "github.com/philippgille/chromem-go"
)

// Embedder backend identifiers. Each backend records a DISTINCT embedder_id so a
// CI/offline row is never mistaken for a real RAG measurement (Open Q3). These
// strings are persisted in the chromem collection metadata and surfaced in the
// result.v2 row; the OPENAI_API_KEY value itself is NEVER logged or persisted
// (T-83-01-01 secret-leak mitigation).
const (
	embedderIDOpenAI = "openai-text-embedding-3-small"
	embedderIDOllama = "ollama-nomic-embed-text"
	embedderIDStub   = "stub-deterministic"
)

// ollamaModel and ollamaBaseURL pin the fallback embedder. The base URL is a
// constant — it is NEVER accepted as a tool/function argument (T-83-01-02 SSRF
// mitigation).
const (
	ollamaModel    = "nomic-embed-text"
	ollamaBaseURL  = "http://localhost:11434/api"
	ollamaHostPort = "localhost:11434"
)

// forceStubEnv, when set to a non-empty value, forces selectEmbedder to return
// the deterministic stub regardless of OPENAI_API_KEY / Ollama reachability.
// Used by hermetic unit tests so they never touch the network.
const forceStubEnv = "HELIX_RAG_FORCE_STUB"

// pinnedEmbedderEnv, when set to one of the known embedder_id strings, forces
// selectEmbedder to deterministically return THAT embedder rather than
// re-probing availability — and to FAIL CLOSED if the pinned embedder is not
// actually usable in this process. The parent (runRAGCell) records the
// embedder_id from its own cold index build and forwards it to the spawned
// server via this env so the warm-reopen query embedder cannot silently diverge
// from the document vectors on disk (WR-01: chromem does not persist the
// EmbeddingFunc, so without pinning the subprocess re-derives selection and a
// transient Ollama dial flake would answer queries from stub-space against
// Ollama-space document vectors while the row still claims ollama-*).
const pinnedEmbedderEnv = "HELIX_RAG_EMBEDDER"

// stubDim is the fixed dimensionality of the deterministic stub embedding.
const stubDim = 256

// selectEmbedder chooses the embedding backend by availability and returns the
// chromem EmbeddingFunc plus its distinct embedder_id. Order:
//
//  1. OpenAI text-embedding-3-small  when OPENAI_API_KEY is set
//  2. Ollama nomic-embed-text        when the Ollama daemon is reachable
//  3. deterministic stub             otherwise (hermetic CI)
//
// The OPENAI_API_KEY value is read only to decide availability and is handed to
// chromem's built-in func; it is never logged here.
//
// If pinnedEmbedderEnv is set, selectEmbedder skips probing and returns exactly
// the pinned backend, failing closed (returning an error) when that backend is
// not usable in this process. This keeps a warm-reopen subprocess consistent
// with the embedder_id the parent recorded against the on-disk document vectors
// (WR-01).
func selectEmbedder() (chromem.EmbeddingFunc, string, error) {
	if pinned := os.Getenv(pinnedEmbedderEnv); pinned != "" {
		return embedderFor(pinned)
	}
	if os.Getenv(forceStubEnv) != "" {
		return stubEmbedder(), embedderIDStub, nil
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		return chromem.NewEmbeddingFuncDefault(), embedderIDOpenAI, nil
	}
	if ollamaReachable() {
		return chromem.NewEmbeddingFuncOllama(ollamaModel, ollamaBaseURL), embedderIDOllama, nil
	}
	return stubEmbedder(), embedderIDStub, nil
}

// embedderFor returns the EmbeddingFunc for an explicitly pinned embedder_id,
// failing closed when the named backend is not usable in THIS process. This is
// the authoritative-selection seam for WR-01: the parent forwards the embedder_id
// it built the index with, and the subprocess must either honor it exactly or
// refuse to serve — it must never silently fall back to a different embedder
// while the recorded provenance still claims the pinned one.
func embedderFor(id string) (chromem.EmbeddingFunc, string, error) {
	switch id {
	case embedderIDOpenAI:
		if os.Getenv("OPENAI_API_KEY") == "" {
			return nil, "", fmt.Errorf("ragindex: pinned embedder %q requires OPENAI_API_KEY, which is unset in this process", id)
		}
		return chromem.NewEmbeddingFuncDefault(), embedderIDOpenAI, nil
	case embedderIDOllama:
		if !ollamaReachable() {
			return nil, "", fmt.Errorf("ragindex: pinned embedder %q is unreachable in this process (Ollama dial failed at %s)", id, ollamaHostPort)
		}
		return chromem.NewEmbeddingFuncOllama(ollamaModel, ollamaBaseURL), embedderIDOllama, nil
	case embedderIDStub:
		return stubEmbedder(), embedderIDStub, nil
	default:
		return nil, "", fmt.Errorf("ragindex: unknown pinned embedder %q (want one of %q, %q, %q)", id, embedderIDOpenAI, embedderIDOllama, embedderIDStub)
	}
}

// ollamaReachable probes the pinned Ollama host:port with a short dial timeout.
// It never dials a caller-supplied address (SSRF mitigation).
func ollamaReachable() bool {
	conn, err := net.DialTimeout("tcp", ollamaHostPort, 250*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// stubEmbedder returns a deterministic, network-free EmbeddingFunc: a fixed-dim
// bag-of-bytes hash of the input text. Identical text always yields an identical
// vector; different text (almost certainly) yields a different one. It exists so
// the whole package is unit-testable with zero network and so a CI row carries
// embedder_id == "stub-deterministic" (never mistaken for a real measurement).
func stubEmbedder() chromem.EmbeddingFunc {
	return func(_ context.Context, text string) ([]float32, error) {
		// Byte-value histogram: each byte contributes to the dim indexed by its
		// value. This is position-independent, so two strings that share many
		// bytes (e.g. a query that is a substring of a chunk) land close in
		// cosine space. It is a crude proxy for a real embedder by design and is
		// recorded as embedder_id "stub-deterministic" so it is never mistaken
		// for a real measurement.
		vec := make([]float32, stubDim)
		for i := 0; i < len(text); i++ {
			vec[int(text[i])%stubDim] += 1
		}
		// L2-normalize so chromem's cosine similarity is well-defined and the
		// zero vector (empty text) is avoided by seeding a constant component.
		vec[0] += 1
		var norm float32
		for _, v := range vec {
			norm += v * v
		}
		if norm > 0 {
			inv := float32(1.0 / math.Sqrt(float64(norm)))
			for i := range vec {
				vec[i] *= inv
			}
		}
		return vec, nil
	}
}
