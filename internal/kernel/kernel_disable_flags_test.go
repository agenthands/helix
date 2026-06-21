package kernel

import "testing"

// TestKernelDisableFlags asserts the Phase 76 (ABLATE-05 / ABLATE-07) kernel
// subsystem-disable flags are carried on KernelConfig and surfaced via
// accessors, with zero-value defaulting to ENABLED (opt-in disable, D-02).
//
// The accessors read only k.config, so a bare struct literal kernel is
// sufficient (same approach as notifier_test.go's newTestKernel).
func TestKernelDisableFlags(t *testing.T) {
	t.Run("StructuredEditDisabled true when flag set", func(t *testing.T) {
		k := &Kernel{config: KernelConfig{DisableStructuredEditSubsystem: true}}
		if !k.StructuredEditDisabled() {
			t.Fatalf("StructuredEditDisabled() = false, want true")
		}
		// Setting only the structured-edit flag must not disable LSP.
		if k.LSPSubsystemDisabled() {
			t.Fatalf("LSPSubsystemDisabled() = true, want false (flag not set)")
		}
	})

	t.Run("LSPSubsystemDisabled true when flag set", func(t *testing.T) {
		k := &Kernel{config: KernelConfig{DisableLSPSubsystem: true}}
		if !k.LSPSubsystemDisabled() {
			t.Fatalf("LSPSubsystemDisabled() = false, want true")
		}
		// Setting only the LSP flag must not disable structured edits.
		if k.StructuredEditDisabled() {
			t.Fatalf("StructuredEditDisabled() = true, want false (flag not set)")
		}
	})

	t.Run("zero-value KernelConfig defaults both to false", func(t *testing.T) {
		k := &Kernel{config: KernelConfig{}}
		if k.StructuredEditDisabled() {
			t.Fatalf("StructuredEditDisabled() = true, want false (default-enabled)")
		}
		if k.LSPSubsystemDisabled() {
			t.Fatalf("LSPSubsystemDisabled() = true, want false (default-enabled)")
		}
	})
}
