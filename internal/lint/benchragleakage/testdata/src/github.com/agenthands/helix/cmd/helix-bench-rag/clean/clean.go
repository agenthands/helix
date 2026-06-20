// This package's import path under the testdata GOPATH starts with the checked
// prefix `github.com/agenthands/helix/cmd/helix-bench-rag`, but it imports only
// a permitted (stdlib) package — the standalone baseline_rag server links only
// the MCP SDK + bench/ragindex + stdlib — so the analyzer MUST stay silent.
// (No analysistest directive on any line.)
package clean

import _ "os"
