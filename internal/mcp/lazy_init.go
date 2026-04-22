package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// LazyInitMiddleware transparently activates the workspace on the first
// tools/call if no workspace is active (LAZY-01). Thread-safe via
// sync.Once per workspace path (LAZY-02).
type LazyInitMiddleware struct {
	activateFn  func(ctx context.Context, path string) error
	isActiveFn  func() bool
	defaultRoot string
	mu          sync.Mutex
	initOnce    map[string]*sync.Once
	logger      *slog.Logger
}

// NewLazyInitMiddleware creates a lazy init middleware.
// activateFn is called to activate a workspace (typically daemon's activate callback).
// isActiveFn returns true if any workspace is currently active.
// defaultRoot is the fallback project root (from config or forwarder cwd).
func NewLazyInitMiddleware(activateFn func(ctx context.Context, path string) error, isActiveFn func() bool, defaultRoot string, logger *slog.Logger) *LazyInitMiddleware {
	return &LazyInitMiddleware{
		activateFn:  activateFn,
		isActiveFn:  isActiveFn,
		defaultRoot: defaultRoot,
		initOnce:    make(map[string]*sync.Once),
		logger:      logger,
	}
}

func (m *LazyInitMiddleware) getOnce(path string) *sync.Once {
	m.mu.Lock()
	defer m.mu.Unlock()
	if once, ok := m.initOnce[path]; ok {
		return once
	}
	once := &sync.Once{}
	m.initOnce[path] = once
	return once
}

// resolveRoot extracts workspace path from tool call arguments.
// Checks for repo_path first, then falls back to defaultRoot.
func (m *LazyInitMiddleware) resolveRoot(req mcpsdk.Request) string {
	ctr, ok := req.(*mcpsdk.CallToolRequest)
	if !ok || ctr == nil || ctr.Params == nil {
		return m.defaultRoot
	}
	// Check for repo_path in arguments (used by activate_project).
	// Arguments is json.RawMessage; unmarshal to extract repo_path.
	if len(ctr.Params.Arguments) > 0 {
		var args map[string]any
		if err := json.Unmarshal(ctr.Params.Arguments, &args); err == nil {
			if rp, ok := args["repo_path"]; ok {
				if s, ok := rp.(string); ok && s != "" {
					return s
				}
			}
		}
	}
	return m.defaultRoot
}

// Middleware returns the mcpsdk.Middleware function.
func (m *LazyInitMiddleware) Middleware() mcpsdk.Middleware {
	return func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
		return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
			if method != "tools/call" {
				return next(ctx, method, req)
			}
			if !m.isActiveFn() {
				root := m.resolveRoot(req)
				if root != "" {
					once := m.getOnce(root)
					var initErr error
					once.Do(func() {
						m.logger.Info("lazy init: activating workspace", "root", root)
						initErr = m.activateFn(ctx, root)
						if initErr != nil {
							m.logger.Error("lazy init: activation failed", "root", root, "error", initErr)
						}
					})
					if initErr != nil {
						// Return actionable error per Pitfall 6.
						return &mcpsdk.CallToolResult{
							Content: []mcpsdk.Content{
								&mcpsdk.TextContent{Text: "Workspace activation failed: " + initErr.Error() + ". Call activate_project explicitly."},
							},
							IsError: true,
						}, nil
					}
				}
			}
			return next(ctx, method, req)
		}
	}
}

// InstallLazyInitMiddleware wires the lazy init middleware onto the MCP SDK server.
// MUST be called LAST (after InstallSuggestionMiddleware) so it runs FIRST in the
// LIFO middleware chain -- before TelemetryMiddleware's deadline (Pitfall 3).
func InstallLazyInitMiddleware(server *mcpsdk.Server, activateFn func(ctx context.Context, path string) error, isActiveFn func() bool, defaultRoot string, logger *slog.Logger) {
	m := NewLazyInitMiddleware(activateFn, isActiveFn, defaultRoot, logger)
	server.AddReceivingMiddleware(m.Middleware())
}
