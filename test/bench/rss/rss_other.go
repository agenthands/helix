//go:build !linux && !darwin

package rss

// currentRSS is a fallback for GOOS values without a platform implementation.
// Callers should treat ErrUnsupported as a soft failure (skip RSS reporting).
func currentRSS() (uint64, error) {
	return 0, ErrUnsupported
}
