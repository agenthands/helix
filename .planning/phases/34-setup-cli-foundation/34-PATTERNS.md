# Phase 34: Setup CLI Foundation - Pattern Map

**Mapped:** 2026-04-21
**Files analyzed:** 7 new files
**Analogs found:** 7 / 7

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/cli/root.go` (modify) | controller | request-response | self | exact |
| `internal/cli/setup.go` | controller | request-response | `internal/cli/root.go` | exact |
| `internal/cli/setup_clients.go` | service | file-I/O + subprocess | `internal/langregistry/installer.go` | role-match |
| `internal/cli/setup_detect.go` | utility | transform | `internal/skill/workflow/skill.go` (gatherProjectInfo) | exact |
| `internal/cli/setup_health.go` | service | request-response | `internal/daemon/daemon.go` | partial |
| `internal/cli/setup_output.go` | utility | transform | (no direct analog -- new pattern) | none |
| `internal/cli/setup_test.go` | test | -- | `internal/langregistry/installer_test.go` + `internal/langregistry/registry_test.go` | exact |

## Pattern Assignments

### `internal/cli/root.go` (modify -- add setup subcommand)

**Analog:** self -- `internal/cli/root.go`

**Pattern: Adding a subcommand** (lines 18-49):
The root command is created by `NewRootCommand()`. The setup subcommand must be added via `rootCmd.AddCommand(...)` before returning. The key constraint: `RunE: runRoot` on line 22 must remain so bare `serena` continues to enter stdio mode (Pitfall 3 from RESEARCH.md -- cobra runs `RunE` when no subcommand matches even when subcommands exist).

```go
// internal/cli/root.go lines 18-27 -- preserve this structure
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "serena",
		Short: "Serena code intelligence MCP server",
		Long:  "Serena 2.0 - LSP-backed MCP runtime for semantic code operations",
		RunE:  runRoot,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	// ... flags ...
	// ADD: rootCmd.AddCommand(newSetupCommand())
	return rootCmd
}
```

**Import pattern** (lines 1-14):
```go
import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/postfix/serena/internal/config"
	"github.com/postfix/serena/internal/daemon"
	"github.com/postfix/serena/internal/forwarder"
	"github.com/postfix/serena/internal/obs"
)
```

---

### `internal/cli/setup.go` (controller, request-response)

**Analog:** `internal/cli/root.go`

**Cobra command creation pattern** (model after lines 18-49 of root.go):
```go
// internal/cli/root.go lines 18-49
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "serena",
		Short: "Serena code intelligence MCP server",
		Long:  "Serena 2.0 - LSP-backed MCP runtime for semantic code operations",
		RunE:  runRoot,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	rootCmd.Flags().String("mode", "auto", "Transport mode: stdio, http, auto")
	rootCmd.Flags().Bool("serve", false, "Run as daemon directly (skip forwarder)")
	// ...
	return rootCmd
}
```

**Flag extraction pattern** (model after lines 52-71 of root.go):
```go
// internal/cli/root.go lines 52-71
func runRoot(cmd *cobra.Command, args []string) error {
	showVersion, _ := cmd.Flags().GetBool("version")
	if showVersion {
		fmt.Println("serena version 2.0.0-dev")
		return nil
	}
	serve, _ := cmd.Flags().GetBool("serve")
	mode, _ := cmd.Flags().GetString("mode")
	// ...
}
```

**Setup command needs these flags (from CONTEXT.md):**
- `--global` (bool) -- D-05
- `--uninstall` (bool) -- D-03
- `--skip-install` (bool) -- D-09
- `--dry-run` (bool) -- D-12
- `--output` (string) -- D-06, only for generic client

**Entry point:** `cmd/serena/main.go` calls `cli.NewRootCommand().Execute()` (line 11). No changes needed there.

---

### `internal/cli/setup_clients.go` (service, file-I/O + subprocess)

**Analog:** `internal/langregistry/installer.go`

**Interface + strategy dispatch pattern** (model after installer.go lines 18-66):
The Installer uses a strategy pattern where `Resolve()` dispatches to tier-specific methods. The ClientRegistrar should follow the same pattern: a common interface with per-client implementations.

```go
// internal/langregistry/installer.go lines 44-66
func (i *Installer) Resolve(ctx context.Context, entry LSEntry) (command string, args []string, err error) {
	// Tier 1: Check PATH.
	if path, lookErr := exec.LookPath(entry.Command); lookErr == nil {
		i.logger.Debug("language server found in PATH", "language", entry.Language, "path", path)
		return path, entry.Args, nil
	}
	// Tier 2: Managed download if enabled and install info is available.
	if i.config.AutoInstall && entry.Install != nil {
		// ...
	}
	// Tier 3: Helpful error.
	return "", nil, fmt.Errorf("language server %q not found for %q; install with: %s",
		entry.Command, entry.Language, entry.InstallHint())
}
```

**Subprocess execution pattern** (model after installer.go lines 97-109):
```go
// internal/langregistry/installer.go lines 97-109
func (i *Installer) installNPM(ctx context.Context, entry LSEntry) (string, error) {
	pkg := entry.Install.Package
	if entry.Install.Version != "" {
		pkg += "@" + entry.Install.Version
	}
	cmd := exec.CommandContext(ctx, "npm", "install", "-g", pkg)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("npm install failed for %s: %w", pkg, err)
	}
	return exec.LookPath(entry.Command)
}
```

**Error wrapping pattern** (consistent throughout installer.go):
```go
return "", fmt.Errorf("npm install failed for %s: %w", pkg, err)
```

**Skill interface pattern for reference** (from `internal/skill/skill.go` lines 22-37):
```go
type Skill interface {
	Name() string
	Description() string
	Init(deps SkillDeps) error
}
type ToolProvider interface {
	Skill
	Tools() []*mcp.ToolDef
}
```
The ClientRegistrar interface should follow this same Go idiom: small interface, method names that describe the action.

---

### `internal/cli/setup_detect.go` (utility, transform)

**Analog:** `internal/skill/workflow/skill.go` lines 141-219

**Language detection by file walking** (exact pattern to adapt):
```go
// internal/skill/workflow/skill.go lines 141-219
func (s *WorkflowSkill) gatherProjectInfo() OnboardingData {
	projectRoot := filepath.Dir(s.projectDir)
	data := OnboardingData{ProjectRoot: projectRoot}

	entries, err := os.ReadDir(projectRoot)
	if err != nil {
		s.logger.Warn("failed to read project root", "error", err)
		return data
	}

	langExtensions := map[string]string{
		".go": "Go", ".py": "Python", ".js": "JavaScript",
		// ...20 extensions...
	}

	detectedLangs := make(map[string]bool)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if entry.IsDir() {
			subEntries, err := os.ReadDir(filepath.Join(projectRoot, name))
			// ...walk one level into subdirs...
		}
	}
}
```

**Key improvement for setup:** Instead of the hardcoded `langExtensions` map, setup should use `langregistry.Registry.ByExtension()` which covers 52 languages:

```go
// internal/langregistry/registry.go lines 110-125
func (r *Registry) ByExtension(ext string) []LSEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ext = strings.ToLower(ext)
	var result []LSEntry
	for _, e := range r.entries {
		for _, fe := range e.FileExts {
			if strings.ToLower(fe) == ext {
				result = append(result, e)
				break
			}
		}
	}
	return result
}
```

The setup_detect.go should combine the file-walking pattern from workflow/skill.go with `Registry.ByExtension()` for matching, replacing the hardcoded map.

---

### `internal/cli/setup_health.go` (service, request-response)

**Analog:** `internal/daemon/daemon.go` (partial match)

**Daemon construction pattern** (lines 1-40 of daemon.go):
```go
import (
	"context"
	"log/slog"
	// ...
	"github.com/postfix/serena/internal/config"
	"github.com/postfix/serena/internal/langregistry"
	// ...
)
```

**Logger pattern** (from root.go lines 79-97):
```go
var handler slog.Handler
if jsonLog {
	handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
} else {
	handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
}
logger := slog.New(handler)
```

**Note:** Health check is the most complex sub-component. It needs to start the daemon briefly, send initialization requests per detected language, and verify LS responses. The daemon construction in `daemon.go` shows the bootstrap sequence. This file may need a lightweight probe mode rather than full daemon start -- planner should decide approach.

---

### `internal/cli/setup_output.go` (utility, transform)

**No direct analog in codebase.** This is the first colored terminal output in the project.

**slog pattern to follow** (from root.go line 92):
```go
logger := slog.New(handler)
```

**New dependency:** `fatih/color` (not yet in go.mod). The research recommends it for cross-platform NO_COLOR support.

**Pattern from RESEARCH.md** -- compact checkmark/cross output:
```
checkmark claude-code registered
checkmark gopls installed
cross pyright failed: ...
```

---

### `internal/cli/setup_test.go` (test)

**Analog:** `internal/langregistry/installer_test.go` + `internal/langregistry/registry_test.go`

**Test imports pattern** (installer_test.go lines 1-11):
```go
package langregistry

