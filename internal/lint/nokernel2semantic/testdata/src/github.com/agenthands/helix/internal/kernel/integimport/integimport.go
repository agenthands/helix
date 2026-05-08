// This package's import path under the testdata GOPATH starts with the
// checked prefix `github.com/agenthands/helix/internal/kernel`, AND it
// imports the explicitly allowlisted `internal/semantic/integ`. The
// analyzer MUST stay silent. No analysistest directive on any line.
package integimport

import _ "github.com/agenthands/helix/internal/semantic/integ"
