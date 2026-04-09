// benchgate is a Go-native benchmark regression gate for Serena.
//
// It reads two benchfmt text files (a committed baseline and a new run),
// parses them using golang.org/x/perf/benchfmt (Pitfall 9 mitigation: we
// do NOT parse human-oriented benchstat text), and fails when a benchmark
// regresses by more than the configured time/allocs threshold AND the
// regression is statistically significant per Welch's t-test at the given
// alpha (D-01: no escape hatch — both conditions required).
//
// Tiered defaults (Phase 9 D-01):
//
//	PR tier      (default): time 15%, allocs 25%, alpha 0.05
//	Release tier (--release-tier): time 10%, allocs 20%, alpha 0.05
//
// p99 is reported by benchstat elsewhere but is deliberately NOT gated here
// (phase Q5 + Pitfall 10).
//
// Exit codes:
//
//	0 — no blocking regression (including --warn-only, which always exits 0)
//	1 — at least one blocking regression (delta > threshold AND p < alpha)
//	2 — usage / I/O / parse error
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

	"golang.org/x/perf/benchfmt"
	"golang.org/x/perf/benchmath"
)

// Gated units. Time unit is "sec/op" after benchfmt tidies the raw "ns/op"
// values. Allocs unit is "allocs/op".
const (
	unitTime   = "sec/op"
	unitAllocs = "allocs/op"
)

// sampleSet holds the per-sample values of a single (benchmark, unit) pair.
// We keep every sample so benchmath can compute Welch's t-test, not just
// the mean.
type sampleSet struct {
	values []float64
}

