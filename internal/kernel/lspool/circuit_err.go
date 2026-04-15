package lspool

import serr "github.com/postfix/serena/internal/errors"

// ErrCircuitOpen re-exports the canonical sentinel for backward compatibility.
// Callers should migrate to serr.ErrCircuitOpen in Phase 23.
var ErrCircuitOpen = serr.ErrCircuitOpen
