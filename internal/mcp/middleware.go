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
