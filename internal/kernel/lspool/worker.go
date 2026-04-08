package lspool

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/postfix/serena/internal/kernel/jsonrpc"
	gen "github.com/postfix/serena/protocol/gen"
)

// WorkerState represents the lifecycle state of an LS worker.
type WorkerState int32

const (
	// WorkerStarting is the initial state before the process is launched.
	WorkerStarting WorkerState = iota
	// WorkerInitializing is the state during LSP initialize/initialized handshake.
	WorkerInitializing
	// WorkerReady means the worker is fully initialized and can handle requests.
	WorkerReady
	// WorkerShuttingDown means a graceful shutdown is in progress.
	WorkerShuttingDown
	// WorkerStopped means the worker has fully stopped.
	WorkerStopped
)

// String returns the string representation of the worker state.
func (s WorkerState) String() string {
	switch s {
	case WorkerStarting:
		return "starting"
	case WorkerInitializing:
		return "initializing"
	case WorkerReady:
		return "ready"
	case WorkerShuttingDown:
		return "shutting_down"
	case WorkerStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// ErrWorkerNotReady is returned when a request is made to a non-Ready worker.
var ErrWorkerNotReady = errors.New("worker is not in Ready state")

// ErrNotSupported is returned when the LS does not support a requested capability.
var ErrNotSupported = errors.New("capability not supported by language server")

// pendingDidOpen holds a buffered didOpen notification for replay after initialization.
type pendingDidOpen struct {
	method string
	params interface{}
}

// WorkerMetrics tracks usage statistics for adaptive TTL computation.
type WorkerMetrics struct {
	mu           sync.Mutex
	StartedAt    time.Time
	LastUsedAt   time.Time
	UseCount     int64
	ReuseTimes   []time.Time // last N reuse timestamps for gap calculation
	ReuseScore   float64     // decaying score per D-03
	ColdStartMs  int64       // time from Start to Ready in milliseconds
}

const maxReuseTimes = 20

// OnReuse records a reuse event and updates the decaying score.
func (m *WorkerMetrics) OnReuse() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.UseCount++
	elapsed := now.Sub(m.LastUsedAt).Seconds()
	// Decay: score = decay(score, elapsed) + 1
	// Using exponential decay: score * exp(-elapsed/300)
	m.ReuseScore = m.ReuseScore*math.Exp(-elapsed/300.0) + 1.0
	m.LastUsedAt = now
	m.ReuseTimes = append(m.ReuseTimes, now)
	if len(m.ReuseTimes) > maxReuseTimes {
		m.ReuseTimes = m.ReuseTimes[len(m.ReuseTimes)-maxReuseTimes:]
	}
}

// TTL computes the adaptive TTL based on decaying reuse score (per D-03).
// Returns the TTL in seconds.
func (m *WorkerMetrics) TTL(baseTTL, ceilingTTL int) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	if baseTTL == 0 {
		return 0 // TTL=0 means no idle timeout (D-07)
	}

	// ttl = clamp(base_ttl * f(score), base_ttl, ceiling)
	// f(score) = 1 + log2(1 + score)
	factor := 1.0 + math.Log2(1.0+m.ReuseScore)
	ttl := float64(baseTTL) * factor
	if ttl < float64(baseTTL) {
		ttl = float64(baseTTL)
	}
	if ttl > float64(ceilingTTL) {
		ttl = float64(ceilingTTL)
	}
	return int(ttl)
}

// IdleDuration returns the duration since last use.
func (m *WorkerMetrics) IdleDuration() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return time.Since(m.LastUsedAt)
}

// Worker represents a single LS worker with a 5-state machine.
type Worker struct {
	id           string
	language     string
	workDir      string
	lsCommand    string
	lsArgs       []string
	quirks       QuirkAdapter
	process      *ProcessHandle
	state        atomic.Int32
	metrics      WorkerMetrics
	mu           sync.RWMutex
	pendingOpen  []pendingDidOpen // buffered during Initializing (per Pitfall 2)
	capabilities gen.ServerCapabilities
	logger       *slog.Logger
}

// NewWorker creates a new LS worker.
func NewWorker(id, language, workDir string, lsCommand string, lsArgs []string, logger *slog.Logger) *Worker {
	w := &Worker{
		id:        id,
		language:  language,
		workDir:   workDir,
		lsCommand: lsCommand,
		lsArgs:    lsArgs,
		logger:    logger.With("worker_id", id, "language", language),
	}
	w.state.Store(int32(WorkerStarting))
	return w
}

