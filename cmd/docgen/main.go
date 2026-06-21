// Command docgen generates tool and language tables in README.md from Go source.
//
// It imports all skill packages to trigger init() registration, then reads
// skill.ToolProviders() and langregistry.Entries() to build markdown tables.
// These tables are inserted between marker comments in README.md.
//
// Usage:
//
//	go run ./cmd/docgen              # regenerate README.md
//	go run ./cmd/docgen --check      # exit 1 if README.md would change (CI mode)
//	go run ./cmd/docgen --readme=X   # use a different README path
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	// Blank imports trigger skill.Register() via init() (same as daemon/imports.go).
	_ "github.com/agenthands/helix/internal/kernel/diag"
	_ "github.com/agenthands/helix/internal/kernel/edit"
	_ "github.com/agenthands/helix/internal/kernel/fileops"
	_ "github.com/agenthands/helix/internal/kernel/health"
	_ "github.com/agenthands/helix/internal/kernel/help"
	_ "github.com/agenthands/helix/internal/kernel/symbols"
	_ "github.com/agenthands/helix/internal/profile"
	_ "github.com/agenthands/helix/internal/skill/memory"
	_ "github.com/agenthands/helix/internal/skill/repomap"
	_ "github.com/agenthands/helix/internal/skill/semantic"
	_ "github.com/agenthands/helix/internal/skill/workflow"

	"github.com/agenthands/helix/internal/langregistry"
	"github.com/agenthands/helix/internal/skill"
)

// specialNames maps language registry keys to human-readable display names.
var specialNames = map[string]string{
	"cpp":      "C/C++",
	"cpp_ccls": "C/C++ (ccls)",
	"csharp":   "C#",
	"fsharp":   "F#",
}

func main() {
	readmePath := flag.String("readme", "README.md", "path to README.md")
	check := flag.Bool("check", false, "check mode: exit 1 if README.md would change")
	flag.Parse()

	content, err := os.ReadFile(*readmePath)
	if err != nil {
		log.Fatalf("reading %s: %v", *readmePath, err)
	}

	toolTable := generateToolTable()
	langTable := generateLanguageTable()

	result := string(content)
	result, err = replaceSection(result, "BEGIN TOOLS", "END TOOLS", toolTable)
	if err != nil {
		log.Fatalf("replacing tool section: %v", err)
	}
	result, err = replaceSection(result, "BEGIN LANGUAGES", "END LANGUAGES", langTable)
	if err != nil {
		log.Fatalf("replacing language section: %v", err)
	}

	if *check {
		if result != string(content) {
			fmt.Fprintf(os.Stderr, "README.md is out of date. Run 'go run ./cmd/docgen' to regenerate.\n")
			os.Exit(1)
		}
		fmt.Println("README.md is up to date.")
		return
	}

	if err := os.WriteFile(*readmePath, []byte(result), 0644); err != nil {
		log.Fatalf("writing %s: %v", *readmePath, err)
	}
	fmt.Printf("Updated %s\n", *readmePath)
}

// replaceSection replaces content between marker comments in a document.
// Markers are HTML comments of the form <!-- BEGIN X --> and <!-- END X -->.
func replaceSection(content, beginMarker, endMarker, newContent string) (string, error) {
	beginTag := "<!-- " + beginMarker + " -->"
	endTag := "<!-- " + endMarker + " -->"
	begin := strings.Index(content, beginTag)
	end := strings.Index(content, endTag)
	if begin == -1 || end == -1 {
		return "", fmt.Errorf("markers not found: %s / %s", beginMarker, endMarker)
	}
	endOfEndTag := end + len(endTag)
	return content[:begin] + beginTag + "\n" + newContent + "\n" + endTag + content[endOfEndTag:], nil
}

// generateToolTable builds a markdown table of all registered MCP tools.
func generateToolTable() string {
	providers := skill.ToolProviders()

	var sb strings.Builder
	sb.WriteString("| Tool | Category | Description |\n")
	sb.WriteString("|------|----------|-------------|\n")

	for _, tp := range providers {
		category := tp.Name()
		for _, tool := range tp.Tools() {
			desc := tool.Description
			// Truncate long descriptions at first sentence.
			if idx := strings.Index(desc, ". "); idx > 0 && idx < 120 {
				desc = desc[:idx+1]
			}
			// Escape pipes in description.
			desc = strings.ReplaceAll(desc, "|", "\\|")
			sb.WriteString(fmt.Sprintf("| `%s` | %s | %s |\n", tool.Name, category, desc))
		}
	}

	return sb.String()
}

// generateLanguageTable builds a markdown table of all registered languages.
func generateLanguageTable() string {
	reg, err := langregistry.NewRegistry()
	if err != nil {
		log.Fatalf("creating language registry: %v", err)
	}

	entries := reg.Entries()

	var sb strings.Builder
	sb.WriteString("| Language | LS Command | File Extensions | Install |\n")
	sb.WriteString("|----------|------------|-----------------|--------|\n")

	for _, e := range entries {
		name := displayName(e.Language)
		exts := strings.Join(e.FileExts, ", ")
		install := e.InstallHint()
		// Escape pipes.
		install = strings.ReplaceAll(install, "|", "\\|")
		sb.WriteString(fmt.Sprintf("| %s | `%s` | %s | %s |\n", name, e.Command, exts, install))
	}

	return sb.String()
}

// displayName converts a language registry key to a human-readable name.
func displayName(key string) string {
	if name, ok := specialNames[key]; ok {
		return name
	}
	parts := strings.SplitN(key, "_", 2)
	base := strings.Title(parts[0]) //nolint:staticcheck // strings.Title is fine for simple ASCII keys
	if len(parts) == 2 {
		variant := strings.Title(parts[1]) //nolint:staticcheck
		return base + " (" + variant + ")"
	}
	return base
}
