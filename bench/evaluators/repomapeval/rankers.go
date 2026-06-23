package repomapeval

import "math/rand/v2"

// DiscriminatorMargin is the committed absolute nDCG@10 margin the forward
// ranking MUST beat max(reversed, seeded-random) by, aggregated (mean) across
// the corpus (D-09). It is set ABOVE the observed forward-vs-adversarial spread
// on the authored corpus so a near-tie on a small corpus cannot sneak a vacuous
// pass through, and documented in CORPUS.md with its rationale. ≥0.2 is the
// guide (D-09/A6); 0.30 leaves headroom under the observed ≈1.0-vs-low spread.
const DiscriminatorMargin = 0.30

// discriminatorSeed / discriminatorSeed2 seed the seeded-random ranker's PCG so
// its shuffle is byte-identical across runs (the committed-baseline determinism
// contract). They are arbitrary fixed constants, NOT security-sensitive.
const (
	discriminatorSeed  uint64 = 0x9E3779B97F4A7C15
	discriminatorSeed2 uint64 = 0xD1B54A32D192ED03
)

// reversedRanker returns forward reversed — the adversarial worst-case
// discriminator. Reversing a gold-front-loaded ranking pushes the gold IDs off
// the top-k nDCG discount window, so nDCG@10 collapses relative to forward. It
// does not mutate the input slice.
func reversedRanker(forward []string) []string {
	out := make([]string, len(forward))
	for i, id := range forward {
		out[len(forward)-1-i] = id
	}
	return out
}

// seededRandomRanker returns a deterministically shuffled copy of forward — the
// chance-floor discriminator. It uses math/rand/v2 rand.NewPCG(seed, seed2) for
// byte-identical seeded output (NOT time-seeded math/rand — Don't-Hand-Roll, the
// aggregator/bootstrap.go PCG pattern). It does not mutate the input slice.
func seededRandomRanker(forward []string, seed, seed2 uint64) []string {
	out := append([]string(nil), forward...)
	rng := rand.New(rand.NewPCG(seed, seed2))
	rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}
