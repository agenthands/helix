package forwarder

import (
	cryptoRand "crypto/rand"
	"fmt"
	"io"
)

// generateSessionID creates a unique session identifier using crypto/rand.
//
// Phase 94 RETIRE-01: the stdio forwarder head (RunForwarder) was the original
// home of this helper, but that head is deleted. The sole remaining caller is the
// retained CLI one-shot dial path (CallTool, oneshot.go:57), so the helper stays
// in the forwarder package — it is the daemon's session-isolation key.
//
// WR-06(b): a crypto/rand failure is unrecoverable here — silently returning an
// all-zero ID would collapse every concurrent CLI call into the same session and
// merge their state. On a healthy POSIX system this never fails, but a chroot
// without /dev/urandom or a tightly-jailed container can hit it. Panic instead of
// returning a useless ID.
func generateSessionID() string {
	b := make([]byte, 16)
	if _, err := io.ReadFull(cryptoRand.Reader, b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return fmt.Sprintf("%x", b)
}
