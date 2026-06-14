// Package main is a minimal probe binary that imports bleve so the linker pulls in
// bleve and its transitive graph. Used by Phase 64-01 to measure binary-size delta
// against the equivalent helix build without bleve in scope. Not intended to run.
package main

import (
	_ "github.com/blevesearch/bleve/v2"
)

func main() {}