// Start launches the LS process and performs the LSP initialize handshake.
func (w *Worker) Start(ctx context.Context) error {
	startTime := time.Now()

	// Set state to Starting.
	w.state.Store(int32(WorkerStarting))

	// Apply language quirks via QuirkAdapter.
	command := w.lsCommand
	args := w.lsArgs
	var initOptions interface{}
	if w.quirks != nil {
		initOptions = w.quirks.InitOptions(w.workDir)
	}

	// Start the process.
	w.process = NewProcessHandle(command, args, w.workDir, nil, w.logger)
	if err := w.process.Start(ctx, w.id); err != nil {
		w.state.Store(int32(WorkerStopped))
		return fmt.Errorf("starting LS process: %w", err)
	}

	// Set state to Initializing.
	w.state.Store(int32(WorkerInitializing))

	// Build InitializeParams.
	rootURI := "file://" + w.workDir
	initParams := gen.InitializeParams{}
	initParams.RootUri = rootURI
	initParams.ProcessId = int32(processID())
	trueVal := true
	initParams.Capabilities = gen.ClientCapabilities{
		TextDocument: &gen.TextDocumentClientCapabilities{
			DocumentSymbol: &gen.DocumentSymbolClientCapabilities{
				HierarchicalDocumentSymbolSupport: &trueVal,
			},
		},
	}
	initParams.InitializationOptions = initOptions
	initParams.WorkspaceFolders = []gen.WorkspaceFolder{
		{URI: rootURI, Name: w.workDir},
	}

	// Send initialize request.
	var initResult gen.InitializeResult
	if err := w.process.Conn().Call(ctx, "initialize", initParams, &initResult); err != nil {
		_ = w.process.Stop(ctx)
		w.state.Store(int32(WorkerStopped))
		return fmt.Errorf("LSP initialize: %w", err)
	}

	// Store server capabilities.
	w.mu.Lock()
	w.capabilities = initResult.Capabilities
	w.mu.Unlock()

	// Send initialized notification.
	if err := w.process.Conn().Notify(ctx, "initialized", gen.InitializedParams{}); err != nil {
		w.logger.Warn("initialized notification failed", "error", err)
	}

	// Call PostInitialize hook if quirks adapter is set.
	if w.quirks != nil {
		adapter := NewLSAdapter(w)
		if err := w.quirks.PostInitialize(ctx, adapter); err != nil {
			w.logger.Warn("PostInitialize hook failed", "error", err)
		}
	}

	// Replay buffered didOpen notifications.
	w.mu.Lock()
	pending := w.pendingOpen
	w.pendingOpen = nil
	w.mu.Unlock()

	for _, p := range pending {
		if err := w.process.Conn().Notify(ctx, p.method, p.params); err != nil {
			w.logger.Warn("replaying buffered didOpen failed", "method", p.method, "error", err)
		}
	}

	// Set state to Ready.
	w.state.Store(int32(WorkerReady))

	coldStart := time.Since(startTime).Milliseconds()
	w.metrics.mu.Lock()
	w.metrics.StartedAt = startTime
	w.metrics.LastUsedAt = time.Now()
	w.metrics.ColdStartMs = coldStart
	w.metrics.mu.Unlock()

	w.logger.Info("LS worker ready", "cold_start_ms", coldStart)
	return nil
}

// Request sends a request to the LS, gated on Ready state.
func (w *Worker) Request(ctx context.Context, method string, params interface{}, result interface{}) error {
	if WorkerState(w.state.Load()) != WorkerReady {
		return ErrWorkerNotReady
	}
	err := w.process.Conn().Call(ctx, method, params, result)
	if err == nil {
		w.metrics.OnReuse()
	}
	return err
}

// Notify sends a notification to the LS.
// Per Pitfall 2: if Initializing and method is textDocument/didOpen, buffer it.
func (w *Worker) Notify(ctx context.Context, method string, params interface{}) error {
	state := WorkerState(w.state.Load())

	if state == WorkerInitializing && method == "textDocument/didOpen" {
		w.mu.Lock()
		w.pendingOpen = append(w.pendingOpen, pendingDidOpen{method: method, params: params})
		w.mu.Unlock()
		return nil
	}

	if state != WorkerReady && state != WorkerInitializing {
		return ErrWorkerNotReady
	}

	return w.process.Conn().Notify(ctx, method, params)
}

// Stop performs graceful shutdown of the LS worker.
func (w *Worker) Stop(ctx context.Context) error {
	w.state.Store(int32(WorkerShuttingDown))
	w.logger.Info("stopping LS worker")

	var err error
	if w.process != nil {
		err = w.process.Stop(ctx)
	}

	w.state.Store(int32(WorkerStopped))
	return err
}

// State returns the current worker state.
func (w *Worker) State() WorkerState {
	return WorkerState(w.state.Load())
}

// Metrics returns a pointer to the worker metrics.
func (w *Worker) Metrics() *WorkerMetrics {
	return &w.metrics
}

// Language returns the language this worker serves.
func (w *Worker) Language() string {
	return w.language
}

// WorkDir returns the working directory this worker is rooted at.
func (w *Worker) WorkDir() string {
	return w.workDir
}

// ID returns the worker ID.
func (w *Worker) ID() string {
	return w.id
}

// Capabilities returns the server capabilities reported during initialization.
func (w *Worker) Capabilities() gen.ServerCapabilities {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.capabilities
}

// Conn returns the underlying JSON-RPC connection (for adapter use).
func (w *Worker) Conn() *jsonrpc.Conn {
	if w.process != nil {
		return w.process.Conn()
	}
	return nil
}

// Pid returns the LS process ID, or -1 if not started.
func (w *Worker) Pid() int {
	if w.process != nil {
		return w.process.Pid()
	}
	return -1
}

// SetQuirks sets the QuirkAdapter for this worker. Must be called before Start.
func (w *Worker) SetQuirks(q QuirkAdapter) {
	w.quirks = q
}

// NormalizeSymbolName delegates symbol name normalization to the QuirkAdapter.
func (w *Worker) NormalizeSymbolName(name string) string {
	if w.quirks != nil {
		return w.quirks.NormalizeSymbolName(name)
	}
	return name
}

// processID returns the current process ID for the LSP initialize handshake.
func processID() int {
	return pid()
}
