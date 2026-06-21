package mcp

import (
	"context"
	"fmt"
	"log/slog"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	serr "github.com/agenthands/helix/internal/errors"
)

// InstallProfileEnforcementMiddleware wires the tools/call profile/mode
// enforcement middleware onto the MCP SDK server (Phase 91 SEC-01).
//
// MUST be installed AFTER InstallGuardrailMiddleware and BEFORE
// InstallLazyInitMiddleware so LIFO execution order is
// LazyInit → ProfileEnforce → Guardrail → Suggestion → ProfileFilter →
// Telemetry → handler. The LazyInit-last invariant is preserved
// (see internal/mcp/lazy_init.go:106-112): LazyInit must still execute first
// so the workspace is activated before any per-tool deadline or authz check,
// and ProfileEnforce executes before Guardrail so an out-of-profile call is
// refused before guardrail receipt evaluation runs.
//
// getSession is the SAME closure wired to TelemetryMiddleware,
// ProfileFilterMiddleware, and GuardrailMiddleware (single source of truth for
// session state, per the D-01 thread-safety invariant). Authz is enforced
// server-side here because the daemon session is authoritative; the CLI never
// sees profile/mode and a CLI-only check would be bypassable (T-91-07).
func InstallProfileEnforcementMiddleware(server *mcpsdk.Server, getSession func(ctx context.Context) *SessionInfo, logger *slog.Logger) {
	server.AddReceivingMiddleware(ProfileEnforcementMiddleware(getSession, logger))
}

// ProfileEnforcementMiddleware returns a middleware that gates every
// tools/call on the session's already-resolved AllowedTools whitelist
// (Phase 91 SEC-01, threat T-91-05). A tool not present in the whitelist is
// refused with a typed serr.PermissionDenied error so errors.Is(err,
// serr.ErrPermissionDenied) round-trips to the CLI (T-91-09: the deny is
// returned as an ERROR, not an IsError CallToolResult).
//
// Passthrough cases (allow):
//   - method != "tools/call" (tools/list, initialize, ...)
//   - the request is not a *CallToolRequest (defensive)
//   - getSession is nil, or returns a nil session (no whitelist available)
//   - the session's AllowedTools is nil (nil == all tools, matching
//     ProfileFilterMiddleware's nil semantics)
//
// The refusal message names ONLY the requested tool plus the active
// profile/mode (all already public to the caller); it never enumerates other
// tools (T-91-08).
func ProfileEnforcementMiddleware(getSession func(ctx context.Context) *SessionInfo, logger *slog.Logger) mcpsdk.Middleware {
	return func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
		return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
			// Early-out for non-tools/call methods.
			if method != "tools/call" {
				return next(ctx, method, req)
			}

			// Extract the tool request.
			ctr, ok := req.(*mcpsdk.CallToolRequest)
			if !ok || ctr == nil || ctr.Params == nil {
				return next(ctx, method, req)
			}

			// No session closure → no whitelist → allow.
			if getSession == nil {
				return next(ctx, method, req)
			}
			sess := getSession(ctx)
			if sess == nil {
				return next(ctx, method, req)
			}

			// Snapshot is the consistent multi-field read; AllowedTools is a
			// defensive copy safe to iterate without holding the session lock.
			snap := sess.Snapshot()
			if snap.AllowedTools == nil {
				// nil whitelist == all tools allowed (ProfileFilter parity).
				return next(ctx, method, req)
			}

			// Membership check: linear scan of the resolved whitelist.
			name := ctr.Params.Name
			for _, allowed := range snap.AllowedTools {
				if allowed == name {
					return next(ctx, method, req)
				}
			}

			// Refuse: tool is not available in the active profile/mode.
			if logger != nil {
				logger.Debug("profile/mode enforcement: refused tools/call",
					"tool", name,
					"profile", snap.Profile,
					"mode", snap.Mode,
				)
			}
			return nil, serr.New(
				serr.PermissionDenied,
				fmt.Sprintf("tool %q is not available in profile=%s mode=%s", name, snap.Profile, snap.Mode),
			).WithTool(name)
		}
	}
}
