package aggregator

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"sort"
	"time"

	"github.com/agenthands/helix/bench/cost"
)

// aggregate.go is the STATS-02/03/04 + COST-03 orchestrator: a PURE function
// that loads a multi-run durable tree (Plan 05 Load), performs the two-level
// reduction (D-07) — Level 1 reduces each (task,mode)'s N runs to one per-task
// scalar per metric, Level 2 bootstraps the across-task vector via BCaInterval —
// and renders leaderboard.md + cost_quality.md. It WRITES NOTHING when Load
// returns a deficiency (D-05 fail-closed). It REUSES the Wave-2 primitives
// (BCaInterval/StatMean, PassAtK, perResultUSD/costPerSolvedTask) — it does not
// reimplement any of them. All randomness flows through ONE seeded RNG per call
// so reports are byte-deterministic (D-08).

// defaultIterations is the BCa replicate floor (D-07): fewer than 10000 is
// raised to 10000 so a thin caller config never degrades CI quality.
const defaultIterations = 10000

// defaultCILevel is the standard 95% confidence level when the caller leaves it 0.
const defaultCILevel = 0.95

// Config is the orchestrator's injected configuration. ExpectedN is the caller's
// --runs/manifest value (NEVER len(glob), Pitfall 3). Seed makes the bootstrap
// reproducible (D-08). Iterations is floored to 10000. CILevel defaults to 0.95.
// KValues are the pass@k columns (default {1, ExpectedN}). CostTablePath is the
// pinned cost table; Today is the injected freshness clock (never time.Now()).
type Config struct {
	ExpectedN     int
	Seed          uint64
	Iterations    int
	CILevel       float64
	KValues       []int
	CostTablePath string
	Today         time.Time
}

// withDefaults returns a copy of cfg with the documented floors/defaults applied.
func (c Config) withDefaults() Config {
	out := c
	if out.Iterations < defaultIterations {
		out.Iterations = defaultIterations
	}
	if out.CILevel <= 0 || out.CILevel >= 1 {
		out.CILevel = defaultCILevel
	}
	if len(out.KValues) == 0 {
		out.KValues = []int{1, out.ExpectedN}
	}
	return out
}

// Aggregate is the pure orchestrator. It loads runDir (fail-closed on a deficient
// cell), reduces every metric over the two levels, computes BCa CIs through a
// single seeded RNG, renders both reports, writes them atomically into runDir,
// and returns the built *Report. On a Load deficiency it returns (nil, error)
// and writes NOTHING (D-05).
func Aggregate(runDir string, cfg Config) (*Report, error) {
	cfg = cfg.withDefaults()

	// Fail-closed load (D-05): any deficient (task,mode) returns an error and we
	// write no reports.
	loaded, err := Load(runDir, cfg.ExpectedN)
	if err != nil {
		return nil, err
	}

	// Cost table is loaded ONCE; pricing per result reuses bench/cost (Open Q1).
	//
	// WR-03 fail-CLOSED on a cost-table LOAD failure: a missing, unparseable, or
	// empty cost table is an operator/config error, NOT a per-row pricing gap.
	// Previously ctErr was swallowed and degraded EVERY cost cell to an em-dash
	// while the CLI still reported success and exited 0 — undermining the D-13
	// fail-closed freshness-gate intent at the aggregator boundary. A LOAD failure
	// now returns a hard error and writes NOTHING.
	//
	// This is DISTINCT from the legitimate per-cell soft em-dash: a row whose
	// model_id has no price (cost.PriceFor returns an error) or whose tokens are
	// all nil still degrades to "—" inside reduceCostRow, because that is missing
	// DATA for one cell, not a broken cost table.
	ct, err := cost.LoadCostTable(cfg.CostTablePath)
	if err != nil {
		return nil, fmt.Errorf("aggregate: cost table unavailable: %w", err)
	}
	validUntil := costTableValidUntil(ct)

	alpha := 1 - cfg.CILevel
	// ONE RNG per Aggregate, threaded into every BCaInterval (D-08).
	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed^0x9e3779b97f4a7c15))

	modes := discoverModes(loaded)
	tasks := loaded.Tasks()

	footer := Footer{
		Seed:                cfg.Seed,
		Iterations:          cfg.Iterations,
		CILevel:             cfg.CILevel,
		Runs:                cfg.ExpectedN,
		CostTableValidUntil: validUntil,
	}

	// IN-01: the pass@N leaderboard column actually holds pass@kN where kN is the
	// largest configured k <= ExpectedN (pickKN), which need not equal ExpectedN
	// (e.g. KValues={1,2}, ExpectedN=5 -> pass@2). Carry the real k so the header
	// is self-describing instead of a misleading hard-coded "pass@N".
	rep := &Report{Footer: footer, PassNK: pickKN(cfg.KValues, cfg.ExpectedN)}

	for _, mode := range modes {
		leader := reduceLeaderRow(loaded, tasks, mode, cfg, alpha, rng)
		rep.Leaderboard = append(rep.Leaderboard, leader)

		costRow := reduceCostRow(loaded, tasks, mode, ct, cfg, alpha, rng)
		rep.Cost = append(rep.Cost, costRow)
	}

	// Render + atomically write both artifacts (only reached on success).
	lb := renderLeaderboard(rep.Leaderboard, rep.PassNK, rep.Footer)
	cq := renderCostQuality(rep.Cost, rep.Footer)
	if err := writeReport(runDir, "leaderboard.md", lb); err != nil {
		return nil, err
	}
	if err := writeReport(runDir, "cost_quality.md", cq); err != nil {
		return nil, err
	}
	return rep, nil
}

