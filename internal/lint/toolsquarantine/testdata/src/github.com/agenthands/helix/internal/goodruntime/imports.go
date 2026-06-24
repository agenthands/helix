// This package's import path under the testdata GOPATH does NOT start with the
// tools/ prefix, so the analyzer inspects it — but it imports only a permitted
// stdlib package, so the analyzer MUST stay silent. (No analysistest directive
// on any line; analysistest fails on any spurious diagnostic.)
package goodruntime

import _ "os/exec"
