// This package's import path under the testdata GOPATH starts with the
// checked prefix `github.com/agenthands/helix/internal/kernel`, so the
// analyzer MUST report on its `internal/semantic/...` import.
package badpkg

import _ "github.com/agenthands/helix/internal/semantic/foo" // want `internal/kernel/\* must not import internal/semantic/\*`