// discoverModes returns the sorted set of modes present across all tasks, so the
// row order is deterministic.
func discoverModes(loaded *Loaded) []string {
	seen := map[string]bool{}
	for _, task := range loaded.Tasks() {
		for _, mode := range loaded.Modes(task) {
			seen[mode] = true
		}
	}
	modes := make([]string, 0, len(seen))
	for m := range seen {
		modes = append(modes, m)
	}
	sort.Strings(modes)
	return modes
}

// reduceLeaderRow performs the two-level reduction for one (mode x benchmark):
// Level 1 reduces each task's N runs to a per-task scalar per metric; Level 2
// bootstraps the across-task vector via BCaInterval. A metric absent across every
// task yields a null CI (rendered em-dash).
func reduceLeaderRow(loaded *Loaded, tasks []string, mode string, cfg Config, alpha float64, rng *rand.Rand) LeaderRow {
	row := LeaderRow{Mode: mode, Benchmark: "internal-toolbench"}

	// Per-task vectors for each continuous metric (Level 1 -> across-task vector).
	var successVec []float64 // per-task success-rate (== pass@1)
	var passNVec []float64
	var tiVec, toVec, tcVec, frVec, locVec []float64

	kN := pickKN(cfg.KValues, cfg.ExpectedN)

	for _, task := range tasks {
		rows := loaded.Rows(task, mode)
		if len(rows) == 0 {
			continue
		}
		// Level 1 boolean: success-rate c/n.
		c, nBool := successCount(rows)
		if nBool > 0 {
			rate := float64(c) / float64(nBool)
			successVec = append(successVec, rate)
			// pass@N via the unbiased estimator (pass@1 == c/n is the same vector).
			passNVec = append(passNVec, PassAtK(nBool, c, kN))
		}
		// Level 1 continuous: mean over non-nil runs.
		if v, ok := meanInt(rows, func(m rowMetrics) *int { return m.TokensInput }); ok {
			tiVec = append(tiVec, v)
		}
		if v, ok := meanInt(rows, func(m rowMetrics) *int { return m.TokensOutput }); ok {
			toVec = append(toVec, v)
		}
		if v, ok := meanInt(rows, func(m rowMetrics) *int { return m.ToolCalls }); ok {
			tcVec = append(tcVec, v)
		}
		if v, ok := meanInt(rows, func(m rowMetrics) *int { return m.FilesRead }); ok {
			frVec = append(frVec, v)
		}
		if v, ok := meanFloat(rows, func(m rowMetrics) *float64 { return m.EditLocality }); ok {
			locVec = append(locVec, v)
		}
	}

	// Level 2: BCa CI of the mean across tasks for every metric.
	//
	// DETERMINISM CONTRACT (IN-03): a single *rand.Rand is threaded sequentially
	// through every bca() call below, and bca() consumes NO RNG when its metric
	// vector is empty (see bca's early return). Therefore the bootstrap draws of
	// each metric depend on (a) the FIXED textual ORDER of these reductions and
	// (b) which earlier metrics were present/absent. Output stays byte-deterministic
	// for a fixed input, but REORDERING these lines — or adding/removing a metric
	// above an existing one — silently shifts every subsequent metric's CI.
	// Treat this metric-reduction order as part of the locked determinism contract.
	row.TaskSuccess = bca(successVec, cfg.Iterations, alpha, rng)
	row.PassAt1 = row.TaskSuccess // c/n identity (pass@1 == success-rate)
	row.PassAtN = bca(passNVec, cfg.Iterations, alpha, rng)
	row.TokensInput = bca(tiVec, cfg.Iterations, alpha, rng)
	row.TokensOutput = bca(toVec, cfg.Iterations, alpha, rng)
	row.ToolCalls = bca(tcVec, cfg.Iterations, alpha, rng)
	row.FilesRead = bca(frVec, cfg.Iterations, alpha, rng)
	row.EditLocality = bca(locVec, cfg.Iterations, alpha, rng)
	return row
}

