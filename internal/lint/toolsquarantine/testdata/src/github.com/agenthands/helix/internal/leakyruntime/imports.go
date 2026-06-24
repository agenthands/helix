// This package's import path under the testdata GOPATH does NOT start with the
// tools/ prefix (github.com/agenthands/helix/tools), so the analyzer inspects it
// and MUST report on its forbidden dev-time tools/ import. The deliberate
// violation lives ONLY under testdata/, which the go tool ignores, so make vet
// on the real tree stays green.
//
// This is the RED anti-vacuity fixture (T-106-03): deleting the analysistest
// directive on the import line below makes TestAnalyzer_RejectsRuntimeImportingTools
// FAIL, proving the analyzer actually fires on a planted runtime to tools/ leak.
package leakyruntime

import _ "github.com/agenthands/helix/tools/dspytune" // want `runtime package .* must not import .*`
