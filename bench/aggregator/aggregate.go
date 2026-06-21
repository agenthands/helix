package aggregator

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"sort"
	"time"

	"github.com/agenthands/helix/bench/canary"
	"github.com/agenthands/helix/bench/cost"
	"github.com/agenthands/helix/bench/runtime"
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
		// Phase 86 (Plan 05) ADDITIVE contamination-canary column: populate
		// CanaryPassRate AFTER the determinism-locked metric reductions above. It is a
		// flat pooled rate that consumes NO RNG (no bca call), so it cannot perturb the
		// IN-03 metric-order/presence bootstrap contract; it only reads the same loaded
		// rows. NEVER alters the existing leaderboard columns.
		leader.CanaryPassRate = reduceCanaryRate(loaded, tasks, mode)
		// Phase 87 (VERIFIED-02) ADDITIVE raw-vs-UTBoost-rescored side-by-side columns:
		// populate RawScore/RescoredScore AFTER the determinism-locked metric reductions
		// (mirror the CanaryPassRate assignment above). Both are flat pooled rates that
		// consume NO RNG (no bca call), so they cannot perturb the IN-03 metric-order/
		// presence bootstrap contract or the locked leaderboard/cost goldens; they only
		// read the same loaded rows. They are NOT rendered into leaderboard.md/
		// cost_quality.md (so the existing goldens stay byte-for-byte — Phase 86
		// CanaryPassRate discipline); the SWE-bench render is downstream Phase 89.
		leader.RawScore, leader.RescoredScore = reduceSwebenchScores(loaded, tasks, mode)
		// Phase 89 (REPORT-01) ADDITIVE verified_correctness leaderboard column:
		// populate AFTER the determinism-locked metric reductions (mirror the
		// CanaryPassRate/RawScore assignments above). It is a flat pooled rate that
		// consumes NO RNG, so it cannot perturb the IN-03 metric-order/presence
		// bootstrap contract; it only reads the same loaded rows.
		leader.VerifiedCorrectness = reduceVerifiedCorrectness(loaded, tasks, mode)

		costRow := reduceCostRow(loaded, tasks, mode, ct, cfg, alpha, rng)
		// Phase 89 (REPORT-01) SINGLE-SOURCE the leaderboard cost_per_solved column
		// from the SAME reduceCostRow output (Pitfall 3 / IN-02): the leaderboard cost
		// MUST equal cost_quality.md's cost for the same (mode x benchmark) — do NOT
		// run a second cost reduce.
		leader.CostPerSolved = costRow.CostPerSolved
		rep.Leaderboard = append(rep.Leaderboard, leader)
		rep.Cost = append(rep.Cost, costRow)
	}

	// Phase 85 (ADAPTER-AIDER-01) per-language pass-rate slice — the SC#1 substrate.
	// Purely additive: it reads the same loaded rows, never touches the RNG, and
	// does NOT alter the (mode x benchmark) leaderboard/cost output above. Rendering
	// it into a report file is downstream Phase 89; here it only needs to exist on
	// the returned Report and be correct.
	rep.ByLanguage = reduceLanguageRows(loaded)

	// Phase 89 (REPORT-03) aggregate-time ablation reduce. It threads the SAME
	// single seeded rng AFTER the per-mode leaderboard/cost loop above — so the
	// already-computed leaderboard/cost BCa CIs are byte-for-byte unchanged (their
	// rng draws are complete), and the ablation draws are taken last in a fixed,
	// deterministic order. The full vs no_semantic pair is computed HERE from the
	// loaded full + no_semantic rows (deltas.go deliberately omits it — Pitfall 2).
	rep.Ablations = reduceAblations(loaded, tasks, cfg, alpha, rng)

	// INFRA-05 (T-89-02-02): collect the contaminated (task,mode) cells AFTER the
	// per-mode reduce loop (the headline already excluded them). They are carried on
	// the Report and rendered as a deterministic, sorted leaderboard.md footnote so an
	// excluded cell is auditable — never silently dropped.
	rep.Contaminated = contaminatedCells(loaded, tasks, modes)

	// Render + atomically write ALL 4 artifacts through the shared renderAll path
	// (only reached on success). renderAll is the SINGLE render entry point both
	// Aggregate and the helix-bench `report --run-id` subcommand call, so the two
	// surfaces are byte-identical by construction (REPORT-05 byte-reproducibility).
	if err := renderAll(rep, runDir); err != nil {
		return nil, err
	}
	return rep, nil
}

