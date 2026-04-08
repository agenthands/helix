package lspool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/postfix/serena/internal/langregistry"
)

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

func (c *ClangdAdapter) PostInitialize(_ context.Context, _ *LSAdapter) error {
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

// PostInitialize creates the jdtls workspace data directory if it doesn't exist.
func (j *JdtlsAdapter) PostInitialize(_ context.Context, _ *LSAdapter) error {
	return nil
}

// EnsureDataDir creates the jdtls workspace data directory for the given workDir.
// Returns the path to the data directory.
func (j *JdtlsAdapter) EnsureDataDir(workDir string) (string, error) {
	dataDir := filepath.Join(workDir, ".jdtls-data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", err
	}
	return dataDir, nil
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

// adapterFactory maps language keys to QuirkAdapter constructors.
var adapterFactory = map[string]func(langregistry.LSEntry) QuirkAdapter{
	"go":         func(e langregistry.LSEntry) QuirkAdapter { return &GoplsAdapter{Entry: e} },
	"rust":       func(e langregistry.LSEntry) QuirkAdapter { return &RustAnalyzerAdapter{Entry: e} },
	"c":          func(e langregistry.LSEntry) QuirkAdapter { return &ClangdAdapter{Entry: e} },
	"cpp":        func(e langregistry.LSEntry) QuirkAdapter { return &ClangdAdapter{Entry: e} },
	"java":       func(e langregistry.LSEntry) QuirkAdapter { return &JdtlsAdapter{Entry: e} },
	"vue":        func(e langregistry.LSEntry) QuirkAdapter { return &VueAdapter{Entry: e} },
}

// GetQuirkAdapter returns the language-specific QuirkAdapter for the given entry.
// If no specific adapter exists, returns a DefaultQuirkAdapter wrapping the entry.
func GetQuirkAdapter(entry langregistry.LSEntry) QuirkAdapter {
	if factory, ok := adapterFactory[entry.Language]; ok {
		return factory(entry)
	}
	return &DefaultQuirkAdapter{Entry: entry}
}
