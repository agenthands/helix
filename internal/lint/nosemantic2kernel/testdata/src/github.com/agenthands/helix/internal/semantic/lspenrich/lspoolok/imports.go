// This package's import path is rooted under `internal/semantic/`, but the
// only kernel-prefixed import it uses is `internal/kernel/lspool` — that is on
// the explicit carve-out allow-list (Phase 61 ENRICH-01). The analyzer MUST
// stay silent here.
package lspoolok

import _ "github.com/agenthands/helix/internal/kernel/lspool"
