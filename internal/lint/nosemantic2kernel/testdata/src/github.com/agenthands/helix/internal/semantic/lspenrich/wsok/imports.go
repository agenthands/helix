// This package is rooted under `internal/semantic/` but only imports
// `internal/workspace` — that path is OUTSIDE the forbidden prefix
// (`internal/kernel/...`) entirely, so the analyzer never even considers it
// a candidate. The analyzer MUST stay silent.
package wsok

import _ "github.com/agenthands/helix/internal/workspace"
