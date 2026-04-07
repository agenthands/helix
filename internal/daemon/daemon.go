package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/postfix/serena/internal/config"
	"github.com/postfix/serena/internal/workspace"
)

// Daemon is the persistent supervisor process (DMN-01).
// It manages workspace registry, listeners, and survives client disconnects (DMN-02).
type Daemon struct {
	config         *config.SerenaConfig
	logger         *slog.Logger
	workspaces     *workspace.Registry
	socketListener net.Listener
}

// New creates a new Daemon with the given config and logger.
func New(cfg *config.SerenaConfig, logger *slog.Logger) *Daemon {
	return &Daemon{
		config:     cfg,
		logger:     logger,
		workspaces: workspace.NewRegistry(),
	}
}

// Run starts the daemon and blocks until shutdown (DMN-01, DMN-12).
// Signal handlers are registered FIRST per Pitfall 3.
func (d *Daemon) Run(ctx context.Context) error {
	// Register signal handlers FIRST (Pitfall 3: before any goroutine starts)
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	// Check for stale socket and clean up (DMN-13)
	if err := ensureSocket(d.config.Daemon.SocketPath); err != nil {
		return fmt.Errorf("socket check: %w", err)
	}
	if err := createSocketDir(d.config.Daemon.SocketPath); err != nil {
		return fmt.Errorf("creating socket directory: %w", err)
	}

	g, gctx := errgroup.WithContext(ctx)

	// Unix socket listener for forwarder connections (DMN-03 transport, wired in Plan 03)
	g.Go(func() error {
		return d.listenSocket(gctx)
	})

	d.logger.Info("daemon started",
		"socket", d.config.Daemon.SocketPath,
		"http_addr", d.config.Daemon.HTTPAddr,
		"workspaces", d.workspaces.WorkspaceCount(),
	)

	err := g.Wait()
	d.shutdown()
	return err
}

// listenSocket starts the Unix domain socket listener.
func (d *Daemon) listenSocket(ctx context.Context) error {
	ln, err := net.Listen("unix", d.config.Daemon.SocketPath)
	if err != nil {
		return fmt.Errorf("listen unix %s: %w", d.config.Daemon.SocketPath, err)
	}
	d.socketListener = ln
	d.logger.Info("unix socket listener started", "path", d.config.Daemon.SocketPath)

	// Wait for context cancellation to close listener
	<-ctx.Done()
	ln.Close()
	return ctx.Err()
}

// Workspaces returns the workspace registry (for use by MCP tools).
func (d *Daemon) Workspaces() *workspace.Registry {
	return d.workspaces
}
