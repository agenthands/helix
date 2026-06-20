package ragindex

import (
	"context"
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
	ollamaModel   = "nomic-embed-text"
	ollamaBaseURL = "http://localhost:11434/api"
	ollamaHostPort = "localhost:11434"
)

// forceStubEnv, when set to a non-empty value, forces selectEmbedder to return
// the deterministic stub regardless of OPENAI_API_KEY / Ollama reachability.
// Used by hermetic unit tests so they never touch the network.
const forceStubEnv = "HELIX_RAG_FORCE_STUB"

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
func selectEmbedder() (chromem.EmbeddingFunc, string) {
	if os.Getenv(forceStubEnv) != "" {
		return stubEmbedder(), embedderIDStub
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		return chromem.NewEmbeddingFuncDefault(), embedderIDOpenAI
	}
	if ollamaReachable() {
		return chromem.NewEmbeddingFuncOllama(ollamaModel, ollamaBaseURL), embedderIDOllama
	}
	return stubEmbedder(), embedderIDStub
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
