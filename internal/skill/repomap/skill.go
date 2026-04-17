// Package repomap implements the repomap skill, contributing 2 MCP tools
// for repository structure mapping and task-focused context retrieval.
package repomap

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	serr "github.com/postfix/serena/internal/errors"
	"github.com/postfix/serena/internal/mcp"
	"github.com/postfix/serena/internal/repomap"
	"github.com/postfix/serena/internal/skill"
	"github.com/postfix/serena/internal/treesitter"
)

const (
	defaultRepoMapBudget = 4096
	defaultContextBudget = 2048
	maxTokenBudget       = 32768
	minTokenBudget       = 64
)

// RepoMapSkill implements skill.ToolProvider, exposing get_repo_map and
// get_context as MCP tools backed by the repomap graph and PageRank ranking.
type RepoMapSkill struct {
	cache    *repomap.TagCache
	elider   *repomap.ElisionRenderer
	graph    *repomap.FileGraph
	graphVer int64 // TagCache version when graph was last built
	mu       sync.Mutex
	logger   *slog.Logger
	rootDir  string // workspace root, resolved lazily via os.Getwd if empty
}

func init() {
	skill.Register(&RepoMapSkill{})
}

// Name returns the skill identifier.
func (s *RepoMapSkill) Name() string { return "repomap" }

// Description returns a human-readable description.
func (s *RepoMapSkill) Description() string {
	return "Repository structure map with ranked symbols"
}

// Init creates the underlying TagCache and renderers from skill dependencies.
// Per plan decision: does NOT modify SkillDeps. Creates own dependencies from ProjectDir.
func (s *RepoMapSkill) Init(deps skill.SkillDeps) error {
	s.logger = deps.Logger
	if s.logger == nil {
		s.logger = slog.Default()
	}

	// Create tag cache from project dir.
	dbPath := filepath.Join(deps.ProjectDir, "index", "tags.db")
	cache, err := repomap.NewTagCache(dbPath)
	if err != nil {
		return serr.Wrap(serr.Internal, "creating tag cache for repomap skill", err)
	}
	s.cache = cache

	// Create grammar registry and elision renderer for output formatting.
	registry := treesitter.NewGrammarRegistry()
	s.elider = repomap.NewElisionRenderer(registry)

	return nil
}

// Tools returns the 2 MCP tool definitions for repomap operations.
func (s *RepoMapSkill) Tools() []*mcp.ToolDef {
	return []*mcp.ToolDef{
		s.getRepoMapTool(),
		s.getContextTool(),
	}
}

func (s *RepoMapSkill) getRepoMapTool() *mcp.ToolDef {
	return &mcp.ToolDef{
		Name: "get_repo_map",
		Description: "Get a ranked structural overview of the repository. Returns file paths organized " +
			"as a tree with elided symbol signatures, ranked by structural importance (PageRank). " +
			"Use this to understand the overall codebase structure. The token_budget parameter " +
			"controls the maximum output size (in approximate tokens).",
		RegisterFn: func(server interface{}) error {
			return nil
		},
	}
}

func (s *RepoMapSkill) getContextTool() *mcp.ToolDef {
	return &mcp.ToolDef{
		Name: "get_context",
		Description: "Get the most relevant code context for a set of files or a task. Returns symbols " +
			"ranked by relevance to the specified files, within the token budget. Use this when you " +
			"need to understand code related to specific files you're working with.",
		RegisterFn: func(server interface{}) error {
			return nil
		},
	}
}

// ExecuteTool runs a repomap tool by name with the given JSON arguments.
func (s *RepoMapSkill) ExecuteTool(name string, args map[string]interface{}) (string, error) {
	switch name {
	case "get_repo_map":
		return s.execGetRepoMap(args)
	case "get_context":
		return s.execGetContext(args)
	default:
		return "", serr.New(serr.InvalidArgs, "unknown repomap tool").WithTool(name)
	}
}

// execGetRepoMap handles the get_repo_map tool: uniform PageRank per D-05.
func (s *RepoMapSkill) execGetRepoMap(args map[string]interface{}) (string, error) {
	budget := extractTokenBudget(args, defaultRepoMapBudget)

	if err := s.ensureGraph(); err != nil {
		return "", serr.Wrap(serr.Internal, "building reference graph", err).WithTool("get_repo_map")
	}

	ranked := s.graph.RankFiles(0.85, nil)
	if len(ranked) == 0 {
		return "No files found in repository.", nil
	}

	output := s.renderBudgeted(ranked, budget)
	if output == "" {
		return "No files found in repository.", nil
	}

	return output, nil
}

