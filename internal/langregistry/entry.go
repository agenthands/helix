// Package langregistry provides an embedded language server registry with 40+ entries
// and YAML override support. It is the data layer consumed by the LS pool.
package langregistry

import "fmt"

// LSEntry describes a language server binary, its arguments, file associations,
// and optional managed-install metadata.
type LSEntry struct {
	// Language is the canonical language key (e.g. "go", "python").
	Language string
	// Command is the LS binary name (e.g. "gopls", "pyright-langserver").
	Command string
	// Args are the command-line arguments for the LS binary.
	Args []string
	// InitOptions are passed as initializationOptions in the LSP initialize request.
	InitOptions map[string]any
	// NeedsWorkspace indicates whether the LS requires workspace folders.
	NeedsWorkspace bool
	// FileExts lists file extensions associated with this language (e.g. [".go"]).
	FileExts []string
	// Install holds managed-install metadata. nil means system-only.
	Install *InstallInfo
	// IgnoredDirs lists directories to skip for this language.
	IgnoredDirs []string
}

// InstallInfo describes how to install a language server when it is not found in PATH.
type InstallInfo struct {
	// Type is the installer type: "npm", "pip", "binary", "cargo", "gem", "dotnet", "system".
	Type string
	// Package is the package name (e.g. "pyright", "bash-language-server").
	Package string
	// Version is the pinned version string.
	Version string
	// URLs maps platform keys (e.g. "darwin-arm64") to download URLs for binary installs.
	URLs map[string]string
	// SHA256 maps platform keys to expected checksums for binary installs.
	SHA256 map[string]string
}

// InstallHint returns a human-readable install instruction for this entry.
func (e LSEntry) InstallHint() string {
	if e.Install == nil {
		return fmt.Sprintf("Install %q manually and ensure it is in your PATH.", e.Command)
	}
	switch e.Install.Type {
	case "npm":
		if e.Install.Version != "" {
			return fmt.Sprintf("npm install -g %s@%s", e.Install.Package, e.Install.Version)
		}
		return fmt.Sprintf("npm install -g %s", e.Install.Package)
	case "pip":
		if e.Install.Version != "" {
			return fmt.Sprintf("pip install %s==%s", e.Install.Package, e.Install.Version)
		}
		return fmt.Sprintf("pip install %s", e.Install.Package)
	case "cargo":
		if e.Install.Version != "" {
			return fmt.Sprintf("cargo install %s --version %s", e.Install.Package, e.Install.Version)
		}
		return fmt.Sprintf("cargo install %s", e.Install.Package)
	case "gem":
		return fmt.Sprintf("gem install %s", e.Install.Package)
	case "dotnet":
		return fmt.Sprintf("dotnet tool install -g %s", e.Install.Package)
	case "binary":
		return fmt.Sprintf("Download %s from the project's release page and add it to your PATH.", e.Install.Package)
	default:
		return fmt.Sprintf("Install %q (%s) and ensure it is in your PATH.", e.Command, e.Install.Type)
	}
}
