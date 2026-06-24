// This package's import path (github.com/agenthands/helix/internal/toolsupport)
// shares the bare substring "tools" with the forbidden tools/ prefix but lacks
// the slash boundary — and, crucially, it imports nothing under tools/. The
// analyzer MUST stay silent.
//
// This is the slash-boundary regression guard (the same noduckdb /
// ablationleakage discipline): a bare strings.HasPrefix(path, toolsPrefix) on
// the IMPORT side, or treating a bare "tools" substring in a PACKAGE path as a
// match, would over-flag legitimate sibling packages. No //want directive:
// analysistest fails the test if the analyzer reports anything here.
package toolsupport

import _ "os"
