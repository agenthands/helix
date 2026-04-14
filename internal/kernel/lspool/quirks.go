package lspool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/postfix/serena/internal/langregistry"
	gen "github.com/postfix/serena/protocol/gen"
)

// ArgsModifier is an optional interface that QuirkAdapters can implement
// to inject extra command-line arguments when starting the LS process.
type ArgsModifier interface {
	ExtraArgs(workDir string, args []string) []string
}

// QuirkAdapter provides per-language behavioral hooks for LS workers.
// Languages with no special behavior use DefaultQuirkAdapter.
type QuirkAdapter interface {
	// InitOptions returns language-specific initialization options.
	// workDir is the workspace root for workspace-dependent options.
	InitOptions(workDir string) map[string]any
	// NotificationHandlers returns handlers for LS-specific notifications.
	NotificationHandlers() map[string]func(params json.RawMessage)
	// NormalizeSymbolName adjusts symbol names for language conventions.
	NormalizeSymbolName(name string) string
	// PostInitialize is called after successful LSP initialize handshake.
	PostInitialize(ctx context.Context, adapter *LSAdapter) error
}

// DefaultQuirkAdapter is a no-op implementation for low-quirk languages.
// It returns the entry's InitOptions unchanged and provides identity symbol normalization.
type DefaultQuirkAdapter struct {
	Entry langregistry.LSEntry
}

func (d *DefaultQuirkAdapter) InitOptions(_ string) map[string]any {
	return d.Entry.InitOptions
}

func (d *DefaultQuirkAdapter) NotificationHandlers() map[string]func(params json.RawMessage) {
	return nil
}

func (d *DefaultQuirkAdapter) NormalizeSymbolName(name string) string {
	return name
}

func (d *DefaultQuirkAdapter) PostInitialize(_ context.Context, _ *LSAdapter) error {
	return nil
}

// GoplsAdapter provides Go-specific quirks for gopls.
// Adds experimentalWorkspaceModule to init options and strips package prefixes from symbols.
type GoplsAdapter struct {
	Entry langregistry.LSEntry
}

func (g *GoplsAdapter) InitOptions(_ string) map[string]any {
	opts := make(map[string]any)
	for k, v := range g.Entry.InitOptions {
		opts[k] = v
	}
	opts["experimentalWorkspaceModule"] = true
	return opts
}

func (g *GoplsAdapter) NotificationHandlers() map[string]func(params json.RawMessage) {
	return nil
}

