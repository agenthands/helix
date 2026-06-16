// This package's import path under the testdata GOPATH starts with the checked
// prefix `github.com/agenthands/helix/bench/runners`, so the analyzer MUST
// report on its forbidden disabled-subsystem import. The deliberate violation
// lives ONLY under testdata/ (Pitfall 3), which the go tool ignores, so
// `go vet ./...` on the real tree stays green.
package badrunner

import _ "github.com/agenthands/helix/internal/semantic/store" // want `ablation-gated .* must not import .*`
