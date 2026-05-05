// This package is rooted under `internal/semantic/` but is OUTSIDE the
// narrowed checked prefix (`internal/semantic/lspenrich/`). Even though it
// imports `internal/kernel` (which would be forbidden if checked), the
// analyzer MUST stay silent because the importing package is a sibling of
// lspenrich, not under it. The narrow scope is load-bearing — the broader
// internal/semantic/* tree contains established kernel-importing packages
// (Phase 60 internal/semantic/live/service implements kernel.EditNotifier).
package siblingok

import _ "github.com/agenthands/helix/internal/kernel"