// reduceCostRow builds the COST-03 cost_quality.md row for one (mode x
// benchmark): per-solved-task mean USD feeds both the cost_per_solved_task point
// (costPerSolvedTask) and the per-solved-task USD vector for the cost BCa CI; the
// FAIR-03 CV detector flags any (task,mode) whose per-run USD CV exceeds 0.05.
func reduceCostRow(loaded *Loaded, tasks []string, mode string, ct cost.CostTable, cfg Config, alpha float64, rng *rand.Rand) CostRow {
	row := CostRow{Mode: mode, Benchmark: "internal-toolbench"}

	perSolvedUSD := map[string]float64{} // task -> mean USD (solved tasks only)
	var solvedUSDVec []float64           // per-solved-task mean USD (cost CI unit)

	for _, task := range tasks {
		rows := loaded.Rows(task, mode)
		if len(rows) == 0 {
			continue
		}
		perRunUSD := perRunUSDs(rows, ct, cfg.Today)
		// FAIR-03 CV detector over per-run USD (A2): flag CV > 0.05.
		if cv := coefVariation(perRunUSD); cv > cvThreshold {
			row.VarianceFlags = append(row.VarianceFlags, VarianceFlag{Task: task, Mode: mode, CV: cv})
		}
		// Solved-task filter (D-12): only tasks whose success-rate marks them solved
		// contribute to cost_per_solved_task. "Solved" here == majority/all success;
		// we use task_success==true on the cell (success-rate > 0.5) as the gate.
		c, nBool := successCount(rows)
		solved := nBool > 0 && float64(c)/float64(nBool) > 0.5
		if solved && len(perRunUSD) > 0 {
			meanUSD := mean(perRunUSD)
			perSolvedUSD[task] = meanUSD
			solvedUSDVec = append(solvedUSDVec, meanUSD)
		}
	}

	// Point estimate via the COST-02 primitive; CI via BCa over the per-solved-
	// task USD vector. IN-02: the primitive's RETURNED value is the published
	// point — previously only its ok flag was read and the headline silently came
	// from bca()'s own StatMean. The two are equal by construction today, but
	// sourcing the point from costPerSolvedTask keeps the rendered headline
	// honest if either reduction later diverges (e.g. weighting). The BCa Lo/Hi
	// still come from the bootstrap over solvedUSDVec.
	point, ok := costPerSolvedTask(perSolvedUSD)
	if !ok {
		row.CostPerSolved = ciValue{OK: false}
		return row
	}
	ci := bca(solvedUSDVec, cfg.Iterations, alpha, rng)
	if ci.OK {
		ci.Point = point // publish the COST-02 primitive's value as the headline
	}
	row.CostPerSolved = ci
	return row
}

