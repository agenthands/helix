// Package repomap implements the repomap skill, contributing 2 MCP tools
// for repository structure mapping and task-focused context retrieval.
package repomap

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/guardrails"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/repomap"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/skill"
	"github.com/agenthands/helix/internal/treesitter"
	"github.com/agenthands/helix/internal/workspace"
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
	cache          *repomap.TagCache
	extractor      *repomap.TagExtractor
	renderer       *repomap.TreeRenderer // nil until ensureCache sets rootDir
	elider         *repomap.ElisionRenderer
	graph          *repomap.FileGraph
	graphVer       int64 // TagCache version when graph was last built
	mu             sync.Mutex
	logger         *slog.Logger
	rootDir        string // workspace root, resolved lazily via os.Getwd if empty
	cachePopulated bool
	enrichFn       func(graph *repomap.FileGraph) // optional LSP enrichment callback
	registry       *treesitter.GrammarRegistry
	fallbackDeps   *FallbackDeps

	// metrics is the optional MetricsSink wired by the daemon at startup
	// (Phase 53 D-15). Defaults to repomap.NoopSink{} so a never-wired
	// skill is safe. Used by walkAndExtract's GetOrExtract dispatcher to
	// observe per-extractor latency (Q-2 Option 2 — extractor type is
	// known here, not in cache.go).
	metrics repomap.MetricsSink

	// semanticLookup is the optional Phase 65 strangler-fig seam
	// (INTEG-01 / INTEG-02). When wired and Available(), get_repo_map /
	// get_context delegate ranking to lookup.RankFiles / RankFromSeeds and
	// stamp source="semantic" on the envelope; otherwise the existing v1.9
	// tree-sitter + PageRank path runs and stamps source="tree_sitter" or
	// source="fallback" per ChooseSource (Pitfall §3 priority ladder).
	// nil-normalized to integ.NoopLookup{} via the lookup() accessor.
	semanticLookup integ.SemanticLookup

	// cfgGate is the daemon-side ConfigGate (small interface — just
	// SemanticIndexEnabled()). Wired by SetConfigGate at daemon post-init.
	// nil-tolerant: ChooseSource treats nil as "feature off" → SourceTreeSitter.
	cfgGate integ.ConfigGate
}

// FallbackDeps holds dependencies for LSP-based fallback tag extraction.
// Wired by the daemon after kernel creation via SetFallbackDeps.
type FallbackDeps struct {
	AcquireFn func(ctx context.Context, lang string) (repomap.SymbolRequester, func(), error)
	Extractor *repomap.FallbackExtractor
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

	// Default metrics sink to a no-op so a never-wired skill is safe.
	// Daemon post-init (12d) overwrites this with *obs.Metrics.
	s.metrics = repomap.NoopSink{}

	return nil
}

// SetWorkspaceRoot updates the workspace root directory for tag extraction.
// Called by the daemon when activate_project sets a new workspace.
// Invalidates the cache so the next tool call re-walks the workspace.
func (s *RepoMapSkill) SetWorkspaceRoot(root string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rootDir = root
	s.cachePopulated = false
	s.renderer = nil // recreate with new rootDir on next use
}

// Cache returns the underlying TagCache for external consumers (e.g., LSP enrichment).
func (s *RepoMapSkill) Cache() *repomap.TagCache {
	return s.cache
}

// SetEnrichFn sets the optional LSP enrichment callback.
// Called by the daemon after kernel creation to enable cross-file LSP references.
func (s *RepoMapSkill) SetEnrichFn(fn func(graph *repomap.FileGraph)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enrichFn = fn
}

// HasEnrichFn reports whether an LSP enrichment callback is currently
// installed. Used by the Phase 76 no_lsp daemon wiring test to assert that
// SetEnrichFn was structurally skipped under disable_lsp_subsystem (D-09).
func (s *RepoMapSkill) HasEnrichFn() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enrichFn != nil
}

// HasFallbackDeps reports whether the pool-leasing fallback extraction
// dependencies are wired. Used by the Phase 76 no_lsp daemon wiring test to
// assert the D-10-audited repomap-fallback AcquireFn lease path is not wired
// under disable_lsp_subsystem.
func (s *RepoMapSkill) HasFallbackDeps() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fallbackDeps != nil
}

