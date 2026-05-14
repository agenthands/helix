package retrieval_test

// Phase 69-02 compile-time guard: assert that the bleve meta-key constants
// remain EXPORTED so cross-package consumers (internal/semantic/compact in
// Plan 69-03, internal/daemon in Plan 69-05) can reference them without a
// re-export shim. A future refactor that downcases the leading letter on
// any of these names breaks this compile.

import (
	"github.com/agenthands/helix/internal/semantic/retrieval"
)

var (
	_ = retrieval.MetaKeyCorpusVersion
	_ = retrieval.MetaKeyIndexedFiles
	_ = retrieval.MetaKeyLastCompactAt
)
