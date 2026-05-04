package daemon

import (
	// Blank imports trigger skill.Register() via init() (Caddy-style).
	_ "github.com/agenthands/helix/internal/kernel/diag"
	_ "github.com/agenthands/helix/internal/kernel/edit"
	_ "github.com/agenthands/helix/internal/kernel/fileops"
	_ "github.com/agenthands/helix/internal/kernel/symbols"
	_ "github.com/agenthands/helix/internal/profile"
	_ "github.com/agenthands/helix/internal/skill/memory"
	_ "github.com/agenthands/helix/internal/skill/repomap"
	_ "github.com/agenthands/helix/internal/skill/workflow"

	// Phase 59 (semantic extraction): the per-language extraction providers
	// — goextract / tsextract / pyextract — and the scheduler are imported
	// NON-BLANK in daemon.go and constructed explicitly in steps 6c and 6d.
	//
	// D-02 hard invariant rejects init()-style provider registration:
	// constructing the providers from main bootstrap is the only way to
	// guarantee each provider receives the daemon-singleton
	// *treesitter.GrammarRegistry (BUG-04 / EXTRACT-05). A blank import here
	// would silently allow an init() to spin up a *second* GrammarRegistry,
	// breaking the singleton invariant. Future readers: do NOT add
	//   _ "github.com/agenthands/helix/internal/semantic/extract/golang"
	// here. The daemon wires them via explicit NewProvider() calls.
)
