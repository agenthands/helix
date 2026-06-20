package langregistry

// defaultEntries contains all embedded language server definitions.
// Grouped by quirk level: LOW first, then MEDIUM, then HIGH.
var defaultEntries = map[string]LSEntry{

	// ---------------------------------------------------------------
	// LOW quirk languages -- standard --stdio, minimal init options
	// ---------------------------------------------------------------

	"python": {
		Language: "python", Command: "pyright-langserver", Args: []string{"--stdio"},
		NeedsWorkspace: true, FileExts: []string{".py", ".pyi"},
		Install: &InstallInfo{Type: "pip", Package: "pyright"},
	},
	"python_jedi": {
		Language: "python_jedi", Command: "jedi-language-server", Args: []string{},
		NeedsWorkspace: true, FileExts: []string{".py", ".pyi"},
		Install: &InstallInfo{Type: "pip", Package: "jedi-language-server"},
	},
	"python_ty": {
		Language: "python_ty", Command: "ty", Args: []string{"server"},
		NeedsWorkspace: true, FileExts: []string{".py", ".pyi"},
		Install: &InstallInfo{Type: "pip", Package: "ty"},
	},
	"typescript": {
		Language: "typescript", Command: "typescript-language-server", Args: []string{"--stdio"},
		NeedsWorkspace: true, FileExts: []string{".ts", ".tsx", ".mts", ".cts"},
		Install: &InstallInfo{Type: "npm", Package: "typescript-language-server", Version: "5.1.3"},
	},
	"bash": {
		Language: "bash", Command: "bash-language-server", Args: []string{"start"},
		NeedsWorkspace: false, FileExts: []string{".sh", ".bash", ".zsh"},
		Install: &InstallInfo{Type: "npm", Package: "bash-language-server"},
	},
	"ruby": {
		Language: "ruby", Command: "ruby-lsp", Args: []string{},
		NeedsWorkspace: true, FileExts: []string{".rb", ".rake", ".gemspec"},
		Install: &InstallInfo{Type: "gem", Package: "ruby-lsp"},
	},
	"ruby_solargraph": {
		Language: "ruby_solargraph", Command: "solargraph", Args: []string{"stdio"},
		NeedsWorkspace: true, FileExts: []string{".rb", ".rake", ".gemspec"},
		Install: &InstallInfo{Type: "gem", Package: "solargraph"},
	},
	"php": {
		Language: "php", Command: "intelephense", Args: []string{"--stdio"},
		NeedsWorkspace: true, FileExts: []string{".php"},
		Install: &InstallInfo{Type: "npm", Package: "intelephense"},
	},
	"php_phpactor": {
		Language: "php_phpactor", Command: "phpactor", Args: []string{"language-server"},
		NeedsWorkspace: true, FileExts: []string{".php"},
	},
	"perl": {
		Language: "perl", Command: "perl", Args: []string{"-MPerl::LanguageServer", "-e", "Perl::LanguageServer::run"},
		NeedsWorkspace: false, FileExts: []string{".pl", ".pm"},
	},
	"dart": {
		Language: "dart", Command: "dart", Args: []string{"language-server", "--protocol=lsp"},
		NeedsWorkspace: true, FileExts: []string{".dart"},
	},
	"erlang": {
		Language: "erlang", Command: "erlang_ls", Args: []string{},
		NeedsWorkspace: true, FileExts: []string{".erl", ".hrl"},
	},
	"fortran": {
		Language: "fortran", Command: "fortls", Args: []string{"--stdio"},
		NeedsWorkspace: true, FileExts: []string{".f90", ".f95", ".f03", ".f08", ".f", ".for"},
		Install: &InstallInfo{Type: "pip", Package: "fortls"},
	},
	"yaml": {
		Language: "yaml", Command: "yaml-language-server", Args: []string{"--stdio"},
		NeedsWorkspace: false, FileExts: []string{".yaml", ".yml"},
		Install: &InstallInfo{Type: "npm", Package: "yaml-language-server"},
	},
	"toml": {
		Language: "toml", Command: "taplo", Args: []string{"lsp", "stdio"},
		NeedsWorkspace: false, FileExts: []string{".toml"},
		Install: &InstallInfo{Type: "cargo", Package: "taplo-cli"},
	},
	"markdown": {
		Language: "markdown", Command: "marksman", Args: []string{"server"},
		NeedsWorkspace: false, FileExts: []string{".md", ".markdown"},
		Install: &InstallInfo{Type: "binary", Package: "marksman"},
	},
	"zig": {
		Language: "zig", Command: "zls", Args: []string{},
		NeedsWorkspace: true, FileExts: []string{".zig"},
	},
	"nix": {
		Language: "nix", Command: "nixd", Args: []string{},
		NeedsWorkspace: false, FileExts: []string{".nix"},
	},
	"ocaml": {
		Language: "ocaml", Command: "ocamllsp", Args: []string{},
		NeedsWorkspace: true, FileExts: []string{".ml", ".mli"},
	},
	"lean4": {
		Language: "lean4", Command: "lean", Args: []string{"--server"},
		NeedsWorkspace: true, FileExts: []string{".lean"},
	},
	"luau": {
		Language: "luau", Command: "luau-lsp", Args: []string{"lsp"},
		NeedsWorkspace: false, FileExts: []string{".luau"},
	},
	"solidity": {
		Language: "solidity", Command: "solidity-ls", Args: []string{"--stdio"},
		NeedsWorkspace: true, FileExts: []string{".sol"},
		Install: &InstallInfo{Type: "npm", Package: "@nomicfoundation/solidity-language-server"},
	},
	"systemverilog": {
		Language: "systemverilog", Command: "verible-verilog-ls", Args: []string{},
		NeedsWorkspace: false, FileExts: []string{".sv", ".svh", ".v"},
	},
	"ansible": {
		Language: "ansible", Command: "ansible-language-server", Args: []string{"--stdio"},
		NeedsWorkspace: false, FileExts: []string{".yml", ".yaml"},
		Install: &InstallInfo{Type: "npm", Package: "@ansible/ansible-language-server"},
	},
	"elm": {
		Language: "elm", Command: "elm-language-server", Args: []string{"--stdio"},
		NeedsWorkspace: true, FileExts: []string{".elm"},
		Install: &InstallInfo{Type: "npm", Package: "@elm-tooling/elm-language-server"},
	},
	"pascal": {
		Language: "pascal", Command: "pasls", Args: []string{},
		NeedsWorkspace: true, FileExts: []string{".pas", ".pp", ".lpr"},
	},
	"r": {
		Language: "r", Command: "R", Args: []string{"--slave", "-e", "languageserver::run()"},
		NeedsWorkspace: true, FileExts: []string{".r", ".R", ".rmd"},
	},
	"al": {
		Language: "al", Command: "al-language-server", Args: []string{"--stdio"},
		NeedsWorkspace: true, FileExts: []string{".al"},
	},
	"swift": {
		Language: "swift", Command: "sourcekit-lsp", Args: []string{},
		NeedsWorkspace: true, FileExts: []string{".swift"},
	},
	"hlsl": {
		Language: "hlsl", Command: "shader_language_server", Args: []string{"--stdio"},
		NeedsWorkspace: false, FileExts: []string{".hlsl", ".hlsli", ".fx"},
		Install: &InstallInfo{Type: "cargo", Package: "shader_language_server"},
	},
	"matlab": {
		Language: "matlab", Command: "matlab-language-server", Args: []string{"--stdio"},
		NeedsWorkspace: true, FileExts: []string{".m"},
	},

	// ---------------------------------------------------------------
	// MEDIUM quirk languages -- custom init options, special handling
	// ---------------------------------------------------------------

	"go": {
		Language: "go", Command: "gopls", Args: []string{"serve"},
		NeedsWorkspace: true, FileExts: []string{".go"},
		InitOptions: map[string]any{
			"experimentalWorkspaceModule": true,
		},
		IgnoredDirs: []string{"vendor"},
	},
	"rust": {
		Language: "rust", Command: "rust-analyzer", Args: nil,
		NeedsWorkspace: true, FileExts: []string{".rs"},
		InitOptions: map[string]any{
			"cargo": map[string]any{
				"buildScripts": map[string]any{"enable": true},
			},
		},
	},
	"cpp": {
		Language: "cpp", Command: "clangd", Args: []string{"--background-index"},
		NeedsWorkspace: false, FileExts: []string{".c", ".cpp", ".cc", ".cxx", ".h", ".hpp", ".hxx"},
		Install: &InstallInfo{Type: "binary", Package: "clangd"},
	},
	"cpp_ccls": {
		Language: "cpp_ccls", Command: "ccls", Args: []string{},
		NeedsWorkspace: false, FileExts: []string{".c", ".cpp", ".cc", ".cxx", ".h", ".hpp", ".hxx"},
	},
	"csharp": {
		Language: "csharp", Command: "csharp-ls", Args: []string{},
		NeedsWorkspace: true, FileExts: []string{".cs"},
		Install: &InstallInfo{Type: "dotnet", Package: "csharp-ls"},
	},
	"csharp_omnisharp": {
		Language: "csharp_omnisharp", Command: "OmniSharp", Args: []string{"-lsp", "--stdio"},
		NeedsWorkspace: true, FileExts: []string{".cs"},
		Install: &InstallInfo{Type: "binary", Package: "OmniSharp"},
	},
	"kotlin": {
		Language: "kotlin", Command: "kotlin-language-server", Args: []string{"--stdio"},
		NeedsWorkspace: true, FileExts: []string{".kt", ".kts"},
		Install: &InstallInfo{Type: "binary", Package: "kotlin-language-server"},
	},
	"scala": {
		Language: "scala", Command: "metals", Args: []string{},
		NeedsWorkspace: true, FileExts: []string{".scala", ".sc", ".sbt"},
	},
	"powershell": {
		Language: "powershell", Command: "pwsh", Args: []string{"-NoLogo", "-NoProfile", "-Command", "Import-Module PowerShellEditorServices; Start-EditorServices -Stdio"},
		NeedsWorkspace: false, FileExts: []string{".ps1", ".psm1", ".psd1"},
		Install: &InstallInfo{Type: "binary", Package: "PowerShellEditorServices"},
	},
	"fsharp": {
		Language: "fsharp", Command: "fsautocomplete", Args: []string{"--adaptive-lsp-server-enabled"},
		NeedsWorkspace: true, FileExts: []string{".fs", ".fsi", ".fsx"},
		Install: &InstallInfo{Type: "dotnet", Package: "fsautocomplete"},
	},
	"elixir": {
		Language: "elixir", Command: "elixir-ls", Args: []string{},
		NeedsWorkspace: true, FileExts: []string{".ex", ".exs"},
		Install: &InstallInfo{Type: "binary", Package: "elixir-ls"},
	},
	"haskell": {
		Language: "haskell", Command: "haskell-language-server-wrapper", Args: []string{"--lsp"},
		NeedsWorkspace: true, FileExts: []string{".hs", ".lhs"},
	},
	"lua": {
		Language: "lua", Command: "lua-language-server", Args: []string{},
		NeedsWorkspace: true, FileExts: []string{".lua"},
		Install: &InstallInfo{Type: "binary", Package: "lua-language-server"},
	},
	"julia": {
		Language: "julia", Command: "julia", Args: []string{"--startup-file=no", "--history-file=no", "-e", "using LanguageServer; runserver()"},
		NeedsWorkspace: true, FileExts: []string{".jl"},
	},
	"clojure": {
		Language: "clojure", Command: "clojure-lsp", Args: []string{},
		NeedsWorkspace: true, FileExts: []string{".clj", ".cljs", ".cljc", ".edn"},
		Install: &InstallInfo{Type: "binary", Package: "clojure-lsp"},
	},
	"terraform": {
		Language: "terraform", Command: "terraform-ls", Args: []string{"serve"},
		NeedsWorkspace: true, FileExts: []string{".tf", ".tfvars"},
		Install: &InstallInfo{Type: "binary", Package: "terraform-ls"},
	},
	"groovy": {
		Language: "groovy", Command: "groovy-language-server", Args: []string{},
		NeedsWorkspace: true, FileExts: []string{".groovy", ".gradle"},
		Install: &InstallInfo{Type: "binary", Package: "groovy-language-server"},
	},
	"rego": {
		Language: "rego", Command: "regal", Args: []string{"language-server"},
		NeedsWorkspace: false, FileExts: []string{".rego"},
		Install: &InstallInfo{Type: "binary", Package: "regal"},
	},

	// ---------------------------------------------------------------
	// HIGH quirk languages -- companion servers, complex setup
	// ---------------------------------------------------------------

	"java": {
		Language: "java", Command: "jdtls", Args: nil,
		NeedsWorkspace: true, FileExts: []string{".java"},
		Install: &InstallInfo{Type: "binary", Package: "eclipse.jdt.ls"},
	},
	"vue": {
		Language: "vue", Command: "vue-language-server", Args: []string{"--stdio"},
		NeedsWorkspace: true, FileExts: []string{".vue"},
		Install: &InstallInfo{Type: "npm", Package: "@vue/language-server"},
	},
	"typescript_vts": {
		Language: "typescript_vts", Command: "vue-language-server", Args: []string{"--stdio"},
		NeedsWorkspace: true, FileExts: []string{".ts", ".tsx", ".js", ".jsx"},
		Install: &InstallInfo{Type: "npm", Package: "@vue/language-server"},
	},
}