// SetFallbackDeps sets the fallback extraction dependencies for languages without tree-sitter grammars.
// Called by the daemon after kernel creation to enable LSP documentSymbol fallback (D-33-01).
func (s *RepoMapSkill) SetFallbackDeps(deps *FallbackDeps) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fallbackDeps = deps
}

// SetMetricsSink wires the repomap.MetricsSink for per-extractor latency
// observation (Q-2 Option 2). Called from internal/daemon/daemon.go post-init
// (block 12d). Phase 53 D-15. A nil argument is normalized to NoopSink{} so
// the dispatcher's emission sites never need nil-checks.
//
// Decision (Plan 53-03 setter pattern): adding the sink as a setter rather
// than a constructor argument matches the existing SetEnrichFn / SetFallbackDeps
// pattern and keeps daemon post-init wiring uniform.
func (s *RepoMapSkill) SetMetricsSink(sink repomap.MetricsSink) {
	if sink == nil {
		sink = repomap.NoopSink{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics = sink
}

// SetSemanticLookup wires the integ.SemanticLookup seam (Phase 65 65-05,
// INTEG-01 / INTEG-02 / INTEG-05). Joins the SetEnrichFn / SetFallbackDeps /
// SetMetricsSink family at daemon post-init. nil-tolerant: a nil argument
// is the equivalent of "never wired" — the lookup() accessor normalizes the
// nil to integ.NoopLookup{} so the priority-ladder gate at ChooseSource
// always sees a valid SemanticLookup interface value.
//
// Threat T-65-05-04 (M-readtier) disposition: the production lookup adapter
// is read-only (grep canary at internal/daemon/integ_lookup_test.go); the
// skill never reaches snapshot-write surfaces through this setter.
func (s *RepoMapSkill) SetSemanticLookup(lookup integ.SemanticLookup) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.semanticLookup = lookup
}

// SetConfigGate wires the daemon-side ConfigGate (Phase 65 65-05). The
// ChooseSource priority ladder consults cfg.SemanticIndexEnabled() FIRST
// (D-04 / Pitfall §3); a nil cfg is treated as "feature off" so the steady-
// state v1.9 path renders source="tree_sitter".
func (s *RepoMapSkill) SetConfigGate(cfg integ.ConfigGate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfgGate = cfg
}

// lookup returns the wired SemanticLookup, normalizing nil to
// integ.NoopLookup{} so the priority-ladder gate at ChooseSource always sees
// a valid interface value. Mirrors metricsSink()'s nil-normalizer pattern.
func (s *RepoMapSkill) lookup() integ.SemanticLookup {
	s.mu.Lock()
	l := s.semanticLookup
	s.mu.Unlock()
	if l == nil {
		return integ.NoopLookup{}
	}
	return l
}

// configGate returns the wired ConfigGate (or nil). ChooseSource is
// nil-tolerant; returning nil here is the equivalent of "feature off".
func (s *RepoMapSkill) configGate() integ.ConfigGate {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfgGate
}

// metricsSink returns the wired sink, normalizing nil to NoopSink{}. Used
// by walkAndExtract so test-constructed skills (e.g. &RepoMapSkill{}, which
// bypasses Init) never panic on a nil dereference at the emission site.
func (s *RepoMapSkill) metricsSink() repomap.MetricsSink {
	s.mu.Lock()
	sink := s.metrics
	s.mu.Unlock()
	if sink == nil {
		return repomap.NoopSink{}
	}
	return sink
}

// SetRegistry injects the canonical GrammarRegistry constructed at daemon bootstrap
// and lazily builds registry-dependent renderers/extractors. Called by the daemon
// post-init (BUG-04, D-01/D-02/D-03). Idempotent and nil-safe.
func (s *RepoMapSkill) SetRegistry(registry *treesitter.GrammarRegistry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if registry == nil || s.registry != nil {
		return
	}
	s.registry = registry
	s.elider = repomap.NewElisionRenderer(registry)
	if extractor, err := repomap.NewTagExtractor(registry); err == nil {
		s.extractor = extractor
	} else {
		s.logger.Warn("tag extractor creation failed, tree-sitter extraction disabled", "error", err)
	}
}

// GetRepoMapSkill returns the registered RepoMapSkill instance for post-init wiring.
// Returns nil if the skill has not been registered.
func GetRepoMapSkill() *RepoMapSkill {
	s, ok := skill.Get("repomap")
	if !ok {
		return nil
	}
	rs, ok := s.(*RepoMapSkill)
	if !ok {
		return nil
	}
	return rs
}

// Tools returns the 2 MCP tool definitions for repomap operations.
func (s *RepoMapSkill) Tools() []*mcp.ToolDef {
	return []*mcp.ToolDef{
		s.getRepoMapTool(),
		s.getContextTool(),
	}
}

// --- help text constants ---

const getRepoMapHelp = `## Usage Examples

Get a ranked overview of the repository:
  get_repo_map()

Get a larger overview with more detail:
  get_repo_map(token_budget=8192)

## Common Patterns
- Default token_budget is 4096; increase for more detail, decrease for a quick overview
- Files are ranked by structural importance using PageRank
- Use at the start of a session to understand the codebase structure`

const getContextHelp = `## Usage Examples

Get context relevant to specific files:
  get_context(files=["src/auth/login.go", "src/models/user.go"])

Get more context with a larger budget:
  get_context(files=["src/server.go"], token_budget=4096)

## Common Patterns
- Pass the files you are currently working with for personalized ranking
- Symbols are ranked by relevance to the specified files
- Use to discover related code when working on a feature`

func (s *RepoMapSkill) getRepoMapTool() *mcp.ToolDef {
	return &mcp.ToolDef{
		Name: "get_repo_map",
		Description: "Get a ranked structural overview of the repository. Returns file paths organized " +
			"as a tree with elided symbol signatures, ranked by structural importance (PageRank). " +
			"Use this to understand the overall codebase structure. The token_budget parameter " +
			"controls the maximum output size (in approximate tokens).",
		BriefDescription: "Get a ranked overview of repository structure and key files",
		HelpText:         getRepoMapHelp,
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
		BriefDescription: "Get relevant code context for a set of files",
		HelpText:         getContextHelp,
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
//
// Phase 65 65-05 strangler-fig integration (INTEG-01 + INTEG-05):
//
//  1. ChooseSource consults the wired ConfigGate / SemanticLookup via the
//     priority ladder (Pitfall §3): config off → SourceTreeSitter; cfg on
//     but lookup unavailable → SourceFallback + index_disabled (defensive
//     D-05); cfg on + lookup available → semantic path (or SourceFallback +
//     classified reason on err).
//  2. Tree text comes from EITHER the lookup (semantic path: RankFiles +
//     adaptRankedFiles → renderer) OR the v1.9 path (renderV19); the
//     existing TreeRenderer.RenderBudgeted is the only renderer in either
//     branch (INTEG-01: zero source change to internal/repomap engine).
//  3. Output is JSON-wrapped via integ.MarshalEnvelope (Pitfall §2): the
//     tree text is preserved verbatim under "tree", the envelope adds
//     source / fallback_reason / graph_version / freshness.
func (s *RepoMapSkill) execGetRepoMap(args map[string]interface{}) (string, error) {
	budget := extractTokenBudget(args, defaultRepoMapBudget)
	ctx := context.Background()
	ws := s.workspaceKey()

	cfg := s.configGate()
	lookup := s.lookup()
	src, reason := integ.ChooseSource(cfg, lookup, nil)

	var (
		treeText string
		graphVer uint64
	)
	switch src {
	case integ.SourceSemantic:
		ranked, lerr := lookup.RankFiles(ctx, ws)
		if lerr != nil {
			// Reclassify: semantic-on err → SourceFallback + classified reason.
			src, reason = integ.ChooseSource(cfg, lookup, lerr)
			text, v19Err := s.renderV19(budget, nil)
			if v19Err != nil {
				return "", serr.Wrap(serr.Internal, "building reference graph", v19Err).WithTool("get_repo_map")
			}
			treeText = text
		} else {
			adapted := adaptRankedFiles(ranked)
			if err := s.ensureRenderer(); err != nil {
				return "", serr.Wrap(serr.Internal, "preparing renderer", err).WithTool("get_repo_map")
			}
			treeText = s.renderer.RenderBudgeted(adapted, budget)
			// Capture graph_version from one of the ranked entries
			// (RankedFile.GraphVersion is per-call stable from the Phase 62
			// D-07 contract); fall back to lookup.Status if the ranked list
			// is empty.
			if len(ranked) > 0 {
				graphVer = ranked[0].GraphVersion
			} else if status, sErr := lookup.Status(ctx, ws); sErr == nil {
				graphVer = status.GraphVersion
			}
		}
	default:
		// SourceTreeSitter or SourceFallback (defensive index_disabled): run
		// the v1.9 path. Plain "fallback" classification has no graph_version.
		text, v19Err := s.renderV19(budget, nil)
		if v19Err != nil {
			return "", serr.Wrap(serr.Internal, "building reference graph", v19Err).WithTool("get_repo_map")
		}
		treeText = text
	}

	if treeText == "" {
		treeText = "No files found in repository."
	}

	env := integ.Envelope{
		Source:         src,
		FallbackReason: reason,
		GraphVersion:   graphVer,
		Freshness:      computeFreshness(ctx, lookup, ws),
	}
	out, mErr := integ.MarshalEnvelope(env, map[string]any{"tree": treeText})
	if mErr != nil {
		return "", serr.Wrap(serr.Internal, "marshaling envelope", mErr).WithTool("get_repo_map")
	}
	// Phase 66 D-01 GUARD-03: issue receipt on success path ONLY.
	guardrails.IssueReceiptOnSuccess(ctx, guardrails.ClassStructuralOverview,
		guardrails.StructuralOverviewScope{
			RootPath:  s.resolveRoot(),
			Depth:     0,
			FileCount: 0,
			MaxTokens: budget,
		}, "get_repo_map")
	return string(out), nil
}

// execGetContext handles the get_context tool: personalized PageRank per D-04.
//
// Phase 65 65-05 strangler-fig integration (INTEG-02 + INTEG-05): mirrors
// execGetRepoMap's source-selection ladder, but the semantic arm calls
// lookup.RankFromSeeds(ctx, ws, files) instead of RankFiles. Output is JSON-
// wrapped (Pitfall §2). Same INTEG-01 contract — internal/repomap untouched.
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
		if filepath.IsAbs(fp) && !strings.HasPrefix(fp, s.resolveRoot()) {
			s.logger.Warn("rejected absolute file path outside workspace root", "path", fp)
			return "", serr.New(serr.InvalidArgs, fmt.Sprintf("file path %q is outside workspace root", fp)).WithTool("get_context")
		}
		files = append(files, fp)
	}

	budget := extractTokenBudget(args, defaultContextBudget)

	// task_description is accepted but reserved per D-08.
	if desc, ok := args["task_description"].(string); ok && desc != "" {
		s.logger.Debug("get_context task_description received (reserved for future use)", "description", desc)
	}

	ctx := context.Background()
	ws := s.workspaceKey()

	cfg := s.configGate()
	lookup := s.lookup()
	src, reason := integ.ChooseSource(cfg, lookup, nil)

	var (
		treeText string
		graphVer uint64
	)
	switch src {
	case integ.SourceSemantic:
		ranked, lerr := lookup.RankFromSeeds(ctx, ws, files)
		if lerr != nil {
			src, reason = integ.ChooseSource(cfg, lookup, lerr)
			text, v19Err := s.renderV19(budget, files)
			if v19Err != nil {
				return "", serr.Wrap(serr.Internal, "building reference graph", v19Err).WithTool("get_context")
			}
			treeText = text
		} else {
			adapted := adaptRankedFiles(ranked)
			if err := s.ensureRenderer(); err != nil {
				return "", serr.Wrap(serr.Internal, "preparing renderer", err).WithTool("get_context")
			}
			treeText = s.renderer.RenderBudgeted(adapted, budget)
			if len(ranked) > 0 {
				graphVer = ranked[0].GraphVersion
			} else if status, sErr := lookup.Status(ctx, ws); sErr == nil {
				graphVer = status.GraphVersion
			}
		}
	default:
		text, v19Err := s.renderV19(budget, files)
		if v19Err != nil {
			return "", serr.Wrap(serr.Internal, "building reference graph", v19Err).WithTool("get_context")
		}
		treeText = text
	}

	if treeText == "" {
		treeText = "No files found in repository."
	}

	env := integ.Envelope{
		Source:         src,
		FallbackReason: reason,
		GraphVersion:   graphVer,
		Freshness:      computeFreshness(ctx, lookup, ws),
	}
	out, mErr := integ.MarshalEnvelope(env, map[string]any{"tree": treeText})
	if mErr != nil {
		return "", serr.Wrap(serr.Internal, "marshaling envelope", mErr).WithTool("get_context")
	}
	// Phase 66 D-01 GUARD-03: issue receipt on success path ONLY.
	guardrails.IssueReceiptOnSuccess(ctx, guardrails.ClassContextGathered,
		guardrails.ContextGatheredScope{
			FileSet:         files,
			TargetSymbols:   nil,
			TaskHash:        computeContextTaskHash(files),
			TokenBudgetUsed: len(treeText) / 4, // conservative token estimate
			MaxTokens:       budget,
		}, "get_context")
	return string(out), nil
}

// renderV19 runs the v1.9 tree-sitter + PageRank path. When seedFiles is
// non-nil and non-empty the call uses Personalized PageRank (one-weight on
// each seed); otherwise uniform PageRank. Extracted from the previous
// execGetRepoMap / execGetContext bodies so the strangler-fig branches can
// share the v1.9 rendering pipeline without duplicating the ensureGraph +
// RankFiles + RenderBudgeted block.
func (s *RepoMapSkill) renderV19(budget int, seedFiles []string) (string, error) {
	if err := s.ensureGraph(); err != nil {
		return "", err
	}
	var personalization map[string]float64
	if len(seedFiles) > 0 {
		personalization = make(map[string]float64, len(seedFiles))
		for _, fp := range seedFiles {
			personalization[fp] = 1.0
		}
	}
	ranked := s.graph.RankFiles(0.85, personalization)
	if len(ranked) == 0 {
		return "", nil
	}
	return s.renderer.RenderBudgeted(ranked, budget), nil
}

// ensureRenderer guarantees s.renderer is constructed for the semantic-on
// branches that bypass ensureCache (the lookup provides the ranked file list,
// so the cache walk is unnecessary on the semantic path). The renderer is
// created lazily because rootDir may be set after Init via SetWorkspaceRoot.
func (s *RepoMapSkill) ensureRenderer() error {
	s.mu.Lock()
	if s.renderer != nil {
		s.mu.Unlock()
		return nil
	}
	root := s.resolveRoot()
	s.mu.Unlock()
	if root == "" || root == "." {
		return serr.New(serr.Internal, "workspace root not set; call activate_project first")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.renderer != nil {
		return nil
	}
	s.renderer = repomap.NewTreeRenderer(s.elider, s.cache, root)
	return nil
}

// workspaceKey returns the WorkspaceKey for the current rootDir. Single-
// workspace daemons (Phase 65 baseline) return a key with RepoRoot set;
// multi-workspace expansion will swap this for a real registry lookup
// without changing the strangler-fig call sites.
func (s *RepoMapSkill) workspaceKey() workspace.WorkspaceKey {
	return workspace.WorkspaceKey{RepoRoot: s.resolveRoot()}
}

// skipDirs contains directory names to skip during workspace walk.
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "__pycache__": true, ".helix": true,
	"vendor": true, ".venv": true, "dist": true, "build": true,
}

// ensureCache lazily walks the workspace and populates TagCache.
// Called before graph building on each tool execution.
// Uses check-lock-check to avoid holding mu during the slow walkAndExtract.
func (s *RepoMapSkill) ensureCache() error {
	s.mu.Lock()
	if s.cachePopulated {
		s.mu.Unlock()
		return nil
	}
	root := s.resolveRoot()
	s.mu.Unlock()

	if root == "" || root == "." {
		return serr.New(serr.Internal, "workspace root not set; call activate_project first")
	}
	if err := s.walkAndExtract(context.Background(), root); err != nil {
		return serr.Wrap(serr.Internal, "walking workspace for tag extraction", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cachePopulated = true
	// Create TreeRenderer now that we know rootDir.
	s.renderer = repomap.NewTreeRenderer(s.elider, s.cache, root)
	return nil
}

// walkAndExtract walks the workspace root and populates TagCache using TagExtractor.
func (s *RepoMapSkill) walkAndExtract(ctx context.Context, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip entries with errors
		}
		if d.IsDir() && skipDirs[d.Name()] {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		// T-30-02: skip symlinks to prevent symlink escape.
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}

		lang := repomap.LangFromExt(path)
		if lang == "" {
			return nil // skip unsupported file types
		}

		// Resolve the sink once per file so the closure captures a stable
		// reference even if SetMetricsSink races with walkAndExtract.
		sink := s.metricsSink()

		_, extractErr := s.cache.GetOrExtract(path, func() ([]repomap.Tag, error) {
			// Q-2 Option 2: extract latency observation lives here, not in
			// internal/repomap/cache.go, because the extractor type is only
			// known at this dispatch layer. The cache emits the hit/miss
			// counter (D-03); this dispatcher emits the latency histogram
			// (D-05/D-06/D-07). Observe AFTER the call so failed extractions
			// (returning err) still record their latency.
			//
			// Primary path: tree-sitter extraction
			if s.extractor != nil && (s.registry == nil || s.registry.SupportsLanguage(lang)) {
				start := time.Now()
				source, readErr := os.ReadFile(path)
				if readErr != nil {
					sink.RepoMapExtractObserve(lang, repomap.ExtractorTreesitter, time.Since(start).Seconds())
					return nil, readErr
				}
				tags, tagErr := s.extractor.Extract(source, path, lang)
				sink.RepoMapExtractObserve(lang, repomap.ExtractorTreesitter, time.Since(start).Seconds())
				if tagErr != nil {
					// Log and skip: unsupported language for this extractor is not fatal.
					s.logger.Debug("tag extraction skipped", "path", path, "lang", lang, "error", tagErr)
					return nil, nil
				}
				return tags, nil
			}

			// Fallback path: LSP documentSymbol for languages without tree-sitter grammars (D-33-01)
			if s.fallbackDeps != nil && s.fallbackDeps.AcquireFn != nil {
				start := time.Now()
				requester, release, acqErr := s.fallbackDeps.AcquireFn(ctx, lang)
				if acqErr != nil {
					sink.RepoMapExtractObserve(lang, repomap.ExtractorLSP, time.Since(start).Seconds())
					// D-33-02: missing LSP is debug log + empty tags, not an error
					s.logger.Debug("fallback extraction skipped: no LS available", "path", path, "lang", lang, "error", acqErr)
					return nil, nil
				}
				defer release()
				uri := "file://" + path
				tags, fbErr := s.fallbackDeps.Extractor.Extract(ctx, requester, path, uri)
				sink.RepoMapExtractObserve(lang, repomap.ExtractorLSP, time.Since(start).Seconds())
				if fbErr != nil {
					s.logger.Debug("fallback extraction failed", "path", path, "lang", lang, "error", fbErr)
					return nil, nil
				}
				return tags, nil
			}

			// No extractor available for this language — third "fallback" branch
			// per D-07 (extractor=="fallback"). Still observe so operators can
			// see how often files in unsupported languages reach this branch.
			start := time.Now()
			sink.RepoMapExtractObserve(lang, repomap.ExtractorFallback, time.Since(start).Seconds())
			return nil, nil
		})
		if extractErr != nil {
			s.logger.Debug("cache population skipped", "path", path, "error", extractErr)
		}
		return nil // never abort walk on individual file errors
	})
}

// ensureGraph lazily builds the file graph with dirty-flag caching.
// Rebuilds only when TagCache.Version() changes.
func (s *RepoMapSkill) ensureGraph() error {
	if err := s.ensureCache(); err != nil {
		return err
	}

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

	// Opportunistic LSP enrichment (RMAP-08).
	if s.enrichFn != nil {
		s.enrichFn(graph)
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

// computeContextTaskHash returns a short hex hash of the canonical file set
// for use as the TaskHash in a ContextGatheredScope receipt.
// sha256 of sorted file paths joined with NUL, truncated to 16 hex chars.
func computeContextTaskHash(files []string) string {
	h := sha256.New()
	for _, f := range files {
		h.Write([]byte(f))
		h.Write([]byte{0})
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}
