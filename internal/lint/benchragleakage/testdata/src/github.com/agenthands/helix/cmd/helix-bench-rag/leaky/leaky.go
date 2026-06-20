// This package's import path under the testdata GOPATH starts with the checked
// prefix `github.com/agenthands/helix/cmd/helix-bench-rag`, so the analyzer MUST
// report on its forbidden daemon-side import. The deliberate violation lives
// ONLY under testdata/ (which the go tool ignores), so `go vet ./...` on the
// real tree stays green.
package leaky

import _ "github.com/agenthands/helix/internal/kernel" // want `isolated .* must not import .*`
