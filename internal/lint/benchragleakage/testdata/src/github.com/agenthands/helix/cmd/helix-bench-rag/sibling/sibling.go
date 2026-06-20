// Slash-boundary regression guard: `internal/kernelextra` shares the bare-
// HasPrefix collision surface with the forbidden `internal/kernel` prefix but
// lacks the slash boundary. The analyzer MUST stay silent (no want directive).
package sibling

import _ "github.com/agenthands/helix/internal/kernelextra"
