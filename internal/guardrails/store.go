package guardrails

import (
	"container/list"
	"sync"
	"time"

	"github.com/agenthands/helix/internal/workspace"
)

// MetricsSink is the metrics interface consumed by the Store.
// Production wiring (Plan 04) passes an adapter around *obs.Metrics.
// Tests pass a fake that captures calls.
type MetricsSink interface {
	ReceiptIssuedInc(class string)
	ReceiptExpiredInc(reason string)
	ReceiptLookupInc(outcome string)
}

// IssueFields carries the ancillary fields for receipt issuance that
// the store needs beyond the class and scope (which are already typed).
type IssueFields struct {
	SnapshotID    uint64
	GraphVersion  uint64
	Freshness     string
	ScoreStatus   string
	ClusterStatus string
	PendingLSPFiles int
	LSPCoverage   float64
	IssuingTool   string
	IssuingCallID string
	TraceID       string
	TTL           time.Duration // 0 = use store default (5 minutes)
}

const (
	defaultTTL     = 5 * time.Minute
	defaultLRUCap  = 10000
	defaultJanitor = 30 * time.Second
)

// StoreOptions configures a Store.
type StoreOptions struct {
	LRUCap  int           // per-workspace LRU cap; 0 = 10000
	Metrics MetricsSink   // nil = no-op
	Now     func() time.Time // nil = time.Now
}

// wsEntry holds the per-workspace sync.Map and LRU bookkeeping.
type wsEntry struct {
	mu      sync.Mutex
	store   map[ReceiptID]*Receipt
	lru     *list.List            // front = most-recently-issued, back = LRU
	lruKeys map[ReceiptID]*list.Element
	cap     int
}

// Store is the in-memory per-workspace receipt store.
// Issue, Get, InvalidateOnGraphVersionAdvance are safe for concurrent use.
type Store struct {
	opts    StoreOptions
	lruCap  int
	now     func() time.Time
	metrics MetricsSink

	mu   sync.RWMutex
	ws   map[workspace.WorkspaceKey]*wsEntry

	done    chan struct{}
	closedMu sync.Mutex
	closed  bool
}

// NewStore creates a Store with a 30-second janitor ticker.
func NewStore(opts StoreOptions) *Store {
	tick := time.NewTicker(defaultJanitor)
	s := newStore(opts, tick.C)
	// Ensure we stop the ticker when done.
	go func() {
		<-s.done
		tick.Stop()
	}()
	return s
}

// NewStoreWithTicker creates a Store with a caller-supplied ticker channel
// (for tests with a controllable fake clock).
func NewStoreWithTicker(opts StoreOptions, tick <-chan time.Time) *Store {
	return newStore(opts, tick)
}

func newStore(opts StoreOptions, tick <-chan time.Time) *Store {
	cap := opts.LRUCap
	if cap <= 0 {
		cap = defaultLRUCap
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	met := opts.Metrics
	if met == nil {
		met = noopMetrics{}
	}
	s := &Store{
		opts:    opts,
		lruCap:  cap,
		now:     now,
		metrics: met,
		ws:      make(map[workspace.WorkspaceKey]*wsEntry),
		done:    make(chan struct{}),
	}
	go s.janitorLoop(tick)
	return s
}

// Close stops the janitor goroutine. Idempotent.
func (s *Store) Close() {
	s.closedMu.Lock()
	defer s.closedMu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.done)
	}
}

// janitorLoop sweeps expired entries on each tick.
func (s *Store) janitorLoop(tick <-chan time.Time) {
	for {
		select {
		case <-s.done:
			return
		case <-tick:
			s.sweepExpired()
		}
	}
}

func (s *Store) sweepExpired() {
	now := s.now()
	s.mu.RLock()
	workspaces := make([]workspace.WorkspaceKey, 0, len(s.ws))
	for k := range s.ws {
		workspaces = append(workspaces, k)
	}
	s.mu.RUnlock()

	for _, ws := range workspaces {
		s.mu.RLock()
		entry, ok := s.ws[ws]
		s.mu.RUnlock()
		if !ok {
			continue
		}
		entry.mu.Lock()
		var toDelete []ReceiptID
		for id, r := range entry.store {
			if now.After(r.ExpiresAt) {
				toDelete = append(toDelete, id)
			}
		}
		for _, id := range toDelete {
			s.removeFromEntry(entry, id)
			s.metrics.ReceiptExpiredInc("ttl")
		}
		entry.mu.Unlock()
	}
}

