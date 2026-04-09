package rss

import (
	"errors"
	"runtime"
	"testing"
)

func TestCurrentRSS(t *testing.T) {
	bytes, err := CurrentRSS()
	switch runtime.GOOS {
	case "linux", "darwin":
		if err != nil {
			t.Fatalf("CurrentRSS() on %s: unexpected error: %v", runtime.GOOS, err)
		}
		if bytes == 0 {
			t.Fatalf("CurrentRSS() on %s: expected non-zero RSS, got 0", runtime.GOOS)
		}
		// Sanity floor: any live Go process will be well above 1 MiB resident.
		if bytes < 1<<20 {
			t.Fatalf("CurrentRSS() on %s: RSS %d bytes below 1 MiB sanity floor", runtime.GOOS, bytes)
		}
	default:
		if !errors.Is(err, ErrUnsupported) {
			t.Fatalf("CurrentRSS() on %s: expected ErrUnsupported, got err=%v bytes=%d", runtime.GOOS, err, bytes)
		}
		if bytes != 0 {
			t.Fatalf("CurrentRSS() on %s: expected 0 bytes, got %d", runtime.GOOS, bytes)
		}
	}
}
