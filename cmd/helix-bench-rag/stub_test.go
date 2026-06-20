package main

import (
	"context"

	"github.com/agenthands/helix/bench/ragindex"
)

// stubIndex is a deterministic, no-network querier for the tool tests. It never
// touches chromem or the network, so the cmd tests run fully hermetically.
type stubIndex struct {
	results []ragindex.Result
	err     error
}

func newStubIndex() *stubIndex {
	return &stubIndex{
		results: []ragindex.Result{
			{ChunkID: "a.go#0", RelPath: "a.go", Content: "package a", Similarity: 0.9},
		},
	}
}

func (s *stubIndex) Query(_ context.Context, _ string, _ int) ([]ragindex.Result, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.results, nil
}
