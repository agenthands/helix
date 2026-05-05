// This package's import path under the testdata GOPATH starts with the
// checked prefix github.com/agenthands/helix/internal/kernel, but it
// imports nothing from internal/semantic, so the analyzer MUST stay
// silent. (No analysistest directive on any line.)
package goodpkg

import _ "fmt"