// perRunUSDs prices each run's tokens via the cost-table join (reusing
// perResultUSD). A run whose model_id cannot be priced or whose tokens are all
// nil contributes no USD (excluded). The returned slice is per-run, run-index
// ascending (rows are already sorted), so CV is computed over comparable units.
func perRunUSDs(rows []Row, ct cost.CostTable, today time.Time) []float64 {
	var out []float64
	for _, r := range rows {
		modelID := rowModelID(r)
		if modelID == "" {
			continue
		}
		priced, err := cost.PriceFor(ct, modelID, today)
		if err != nil {
			continue
		}
		if usd, ok := perResultUSD(r.Metrics, priced); ok {
			out = append(out, usd)
		}
	}
	return out
}

// rowModelID reads the open provenance `model_id` from a row's preserved doc.
func rowModelID(r Row) string {
	raw, ok := r.Doc["model_id"]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

// successCount returns (#runs with task_success==true, #runs with a non-nil
// task_success). A nil task_success is excluded from BOTH counts so it is never
// fabricated as a failure (Pitfall 4).
func successCount(rows []Row) (c, n int) {
	for _, r := range rows {
		if r.Metrics.TaskSuccess == nil {
			continue
		}
		n++
		if *r.Metrics.TaskSuccess {
			c++
		}
	}
	return c, n
}

// meanInt reduces a (task,mode)'s N runs to the mean of a *int metric over the
// NON-NIL runs. ok==false when the metric is nil on every run (Level-1 absent).
func meanInt(rows []Row, get func(rowMetrics) *int) (float64, bool) {
	var sum float64
	var n int
	for _, r := range rows {
		if p := get(r.Metrics); p != nil {
			sum += float64(*p)
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	return sum / float64(n), true
}

// meanFloat is meanInt for a *float64 metric.
func meanFloat(rows []Row, get func(rowMetrics) *float64) (float64, bool) {
	var sum float64
	var n int
	for _, r := range rows {
		if p := get(r.Metrics); p != nil {
			sum += *p
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	return sum / float64(n), true
}

// mean is the arithmetic mean of a non-empty slice (0 for empty).
func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, v := range xs {
		s += v
	}
	return s / float64(len(xs))
}

// bca is the Level-2 helper: BCaInterval of StatMean over the across-task vector,
// returning a ciValue. An empty vector yields a null CI (rendered em-dash).
func bca(vec []float64, iterations int, alpha float64, rng *rand.Rand) ciValue {
	if len(vec) == 0 {
		// IN-03: an empty metric vector returns WITHOUT touching rng, so the
		// presence/absence of one metric shifts the bootstrap draws of every
		// subsequent metric. This is the source of the metric-order/presence
		// sensitivity documented in reduceLeaderRow's determinism contract.
		return ciValue{OK: false}
	}
	lo, hi, ok := BCaInterval(vec, StatMean, iterations, alpha, rng)
	if !ok {
		return ciValue{OK: false}
	}
	return ciValue{Point: StatMean(vec), Lo: lo, Hi: hi, OK: true}
}

// pickKN chooses the pass@N column's k: the largest configured k that does not
// exceed ExpectedN, defaulting to ExpectedN.
func pickKN(kValues []int, expectedN int) int {
	kN := expectedN
	for _, k := range kValues {
		if k > 1 && k <= expectedN {
			kN = k
		}
	}
	if kN < 1 {
		kN = 1
	}
	return kN
}

// costTableValidUntil returns the earliest valid_until across the cost table's
// rows (the binding freshness horizon for the whole table). Empty when no rows.
func costTableValidUntil(ct cost.CostTable) string {
	earliest := ""
	for _, r := range ct.Rows {
		if earliest == "" || r.ValidUntil < earliest {
			earliest = r.ValidUntil
		}
	}
	return earliest
}