// renderAll is the Phase 89 (REPORT-05) SHARED render path: it turns a reduced
// *Report into ALL 4 byte-stable markdown artifacts — leaderboard.md (incl. the
// INFRA-05 contamination footnote), cost_quality.md (incl. the REPORT-04 scatter),
// per_language.md (REPORT-02), and ablations.md (REPORT-03) — and atomically writes
// each into runDir via writeReport. It is the SINGLE render entry point both
// Aggregate and `helix-bench report --run-id` invoke, so the two surfaces produce
// byte-identical reports over the same run tree (no render drift — T-89-03-02). It
// is pure-deterministic: every renderer sorts before emit and consumes NO RNG, so a
// second renderAll over the same *Report yields the same bytes (double-render
// diff-empty). The scatter is built HERE (not by the caller) so report and aggregate
// share the exact same single-source cost/verified_correctness join.
func renderAll(rep *Report, runDir string) error {
	// Phase 89 (REPORT-04) scatter points: SINGLE-SOURCE cost from rep.Cost and
	// verified_correctness from rep.Leaderboard, joined by (mode x benchmark), so
	// the scatter's axes are the same numbers the leaderboard/cost tables publish.
	scatter := buildScatterPoints(rep.Leaderboard, rep.Cost)

	// INFRA-05: append the contamination footnote to the rendered leaderboard so the
	// excluded (task,mode) cells are auditable in the published artifact.
	lb := renderLeaderboard(rep.Leaderboard, rep.PassNK, rep.Footer)
	lb = appendContaminationFootnote(lb, rep.Contaminated)
	cq := renderCostQuality(rep.Cost, scatter, rep.Footer)
	pl := renderPerLanguage(rep.ByLanguage, rep.Footer)
	ab := renderAblations(rep.Ablations, rep.Footer)

	for _, out := range []struct{ name, content string }{
		{"leaderboard.md", lb},
		{"cost_quality.md", cq},
		{"per_language.md", pl},
		{"ablations.md", ab},
	} {
		if err := writeReport(runDir, out.name, out.content); err != nil {
			return err
		}
	}
	return nil
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
		// INFRA-05 canary EXCLUSION (T-89-02-01): split the cell's rows into
		// clean/contaminated and feed ONLY the clean rows into every headline reduce
		// below. A row whose completion echoed the canary Sentinel never reaches the
		// pass@1/verified_correctness/cost vectors — fail-safe so contaminated data
		// can never silently inflate the headline. reduceCanaryRate (the MEASUREMENT)
		// deliberately keeps reading ALL rows; only the headline EXCLUDES.
		rows, _ = cleanRows(rows)
		if len(rows) == 0 {
			// The whole cell was contaminated — it contributes nothing to the headline.
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

// reduceLanguageRows builds the Phase 85 (ADAPTER-AIDER-01) per-language pass-rate
// slice — the SC#1 substrate. It groups EVERY loaded row across all (task,mode)
// cells by its `language` doc key (rowLanguage), then computes a per-language
// pass-rate via the SAME success-count scalar path reduceLeaderRow uses
// (successCount): pass-rate == #task_success-true / #non-nil-task_success, so a
// nil task_success is excluded from both numerator and denominator (Pitfall 4
// null discipline — never fabricated as a failure). This is intentionally a flat
// pooled pass-rate across the language's runs, NOT a BCa-bootstrapped CI: SC#1
// needs the SLICE to exist and be correct; the bootstrapped per-language CI is
// downstream (Phase 89). Rows whose language key is absent bucket under "" so a
// pre-language run still contributes without breaking anything. The returned slice
// is sorted by language for determinism (D-08); a language whose runs all carry a
// nil task_success (N==0) is dropped (no honest pass-rate to report).
func reduceLanguageRows(loaded *Loaded) []LanguageRow {
	type acc struct{ c, n int }
	byLang := map[string]*acc{}

	for _, task := range loaded.Tasks() {
		for _, mode := range loaded.Modes(task) {
			rows := loaded.Rows(task, mode)
			// Group this cell's rows by language; a single cell could in principle mix
			// languages, so bucket per row rather than per cell.
			perLang := map[string][]Row{}
			for _, r := range rows {
				lang := rowLanguage(r)
				perLang[lang] = append(perLang[lang], r)
			}
			for lang, lrows := range perLang {
				c, n := successCount(lrows)
				a := byLang[lang]
				if a == nil {
					a = &acc{}
					byLang[lang] = a
				}
				a.c += c
				a.n += n
			}
		}
	}

	out := make([]LanguageRow, 0, len(byLang))
	for lang, a := range byLang {
		if a.n == 0 {
			// No non-nil task_success for this language — no honest pass-rate.
			continue
		}
		out = append(out, LanguageRow{
			Language: lang,
			PassRate: float64(a.c) / float64(a.n),
			N:        a.n,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Language < out[j].Language })
	return out
}

// rowCanary reads the open-provenance `completion` doc key from a row and runs
// bench/canary.IsContaminated over it at SCORE TIME — the contamination flag is
// DERIVED here in the aggregator path, NOT by editing the CCE/RepoBench loaders
// (Plans 03/04 own them). It returns (contaminated, present): present==false when
// the row carries no completion key (a pre-canary artifact), so such a row
// contributes nothing to the canary pass-rate (Pitfall 4 null discipline — never
// fabricated as clean OR contaminated). It mirrors rowLanguage/rowModelID exactly.
func rowCanary(r Row) (contaminated bool, present bool) {
	raw, ok := r.Doc[canary.DocKeyCompletion]
	if !ok {
		return false, false
	}
	var completion string
	if err := json.Unmarshal(raw, &completion); err != nil {
		return false, false
	}
	return canary.IsContaminated(completion), true
}

// cleanRows is the INFRA-05 (T-89-02-01) clean/contaminated split: it partitions a
// cell's rows by rowCanary's verdict — a row that is BOTH present AND contaminated
// (its completion echoed the canary Sentinel) goes to contaminated; EVERY other row
// (a clean completion, OR a pre-canary row with no completion key — Pitfall 4 null
// discipline) stays clean. The headline reduces (reduceLeaderRow, reduceCostRow)
// consume ONLY the clean partition, so a contaminated row can never silently inflate
// the headline (fail-safe). reduceCanaryRate is deliberately NOT routed through this
// — it MEASURES contamination over all rows; only the headline EXCLUDES. Order is
// preserved for determinism.
func cleanRows(rows []Row) (clean, contaminated []Row) {
	for _, r := range rows {
		cont, present := rowCanary(r)
		if present && cont {
			contaminated = append(contaminated, r)
			continue
		}
		clean = append(clean, r)
	}
	return clean, contaminated
}

// contaminatedCells returns the sorted set of (task, mode) cells that contain at
// least one contaminated row, for the INFRA-05 (T-89-02-02) leaderboard footnote.
// A cell is flagged iff cleanRows reports any contaminated row in it. The result is
// sorted (mode, then task) for a deterministic, auditable footnote — contaminated
// cells are EXCLUDED from the headline but never disappear without a trace.
func contaminatedCells(loaded *Loaded, tasks, modes []string) []ContaminatedCell {
	var out []ContaminatedCell
	for _, mode := range modes {
		for _, task := range tasks {
			rows := loaded.Rows(task, mode)
			if len(rows) == 0 {
				continue
			}
			if _, contaminated := cleanRows(rows); len(contaminated) > 0 {
				out = append(out, ContaminatedCell{Task: task, Mode: mode})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Mode != out[j].Mode {
			return out[i].Mode < out[j].Mode
		}
		return out[i].Task < out[j].Task
	})
	return out
}

// reduceCanaryRate computes the Phase 86 (Plan 05) ADDITIVE CanaryPassRate for one
// (mode x benchmark): a flat pooled fraction of rows-carrying-a-completion whose
// completion did NOT echo the canary sentinel (canary "pass" == clean), derived at
// score time via rowCanary. It mirrors the reduceLanguageRows pooled-rate path: a
// row with no completion key is excluded from BOTH numerator and denominator. When
// NO row in the cell carries a completion key the rate is a NULL ci (OK==false,
// rendered em-dash) — never a fabricated 0. It is intentionally NOT a BCa CI (the
// bootstrapped canary CI is downstream Phase 89), so it consumes no RNG and cannot
// perturb the locked determinism contract.
func reduceCanaryRate(loaded *Loaded, tasks []string, mode string) ciValue {
	var clean, total int
	for _, task := range tasks {
		for _, r := range loaded.Rows(task, mode) {
			contaminated, present := rowCanary(r)
			if !present {
				continue
			}
			total++
			if !contaminated {
				clean++
			}
		}
	}
	if total == 0 {
		// No completion data anywhere in this cell — null canary signal (em-dash).
		return ciValue{OK: false}
	}
	rate := float64(clean) / float64(total)
	// A flat pooled point with degenerate [point, point] endpoints: it is a rate,
	// not a bootstrapped interval (Phase 89 owns the CI). OK==true so it renders.
	return ciValue{Point: rate, Lo: rate, Hi: rate, OK: true}
}

// rowSwebenchScores reads the open-provenance raw/rescored doc keys from a row at
// SCORE TIME via the PINNED runtime.SwebenchRawResolvedKey /
// runtime.SwebenchRescoredVerifiedKey consts — the SAME consts Plan 03's
// rescore.ApplyToRow stamps, so producer and reader can NEVER drift to different
// spellings (the canary.DocKeyCompletion shared-const precedent, NOT "agree by
// comment"). It returns (raw, rescored, present): present==false when the row carries
// NEITHER key (a non-SWE-bench artifact), so such a row contributes nothing to either
// rate (Pitfall 4 null discipline — never fabricated). It mirrors rowCanary exactly.
// A row missing a key reads its pointer as nil (excluded from that rate's counts).
func rowSwebenchScores(r Row) (raw *bool, rescored *bool, present bool) {
	raw = rowBoolKey(r, runtime.SwebenchRawResolvedKey)
	rescored = rowBoolKey(r, runtime.SwebenchRescoredVerifiedKey)
	return raw, rescored, raw != nil || rescored != nil
}

// rowBoolKey json.Unmarshals a bool from the named open doc key, returning nil when
// the key is absent or not a bool (excluded — never coerced to a default verdict).
func rowBoolKey(r Row, key string) *bool {
	rawMsg, ok := r.Doc[key]
	if !ok {
		return nil
	}
	var v bool
	if err := json.Unmarshal(rawMsg, &v); err != nil {
		return nil
	}
	return &v
}

// reduceSwebenchScores computes the Phase 87 (VERIFIED-02) ADDITIVE RawScore /
// RescoredScore for one (mode x benchmark): two flat pooled fractions over the rows
// carrying each respective open key, derived at score time via rowSwebenchScores. It
// mirrors reduceCanaryRate's pooled-rate path: a row with no swebench keys is excluded
// from BOTH numerator and denominator of each rate. When NO row carries a given key
// that rate is a NULL ci (OK==false, rendered em-dash) — never a fabricated 0. Each is
// a degenerate [point, point] ciValue (NOT a BCa CI), so it consumes ZERO RNG and
// cannot perturb the locked determinism contract; the bootstrapped CI is downstream
// (Phase 89).
func reduceSwebenchScores(loaded *Loaded, tasks []string, mode string) (rawCI, rescoredCI ciValue) {
	var rawTrue, rawTotal, rescoredTrue, rescoredTotal int
	for _, task := range tasks {
		for _, r := range loaded.Rows(task, mode) {
			raw, rescored, present := rowSwebenchScores(r)
			if !present {
				continue
			}
			if raw != nil {
				rawTotal++
				if *raw {
					rawTrue++
				}
			}
			if rescored != nil {
				rescoredTotal++
				if *rescored {
					rescoredTrue++
				}
			}
		}
	}
	return pooledRate(rawTrue, rawTotal), pooledRate(rescoredTrue, rescoredTotal)
}

// reduceVerifiedCorrectness computes the Phase 89 (REPORT-01) ADDITIVE
// VerifiedCorrectness for one (mode x benchmark): a flat pooled fraction of the
// rows whose decoded verified_correctness *bool (rowMetrics.VerifiedCorrectness,
// load.go:32) is true over the rows whose verdict is non-nil. It mirrors
// reduceCanaryRate/reduceSwebenchScores' pooled-rate path exactly: a nil verdict
// is excluded from BOTH numerator and denominator (Pitfall 4 null discipline —
// never fabricated as a failure). When NO row in the cell carries a verdict the
// rate is a NULL ci (OK==false, rendered em-dash) — never a fabricated 0. It is a
// flat pooled rate (NOT a BCa CI), so it consumes ZERO RNG and cannot perturb the
// locked determinism contract; the bootstrapped verified CI is a later concern.
func reduceVerifiedCorrectness(loaded *Loaded, tasks []string, mode string) ciValue {
	var trueCount, total int
	for _, task := range tasks {
		for _, r := range loaded.Rows(task, mode) {
			v := r.Metrics.VerifiedCorrectness
			if v == nil {
				continue
			}
			total++
			if *v {
				trueCount++
			}
		}
	}
	return pooledRate(trueCount, total)
}

// successVectorForMode builds the across-task per-task success-rate vector for one
// mode — the SAME Level-1 boolean reduction reduceLeaderRow uses for TaskSuccess
// (successCount -> c/n per task). It returns (vec, present): present==false when
// the mode has NO rows in any task (the mode is absent from the loaded tree), so
// the ablation reduce can render an em-dash rather than a fabricated 0.
func successVectorForMode(loaded *Loaded, tasks []string, mode string) (vec []float64, present bool) {
	for _, task := range tasks {
		rows := loaded.Rows(task, mode)
		if len(rows) == 0 {
			continue
		}
		present = true
		c, n := successCount(rows)
		if n > 0 {
			vec = append(vec, float64(c)/float64(n))
		}
	}
	return vec, present
}

// reduceAblations computes the Phase 89 (REPORT-03) aggregate-time full-vs-other
// deltas over the fixed ablationComparisons set. For each comparison it re-reduces
// the full and other modes' task_success vectors to BCa CIs (the SAME bca helper /
// success path the leaderboard uses) and records whether the other operand is
// present. The full vs no_semantic pair is produced HERE — deltas.go deliberately
// omits no_semantic as a delta operand (Pitfall 2). It threads the shared rng in
// the fixed comparison order; bca consumes NO rng for an empty/absent vector so an
// absent operand cannot shift a later comparison's draws.
func reduceAblations(loaded *Loaded, tasks []string, cfg Config, alpha float64, rng *rand.Rand) []AblationRow {
	out := make([]AblationRow, 0, len(ablationComparisons))
	for _, cmp := range ablationComparisons {
		fullVec, fullPresent := successVectorForMode(loaded, tasks, cmp.full)
		otherVec, otherPresent := successVectorForMode(loaded, tasks, cmp.other)
		row := AblationRow{
			Comparison: cmp.name,
			FullCI:     bca(fullVec, cfg.Iterations, alpha, rng),
			OtherCI:    bca(otherVec, cfg.Iterations, alpha, rng),
			Present:    fullPresent && otherPresent,
		}
		out = append(out, row)
	}
	return out
}

// buildScatterPoints joins the Phase 89 (REPORT-04) scatter inputs SINGLE-SOURCE:
// cost from the cost rows and verified_correctness from the leaderboard rows,
// keyed by (mode x benchmark). Each leaderboard row yields one ScatterPoint whose
// Cost is the matching cost row's CostPerSolved (null if no cost row) and whose
// VerifiedCorrectness is the row's already-reduced value. A point whose cost or
// verified_correctness is null is still emitted but renderScatter declines to plot
// it (never a fabricated 0). The output order mirrors rep.Leaderboard; renderScatter
// re-sorts deterministically before plotting.
func buildScatterPoints(leader []LeaderRow, cost []CostRow) []ScatterPoint {
	costByKey := make(map[string]ciValue, len(cost))
	for _, c := range cost {
		costByKey[c.Mode+"\x00"+c.Benchmark] = c.CostPerSolved
	}
	out := make([]ScatterPoint, 0, len(leader))
	for _, l := range leader {
		out = append(out, ScatterPoint{
			Mode:                l.Mode,
			Benchmark:           l.Benchmark,
			Cost:                costByKey[l.Mode+"\x00"+l.Benchmark],
			VerifiedCorrectness: l.VerifiedCorrectness,
		})
	}
	return out
}

// pooledRate builds a degenerate [point, point] ciValue from a true-count over a
// total. A zero total is a NULL ci (OK==false, em-dash) — never a fabricated 0. It
// consumes NO RNG (the bootstrapped CI is downstream Phase 89).
func pooledRate(trueCount, total int) ciValue {
	if total == 0 {
		return ciValue{OK: false}
	}
	rate := float64(trueCount) / float64(total)
	return ciValue{Point: rate, Lo: rate, Hi: rate, OK: true}
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
		// INFRA-05 canary EXCLUSION (T-89-02-01): the cost headline reduces over CLEAN
		// rows only — a contaminated cell never contributes a cost_per_solved datum,
		// mirroring the leaderboard exclusion. cleanRows is the single source of the
		// clean/contaminated split.
		rows, _ = cleanRows(rows)
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

// rowLanguage reads the additive open provenance `language` key from a row's
// preserved doc (Phase 85, ADAPTER-AIDER-01), mirroring rowModelID exactly. It
// returns "" when the key is absent — a pre-language artifact — which buckets the
// row under the unsliced "" language. NEVER derive the language from task_id here;
// the persisted Cell.Language is the only source (task_id parsing is the
// documented downstream fallback only).
func rowLanguage(r Row) string {
	raw, ok := r.Doc["language"]
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
