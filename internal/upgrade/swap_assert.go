package upgrade

// Compile-time signature assertions ensuring swap_unix.go and
// swap_windows.go expose function-pair signatures that match across
// platforms. Mirrors the cgo/!cgo signature-assertion discipline
// established in Phase 51.1 WR-03 (commit 48550030); without these,
// a drift in one platform variant (e.g., adding a parameter) would
// only surface during a CI matrix run on the other GOOS, costing a
// round-trip.
//
// These vars compile on every platform (no build tag) because Go's
// build-tag-conditional compilation only excludes files, not the
// package's identifier resolution — the unbuilt variant's `swap`
// and `relaunch` are not in scope, but the active variant's are,
// and the type identity is what `var _ =` checks.
var _ = swap
var _ = relaunch

// swapFn and relaunchFn are package-level indirections so tests can
// drive Upgrade through Step 9 (swap) and Step 10 (relaunch) without
// actually mutating os.Executable() or replacing the test runner
// process image. Production code dispatches through these variables;
// tests overwrite them via t.Cleanup-restored swaps. The real relaunch
// never returns (syscall.Exec on Unix, os.Exit on Windows), so any
// test that observes a return value did so via the override.
var (
	swapFn     = swap
	relaunchFn = relaunch
)
