package lspool

// LanguageQuirks holds per-language initialization settings for LS workers (per D-16).
type LanguageQuirks struct {
	// Command is the LS binary command.
	Command string
	// Args are the LS command arguments.
	Args []string
	// InitOptions are passed as initializationOptions in the LSP initialize request.
	InitOptions interface{}
	// NeedsWorkspace indicates whether the LS requires workspace folders in InitializeParams.
	NeedsWorkspace bool
}

// DefaultQuirks maps language names to their LS initialization quirks.
// Phase 2 ships with 4 languages per D-16.
var DefaultQuirks = map[string]LanguageQuirks{
	"go": {
		Command:        "gopls",
		Args:           []string{"serve"},
		NeedsWorkspace: true,
		InitOptions: map[string]interface{}{
			"experimentalWorkspaceModule": true,
		},
	},
	"python": {
		Command:        "pyright-langserver",
		Args:           []string{"--stdio"},
		NeedsWorkspace: true,
		InitOptions:    nil,
	},
	"typescript": {
		Command:        "typescript-language-server",
		Args:           []string{"--stdio"},
		NeedsWorkspace: true,
		InitOptions:    nil,
	},
	"rust": {
		Command:        "rust-analyzer",
		Args:           nil,
		NeedsWorkspace: true,
		InitOptions: map[string]interface{}{
			"cargo": map[string]interface{}{
				"buildScripts": map[string]interface{}{
					"enable": true,
				},
			},
		},
	},
}
