package lspool

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestWorkerMetrics_ScoreDecay_Synctest pins the decaying reuse score behavior
// (worker.go OnReuse, IdleDuration) against a virtual clock. Without synctest
// this test would need real sleeps and be flaky; with synctest the schedule is
// deterministic.
//
// Per CONTEXT D-05 Tier 3: scheduler-sensitive pool logic covered by a
// deterministic unit test. Primary concurrency coverage still lives in
// test/integration/concurrency_test.go Tiers 1+2 (per research assumption A4).
//
// The decay formula from worker.go:86 is:
//
//	score = score * math.Exp(-elapsed/300) + 1
//
// where elapsed is seconds since LastUsedAt. We verify:
//  1. First reuse sets score to ~1.0 (exp(0)*0 + 1).
//  2. After advancing virtual time 300s with no reuse, IdleDuration reports 300s.
//  3. Second reuse after 300s (1 decay tau) produces score ~= 1*exp(-1) + 1 = 1.3679.
//  4. After another 600s (2 taus), third reuse applies compounded decay to
//     ~= 1.3679*exp(-2) + 1 = 1.1851.
func TestWorkerMetrics_ScoreDecay_Synctest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := &WorkerMetrics{
			LastUsedAt: time.Now(),
		}

		// First reuse — score goes from 0 to 1 (elapsed is ~0 so decay factor is 1,
		// 0*1 + 1 = 1).
		m.OnReuse()
		assert.InDelta(t, 1.0, m.ReuseScore, 1e-9,
			"first OnReuse should set score ~= 1")

		// Advance 300 virtual seconds (exactly one decay time-constant).
		time.Sleep(300 * time.Second)
		assert.InDelta(t, (300 * time.Second).Seconds(),
			m.IdleDuration().Seconds(), 1e-6,
			"IdleDuration should track virtual clock")

		// Second reuse after 1 tau: score = 1*exp(-1) + 1 ~= 1.3679.
		m.OnReuse()
		assert.InDelta(t, 1.3678794411714423, m.ReuseScore, 1e-6,
			"second OnReuse after 1 decay-tau should produce 1/e + 1")

		// IdleDuration should now be ~0 (LastUsedAt was just bumped).
		assert.Less(t, m.IdleDuration(), time.Millisecond,
			"IdleDuration should reset to 0 after OnReuse")

		// Advance another 600 virtual seconds (2 taus).
		time.Sleep(600 * time.Second)
		m.OnReuse()
		// expected = 1.3678794411714423 * exp(-2) + 1 ~= 1.1851223516
		assert.InDelta(t, 1.1851223516044767, m.ReuseScore, 1e-9,
			"third OnReuse after 2 taus should apply compounded decay")
	})
}
