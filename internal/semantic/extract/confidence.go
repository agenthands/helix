package extract

// Confidence ladder per SPEC §11.2 — closed enum with float32 labels.
// Phase 59 emits exactly ConfidenceTSOnly. Phase 61's LSP-enrichment
// worker raises emitted facts to ConfidenceTSPlusLocal /
// ConfidenceLSPMerged / ConfidenceLSPOnly per the merge rule.
//
// NOT to be confused with the 7-rung §38.2 type-resolution ladder
// (Phase 62 territory: 1.00/0.90/0.80/0.70/0.60/0.45/0.20).
const (
	ConfidenceLSPOnly     float32 = 1.00
	ConfidenceLSPMerged   float32 = 0.95
	ConfidenceTSPlusLocal float32 = 0.80
	ConfidenceTSOnly      float32 = 0.70
	ConfidenceHeuristic   float32 = 0.45
)
