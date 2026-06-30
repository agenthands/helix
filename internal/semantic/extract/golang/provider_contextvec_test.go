package goextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

// TestContextVec_PopulatedForFunction_NilForContainer proves RELATE-02:
// FingerprintBody (called by the provider) stamps a Random-Indexing context
// vector onto a function body with enough vocabulary, and leaves container
// kinds (struct/type) without one — they have no behavioral body to read
// vocabulary from. This is the end-to-end proof the engine is plumbed through
// real extraction across the shared seam (and, by construction, across all 11
// providers that call FingerprintBody).
func TestContextVec_PopulatedForFunction_NilForContainer(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)

	// The function carries ample vocabulary (>= relatedidx.MinTokens distinct
	// subtokens: transfer, account, balance, ledger, validate, ...).
	src := []byte(`package bank

type Account struct {
	Balance int
	Owner   string
}

func TransferBalance(fromAccount *Account, toAccount *Account, transferAmount int) error {
	if fromAccount.Balance < transferAmount {
		return validateInsufficientLedger(fromAccount)
	}
	fromAccount.Balance -= transferAmount
	toAccount.Balance += transferAmount
	return nil
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
				if sf.ContextVec == nil {
					t.Errorf("function %q: ContextVec is nil, want populated (vocabulary-rich body)", sf.Name)
				}
			}
		case extract.KindStruct, extract.KindClass, extract.KindType, extract.KindInterface:
			sawContainer = true
			if sf.ContextVec != nil {
				t.Errorf("container %q (%s): ContextVec populated, want nil (no behavioral body)", sf.Name, sf.Kind)
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
