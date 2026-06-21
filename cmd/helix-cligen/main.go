// Command helix-cligen generates internal/cli/verbs_gen.go — the committed verb
// catalog populating the Phase 90 verbSpecs spine with one `helix <verb>`
// subcommand per callable tool in the live registry.
//
// It mirrors cmd/docgen: blank-imports the same skill packages so
// skill.ToolProviders() enumerates the identical live surface the daemon
// registers, recovers each tool's *Args struct via a go/packages + go/ast scan
// (scan.go), renders a gofmt-stable file (render.go), and is guarded by a
// --check drift gate.
//
// Usage:
//
//	go run ./cmd/helix-cligen           # regenerate internal/cli/verbs_gen.go
//	go run ./cmd/helix-cligen --check   # exit 1 if verbs_gen.go would change (CI)
//	go run ./cmd/helix-cligen --out X   # write to a different path
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"

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

	"github.com/agenthands/helix/internal/skill"
)

func main() {
	out := flag.String("out", "internal/cli/verbs_gen.go", "path to the generated verbs file")
	check := flag.Bool("check", false, "check mode: exit 1 if the generated file would change")
	flag.Parse()

	// 1. Enumerate the live registry by name + category (the catalog domain).
	registryNames := liveRegistryNames()

	// 2. Recover the tool-name -> *Args binding via the AST scan (VERB-04).
	scanned, err := scanToolArgs("./internal/...")
	if err != nil {
		log.Fatalf("scanning tool args: %v", err)
	}

	// 3. Build the catalog: intersect the scan SUPERSET against the live
	//    registry (drop scan-only core tools like activate_project/ping/echo
	//    that are NOT in any ToolProvider). Every live tool gets an entry; tools
	//    with no typed *Args struct (AddSkillTool map[string]any) carry empty
	//    fields by design.
	infos := buildCatalog(registryNames, scanned)

	// 4. Render the gofmt-stable verbs_gen.go (render.go, Task 2).
	rendered, err := renderVerbsGen(infos, registryNames)
	if err != nil {
		log.Fatalf("rendering verbs_gen.go: %v", err)
	}

	if *check {
		existing, err := os.ReadFile(*out)
		if err != nil {
			log.Fatalf("reading %s: %v", *out, err)
		}
		if rendered != string(existing) {
			fmt.Fprintf(os.Stderr, "verbs_gen.go is out of date. Run 'go run ./cmd/helix-cligen' to regenerate.\n")
			os.Exit(1)
		}
		fmt.Println("verbs_gen.go is up to date.")
		return
	}

	if err := os.WriteFile(*out, []byte(rendered), 0644); err != nil {
		log.Fatalf("writing %s: %v", *out, err)
	}
	fmt.Printf("Updated %s\n", *out)
}

// liveRegistryNames returns the live tool-name -> ToolProvider category map,
// enumerated exactly like cmd/docgen (skill.ToolProviders() -> tp.Tools()).
// Duplicate names (e.g. analyze_blast_radius listed by two providers) collapse
// to a single catalog entry; the first category wins for grouping stability.
func liveRegistryNames() map[string]string {
	names := make(map[string]string)
	for _, tp := range skill.ToolProviders() {
		cat := tp.Name()
		for _, t := range tp.Tools() {
			if _, seen := names[t.Name]; !seen {
				names[t.Name] = cat
			}
		}
	}
	return names
}

// buildCatalog intersects the AST-scan SUPERSET against the live registry name
// set: every live tool yields exactly one *argInfo (with recovered fields when a
// typed *Args struct exists, else empty fields). Scan-only core tools absent
// from the registry are dropped. Output is sorted by tool name for determinism.
func buildCatalog(registryNames map[string]string, scanned map[string]*argInfo) []*argInfo {
	out := make([]*argInfo, 0, len(registryNames))
	for name := range registryNames {
		if info, ok := scanned[name]; ok {
			out = append(out, info)
			continue
		}
		out = append(out, &argInfo{toolName: name}) // map[string]any / catalog-only tool
	}
	sort.Slice(out, func(i, j int) bool { return out[i].toolName < out[j].toolName })
	return out
}
