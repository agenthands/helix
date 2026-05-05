// This package is rooted under `internal/kernel/` (NOT `internal/semantic/`),
// so it is outside the analyzer's checked-package scope. Even though it
// imports `internal/kernel` (which would be forbidden if checked), the
// analyzer MUST stay silent because the importing package itself is not in
// scope.
package outofscope

import _ "github.com/agenthands/helix/internal/kernel"
