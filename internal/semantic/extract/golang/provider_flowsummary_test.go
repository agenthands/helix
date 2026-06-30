package goextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

// TestFlowSummary_PopulatedForFunction_NilForContainer proves FLOW-01: the
// dataflow flow-summary engine is plumbed through the shared FingerprintBody
// seam, so the Go provider stamps a FlowSummary onto a function whose params
// reach a target, and leaves container kinds without one. By construction the
// same holds for all 11 providers (they all call FingerprintBody).
func TestFlowSummary_PopulatedForFunction_NilForContainer(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)

	src := []byte(`package bank

type Account struct {
	Balance int
}

func TransferBalance(fromAccount *Account, amount int) error {
	return validateInsufficientLedger(fromAccount)
}
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "bank.go", Language: "go"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	var sawFunc, sawContainer bool
	for _, sf := range ef.Symbols {
		switch sf.Kind {
		case extract.KindFunction, extract.KindMethod:
			if sf.Name == "TransferBalance" {
				sawFunc = true
				if sf.FlowSummary == nil {
					t.Errorf("function %q: FlowSummary nil, want populated (fromAccount reaches validateInsufficientLedger)", sf.Name)
					continue
				}
				// fromAccount (param 0) must reach validateInsufficientLedger arg0.
				var found bool
				for _, pf := range sf.FlowSummary.Params {
					for _, ca := range pf.CallArgs {
						if ca.Callee == "validateInsufficientLedger" && ca.ArgPos == 0 {
							found = true
						}
					}
				}
				if !found {
					t.Errorf("function %q: FlowSummary %+v has no fromAccount->validateInsufficientLedger arg0 flow", sf.Name, sf.FlowSummary)
				}
			}
		case extract.KindStruct, extract.KindClass, extract.KindType, extract.KindInterface:
			sawContainer = true
			if sf.FlowSummary != nil {
				t.Errorf("container %q (%s): FlowSummary populated, want nil (no behavioral body)", sf.Name, sf.Kind)
			}
		}
	}
	if !sawFunc {
		t.Fatal("did not extract the TransferBalance function symbol")
	}
	if !sawContainer {
		t.Fatal("did not extract the Account container symbol")
	}
}