import (
	"context"
	"log/slog"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

**Table-driven test pattern** (installer_test.go lines 55-76):
```go
func TestInstallerInstallHintVariants(t *testing.T) {
	tests := []struct {
		name     string
		install  *InstallInfo
		command  string
		contains string
	}{
		{"npm", &InstallInfo{Type: "npm", Package: "foo", Version: "1.0"}, "foo", "npm install -g foo@1.0"},
		{"pip", &InstallInfo{Type: "pip", Package: "bar"}, "bar", "pip install bar"},
		// ...
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := LSEntry{Command: tt.command, Install: tt.install}
			assert.Contains(t, e.InstallHint(), tt.contains)
		})
	}
}
```

**Temp directory pattern** (registry_test.go lines 79-103):
```go
func TestRegistryYAMLOverrideDeepMerge(t *testing.T) {
	tmpDir := t.TempDir()
	overrideFile := filepath.Join(tmpDir, "languages.yaml")
	content := `go:
  command: "gopls-custom"
`
	require.NoError(t, os.WriteFile(overrideFile, []byte(content), 0644))
	// ... test logic ...
}
```

**Test naming convention:** `Test<Type><Behavior>` (e.g., `TestInstallerResolvePATH`, `TestRegistryByExtension`). Setup tests should follow: `TestSetupClaudeCodeRegister`, `TestSetupVSCodeMerge`, `TestSetupDetectLanguages`, etc.

**Assertion conventions:** `require` for fatal preconditions, `assert` for test checks. Both from `stretchr/testify`.

---

## Shared Patterns

### slog Logging
**Source:** `internal/cli/root.go` lines 79-92
**Apply to:** All setup files that need logging
```go
var handler slog.Handler
if jsonLog {
	handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
} else {
	handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
}
logger := slog.New(handler)
```

### Error Wrapping
**Source:** `internal/langregistry/installer.go` (throughout)
**Apply to:** All setup files
```go
return fmt.Errorf("descriptive context for %s: %w", arg, err)
```

### Cobra Flag Extraction
**Source:** `internal/cli/root.go` lines 52-71
**Apply to:** `setup.go`
```go
flagValue, _ := cmd.Flags().GetBool("flag-name")
stringValue, _ := cmd.Flags().GetString("string-flag")
```

### Package-Internal Test Convention
**Source:** `internal/langregistry/installer_test.go` line 1, `internal/langregistry/registry_test.go` line 1
**Apply to:** `setup_test.go`
Tests use the same package name (not `_test` suffix), allowing access to unexported functions. Both test files use `package langregistry`, so setup tests should use `package cli`.

### os/exec Subprocess Pattern
**Source:** `internal/langregistry/installer.go` lines 97-109
**Apply to:** `setup_clients.go` for claude-code and gemini-cli registrars
```go
cmd := exec.CommandContext(ctx, "binary", "args"...)
cmd.Stdout = io.Discard  // or os.Stderr for user-visible output
cmd.Stderr = io.Discard
if err := cmd.Run(); err != nil {
	return fmt.Errorf("action failed for %s: %w", target, err)
}
```

### JSON File Read-Merge-Write
**Source:** No existing analog -- new pattern from RESEARCH.md
**Apply to:** `setup_clients.go` for vscode, jetbrains, claude-desktop registrars
```go
func mergeJSONConfig(path, key, serverName string, serverConfig map[string]any) error {
	existing := make(map[string]any)
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &existing)
	}
	servers, ok := existing[key].(map[string]any)
	if !ok {
		servers = make(map[string]any)
	}
	servers[serverName] = serverConfig
	existing[key] = servers
	data, _ := json.MarshalIndent(existing, "", "  ")
	return os.WriteFile(path, data, 0644)
}
```

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/cli/setup_output.go` | utility | transform | No colored terminal output exists in the codebase yet. This is the first file using `fatih/color`. Use RESEARCH.md patterns for checkmark/cross formatting. |

## Metadata

**Analog search scope:** `internal/cli/`, `internal/langregistry/`, `internal/skill/`, `internal/daemon/`, `cmd/serena/`
**Files scanned:** 20+
**Pattern extraction date:** 2026-04-21
