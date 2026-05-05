// This package's import path under the testdata GOPATH starts with the
// checked prefix `github.com/agenthands/helix/internal/kernel`, but it
// imports nothing from `internal/semantic/...`, so the analyzer MUST stay
// silent. (No `// want` directive.)
package goodpkg

import _ "fmt"
