// This package's import path under the testdata GOPATH starts with the
// checked prefix `github.com/agenthands/helix/internal/semantic`, so the
// analyzer MUST report on its `internal/kernel` import (no carve-out match).
package badpkg

import _ "github.com/agenthands/helix/internal/kernel" // want `internal/semantic/\* must not import internal/kernel/\*`
