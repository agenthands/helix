package report

import (
	"encoding/json"
	"fmt"
	"os"
)

// SafetyCompliance is the EVAL-04 guardrail count report derived from Phase 66
// telemetry captured in each EvalResult.GuardrailCompliance.
type SafetyCompliance struct {
	SchemaVersion string                    `json:"schema_version"`
	ByMode        map[string]ModeSafetyAgg `json:"by_mode"`
}

// ModeSafetyAgg aggregates guardrail event counts for one mode.
type ModeSafetyAgg struct {
	Warned         int `json:"warned"`
	Blocked        int `json:"blocked"`
	ReceiptsIssued int `json:"receipts_issued"`
}

// WriteSafetyCompliance aggregates guardrail counts from results and writes
// safety_compliance.json with mode 0600.
func WriteSafetyCompliance(path string, results []EvalResult) error {
	sc := SafetyCompliance{
		SchemaVersion: "1",
		ByMode:        make(map[string]ModeSafetyAgg),
	}

	for _, r := range results {
		agg := sc.ByMode[r.Mode]
		agg.Warned += r.GuardrailCompliance.Warned
		agg.Blocked += r.GuardrailCompliance.Blocked
		agg.ReceiptsIssued += r.GuardrailCompliance.ReceiptsIssued
		sc.ByMode[r.Mode] = agg
	}

	data, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return fmt.Errorf("report.WriteSafetyCompliance marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("report.WriteSafetyCompliance write %q: %w", path, err)
	}
	return nil
}