// NormalizeSymbolName strips package prefix from Go symbols (e.g. "pkg.Foo" -> "Foo").
func (g *GoplsAdapter) NormalizeSymbolName(name string) string {
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

func (g *GoplsAdapter) PostInitialize(_ context.Context, _ *LSAdapter) error {
	return nil
}

// RustAnalyzerAdapter provides Rust-specific quirks for rust-analyzer.
// Enables cargo build scripts in initialization options.
type RustAnalyzerAdapter struct {
	Entry langregistry.LSEntry
}

func (r *RustAnalyzerAdapter) InitOptions(_ string) map[string]any {
	opts := make(map[string]any)
	for k, v := range r.Entry.InitOptions {
		opts[k] = v
	}
	// Ensure cargo buildScripts are enabled.
	if _, ok := opts["cargo"]; !ok {
		opts["cargo"] = map[string]any{
			"buildScripts": map[string]any{
				"enable": true,
			},
		}
	}
	return opts
}

func (r *RustAnalyzerAdapter) NotificationHandlers() map[string]func(params json.RawMessage) {
	return nil
}

func (r *RustAnalyzerAdapter) NormalizeSymbolName(name string) string {
	return name
}

func (r *RustAnalyzerAdapter) PostInitialize(_ context.Context, _ *LSAdapter) error {
	return nil
}

// ClangdAdapter provides C/C++ quirks for clangd.
// Detects compile_commands.json in the workspace.
type ClangdAdapter struct {
	Entry langregistry.LSEntry
}

func (c *ClangdAdapter) InitOptions(workDir string) map[string]any {
	opts := make(map[string]any)
	for k, v := range c.Entry.InitOptions {
		opts[k] = v
	}
	// Check for compile_commands.json in common locations.
	for _, dir := range []string{"", "build", "cmake-build-debug", "cmake-build-release"} {
		candidate := filepath.Join(workDir, dir, "compile_commands.json")
		if _, err := os.Stat(candidate); err == nil {
			opts["compilationDatabasePath"] = filepath.Join(workDir, dir)
			break
		}
	}
	return opts
}

func (c *ClangdAdapter) NotificationHandlers() map[string]func(params json.RawMessage) {
	return nil
}

func (c *ClangdAdapter) NormalizeSymbolName(name string) string {
	return name
}

func (c *ClangdAdapter) PostInitialize(ctx context.Context, adapter *LSAdapter) error {
	// clangd requires textDocument/didOpen before any textDocument/* operations.
	// Without this, documentSymbol/references/hover fail with "trying to get AST for non-added document".
	didOpenFirstFile(ctx, adapter, ".cpp", "cpp")
	didOpenFirstFile(ctx, adapter, ".c", "c")
	return nil
}

// JdtlsAdapter provides Java-specific quirks for Eclipse JDT Language Server.
// Creates workspace data directory and configures jdtls-specific init options.
type JdtlsAdapter struct {
	Entry langregistry.LSEntry
}

func (j *JdtlsAdapter) InitOptions(workDir string) map[string]any {
	opts := make(map[string]any)
	for k, v := range j.Entry.InitOptions {
		opts[k] = v
	}
	return opts
}

func (j *JdtlsAdapter) NotificationHandlers() map[string]func(params json.RawMessage) {
	return nil
}

func (j *JdtlsAdapter) NormalizeSymbolName(name string) string {
	return name
}

// PostInitialize opens a Java file so jdtls indexes the workspace.
func (j *JdtlsAdapter) PostInitialize(ctx context.Context, adapter *LSAdapter) error {
	didOpenFirstFile(ctx, adapter, ".java", "java")
	return nil
}

// ExtraArgs injects -data <dir> so jdtls has a workspace-specific data directory.
// Without this, jdtls may fail to index or conflict across workspaces.
func (j *JdtlsAdapter) ExtraArgs(workDir string, args []string) []string {
	dataDir := filepath.Join(workDir, ".jdtls-data")
	_ = os.MkdirAll(dataDir, 0o755)
	return append([]string{"-data", dataDir}, args...)
}

// VueAdapter provides Vue-specific quirks.
// Vue language servers often require a companion TypeScript server.
type VueAdapter struct {
	Entry langregistry.LSEntry
}

func (v *VueAdapter) InitOptions(_ string) map[string]any {
	opts := make(map[string]any)
	for k, v := range v.Entry.InitOptions {
		opts[k] = v
	}
	return opts
}

func (v *VueAdapter) NotificationHandlers() map[string]func(params json.RawMessage) {
	return nil
}

func (v *VueAdapter) NormalizeSymbolName(name string) string {
	return name
}

func (v *VueAdapter) PostInitialize(_ context.Context, _ *LSAdapter) error {
	return nil
}

// didOpenFirstFile is a shared helper for LS implementations that require a file
// to be opened via textDocument/didOpen before workspace/symbol works.
// tsserver needs it to create a "project", pyright needs it to index workspace files.
func didOpenFirstFile(ctx context.Context, adapter *LSAdapter, ext string, langID string) {
	workDir := adapter.worker.workDir
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ext) {
			continue
		}
		filePath := filepath.Join(workDir, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		uri := "file://" + filePath
		params := gen.DidOpenTextDocumentParams{
			TextDocument: gen.TextDocumentItem{
				URI:        uri,
				LanguageId: langID,
				Version:    1,
				Text:       string(content),
			},
		}
		_ = adapter.worker.Notify(ctx, "textDocument/didOpen", params)
		return
	}
}

// didOpenFirstFileRecursive walks subdirectories to find and open the first file
// matching ext. Used for languages where source files live in subdirectories
// (e.g., Rust src/, Java src/main/java/).
func didOpenFirstFileRecursive(ctx context.Context, adapter *LSAdapter, ext string, langID string) {
	workDir := adapter.worker.workDir
	_ = filepath.WalkDir(workDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ext) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		uri := "file://" + path
		params := gen.DidOpenTextDocumentParams{
			TextDocument: gen.TextDocumentItem{
				URI:        uri,
				LanguageId: langID,
				Version:    1,
				Text:       string(content),
			},
		}
		_ = adapter.worker.Notify(ctx, "textDocument/didOpen", params)
		return filepath.SkipAll // stop after first file
	})
}

// TypeScriptAdapter provides TypeScript-specific quirks for typescript-language-server.
// tsserver only creates a "project" after a file is opened via didOpen.
// Without this, workspace/symbol fails with "No Project" until a file is opened.
type TypeScriptAdapter struct {
	Entry langregistry.LSEntry
}

func (t *TypeScriptAdapter) InitOptions(_ string) map[string]any { return t.Entry.InitOptions }
func (t *TypeScriptAdapter) NotificationHandlers() map[string]func(params json.RawMessage) {
	return nil
}
func (t *TypeScriptAdapter) NormalizeSymbolName(name string) string { return name }
func (t *TypeScriptAdapter) PostInitialize(ctx context.Context, adapter *LSAdapter) error {
	// Try .ts first, fall back to .js for JavaScript-only projects.
	didOpenFirstFile(ctx, adapter, ".ts", "typescript")
	didOpenFirstFile(ctx, adapter, ".js", "javascript")
	return nil
}

// PyrightAdapter provides Python-specific quirks for pyright-langserver.
// Pyright needs a file opened via didOpen before workspace/symbol returns results.
type PyrightAdapter struct {
	Entry langregistry.LSEntry
}

func (p *PyrightAdapter) InitOptions(_ string) map[string]any { return p.Entry.InitOptions }
func (p *PyrightAdapter) NotificationHandlers() map[string]func(params json.RawMessage) {
	return nil
}
func (p *PyrightAdapter) NormalizeSymbolName(name string) string { return name }
func (p *PyrightAdapter) PostInitialize(ctx context.Context, adapter *LSAdapter) error {
	didOpenFirstFile(ctx, adapter, ".py", "python")
	return nil
}