// mean returns the arithmetic mean of the samples. Returns 0 if empty.
func (s *sampleSet) mean() float64 {
	if len(s.values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range s.values {
		sum += v
	}
	return sum / float64(len(s.values))
}

// parsedFile is keyed: benchName -> unit -> sampleSet.
type parsedFile map[string]map[string]*sampleSet

// parseFile reads a benchfmt text file and groups samples by (name, unit).
// Returns a parse error (non-syntactic I/O error) only for hard failures;
// per-line SyntaxErrors are treated as fatal here because a malformed
// baseline or run file indicates an upstream bug we must not silently
// tolerate in a regression gate.
func parseFile(path string) (parsedFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseReader(f, path)
}

func parseReader(r io.Reader, fileName string) (parsedFile, error) {
	out := make(parsedFile)
	rd := benchfmt.NewReader(r, fileName)
	for rd.Scan() {
		rec := rd.Result()
		if se, ok := rec.(*benchfmt.SyntaxError); ok {
			return nil, fmt.Errorf("benchfmt parse error: %w", se)
		}
		res, ok := rec.(*benchfmt.Result)
		if !ok {
			// UnitMetadata or future record types — skip.
			continue
		}
		name := string(res.Name.Full())
		byUnit, ok := out[name]
		if !ok {
			byUnit = make(map[string]*sampleSet)
			out[name] = byUnit
		}
		for _, v := range res.Values {
			set, ok := byUnit[v.Unit]
			if !ok {
				set = &sampleSet{}
				byUnit[v.Unit] = set
			}
			set.values = append(set.values, v.Value)
		}
	}
	if err := rd.Err(); err != nil {
		return nil, fmt.Errorf("benchfmt read error: %w", err)
	}
	return out, nil
}

// breach describes a single (benchmark, unit) regression that exceeds the
// threshold and is statistically significant (if checkedSig is true).
type breach struct {
	name      string
	unit      string
	baseMean  float64
	newMean   float64
	deltaPct  float64 // (new-base)/base
	pValue    float64 // Welch's t-test p-value; 0 means exact or below floor
	threshold float64
	n1, n2    int
}

// compareResult is the full comparison report for a run (including
// non-breaches used for logging).
type compareResult struct {
	breaches      []breach
	missingInNew  []string
	missingInBase []string
	comparedCount int
}

// compare runs the tiered gate over the intersection of baseline and new.
// It does NOT consult the --warn-only flag — that is the caller's
// responsibility so compare() stays pure and testable.
func compare(base, neu parsedFile, timeThreshold, allocsThreshold, alpha float64) compareResult {
	var res compareResult

	// Stable iteration order for deterministic output.
	names := make([]string, 0, len(base))
	for n := range base {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		baseUnits := base[name]
		newUnits, ok := neu[name]
		if !ok {
			res.missingInNew = append(res.missingInNew, name)
			continue
		}

		unitNames := make([]string, 0, len(baseUnits))
		for u := range baseUnits {
			unitNames = append(unitNames, u)
		}
		sort.Strings(unitNames)

		for _, unit := range unitNames {
			baseSet := baseUnits[unit]
			newSet, ok := newUnits[unit]
			if !ok {
				continue
			}
			res.comparedCount++

			// Only gate on time and allocs. All other units (B/op, MB/s,
			// user-defined custom units, p99/ns/op etc.) are logged but
			// never fail the gate per phase Q5.
			var threshold float64
			switch unit {
			case unitTime:
				threshold = timeThreshold
			case unitAllocs:
				threshold = allocsThreshold
			default:
				continue
			}

			baseMean := baseSet.mean()
			newMean := newSet.mean()
			if baseMean == 0 {
				// Cannot compute percentage from zero baseline; skip.
				continue
			}
			deltaPct := (newMean - baseMean) / baseMean
			if deltaPct <= threshold {
				// Improvement or within tolerance. Either way, not a breach.
				continue
			}

			// Statistical significance via Welch's t-test. benchmath's
			// AssumeNothing uses the Mann-Whitney U-test; AssumeNormal
			// uses Welch's t-test which is what benchstat defaults to
			// and what CONTEXT.md D-01 explicitly names.
			//
			// NOTE: "Welch's t-test" is the statistic we want, and
			// benchmath.AssumeNormal.Compare implements it.
			cmp := compareSamples(baseSet.values, newSet.values)
			if cmp.P >= alpha {
				// Regression not statistically significant — do not block.
				continue
			}
			res.breaches = append(res.breaches, breach{
				name:      name,
				unit:      unit,
				baseMean:  baseMean,
				newMean:   newMean,
				deltaPct:  deltaPct,
				pValue:    cmp.P,
				threshold: threshold,
				n1:        cmp.N1,
				n2:        cmp.N2,
			})
		}
	}

	// Track benches that are NEW (present in new run but not in baseline)
	// so they show up in the report as informational warnings.
	for name := range neu {
		if _, ok := base[name]; !ok {
			res.missingInBase = append(res.missingInBase, name)
		}
	}
	sort.Strings(res.missingInNew)
	sort.Strings(res.missingInBase)

	return res
}

// compareSamples runs Welch's t-test on two samples using benchmath. It
// is extracted so tests can verify significance handling independently.
func compareSamples(a, b []float64) benchmath.Comparison {
	t := &benchmath.DefaultThresholds
	sa := benchmath.NewSample(a, t)
	sb := benchmath.NewSample(b, t)
	// AssumeNormal implements Welch's t-test; see benchmath/anormal.go and
	// benchstat's own default assumption.
	return benchmath.AssumeNormal.Compare(sa, sb)
}

// render writes a human-readable report of cmp to w. The output is
// intentionally stable and tab-separated for each breach so CI log
// scraping is easy.
func render(w io.Writer, cmp compareResult, warnOnly bool) {
	if warnOnly {
		fmt.Fprintln(w, "benchgate: WARN-ONLY mode — exit 0 regardless of breaches")
	}
	fmt.Fprintf(w, "benchgate: compared %d (benchmark, unit) pairs\n", cmp.comparedCount)
	if len(cmp.missingInNew) > 0 {
		fmt.Fprintf(w, "benchgate: warning — %d benchmark(s) in baseline not present in new run:\n", len(cmp.missingInNew))
		for _, n := range cmp.missingInNew {
			fmt.Fprintf(w, "  - %s\n", n)
		}
	}
	if len(cmp.missingInBase) > 0 {
		fmt.Fprintf(w, "benchgate: warning — %d new benchmark(s) not present in baseline (no gate applied):\n", len(cmp.missingInBase))
		for _, n := range cmp.missingInBase {
			fmt.Fprintf(w, "  - %s\n", n)
		}
	}
	if len(cmp.breaches) == 0 {
		fmt.Fprintln(w, "OK: no significant regressions detected.")
		return
	}
	fmt.Fprintf(w, "FAIL: %d regression(s) exceed threshold AND are statistically significant (p < alpha):\n", len(cmp.breaches))
	fmt.Fprintln(w, "bench\tunit\tbase\tnew\tdelta_pct\tp_value\tthreshold")
	for _, b := range cmp.breaches {
		fmt.Fprintf(w, "%s\t%s\t%.6g\t%.6g\t%+.2f%%\t%.4f\t%+.0f%%\n",
			b.name, b.unit, b.baseMean, b.newMean,
			100*b.deltaPct, b.pValue, 100*b.threshold)
	}
}

// config is the fully resolved flag state. Encapsulated in a struct so the
// command-line surface is testable from unit tests without shelling out.
type config struct {
	baseline        string
	newPath         string
	timeThreshold   float64
	allocsThreshold float64
	alpha           float64
	releaseTier     bool
	warnOnly        bool
}

// parseFlags parses argv into a config, mirroring the defaults from D-01.
// The release-tier override is applied after parsing so a user may still
// explicitly tighten or relax a single threshold on top of the tier.
func parseFlags(args []string, out io.Writer) (*config, error) {
	fs := flag.NewFlagSet("benchgate", flag.ContinueOnError)
	fs.SetOutput(out)
	cfg := &config{}
	fs.StringVar(&cfg.baseline, "baseline", "", "path to the committed baseline benchfmt file (required)")
	fs.StringVar(&cfg.newPath, "new", "", "path to the new benchfmt file to compare against baseline (required)")
	// D-01 PR defaults: 15% time / 25% allocs at p<0.05.
	fs.Float64Var(&cfg.timeThreshold, "time-threshold", 0.15, "max allowed mean time regression as a fraction (PR default 0.15, release 0.10)")
	fs.Float64Var(&cfg.allocsThreshold, "allocs-threshold", 0.25, "max allowed mean allocs/op regression as a fraction (PR default 0.25, release 0.20)")
	fs.Float64Var(&cfg.alpha, "alpha", 0.05, "Welch's t-test significance threshold (D-01 mandates 0.05; no escape hatch)")
	fs.BoolVar(&cfg.releaseTier, "release-tier", false, "if set, override defaults to 0.10/0.20 per D-01 release tier")
	fs.BoolVar(&cfg.warnOnly, "warn-only", false, "if set, always exit 0 regardless of breaches (used during two-step baseline rollout)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if cfg.baseline == "" || cfg.newPath == "" {
		fs.Usage()
		return nil, errors.New("benchgate: --baseline and --new are required")
	}

	// Apply release-tier overrides ONLY if the user did not explicitly
	// set the thresholds. This lets --release-tier --time-threshold=0.05
	// still work as expected (tighter than release default).
	if cfg.releaseTier {
		explicitTime := false
		explicitAllocs := false
		fs.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "time-threshold":
				explicitTime = true
			case "allocs-threshold":
				explicitAllocs = true
			}
		})
		if !explicitTime {
			cfg.timeThreshold = 0.10
		}
		if !explicitAllocs {
			cfg.allocsThreshold = 0.20
		}
	}
	return cfg, nil
}

// run is the unit-testable main body. It returns the process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	cfg, err := parseFlags(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, err)
		return 2
	}

	base, err := parseFile(cfg.baseline)
	if err != nil {
		fmt.Fprintf(stderr, "benchgate: failed to read baseline %q: %v\n", cfg.baseline, err)
		return 2
	}
	neu, err := parseFile(cfg.newPath)
	if err != nil {
		fmt.Fprintf(stderr, "benchgate: failed to read new run %q: %v\n", cfg.newPath, err)
		return 2
	}

	cmp := compare(base, neu, cfg.timeThreshold, cfg.allocsThreshold, cfg.alpha)
	render(stdout, cmp, cfg.warnOnly)

	if cfg.warnOnly {
		return 0
	}
	if len(cmp.breaches) > 0 {
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
