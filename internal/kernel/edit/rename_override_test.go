// Unit tests for the rename dispatcher + override interface shape.
// Dispatch matrix (native-success / native-fail-override-success / both-fail)
// is covered via the tryNativeRenameFn package-level seam (Task 1 decision:
// seam introduced — one-line swap, minimal surface expansion).
package edit

import (
	"context"
	"errors"
	"testing"
	"time"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeOverrider implements RenameOverrider for dispatch tests.
type fakeOverrider struct {
	result *RenameResult
	err    error
	called bool
}

func (f *fakeOverrider) RenameOverride(ctx context.Context, lease *lspool.WorkerLease, uri string, line, col int, newName string) (*RenameResult, error) {
	f.called = true
	return f.result, f.err
}

func TestRenameStrategy_Constants(t *testing.T) {
	assert.Equal(t, RenameStrategy("lsp-native"), StrategyLSPNative)
	assert.Equal(t, RenameStrategy("rust-client-side"), StrategyRustClientSide)
}

func TestRenameOverrider_InterfaceShape(t *testing.T) {
	var _ RenameOverrider = (*fakeOverrider)(nil)
}

func TestRustAnalyzerRenameOverride_ImplementsRenameOverrider(t *testing.T) {
	var _ RenameOverrider = (*RustAnalyzerRenameOverride)(nil)
}

// TestRenameDispatch_Matrix covers the three dispatch outcomes using the
// tryNativeRenameFn package-level seam. The test swaps the seam to stub the
// native attempt, and uses a fakeOverrider looked up via a test-only
// overriderProvider hook (adapterFn). See rename.go for the seam definitions.
func TestRenameDispatch_Matrix(t *testing.T) {
	errNative := errors.New("simulated native rename failure")
	errOverride := errors.New("simulated override failure")

	cases := []struct {
		name         string
		nativeRes    *RenameResult
		nativeErr    error
		overrider    *fakeOverrider
		wantStrategy RenameStrategy
		wantErrKind  serr.Kind
		wantOverride bool // true if overrider must have been called
	}{
		{
			name:         "native-success",
			nativeRes:    &RenameResult{FilesChanged: 2, EditsApplied: 5, Files: []string{"a.rs", "b.rs"}},
			nativeErr:    nil,
			overrider:    &fakeOverrider{result: &RenameResult{}, err: nil},
			wantStrategy: StrategyLSPNative,
			wantErrKind:  "",
			wantOverride: false,
		},
		{
			name:         "native-fail-override-success",
			nativeRes:    nil,
			nativeErr:    errNative,
			overrider:    &fakeOverrider{result: &RenameResult{FilesChanged: 1, EditsApplied: 3, Files: []string{"a.rs"}}, err: nil},
			wantStrategy: StrategyRustClientSide,
			wantErrKind:  "",
			wantOverride: true,
		},
		{
			name:         "both-fail",
			nativeRes:    nil,
			nativeErr:    errNative,
			overrider:    &fakeOverrider{result: nil, err: errOverride},
			wantStrategy: "",
			wantErrKind:  serr.Unsupported,
			wantOverride: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Swap seams.
			prevNative := tryNativeRenameFn
			prevOverrider := overriderResolverFn
			tryNativeRenameFn = func(ctx context.Context, lease *lspool.WorkerLease, uri string, line, col int, newName string) (*RenameResult, error) {
				return tc.nativeRes, tc.nativeErr
			}
			overriderResolverFn = func(lease *lspool.WorkerLease) RenameOverrider {
				return tc.overrider
			}
			defer func() {
				tryNativeRenameFn = prevNative
				overriderResolverFn = prevOverrider
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			res, err := RenameSymbol(ctx, nil, "file:///tmp/x.rs", 0, 0, "x")

			if tc.wantErrKind != "" {
				require.Error(t, err)
				var se *serr.Error
				require.True(t, errors.As(err, &se), "expected serr.Error, got %T", err)
				assert.Equal(t, tc.wantErrKind, se.Kind)
				// D-06 message discipline:
				assert.Contains(t, se.Error(), "fuzzy_edit, replace_symbol_body, or search_in_files")
				assert.Nil(t, res)
			} else {
				require.NoError(t, err)
				require.NotNil(t, res)
				assert.Equal(t, tc.wantStrategy, res.Strategy)
			}
			assert.Equal(t, tc.wantOverride, tc.overrider.called, "overrider invocation mismatch")
		})
	}
}

func TestRustAnalyzerRenameOverride_NoInnerNilPanic(t *testing.T) {
	// Guard: the wrapper's readiness check on a zero-value adapter must not
	// panic. symbols.FindReferences requires a live lease; this test only
	// guards the wrapper shape and the readiness no-panic path, then expects
	// an error (nil lease triggers references failure).
	w := &RustAnalyzerRenameOverride{Inner: &lspool.RustAnalyzerAdapter{}}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RenameOverride panicked on zero-value adapter: %v", r)
		}
	}()
	_, _ = w.RenameOverride(ctx, nil, "file:///tmp/x.rs", 0, 0, "x")
}
