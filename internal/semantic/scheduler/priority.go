package scheduler

import (
	"container/heap"
	"path/filepath"
	"strings"
)

// FilePriority is the scheduling priority for a single file in the
// initial-walk queue. Lower numeric value = higher priority (the underlying
// container is a min-heap).
type FilePriority int

const (
	// PriorityInflightToolReferenced — tier 1: file is being looked at by a
	// tool call right now. Caller (P05 daemon) supplies the inflight set.
	PriorityInflightToolReferenced FilePriority = 0
	// PriorityRepomapImportant — tier 2: top-quartile PageRank from the
	// repomap engine. Phase 59 ships the tier; P05 wires the actual set.
	PriorityRepomapImportant FilePriority = 1
	// PriorityFirstClassSource — tier 3: remaining first-class language
	// source (go / typescript / python).
	PriorityFirstClassSource FilePriority = 2
	// PriorityNonFirstClass — tier 4: file-row-only emit (D-05 unsupported).
	PriorityNonFirstClass FilePriority = 3
)

// PoppedFile is the consumer-visible payload of a priority-queue Pop. The
// internal heap entry is unexported; PopFile returns this struct so callers
// outside the package can drain the queue without leaking heap internals.
type PoppedFile struct {
	Path     string
	Language string // bounded label: "go" | "typescript" | "python" | "other"
	Priority FilePriority
}

// fileTask is the internal heap entry. The `index` field is heap-managed.
type fileTask struct {
	path     string
	language string
	priority FilePriority
	index    int // heap-internal
}

// priorityQueueImpl satisfies heap.Interface. Min-heap by (priority, path) so
// equal-priority files pop in deterministic lexical order.
type priorityQueueImpl []*fileTask

func (pq priorityQueueImpl) Len() int { return len(pq) }
func (pq priorityQueueImpl) Less(i, j int) bool {
	if pq[i].priority != pq[j].priority {
		return pq[i].priority < pq[j].priority
	}
	return pq[i].path < pq[j].path // deterministic tie-break
}
func (pq priorityQueueImpl) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}
func (pq *priorityQueueImpl) Push(x any) {
	n := len(*pq)
	item := x.(*fileTask)
	item.index = n
	*pq = append(*pq, item)
}
func (pq *priorityQueueImpl) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[0 : n-1]
	return item
}

// PriorityQueue is the consumer-facing priority queue handle. Construct via
// BuildPriorityQueue; drain via Len() + PopFile().
type PriorityQueue struct {
	heap *priorityQueueImpl
}

// Len returns the number of items remaining in the queue.
func (pq *PriorityQueue) Len() int {
	if pq == nil || pq.heap == nil {
		return 0
	}
	return pq.heap.Len()
}

// PopFile removes and returns the highest-priority entry. Panics if the queue
// is empty — callers must check Len() > 0.
func (pq *PriorityQueue) PopFile() PoppedFile {
	t := heap.Pop(pq.heap).(*fileTask)
	return PoppedFile{Path: t.path, Language: t.language, Priority: t.priority}
}

// ClassifyByExtension maps a file path to ("go" | "typescript" | "python" | "other").
// Bounded label for the helix_semantic_extraction_total metric (matches the
// allowlist primed in internal/obs/metrics.go for the "language" label of
// SemanticExtraction). Closed enum: do not extend without updating the
// allowlist test.
func ClassifyByExtension(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return "typescript"
	case ".py":
		return "python"
	default:
		return "other"
	}
}

// BuildPriorityQueue constructs a priority queue from a list of files using
// the 4-tier order in CONTEXT.md D-04:
//
//  1. PriorityInflightToolReferenced — caller passes the set of files
//     referenced by the currently-handled MCP tool call (may be nil/empty).
//  2. PriorityRepomapImportant — caller passes the top-quartile PageRank
//     file set (may be nil if repomap is disabled or cold).
//  3. PriorityFirstClassSource — anything classified as go/typescript/python
//     that wasn't already promoted to tier 1 or 2.
//  4. PriorityNonFirstClass — everything else; emits file-row-only via D-05.
//
// The function does NOT import internal/repomap directly — the caller (P05
// daemon wiring) computes the repomap-important set and passes it in. This
// keeps priority.go testable without a running repomap.
//
// I10 (DEFERRED): Phase 59 ships the priority queue infrastructure but does
// NOT wire repomap PageRank into Scheduler.ScheduleInitialExtraction. P05
// passes a nil/empty repomapImportant set in Phase 59; the
// PriorityRepomapImportant tier is structurally reachable but unused. Phase
// 60+ adds the actual PageRank-set construction in the daemon callback path
// when the live-update pipeline lands. Intentional scope deferral per the
// phase 59 must_haves derivation review (planner I10).
func BuildPriorityQueue(files []string, inflightToolReferenced, repomapImportant map[string]struct{}) *PriorityQueue {
	pqi := &priorityQueueImpl{}
	heap.Init(pqi)
	for _, f := range files {
		p := PriorityFirstClassSource
		lang := ClassifyByExtension(f)
		switch {
		case contains(inflightToolReferenced, f):
			p = PriorityInflightToolReferenced
		case contains(repomapImportant, f):
			p = PriorityRepomapImportant
		case lang == "other":
			p = PriorityNonFirstClass
		}
		heap.Push(pqi, &fileTask{path: f, language: lang, priority: p})
	}
	return &PriorityQueue{heap: pqi}
}

// contains reports whether set contains k. Treats a nil set as empty so
// callers can pass nil for "no inflight" / "no repomap" without allocating.
func contains(set map[string]struct{}, k string) bool {
	if set == nil {
		return false
	}
	_, ok := set[k]
	return ok
}
