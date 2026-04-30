package repomap

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// TreeRenderer produces tree-structured, token-budgeted views of ranked files
// with elided symbol definitions. It combines the PageRank ranking from FileGraph
// with the ElisionRenderer from Phase 27 to produce compact repo maps.
type TreeRenderer struct {
	elider  *ElisionRenderer
	cache   *TagCache
	rootDir string
}

// NewTreeRenderer creates a TreeRenderer for the given workspace root.
func NewTreeRenderer(elider *ElisionRenderer, cache *TagCache, rootDir string) *TreeRenderer {
	return &TreeRenderer{
		elider:  elider,
		cache:   cache,
		rootDir: rootDir,
	}
}

// EstimateTokens approximates token count as len(s)/4.
// Per D-12: character-based token estimation, no external dependency.
func EstimateTokens(s string) int {
	return len(s) / 4
}

// RenderBudgeted renders ranked files as a tree, using binary search on file
// count to maximize coverage within the token budget.
// Per D-13: prune lowest-ranked files first via binary search.
// At least 1 file is always included regardless of budget.
func (r *TreeRenderer) RenderBudgeted(ranked []RankedFile, budget int) string {
	if len(ranked) == 0 {
		return ""
	}

	lower, upper := 1, len(ranked)
	bestOutput := ""
	bestTokens := 0
	okErr := 0.15 // 15% tolerance per aider

	for lower <= upper {
		mid := (lower + upper) / 2
		output := r.renderTree(ranked[:mid])
		tokens := EstimateTokens(output)

		pctErr := math.Abs(float64(tokens-budget)) / float64(budget)

		withinBudget := tokens <= budget
		if (withinBudget && tokens > bestTokens) || (pctErr < okErr && tokens > bestTokens) {
			bestOutput = output
			bestTokens = tokens
		}

		if tokens < budget {
			lower = mid + 1
		} else {
			upper = mid - 1
		}
	}

	// Guarantee at least 1 file.
	if bestOutput == "" {
		bestOutput = r.renderTree(ranked[:1])
	}

	return bestOutput
}

// treeNode represents a node in the directory tree used for rendering.
type treeNode struct {
	name     string
	children []*treeNode
	isFile   bool
	content  string // elided symbol content for files
}

// renderTree produces a tree-structured output with directory nesting and
// elided symbol definitions for each file.
func (r *TreeRenderer) renderTree(ranked []RankedFile) string {
	if len(ranked) == 0 {
		return ""
	}

	// Build tree from relative paths.
	root := &treeNode{name: ""}

	for _, rf := range ranked {
		relPath, err := filepath.Rel(r.rootDir, rf.Path)
		if err != nil {
			relPath = rf.Path
		}

		// Render elided content for this file.
		content := r.renderFileContent(rf.Path)

		// Split path into parts and insert into tree.
		parts := strings.Split(filepath.ToSlash(relPath), "/")
		insertIntoTree(root, parts, content)
	}

	// Render the tree to string.
	var buf bytes.Buffer
	renderNode(&buf, root, 0)
	return buf.String()
}

// renderFileContent reads a file and produces its elided symbol view.
func (r *TreeRenderer) renderFileContent(filePath string) string {
	source, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	lang := LangFromExt(filePath)

	// Try to load tags from cache.
	//
	// Phase 53 Q-2 Option 2: this caller passes a no-op extractFn that returns
	// (nil, nil) — there's no real extractor available at render time. This
	// means:
	//   - The cache lookup hit/miss counter (D-03) WILL fire from
	//     internal/repomap/cache.go on both branches.
	//   - No RepoMapExtractObserve fires here because no real extractor ran.
	// This is intentional — observing 0s for a no-op extractor would muddy the
	// helix_repomap_extract_duration_seconds histogram. The dispatcher in
	// internal/skill/repomap/skill.go owns extract-latency observation.
	tags, err := r.cache.GetOrExtract(filePath, func() ([]Tag, error) {
		return nil, nil // no extraction function available at render time
	})
	if err != nil || len(tags) == 0 {
		return ""
	}

	return r.elider.RenderFile(source, lang, tags)
}

// insertIntoTree inserts a file path (split into parts) into the tree.
func insertIntoTree(node *treeNode, parts []string, content string) {
	if len(parts) == 0 {
		return
	}

	// Find or create child for this part.
	name := parts[0]
	var child *treeNode
	for _, c := range node.children {
		if c.name == name {
			child = c
			break
		}
	}

	if child == nil {
		child = &treeNode{name: name}
		node.children = append(node.children, child)
	}

	if len(parts) == 1 {
		// This is the file leaf.
		child.isFile = true
		child.content = content
	} else {
		// This is a directory node, recurse.
		insertIntoTree(child, parts[1:], content)
	}
}

// renderNode writes the tree node and its children to the buffer with indentation.
func renderNode(buf *bytes.Buffer, node *treeNode, depth int) {
	indent := strings.Repeat("  ", depth)

	// Sort children: directories first, then files, both alphabetically.
	sort.Slice(node.children, func(i, j int) bool {
		iDir := !node.children[i].isFile && len(node.children[i].children) > 0
		jDir := !node.children[j].isFile && len(node.children[j].children) > 0
		if iDir != jDir {
			return iDir // directories first
		}
		return node.children[i].name < node.children[j].name
	})

	for _, child := range node.children {
		if child.isFile {
			buf.WriteString(indent + child.name + "\n")
			if child.content != "" {
				// Indent each line of content by 4 additional spaces.
				lines := strings.Split(child.content, "\n")
				for _, line := range lines {
					if line != "" {
						buf.WriteString(indent + "    " + line + "\n")
					}
				}
			}
		} else {
			// Directory node.
			buf.WriteString(indent + child.name + "/\n")
			renderNode(buf, child, depth+1)
		}
	}
}

// LangFromExt maps file extensions to language identifiers used by tree-sitter.
func LangFromExt(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "tsx"
	case ".js":
		return "javascript"
	case ".java":
		return "java"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp", ".hxx":
		return "cpp"
	case ".cs":
		return "c_sharp"
	case ".kt":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".sh", ".bash":
		return "bash"
	case ".hs":
		return "haskell"
	case ".jl":
		return "julia"
	case ".ml":
		return "ocaml"
	case ".lua":
		return "lua"
	case ".zig":
		return "zig"
	case ".tf", ".hcl":
		return "hcl"
	case ".r", ".R":
		return "r"
	case ".swift":
		return "swift"
	default:
		return ""
	}
}

// renderFileEntry is a helper that renders a single file entry with its elided symbols.
func (r *TreeRenderer) renderFileEntry(relPath string, ranked RankedFile) string {
	var buf bytes.Buffer
	content := r.renderFileContent(ranked.Path)
	fmt.Fprintf(&buf, "%s\n", filepath.Base(relPath))
	if content != "" {
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			if line != "" {
				fmt.Fprintf(&buf, "    %s\n", line)
			}
		}
	}
	return buf.String()
}
