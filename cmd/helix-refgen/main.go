// Command helix-refgen generates internal/cli/skills/helix/reference.md — the
// committed per-verb reference shipped alongside the terse SKILL.md.
//
// It mirrors cmd/docgen and cmd/helix-cligen: blank-imports the same skill
// packages so skill.ToolProviders() enumerates the identical live surface the
// daemon registers, renders a per-verb reference (synopsis from each tool's
// Description, args from cli.VerbSpecsForDocs(), worked example + use-this-not-that
// per groupID), and is guarded by a --check drift gate.
//
// Usage:
//
//	go run ./cmd/helix-refgen           # regenerate internal/cli/skills/helix/reference.md
//	go run ./cmd/helix-refgen --check   # exit 1 if reference.md would change (CI)
//	go run ./cmd/helix-refgen --out X   # write to a different path
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	// Blank imports trigger skill.Register() via init() (same as daemon/imports.go).
	//
	// Blank-import parity rule (reciprocal note in internal/daemon/imports.go):
	// every tool-bearing init()-registered provider must appear in BOTH this file
	// and the daemon so the generated reference matches the runtime tool SET.
	// Literal import-list equality is NOT required — health/help are non-blank in
	// the daemon by design (their explicit RegisterTools also fires init()), and
	// guardrails contributes zero verb rows. The `helix-refgen --check` CI gate
	// (REF-03) is the real anti-drift protection. Never blank-import
	// internal/semantic/extract/* here per D-02 (a second GrammarRegistry would
	// break the singleton).
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

	"github.com/agenthands/helix/internal/skill"
)

func main() {
	out := flag.String("out", "internal/cli/skills/helix/reference.md", "path to reference.md")
	check := flag.Bool("check", false, "check mode: exit 1 if reference.md would change")
	flag.Parse()

	// Force the registry walk to occur (skill.ToolProviders() is read inside
	// renderReference); the blank imports above have already fired init().
	_ = skill.ToolProviders

	rendered := renderReference()

	if *check {
		stale, err := referenceStale(*out, rendered)
		if err != nil {
			log.Fatalf("reading %s: %v", *out, err)
		}
		if stale {
			fmt.Fprintf(os.Stderr, "reference.md is out of date. Run 'go run ./cmd/helix-refgen' to regenerate.\n")
			os.Exit(1)
		}
		fmt.Println("reference.md is up to date.")
		return
	}

	if err := os.WriteFile(*out, []byte(rendered), 0644); err != nil {
		log.Fatalf("writing %s: %v", *out, err)
	}
	fmt.Printf("Updated %s\n", *out)
}

// referenceStale reports whether the on-disk reference at path differs from the
// freshly-rendered content. A missing file in --check mode is a hard error (the
// caller must regenerate first); any other read error is also surfaced.
func referenceStale(path, rendered string) (bool, error) {
	existing, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return rendered != string(existing), nil
}
