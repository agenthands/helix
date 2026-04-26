package edit

import (
	"errors"
	"fmt"
	"testing"

	"github.com/postfix/serena/internal/fuzzy"
)

func TestClassifyOutcome(t *testing.T) {
	ambig := fmt.Errorf("ambig: %w", fuzzy.ErrAmbiguous)
	plain := errors.New("io")

	cases := []struct {
		name     string
		strategy fuzzy.Strategy
		err      error
		want     string
	}{
		{"exact-success", fuzzy.StrategyExact, nil, OutcomeSuccess},
		{"empty-success", "", nil, OutcomeSuccess},
		{"whitespace-fuzzy", fuzzy.StrategyWhitespace, nil, OutcomeFuzzyApplied},
		{"indent-fuzzy", fuzzy.StrategyIndentationFlex, nil, OutcomeFuzzyApplied},
		{"ambiguity", fuzzy.StrategyExact, ambig, OutcomeRefusedAmbiguous},
		{"failed-strategy", fuzzy.StrategyFailed, plain, OutcomeFailed},
		{"empty-with-err", "", plain, OutcomeFailed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ClassifyOutcome(c.strategy, c.err); got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}

// TestNoopSink_ZeroAlloc confirms the NoopSink path is allocation-free
// (Phase 53 D-15 invariant: noop default must not allocate). Plan 53-01
// SUMMARY noted only lspool.NoopSink was alloc-tested; this co-locates the
// edit.NoopSink alloc check.
func TestNoopSink_ZeroAlloc(t *testing.T) {
	var sink MetricsSink = NoopSink{}
	allocs := testing.AllocsPerRun(100, func() {
		sink.EditOutcomeInc("replace_symbol_body", OutcomeSuccess)
	})
	if allocs != 0 {
		t.Fatalf("NoopSink.EditOutcomeInc allocated %v times, want 0", allocs)
	}
}
