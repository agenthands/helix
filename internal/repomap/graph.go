package repomap

import (
	"math"
	"strings"
	"sync"
)

// RankedFile pairs a file path with its PageRank score.
// Used by the rendering layer to produce token-budgeted output.
type RankedFile struct {
	Path  string
	Score float64
}

// Location is a minimal reference location used by EnrichFromLSP.
// It avoids importing the gen package into the repomap package.
type Location struct {
	URI  string
	Line int
}

// FileGraph is an in-memory directed graph where nodes are file paths
// and edges represent cross-file ref-to-def relationships weighted by
// reference count. Per D-02: in-memory only, no persistence of edges.
type FileGraph struct {
	// Edges maps source file -> target file -> weight.
	Edges map[string]map[string]float64
	// Files is the set of all file nodes in the graph.
	Files map[string]bool
	mu    sync.RWMutex
}

// NewFileGraph creates an empty FileGraph with initialized maps.
func NewFileGraph() *FileGraph {
	return &FileGraph{
		Edges: make(map[string]map[string]float64),
		Files: make(map[string]bool),
	}
}

// addEdge adds weight to the edge from src to dst (or creates it).
// Also registers both nodes in the Files set.
func (g *FileGraph) addEdge(src, dst string, weight float64) {
	if g.Edges[src] == nil {
		g.Edges[src] = make(map[string]float64)
	}
	g.Edges[src][dst] += weight
	g.Files[src] = true
	g.Files[dst] = true
}

// NodeCount returns the number of file nodes in the graph.
func (g *FileGraph) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.Files)
}

// EdgeCount returns the total number of edges across all source nodes.
func (g *FileGraph) EdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	count := 0
	for _, targets := range g.Edges {
		count += len(targets)
	}
	return count
}

// BuildGraph constructs a FileGraph from all cached tags.
// Per D-01: file-level graph with cross-file ref-to-def edges.
// Per D-03: edge weight = sqrt(reference count).
func BuildGraph(cache *TagCache) (*FileGraph, error) {
	allTags, err := cache.AllFiles()
	if err != nil {
		return nil, err
	}

	// Index: symbolName -> []filePath for definitions.
	defs := make(map[string][]string)
	// Index: symbolName -> filePath -> count for references.
	refs := make(map[string]map[string]int)

	for filePath, tags := range allTags {
		for _, tag := range tags {
			if tag.Kind == TagDef {
				defs[tag.Name] = append(defs[tag.Name], filePath)
			} else {
				if refs[tag.Name] == nil {
					refs[tag.Name] = make(map[string]int)
				}
				refs[tag.Name][filePath]++
			}
		}
	}

	// F1-B (Phase 46 / BUG-01): weight edges by inverse sqrt of name
	// ambiguity. A name defined in many files is a weaker signal per-edge
	// than a name defined in one file. This dampens common-name collisions
	// (add, log, new, Logger) across languages without any path- or
	// language-specific heuristic. See
	// .planning/phases/46-bug-repomap-lua-fixture/46-RESEARCH.md.
	defDegree := make(map[string]int, len(defs))
	for name, files := range defs {
		defDegree[name] = len(files)
	}

	g := NewFileGraph()

	// Create edges: referencer -> definer, weight = sqrt(refCount) * ambiguityScale.
	for ident, definers := range defs {
		ambiguityScale := 1.0 / math.Sqrt(1.0+float64(defDegree[ident]))
		refFiles, hasRefs := refs[ident]
		if !hasRefs {
			// Self-loop for isolated definitions (aider pattern, weight 0.1, unchanged).
			for _, defFile := range definers {
				g.addEdge(defFile, defFile, 0.1)
			}
			continue
		}
		for refFile, count := range refFiles {
			for _, defFile := range definers {
				if refFile == defFile {
					continue // skip same-file refs
				}
				g.addEdge(refFile, defFile, math.Sqrt(float64(count))*ambiguityScale)
			}
		}
	}

	return g, nil
}

// EnrichFromLSP adds cross-file reference edges from LSP references.
// Called opportunistically when a WorkerLease is already active.
// Per D-10: opportunistic only. Per D-11: additive edges.
func (g *FileGraph) EnrichFromLSP(sourceFile string, refs []Location) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, ref := range refs {
		targetFile := uriToPath(ref.URI)
		if targetFile == "" || targetFile == sourceFile {
			continue
		}
		g.addEdge(targetFile, sourceFile, 1.0)
	}
}

// uriToPath extracts a file path from a file:// URI.
// Returns empty string for non-file URIs.
func uriToPath(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		return strings.TrimPrefix(uri, "file://")
	}
	return uri
}
