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
