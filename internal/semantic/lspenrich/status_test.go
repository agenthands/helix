package lspenrich_test

import (
	"sync"
	"testing"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/lspenrich"
)

// S1: zero-value Status from a freshly-constructed tracker.
//
// We exercise the surface via Manager.Status() (the only public path) by
// constructing a Manager with a small fake LeaseAcquirer and a fresh
// LaneQueue. No jobs processed → all counters zero, no last errors.
func TestStatus_ZeroValueOnFreshManager(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	m := lspenrich.NewManager(q, &noopAcquirer{}, nil, nil, semantic.LSPEnrichmentConfig{}, nil, nil)

	st := m.Status()

	if st.FilesEnriched != 0 || st.FilesDropped != 0 || st.FilesPreempted != 0 || st.FilesPending != 0 {
		t.Errorf("zero-value Status counters not all zero: %+v", st)
	}
	if st.InFlight != 0 {
		t.Errorf("InFlight=%d, want 0", st.InFlight)
	}
	if st.LaneDepths[lspenrich.LaneHigh] != 0 || st.LaneDepths[lspenrich.LaneBackground] != 0 {
		t.Errorf("lane depths not zero: %+v", st.LaneDepths)
	}
	if len(st.LastErrorPerLanguage) != 0 {
		t.Errorf("LastErrorPerLanguage not empty: %+v", st.LastErrorPerLanguage)
	}
}

// S2: after recording 5 OutcomeApplied bumps, FilesEnriched == 5.
func TestStatus_RecordsAppliedOutcomes(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	m := lspenrich.NewManager(q, &noopAcquirer{}, nil, nil, semantic.LSPEnrichmentConfig{}, nil, nil)

	for i := 0; i < 5; i++ {
		m.RecordOutcomeForTesting(lspenrich.OutcomeApplied)
	}
	st := m.Status()
	if st.FilesEnriched != 5 {
		t.Errorf("FilesEnriched=%d, want 5", st.FilesEnriched)
	}
	if st.FilesPending != 0 {
		t.Errorf("FilesPending=%d, want 0 for OutcomeApplied", st.FilesPending)
	}
}

// S2b (W7): OutcomePartialBudget bumps BOTH FilesEnriched AND FilesPending.
// "applied with caveat" — file is queryable but pending re-enrichment.
func TestStatus_PartialBudget_BumpsBothCounters(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	m := lspenrich.NewManager(q, &noopAcquirer{}, nil, nil, semantic.LSPEnrichmentConfig{}, nil, nil)

	m.RecordOutcomeForTesting(lspenrich.OutcomePartialBudget)
	st := m.Status()
	if st.FilesEnriched != 1 {
		t.Errorf("FilesEnriched=%d, want 1 (W7)", st.FilesEnriched)
	}
	if st.FilesPending != 1 {
		t.Errorf("FilesPending=%d, want 1 (W7)", st.FilesPending)
	}
}

// S3: LastErrorPerLanguage updates when recordError is called.
func TestStatus_RecordsLastErrorPerLanguage(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	m := lspenrich.NewManager(q, &noopAcquirer{}, nil, nil, semantic.LSPEnrichmentConfig{}, nil, nil)

	m.RecordErrorForTesting("go", "circuit_open")
	m.RecordErrorForTesting("go", "timeout") // overwrite
	m.RecordErrorForTesting("java", "readiness_timeout")

	st := m.Status()
	if st.LastErrorPerLanguage["go"] != "timeout" {
		t.Errorf("go last error = %q, want %q", st.LastErrorPerLanguage["go"], "timeout")
	}
	if st.LastErrorPerLanguage["java"] != "readiness_timeout" {
		t.Errorf("java last error = %q, want %q", st.LastErrorPerLanguage["java"], "readiness_timeout")
	}
}

// S3b (W11): LastErrorPerLanguage value is the err.Error() string.
func TestStatus_LastErrorIsString_W11(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	m := lspenrich.NewManager(q, &noopAcquirer{}, nil, nil, semantic.LSPEnrichmentConfig{}, nil, nil)

	m.RecordErrorForTesting("go", "circuit_open")
	st := m.Status()
	if got := st.LastErrorPerLanguage["go"]; got != "circuit_open" {
		t.Errorf("LastErrorPerLanguage[go] = %q, want %q", got, "circuit_open")
	}
}

// S4: Status() is RLock-only — concurrent readers must not block.
//
// 50 readers x 50 reads each, with one writer recording errors / outcomes
// concurrently. With -race the test catches any data race.
func TestStatus_ConcurrentReadersDontBlock(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	m := lspenrich.NewManager(q, &noopAcquirer{}, nil, nil, semantic.LSPEnrichmentConfig{}, nil, nil)

	const nReaders = 50
	const nReads = 50

	var wg sync.WaitGroup

	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				m.RecordErrorForTesting("go", "timeout")
				m.RecordOutcomeForTesting(lspenrich.OutcomeApplied)
			}
		}
	}()

	for i := 0; i < nReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < nReads; j++ {
				_ = m.Status()
			}
		}()
	}

	// Wait briefly for readers; writer keeps running until close(stop).
	readersDone := make(chan struct{})
	go func() {
		wgr := &sync.WaitGroup{}
		wgr.Add(nReaders)
		// readers all share `wg`; signal once outer wg has fewer than the
		// writer's 1 outstanding (i.e. all readers done).  Simpler: just
		// stop the writer after a short loop completion grace.
		close(readersDone)
	}()
	_ = readersDone

	close(stop)
	wg.Wait()
}
