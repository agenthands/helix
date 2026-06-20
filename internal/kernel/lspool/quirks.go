package lspool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/agenthands/helix/internal/langregistry"
	gen "github.com/agenthands/helix/protocol/gen"
)

// ArgsModifier is an optional interface that QuirkAdapters can implement
// to inject extra command-line arguments when starting the LS process.
type ArgsModifier interface {
	ExtraArgs(workDir string, args []string) []string
}

// ExperimentalCapabilities is an optional interface that QuirkAdapters can
// implement to advertise LS-specific experimental client capabilities during
// the initialize handshake. The returned map is merged into
// ClientCapabilities.Experimental before "initialize" is dispatched. Used by
// RustAnalyzerAdapter to opt into rust-analyzer's experimental/serverStatus
// notification (see Phase 47 / BUG-02).
type ExperimentalCapabilities interface {
	ExperimentalCapabilities() map[string]any
}

// renameReadinessTimeout bounds WaitUntilRenameReady. rust-analyzer typically
// emits experimental/serverStatus.quiescent=true within ~2-4s on the fixtures
// used for Phase 47; 10s gives generous headroom without letting a stuck
// server wedge the rename path forever (T-47-02).
const renameReadinessTimeout = 10 * time.Second

// QuirkAdapter provides per-language behavioral hooks for LS workers.
// Languages with no special behavior use DefaultQuirkAdapter.
type QuirkAdapter interface {
	// InitOptions returns language-specific initialization options.
	// workDir is the workspace root for workspace-dependent options.
	InitOptions(workDir string) map[string]any
	// NotificationHandlers returns handlers for LS-specific notifications,
	// keyed by JSON-RPC method name (e.g. "experimental/serverStatus",
	// "language/status"). Returning nil or an empty map means this adapter
	// declines to observe any notifications.
	//
	// Phase 56 D-06 contract: each handler runs synchronously on the
	// jsonrpc.Conn.Listen goroutine in the worker. Handlers MUST be
	// non-blocking — atomic ops, channel close/signal, or mutex-guarded
	// flag flips only. Handlers MUST NOT do I/O, MUST NOT call back into
	// the same conn (e.g. issuing another LSP request via the worker), and
	// MUST NOT block on user-controlled timing. A misbehaving handler will
	// stall every subsequent notification on this worker. Panics are
	// recovered by the dispatcher (see lspool.buildDispatcher) and logged
	// at Error level, but the originating handler is still considered buggy
	// and should be fixed.
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
// Enables cargo build scripts in initialization options and wires the
// experimental/serverStatus notification handler so edit.RenameSymbol can
// wait for rename-relevant quiescence before dispatching textDocument/rename
// (BUG-02). Per the Phase 47 RCA, rust-analyzer 1.90 returns
// "No references found at position" from textDocument/rename whenever the
// per-position analysis is still cold; the quiescent signal is the
// deterministic readiness gate.
//
// Semantic-accuracy note (D-05): when the client-side rename fallback
// (introduced in Plan 02) runs, cross-crate trait-impl discovery and
// macro-expansion rename corners are NOT matched with native rust-analyzer
// fidelity. See BUG-DEFER-02.
type RustAnalyzerAdapter struct {
	Entry langregistry.LSEntry

	// quiescent tracks the most recent experimental/serverStatus.quiescent value.
	// Read lock-free via atomic.Bool; writes come from the notification-dispatch
	// goroutine inside Conn.Listen (one writer at a time per worker).
	quiescent atomic.Bool

	// readyCh is closed when quiescent flips to true and recreated when it
	// flips back to false. WaitUntilRenameReady selects on it. readyChMu
	// guards the re-creation / close races (T-47-05).
	readyChMu sync.Mutex
	readyCh   chan struct{}
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

// ExperimentalCapabilities opts rust-analyzer into the experimental/serverStatus
// notification stream. Without this advertisement, rust-analyzer will not emit
// the quiescent signal and the rename readiness gate degrades to timeout-only.
func (r *RustAnalyzerAdapter) ExperimentalCapabilities() map[string]any {
	return map[string]any{"serverStatusNotification": true}
}

// NotificationHandlers returns a handler for experimental/serverStatus that
// flips the adapter's readiness flag. Payload schema (per rust-analyzer LSP
// extensions docs): {health: "ok"|"warning"|"error", quiescent: bool,
// message: string}. Unknown fields are discarded; malformed payloads are a
// no-op (T-47-01).
func (r *RustAnalyzerAdapter) NotificationHandlers() map[string]func(params json.RawMessage) {
	return map[string]func(params json.RawMessage){
		"experimental/serverStatus": func(raw json.RawMessage) {
			var s struct {
				Health    string `json:"health"`
				Quiescent bool   `json:"quiescent"`
				Message   string `json:"message"`
			}
			if err := json.Unmarshal(raw, &s); err != nil {
				return
			}
			if s.Quiescent && (s.Health == "ok" || s.Health == "") {
				r.quiescent.Store(true)
				r.signalReady()
			} else {
				r.quiescent.Store(false)
				r.resetReadyCh()
			}
		},
	}
}

// ensureReadyCh lazily constructs the readiness channel on first access.
func (r *RustAnalyzerAdapter) ensureReadyCh() chan struct{} {
	r.readyChMu.Lock()
	defer r.readyChMu.Unlock()
	if r.readyCh == nil {
		r.readyCh = make(chan struct{})
	}
	return r.readyCh
}

// signalReady closes the readiness channel (idempotent).
func (r *RustAnalyzerAdapter) signalReady() {
	r.readyChMu.Lock()
	defer r.readyChMu.Unlock()
	if r.readyCh == nil {
		// signalReady may be called before any WaitUntilRenameReady caller has
		// observed the adapter. In that case we install a pre-closed channel so
		// the next ensureReadyCh consumer returns immediately — matching the
		// atomic quiescent=true store we just performed. This mirrors the
		// fresh un-closed channel lazily created by ensureReadyCh on its own
		// first-access path.
		ch := make(chan struct{})
		close(ch)
		r.readyCh = ch
		return
	}
	select {
	case <-r.readyCh:
		// already closed — keep it closed
	default:
		close(r.readyCh)
	}
}

// resetReadyCh replaces the readiness channel with a fresh unclosed one so
// subsequent WaitUntilRenameReady callers block until the next quiescent=true.
func (r *RustAnalyzerAdapter) resetReadyCh() {
	r.readyChMu.Lock()
	defer r.readyChMu.Unlock()
	r.readyCh = make(chan struct{})
}

// WaitUntilRenameReady blocks until rust-analyzer reports quiescent=true via
// experimental/serverStatus, ctx is cancelled, or renameReadinessTimeout
// elapses. Returns true iff the server reached quiescence before the timeout
// or cancellation. Safe to call concurrently; Plan 02 consumes this via
// optional-interface type assertion from internal/kernel/edit.
func (r *RustAnalyzerAdapter) WaitUntilRenameReady(ctx context.Context) bool {
	if r.quiescent.Load() {
		return true
	}
	ch := r.ensureReadyCh()
	// Re-check after grabbing the channel to avoid missing a transition that
	// happened between the initial Load and the ensureReadyCh call.
	if r.quiescent.Load() {
		return true
	}
	timer := time.NewTimer(renameReadinessTimeout)
	defer timer.Stop()
	select {
	case <-ch:
		return r.quiescent.Load()
	case <-ctx.Done():
		return false
	case <-timer.C:
		return false
	}
}

// QuiescentChan returns a channel that is closed once rust-analyzer reports
// quiescent=true via experimental/serverStatus.  If the adapter has already
// observed quiescent=true the returned channel is already closed.  Phase 61
// ENRICH-03 readiness gate — the enrichment worker selects on this channel
// (plus ctx.Done) instead of polling.
//
// The channel is recreated when rust-analyzer transitions back to non-
// quiescent (re-indexing); callers MUST re-call QuiescentChan to observe the
// next quiescent transition.
func (r *RustAnalyzerAdapter) QuiescentChan() <-chan struct{} {
	if r == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	if r.quiescent.Load() {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return r.ensureReadyCh()
}

func (r *RustAnalyzerAdapter) NormalizeSymbolName(name string) string {
	return name
}

func (r *RustAnalyzerAdapter) PostInitialize(ctx context.Context, adapter *LSAdapter) error {
	// rust-analyzer requires textDocument/didOpen on target files before
	// textDocument/rename and other file-scoped operations work.
	// Opening only the first file leaves other files invisible to the LS.
	didOpenAllFilesRecursive(ctx, adapter, ".rs", "rust")
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
//
// Phase 56 readiness extension: observes language/status notifications and
// exposes WaitUntilJavaReady so callers can block until BOTH ServiceReady
// and ProjectStatus=OK have been seen (D-07, D-08, D-09).
type JdtlsAdapter struct {
	Entry langregistry.LSEntry

	readyMu      sync.Mutex
	serviceReady chan struct{} // closed when ServiceReady seen
	projectReady chan struct{} // closed when ProjectStatus=OK seen
}

func (j *JdtlsAdapter) InitOptions(workDir string) map[string]any {
	opts := make(map[string]any)
	for k, v := range j.Entry.InitOptions {
		opts[k] = v
	}
	return opts
}

// NotificationHandlers handles jdtls language/status notifications. Payload
// schema is {type: string, message: string}. Phase 56 D-06: handler runs synchronously
// on the Listen goroutine — operations are limited to json.Unmarshal +
// mutex-guarded channel close. No I/O, no LSP calls back to the same conn.
func (j *JdtlsAdapter) NotificationHandlers() map[string]func(json.RawMessage) {
	return map[string]func(json.RawMessage){
		"language/status": func(raw json.RawMessage) {
			var s struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(raw, &s); err != nil {
				return // malformed → no-op (D-13)
			}
			if s.Type == "ServiceReady" && s.Message == "ServiceReady" {
				j.signalServiceReady()
			}
			if s.Type == "ProjectStatus" && s.Message == "OK" {
				j.signalProjectReady()
			}
		},
	}
}

func (j *JdtlsAdapter) ensureServiceReadyCh() chan struct{} {
	j.readyMu.Lock()
	defer j.readyMu.Unlock()
	if j.serviceReady == nil {
		j.serviceReady = make(chan struct{})
	}
	return j.serviceReady
}

func (j *JdtlsAdapter) signalServiceReady() {
	j.readyMu.Lock()
	defer j.readyMu.Unlock()
	if j.serviceReady == nil {
		ch := make(chan struct{})
		close(ch)
		j.serviceReady = ch
		return
	}
	select {
	case <-j.serviceReady:
		// already closed — idempotent
	default:
		close(j.serviceReady)
	}
}

func (j *JdtlsAdapter) ensureProjectReadyCh() chan struct{} {
	j.readyMu.Lock()
	defer j.readyMu.Unlock()
	if j.projectReady == nil {
		j.projectReady = make(chan struct{})
	}
	return j.projectReady
}

func (j *JdtlsAdapter) signalProjectReady() {
	j.readyMu.Lock()
	defer j.readyMu.Unlock()
	if j.projectReady == nil {
		ch := make(chan struct{})
		close(ch)
		j.projectReady = ch
		return
	}
	select {
	case <-j.projectReady:
		// already closed — idempotent
	default:
		close(j.projectReady)
	}
}

// javaReadinessTimeout bounds WaitUntilJavaReady when the caller's context has
// no deadline. Mirrors renameReadinessTimeout (above). 90s is generous vs.
// legacy's 20s hotfix (eclipse_jdtls.py:921-927) which sometimes proceeds
// without ProjectStatus=OK; we keep the strict gate but raise the ceiling.
const javaReadinessTimeout = 90 * time.Second

// WaitUntilJavaReady blocks until BOTH ServiceReady AND ProjectStatus=OK
// have been observed via language/status notifications, ctx is cancelled,
// or javaReadinessTimeout elapses (whichever fires first). Returns nil iff
// both gates closed. Phase 56 D-08 + D-09.
func (j *JdtlsAdapter) WaitUntilJavaReady(ctx context.Context) error {
	svc := j.ensureServiceReadyCh()
	proj := j.ensureProjectReadyCh()

	timer := time.NewTimer(javaReadinessTimeout)
	defer timer.Stop()

	select {
	case <-svc:
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("jdtls ServiceReady timeout after %s", javaReadinessTimeout)
	}
	select {
	case <-proj:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("jdtls ProjectStatus=OK timeout after %s", javaReadinessTimeout)
	}
}

func (j *JdtlsAdapter) NormalizeSymbolName(name string) string {
	return name
}

// PostInitialize opens every Java source file so jdtls has each document
// available for textDocument/* operations. Opening only the first file is
// insufficient: workspace/symbol works off the project index, but hover,
// references, and replace_symbol_body need the specific file to have been
// didOpen'd first — otherwise jdtls returns an empty hover ({"contents":""})
// and "symbol not found" for replace_symbol_body on un-opened files.
func (j *JdtlsAdapter) PostInitialize(ctx context.Context, adapter *LSAdapter) error {
	// Open every Java source file (not just the first). workspace/symbol works
	// off the project index, but textDocument/hover and textDocument/references
	// against a specific file behave more reliably once the file has been
	// didOpen'd. Mirrors the rust-analyzer pattern.
	didOpenAllFilesRecursive(ctx, adapter, ".java", "java")
	return nil
}

// ExtraArgs injects -data <dir> so jdtls has a workspace-specific data directory.
// Without this, jdtls may fail to index or conflict across workspaces.
//
// Test-only override: if the environment variable HELIX_TEST_JDTLS_DATA_DIR is set
// to a non-empty path, it is used verbatim in place of workDir/.jdtls-data. This
// lets integration tests (test/integration/jdtlscache) share a warm jdtls workspace
// across test runs (Phase 48, BUG-03). Production never sets this variable.
func (j *JdtlsAdapter) ExtraArgs(workDir string, args []string) []string {
	dataDir := os.Getenv("HELIX_TEST_JDTLS_DATA_DIR")
	if dataDir == "" {
		dataDir = filepath.Join(workDir, ".jdtls-data")
	}
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

// didOpenAllFiles opens ALL files matching ext in the workspace root via didOpen.
// Used for LS implementations where textDocument/* operations (hover, references,
// documentSymbol) require the target file to have been opened first — opening only
// the first file is insufficient for multi-file workspaces.
func didOpenAllFiles(ctx context.Context, adapter *LSAdapter, ext string, langID string) {
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

// didOpenAllFilesRecursive walks subdirectories and opens ALL files matching ext.
// Used for LS implementations where textDocument/* operations require the target
// file to have been opened first — opening only the first file is insufficient.
func didOpenAllFilesRecursive(ctx context.Context, adapter *LSAdapter, ext string, langID string) {
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
		return nil // continue to next file
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
	// typescript-language-server requires didOpen on every file for textDocument/*
	// operations (hover, references, documentSymbol) to work on that file.
	// Opening only the first file leaves other files invisible to the LS.
	didOpenAllFiles(ctx, adapter, ".ts", "typescript")
	didOpenAllFiles(ctx, adapter, ".js", "javascript")
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