// execGetContext handles the get_context tool: personalized PageRank per D-04.
func (s *RepoMapSkill) execGetContext(args map[string]interface{}) (string, error) {
	// Extract and validate files parameter.
	filesRaw, ok := args["files"]
	if !ok {
		return "", serr.New(serr.InvalidArgs, "'files' parameter is required").WithTool("get_context")
	}

	filesArr, ok := filesRaw.([]interface{})
	if !ok {
		return "", serr.New(serr.InvalidArgs, "'files' parameter must be an array of strings").WithTool("get_context")
	}
	if len(filesArr) == 0 {
		return "", serr.New(serr.InvalidArgs, "'files' parameter must not be empty").WithTool("get_context")
	}

	files := make([]string, 0, len(filesArr))
	for _, f := range filesArr {
		fp, ok := f.(string)
		if !ok {
			return "", serr.New(serr.InvalidArgs, "'files' elements must be strings").WithTool("get_context")
		}
		// T-28-05: path traversal prevention.
		if strings.Contains(fp, "..") {
			s.logger.Warn("rejected file path with path traversal", "path", fp)
			return "", serr.New(serr.InvalidArgs, fmt.Sprintf("file path %q contains '..' (path traversal not allowed)", fp)).WithTool("get_context")
		}
		files = append(files, fp)
	}

	budget := extractTokenBudget(args, defaultContextBudget)

	// task_description is accepted but reserved per D-08.
	if desc, ok := args["task_description"].(string); ok && desc != "" {
		s.logger.Debug("get_context task_description received (reserved for future use)", "description", desc)
	}

	if err := s.ensureGraph(); err != nil {
		return "", serr.Wrap(serr.Internal, "building reference graph", err).WithTool("get_context")
	}

	// Build personalization map: seed files get weight 1.0 per D-04.
	personalization := make(map[string]float64, len(files))
	for _, fp := range files {
		personalization[fp] = 1.0
	}

	ranked := s.graph.RankFiles(0.85, personalization)
	if len(ranked) == 0 {
		return "No files found in repository.", nil
	}

	output := s.renderBudgeted(ranked, budget)
	if output == "" {
		return "No files found in repository.", nil
	}

	return output, nil
}

// ensureGraph lazily builds the file graph with dirty-flag caching.
// Rebuilds only when TagCache.Version() changes.
func (s *RepoMapSkill) ensureGraph() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ver := s.cache.Version()
	if s.graph != nil && s.graphVer == ver {
		return nil // Graph is current.
	}

	graph, err := repomap.BuildGraph(s.cache)
	if err != nil {
		return err
	}
	s.graph = graph
	s.graphVer = ver
	return nil
}

// resolveRoot returns the workspace root directory.
// Falls back to os.Getwd() if rootDir is not set.
func (s *RepoMapSkill) resolveRoot() string {
	if s.rootDir != "" {
		return s.rootDir
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// renderBudgeted renders ranked files within the token budget using binary search
// on file count per D-13. Token estimation: chars/4 per D-12.
func (s *RepoMapSkill) renderBudgeted(ranked []repomap.RankedFile, budget int) string {
	if len(ranked) == 0 {
		return ""
	}

	lower, upper := 1, len(ranked)
	bestOutput := ""
	bestTokens := 0

	for lower <= upper {
		mid := (lower + upper) / 2
		output := s.renderTree(ranked[:mid])
		tokens := len(output) / 4 // D-12: chars/4 estimation

		if tokens <= budget && tokens > bestTokens {
			bestOutput = output
			bestTokens = tokens
		}

		if tokens < budget {
			lower = mid + 1
		} else {
			upper = mid - 1
		}
	}

	return bestOutput
}

// renderTree renders a set of ranked files as an indented tree with elided symbols.
func (s *RepoMapSkill) renderTree(ranked []repomap.RankedFile) string {
	if len(ranked) == 0 {
		return ""
	}

	root := s.resolveRoot()

	// Sort by path for tree grouping.
	sorted := make([]repomap.RankedFile, len(ranked))
	copy(sorted, ranked)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Path < sorted[j].Path
	})

	var buf bytes.Buffer
	for _, rf := range sorted {
		// Make path relative to workspace root.
		relPath := rf.Path
		if root != "" {
			if rel, err := filepath.Rel(root, rf.Path); err == nil {
				relPath = rel
			}
		}

		buf.WriteString(relPath)
		buf.WriteByte('\n')

		// Try to render elided symbols for this file.
		elided := s.elideFile(rf.Path)
		if elided != "" {
			// Indent each line of elided output.
			for _, line := range strings.Split(elided, "\n") {
				buf.WriteString("  ")
				buf.WriteString(line)
				buf.WriteByte('\n')
			}
		}
	}

	return buf.String()
}

// elideFile reads a file and renders its elided symbols using the ElisionRenderer.
func (s *RepoMapSkill) elideFile(filePath string) string {
	source, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	// Determine language from file extension.
	lang := langFromExt(filepath.Ext(filePath))
	if lang == "" {
		return ""
	}

	// Get cached tags for this file.
	tags, err := s.cache.AllFiles()
	if err != nil {
		return ""
	}
	fileTags, ok := tags[filePath]
	if !ok {
		return ""
	}

	return s.elider.RenderFile(source, lang, fileTags)
}

// langFromExt maps file extensions to tree-sitter language names.
func langFromExt(ext string) string {
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
	case ".rs":
		return "rust"
	default:
		return ""
	}
}

// extractTokenBudget extracts and validates the token_budget parameter from args.
// Per T-28-06: cap at 32768, minimum 64.
func extractTokenBudget(args map[string]interface{}, defaultVal int) int {
	raw, ok := args["token_budget"]
	if !ok {
		return defaultVal
	}

	// JSON numbers arrive as float64.
	f, ok := raw.(float64)
	if !ok {
		return defaultVal
	}

	budget := int(f)
	if budget < minTokenBudget {
		budget = minTokenBudget
	}
	if budget > maxTokenBudget {
		budget = maxTokenBudget
	}
	return budget
}
