// This package's import path under the testdata GOPATH starts with the
// checked prefix `github.com/agenthands/helix/internal/kernel`, and it
// imports `internal/semantic/integ_evil` — a bare-prefix lookalike of
// the allowlisted `internal/semantic/integ`. Without a slash-boundary
// check the analyzer would silently allow this; A6 demands it still be
// flagged.
package integlookalike

import _ "github.com/agenthands/helix/internal/semantic/integ_evil" // want `internal/kernel/\* must not import internal/semantic/\*`
