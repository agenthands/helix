package mcp

import (
	"context"
	"log/slog"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// InstallMiddleware adds receiving middleware for logging and error structuring (MCP-04).
func InstallMiddleware(server *mcpsdk.Server, logger *slog.Logger) {
	server.AddReceivingMiddleware(loggingMiddleware(logger))
}

// ProfileResolver provides profile information for middleware filtering.
// This interface avoids a circular import between mcp and profile packages.
type ProfileResolver interface {
	// ToolDescriptionOverrides returns the description override map for the named profile.
	// Returns nil if the profile has no overrides or is not found.
	ToolDescriptionOverrides(profileName string) map[string]string
}

// ProfileFilterMiddleware creates middleware that filters tool listings based on
// the active session's AllowedTools and applies description overrides from the
// profile (PRF-03). For tools/list requests it filters and rewrites descriptions;
// all other methods pass through unchanged.
func ProfileFilterMiddleware(resolver ProfileResolver, getSession func(ctx context.Context) *SessionInfo, logger *slog.Logger) mcpsdk.Middleware {
	return func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
		return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
			result, err := next(ctx, method, req)
			if err != nil {
				return result, err
			}

			if method != "tools/list" {
				return result, nil
			}

			session := getSession(ctx)
			if session == nil {
				return result, nil
			}

			listResult, ok := result.(*mcpsdk.ListToolsResult)
			if !ok {
				return result, nil
			}

			// Take a consistent snapshot so AllowedTools and Profile come from
			// the same point in time even if a concurrent switch_mode is racing
			// with this tools/list (threat T-08-08 mitigation).
			snap := session.Snapshot()

			// Apply AllowedTools filtering if the session has a whitelist.
			if snap.AllowedTools != nil {
				allowed := make(map[string]bool, len(snap.AllowedTools))
				for _, name := range snap.AllowedTools {
					allowed[name] = true
				}
				filtered := make([]*mcpsdk.Tool, 0, len(listResult.Tools))
				for _, tool := range listResult.Tools {
					if allowed[tool.Name] {
						filtered = append(filtered, tool)
					}
				}
				listResult.Tools = filtered
			}

			// Apply description overrides from the profile.
			if resolver != nil {
				overrides := resolver.ToolDescriptionOverrides(snap.Profile)
				if len(overrides) > 0 {
					for _, tool := range listResult.Tools {
						if desc, ok := overrides[tool.Name]; ok {
							tool.Description = desc
						}
					}
				}
			}

			return listResult, nil
		}
	}
}

// loggingMiddleware logs every request with method, duration, and error status.
func loggingMiddleware(logger *slog.Logger) mcpsdk.Middleware {
	return func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
		return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
			start := time.Now()
			result, err := next(ctx, method, req)
			duration := time.Since(start)

			if err != nil {
				logger.Warn("request failed",
					"method", method,
					"duration", duration,
					"error", err,
				)
			} else {
				logger.Info("request handled",
					"method", method,
					"duration", duration,
				)
			}
			return result, err
		}
	}
}
