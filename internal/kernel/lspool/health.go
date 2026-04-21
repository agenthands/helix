package lspool

import (
	"sort"

	gen "github.com/postfix/serena/protocol/gen"
)

// WorkerHealth describes the health state of a single LS worker.
type WorkerHealth struct {
	ID           string   `json:"id"`
	Language     string   `json:"language"`
	WorkDir      string   `json:"work_dir"`
	Command      string   `json:"command"`
	State        string   `json:"state"`                  // "healthy", "healthy (indexing)", "degraded", "failed"
	Capabilities []string `json:"capabilities"`            // e.g. ["hover", "definition", "references"]
	Indexing     bool     `json:"indexing"`
	IndexPct     int      `json:"index_pct,omitempty"`
}

// CircuitHealth describes the health state of a circuit breaker for a language.
type CircuitHealth struct {
	Language string `json:"language"`
	State    string `json:"state"` // "closed", "half_open", "open"
	Failures int    `json:"failures"`
}

// WorkspaceHealth groups workers and circuits for a single workspace root.
type WorkspaceHealth struct {
	Root      string          `json:"root"`
	Languages []string        `json:"languages"`
	Workers   []WorkerHealth  `json:"workers"`
	Circuits  []CircuitHealth `json:"circuits"`
}

// HealthReport is the top-level health snapshot returned by Pool.HealthSnapshot().
type HealthReport struct {
	Workspaces []WorkspaceHealth `json:"workspaces"`
	Summary    string            `json:"summary,omitempty"`
}

// HealthSnapshot captures the current health state of all workers and circuits
// under a single RLock acquisition. The returned HealthReport contains value
// copies -- no references to pool internals are retained.
func (p *Pool) HealthSnapshot() HealthReport {
	p.mu.RLock()

	// Snapshot workers.
	type workerSnap struct {
		id       string
		language string
		workDir  string
		command  string
		state    WorkerState
		caps     gen.ServerCapabilities
	}
	workers := make([]workerSnap, 0, len(p.workers))
	for _, w := range p.workers {
		workers = append(workers, workerSnap{
			id:       w.ID(),
			language: w.Language(),
			workDir:  w.WorkDir(),
			command:  w.Command(),
			state:    w.State(),
			caps:     w.Capabilities(),
		})
	}

	// Snapshot circuits.
	type circuitSnap struct {
		language string
		state    float64
		failures int
	}
	circuits := make([]circuitSnap, 0, len(p.circuits))
	for _, cb := range p.circuits {
		circuits = append(circuits, circuitSnap{
			language: cb.language,
			state:    cb.State(),
			failures: cb.Failures(),
		})
	}

	p.mu.RUnlock()

	// Build a language->circuit state lookup.
	circuitLookup := make(map[string]float64)
	for _, cs := range circuits {
		circuitLookup[cs.language] = cs.state
	}

	// Build WorkerHealth entries and group by workDir.
	workspaceMap := make(map[string]*WorkspaceHealth)
	for _, ws := range workers {
		wh := classifyWorker(ws.id, ws.language, ws.workDir, ws.command, ws.state, ws.caps, circuitLookup)

		wsh, ok := workspaceMap[ws.workDir]
		if !ok {
			wsh = &WorkspaceHealth{
				Root: ws.workDir,
			}
			workspaceMap[ws.workDir] = wsh
		}
		wsh.Workers = append(wsh.Workers, wh)
	}

	// Attach circuit health to matching workspaces or create standalone entries.
	for _, cs := range circuits {
		ch := CircuitHealth{
			Language: cs.language,
			State:    circuitStateString(cs.state),
			Failures: cs.failures,
		}
		// Attach to all workspaces that have workers for this language.
		attached := false
		for _, wsh := range workspaceMap {
			for _, w := range wsh.Workers {
				if w.Language == cs.language {
					wsh.Circuits = append(wsh.Circuits, ch)
					attached = true
					break
				}
			}
		}
		// If no workspace has a worker for this language but the circuit
		// is non-closed, attach to all workspaces as a signal.
		if !attached && cs.state != CircuitClosed {
			for _, wsh := range workspaceMap {
				wsh.Circuits = append(wsh.Circuits, ch)
			}
		}
	}

	// Convert map to sorted slice.
	workspaces := make([]WorkspaceHealth, 0, len(workspaceMap))
	for _, wsh := range workspaceMap {
		sort.Slice(wsh.Workers, func(i, j int) bool {
			return wsh.Workers[i].ID < wsh.Workers[j].ID
		})
		sort.Slice(wsh.Circuits, func(i, j int) bool {
			return wsh.Circuits[i].Language < wsh.Circuits[j].Language
		})
		workspaces = append(workspaces, *wsh)
	}
	sort.Slice(workspaces, func(i, j int) bool {
		return workspaces[i].Root < workspaces[j].Root
	})

	return HealthReport{
		Workspaces: workspaces,
	}
}