// getOrCreateEntry returns the wsEntry for ws, creating it if needed.
func (s *Store) getOrCreateEntry(ws workspace.WorkspaceKey) *wsEntry {
	s.mu.RLock()
	entry, ok := s.ws[ws]
	s.mu.RUnlock()
	if ok {
		return entry
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Double-check after write lock.
	if entry, ok = s.ws[ws]; ok {
		return entry
	}
	entry = &wsEntry{
		store:   make(map[ReceiptID]*Receipt),
		lru:     list.New(),
		lruKeys: make(map[ReceiptID]*list.Element),
		cap:     s.lruCap,
	}
	s.ws[ws] = entry
	return entry
}

// removeFromEntry deletes a receipt from the entry's store and LRU.
// Caller must hold entry.mu.
func (s *Store) removeFromEntry(entry *wsEntry, id ReceiptID) {
	delete(entry.store, id)
	if el, ok := entry.lruKeys[id]; ok {
		entry.lru.Remove(el)
		delete(entry.lruKeys, id)
	}
}

// Issue stores a receipt and emits ReceiptIssuedInc(class).
// Returns the new ReceiptID.
func (s *Store) Issue(ws workspace.WorkspaceKey, class ReceiptClass, scope ReceiptScope, fields IssueFields) (ReceiptID, error) {
	id, err := NewReceiptID()
	if err != nil {
		return "", err
	}

	ttl := fields.TTL
	if ttl <= 0 {
		ttl = defaultTTL
	}
	now := s.now()

	r := &Receipt{
		ID:              id,
		Class:           class,
		SchemaVersion:   1,
		WorkspaceKey:    ws,
		SnapshotID:      fields.SnapshotID,
		GraphVersion:    fields.GraphVersion,
		Freshness:       fields.Freshness,
		ScoreStatus:     fields.ScoreStatus,
		ClusterStatus:   fields.ClusterStatus,
		PendingLSPFiles: fields.PendingLSPFiles,
		LSPCoverage:     fields.LSPCoverage,
		IssuedAt:        now,
		ExpiresAt:       now.Add(ttl),
		IssuingTool:     fields.IssuingTool,
		IssuingCallID:   fields.IssuingCallID,
		TraceID:         fields.TraceID,
		Scope:           scope,
	}

	entry := s.getOrCreateEntry(ws)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	// LRU eviction if at cap.
	for len(entry.store) >= entry.cap {
		// Evict least-recently-issued (back of LRU list).
		back := entry.lru.Back()
		if back == nil {
			break
		}
		evictID := back.Value.(ReceiptID)
		s.removeFromEntry(entry, evictID)
		s.metrics.ReceiptExpiredInc("lru_evicted")
	}

	entry.store[id] = r
	el := entry.lru.PushFront(id)
	entry.lruKeys[id] = el

	s.metrics.ReceiptIssuedInc(string(class))
	// F-08: emit tap-compatible JSONL "receipt issued" line consumed by
	// internal/eval/trace/tap.go. Nil-safe; see store_jsonl.go.
	emitReceiptIssued(string(class), ws.RepoRoot, fields.SnapshotID, fields.GraphVersion, fields.TraceID)
	return id, nil
}

// Get retrieves a receipt from the store.
// Returns (receipt, true) on a fresh hit; (nil, false) otherwise.
// Emits appropriate ReceiptLookupInc and ReceiptExpiredInc metrics.
func (s *Store) Get(ws workspace.WorkspaceKey, id ReceiptID) (*Receipt, bool) {
	s.mu.RLock()
	entry, ok := s.ws[ws]
	s.mu.RUnlock()
	if !ok {
		s.metrics.ReceiptLookupInc("miss")
		return nil, false
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()

	r, ok := entry.store[id]
	if !ok {
		s.metrics.ReceiptLookupInc("miss")
		return nil, false
	}

	// Opportunistic expiry check (even before janitor runs).
	if s.now().After(r.ExpiresAt) {
		s.removeFromEntry(entry, id)
		s.metrics.ReceiptExpiredInc("ttl")
		s.metrics.ReceiptLookupInc("expired")
		return nil, false
	}

	// Cross-workspace forgery check (T-66-06).
	if r.WorkspaceKey != ws {
		s.metrics.ReceiptLookupInc("workspace_mismatch")
		return nil, false
	}

	s.metrics.ReceiptLookupInc("hit")
	return r, true
}

// InvalidateOnGraphVersionAdvance deletes all receipts for ws whose
// GraphVersion < newGV (T-66-02 mitigation). Emits ReceiptExpiredInc("graph_drift")
// for each deleted receipt.
func (s *Store) InvalidateOnGraphVersionAdvance(ws workspace.WorkspaceKey, newGV uint64) {
	s.mu.RLock()
	entry, ok := s.ws[ws]
	s.mu.RUnlock()
	if !ok {
		return
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()

	var toDelete []ReceiptID
	for id, r := range entry.store {
		if r.WorkspaceKey == ws && r.GraphVersion < newGV {
			toDelete = append(toDelete, id)
		}
	}
	for _, id := range toDelete {
		s.removeFromEntry(entry, id)
		s.metrics.ReceiptExpiredInc("graph_drift")
	}
}

// noopMetrics is a no-op MetricsSink used when opts.Metrics is nil.
type noopMetrics struct{}

func (noopMetrics) ReceiptIssuedInc(class string)   {}
func (noopMetrics) ReceiptExpiredInc(reason string)  {}
func (noopMetrics) ReceiptLookupInc(outcome string)  {}
