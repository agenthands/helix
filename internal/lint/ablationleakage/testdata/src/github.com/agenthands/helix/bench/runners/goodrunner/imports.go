// This package's import path under the testdata GOPATH starts with the checked
// prefix `github.com/agenthands/helix/bench/runners`, but it imports only a
// permitted (stdlib) package — a bench runner orchestrates a daemon subprocess
// via os/exec rather than linking the disabled subsystems directly — so the
// analyzer MUST stay silent. (No analysistest directive on any line.)
package goodrunner

import _ "os/exec"