// TotalWorkers returns the total number of workers across all workspaces.
func (r *HealthReport) TotalWorkers() int {
	total := 0
	for _, ws := range r.Workspaces {
		total += len(ws.Workers)
	}
	return total
}

// classifyWorker maps a worker's state and its language circuit to a health classification.
// circuitLookup maps language to circuit state (CircuitClosed/HalfOpen/Open).
func classifyWorker(id, language, workDir, command string, state WorkerState, caps gen.ServerCapabilities, circuitLookup map[string]float64) WorkerHealth {
	wh := WorkerHealth{
		ID:           id,
		Language:     language,
		WorkDir:      workDir,
		Command:      command,
		Capabilities: capabilitiesFromServer(caps),
	}

	// Find the circuit for this language.
	circuitState, ok := circuitLookup[language]
	if !ok {
		circuitState = CircuitClosed
	}

	switch {
	case state == WorkerStopped || state == WorkerShuttingDown:
		wh.State = "failed"
	case circuitState == CircuitOpen:
		wh.State = "failed"
	case state == WorkerStarting || state == WorkerInitializing:
		wh.State = "healthy (indexing)"
		wh.Indexing = true
	case state == WorkerReady && circuitState == CircuitHalfOpen:
		wh.State = "degraded"
	case state == WorkerReady && circuitState == CircuitClosed:
		wh.State = "healthy"
	default:
		wh.State = "healthy"
	}

	return wh
}

// capabilitiesFromServer extracts capability names from the LSP ServerCapabilities.
func capabilitiesFromServer(caps gen.ServerCapabilities) []string {
	var result []string
	if caps.HoverProvider != nil {
		result = append(result, "hover")
	}
	if caps.DefinitionProvider != nil {
		result = append(result, "definition")
	}
	if caps.ReferencesProvider != nil {
		result = append(result, "references")
	}
	if caps.ImplementationProvider != nil {
		result = append(result, "implementation")
	}
	if caps.TypeDefinitionProvider != nil {
		result = append(result, "type_definition")
	}
	if caps.DocumentSymbolProvider != nil {
		result = append(result, "document_symbol")
	}
	if caps.WorkspaceSymbolProvider != nil {
		result = append(result, "workspace_symbol")
	}
	if caps.CodeActionProvider != nil {
		result = append(result, "code_action")
	}
	if caps.DocumentFormattingProvider != nil {
		result = append(result, "formatting")
	}
	if caps.RenameProvider != nil {
		result = append(result, "rename")
	}
	if caps.CallHierarchyProvider != nil {
		result = append(result, "call_hierarchy")
	}
	if caps.CompletionProvider != nil {
		result = append(result, "completion")
	}
	if caps.SignatureHelpProvider != nil {
		result = append(result, "signature_help")
	}
	if caps.DeclarationProvider != nil {
		result = append(result, "declaration")
	}
	return result
}

// circuitStateString converts a circuit state float64 to a string.
func circuitStateString(state float64) string {
	switch state {
	case CircuitClosed:
		return "closed"
	case CircuitHalfOpen:
		return "half_open"
	case CircuitOpen:
		return "open"
	default:
		return "unknown"
	}
}