// ZlsAdapter provides Zig-specific quirks for zls.
// zls requires textDocument/didOpen before textDocument/* operations work.
type ZlsAdapter struct {
	Entry langregistry.LSEntry
}

func (z *ZlsAdapter) InitOptions(_ string) map[string]any { return z.Entry.InitOptions }
func (z *ZlsAdapter) NotificationHandlers() map[string]func(params json.RawMessage) {
	return nil
}
func (z *ZlsAdapter) NormalizeSymbolName(name string) string { return name }
func (z *ZlsAdapter) PostInitialize(ctx context.Context, adapter *LSAdapter) error {
	didOpenFirstFileRecursive(ctx, adapter, ".zig", "zig")
	return nil
}

// SourceKitAdapter provides Swift-specific quirks for sourcekit-lsp.
// sourcekit-lsp requires textDocument/didOpen before textDocument/* operations work.
type SourceKitAdapter struct {
	Entry langregistry.LSEntry
}

func (s *SourceKitAdapter) InitOptions(_ string) map[string]any { return s.Entry.InitOptions }
func (s *SourceKitAdapter) NotificationHandlers() map[string]func(params json.RawMessage) {
	return nil
}
func (s *SourceKitAdapter) NormalizeSymbolName(name string) string { return name }
func (s *SourceKitAdapter) PostInitialize(ctx context.Context, adapter *LSAdapter) error {
	didOpenFirstFileRecursive(ctx, adapter, ".swift", "swift")
	return nil
}

// IntelephenseAdapter provides PHP-specific quirks for intelephense.
// Intelephense requires textDocument/didOpen before textDocument/* operations work.
type IntelephenseAdapter struct {
	Entry langregistry.LSEntry
}

func (i *IntelephenseAdapter) InitOptions(_ string) map[string]any { return i.Entry.InitOptions }
func (i *IntelephenseAdapter) NotificationHandlers() map[string]func(params json.RawMessage) {
	return nil
}
func (i *IntelephenseAdapter) NormalizeSymbolName(name string) string { return name }
func (i *IntelephenseAdapter) PostInitialize(ctx context.Context, adapter *LSAdapter) error {
	didOpenFirstFile(ctx, adapter, ".php", "php")
	return nil
}

// MarkdownAdapter provides Markdown-specific quirks for marksman.
// marksman requires textDocument/didOpen before textDocument/* operations work.
type MarkdownAdapter struct {
	Entry langregistry.LSEntry
}

func (m *MarkdownAdapter) InitOptions(_ string) map[string]any { return m.Entry.InitOptions }
func (m *MarkdownAdapter) NotificationHandlers() map[string]func(params json.RawMessage) {
	return nil
}
func (m *MarkdownAdapter) NormalizeSymbolName(name string) string { return name }
func (m *MarkdownAdapter) PostInitialize(ctx context.Context, adapter *LSAdapter) error {
	didOpenFirstFile(ctx, adapter, ".md", "markdown")
	return nil
}

// adapterFactory maps language keys to QuirkAdapter constructors.
var adapterFactory = map[string]func(langregistry.LSEntry) QuirkAdapter{
	"go":         func(e langregistry.LSEntry) QuirkAdapter { return &GoplsAdapter{Entry: e} },
	"rust":       func(e langregistry.LSEntry) QuirkAdapter { return &RustAnalyzerAdapter{Entry: e} },
	"c":          func(e langregistry.LSEntry) QuirkAdapter { return &ClangdAdapter{Entry: e} },
	"cpp":        func(e langregistry.LSEntry) QuirkAdapter { return &ClangdAdapter{Entry: e} },
	"java":       func(e langregistry.LSEntry) QuirkAdapter { return &JdtlsAdapter{Entry: e} },
	"vue":        func(e langregistry.LSEntry) QuirkAdapter { return &VueAdapter{Entry: e} },
	"typescript": func(e langregistry.LSEntry) QuirkAdapter { return &TypeScriptAdapter{Entry: e} },
	"python":     func(e langregistry.LSEntry) QuirkAdapter { return &PyrightAdapter{Entry: e} },
	"zig":        func(e langregistry.LSEntry) QuirkAdapter { return &ZlsAdapter{Entry: e} },
	"swift":      func(e langregistry.LSEntry) QuirkAdapter { return &SourceKitAdapter{Entry: e} },
	"php":        func(e langregistry.LSEntry) QuirkAdapter { return &IntelephenseAdapter{Entry: e} },
	"markdown":   func(e langregistry.LSEntry) QuirkAdapter { return &MarkdownAdapter{Entry: e} },
}

// GetQuirkAdapter returns the language-specific QuirkAdapter for the given entry.
// If no specific adapter exists, returns a DefaultQuirkAdapter wrapping the entry.
func GetQuirkAdapter(entry langregistry.LSEntry) QuirkAdapter {
	if factory, ok := adapterFactory[entry.Language]; ok {
		return factory(entry)
	}
	return &DefaultQuirkAdapter{Entry: entry}
}
