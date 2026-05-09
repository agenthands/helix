//go:build ignore

// Generator for the 50k-symbol synthetic Go fixture used by Phase 64-01
// (bleve gate benchmark) and reusable by Phase 67 (eval harness).
//
// Invocation: from this directory, `go run gen.go`. Produces ./out/pkg{0..49}/file{0..9}.go
// (500 files total) with ~100 declarations per file (~50 funcs + ~50 types) drawn from a
// fixed 200-word lowercase wordlist. All randomness is seeded from int64(42); re-running
// the generator produces byte-identical output.
//
// Output target: 500 files × ~100 declarations ≈ 50_000 declarations total. Acceptance is:
//   - exactly 500 files
//   - >=25_000 funcs (the rest is types)
//   - byte-identical across runs (sha256 stable)
package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
)

const (
	seed            = int64(42)
	numPkgs         = 50
	filesPerPkg     = 10
	totalFiles      = numPkgs * filesPerPkg // 500
	declsPerFile    = 100
	totalDecls      = totalFiles * declsPerFile // 50_000
	funcsPerFile    = 50
	typesPerFile    = declsPerFile - funcsPerFile // 50
	commentLines    = 3
	wordsPerComment = 3
	wordsPerName    = 3
	outputDir       = "out"
)

// wordlist: 200 lowercase English nouns/verbs. Chosen for byte-determinism — the list
// is committed inline so output does not depend on any external file. Do not reorder.
var wordlist = []string{
	"abandon", "able", "above", "absent", "absorb", "accept", "access", "accident", "account", "accurate",
	"achieve", "acid", "acquire", "across", "action", "active", "actor", "actual", "adapt", "address",
	"adjust", "admit", "adopt", "advance", "advice", "affect", "afford", "agency", "agree", "ahead",
	"albatross", "alert", "alien", "align", "alive", "allow", "almost", "alone", "amount", "ancient",
	"angle", "angry", "animal", "answer", "anxiety", "apply", "approve", "arena", "argue", "armor",
	"arrange", "arrest", "arrive", "artist", "aspect", "assault", "asset", "assume", "atomic", "attack",
	"attend", "auction", "author", "autumn", "avoid", "awake", "aware", "awful", "balance", "barrel",
	"basic", "battle", "beauty", "behave", "belong", "belove", "bench", "better", "beyond", "binary",
	"birch", "bishop", "bitter", "blade", "blame", "bless", "block", "blossom", "border", "bother",
	"bottom", "bounce", "branch", "brave", "breath", "bridge", "brief", "broken", "bronze", "budget",
	"buffalo", "burden", "burst", "butter", "button", "candle", "canvas", "carbon", "career", "carrot",
	"casual", "cattle", "ceiling", "celery", "center", "champion", "change", "channel", "chapter", "charge",
	"charity", "charm", "cheese", "cherry", "chief", "choice", "circle", "civil", "clarify", "classic",
	"client", "climb", "cloud", "cluster", "coach", "coastal", "cobalt", "coffee", "collect", "column",
	"combine", "comfort", "common", "company", "compare", "compete", "concern", "conduct", "confess", "confirm",
	"connect", "consent", "consist", "consume", "contact", "contain", "context", "control", "convert", "convey",
	"copper", "corner", "correct", "cosmic", "cotton", "council", "courage", "cradle", "create", "credit",
	"crimson", "crisis", "criteria", "critic", "crystal", "cuckoo", "current", "cycle", "damage", "dancer",
	"darling", "decide", "declare", "decline", "default", "defend", "define", "degree", "deliver", "demand",
	"dental", "deploy", "deposit", "design", "desire", "destiny", "detail", "detect", "diamond", "dignity",
}

// camelCase combines `n` words from the seeded RNG into a camelCase identifier.
// First word is lower; subsequent are TitleCase. Output is deterministic given the RNG.
func camelCase(r *rand.Rand, n int) string {
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		w := wordlist[r.Intn(len(wordlist))]
		if i == 0 {
			parts[i] = w
		} else {
			parts[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(parts, "")
}

// pascalCase is camelCase with the first word capitalized too. Used for type names so
// `grep -E '^type '` and `grep -E '^func '` are easy to count.
func pascalCase(r *rand.Rand, n int) string {
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		w := wordlist[r.Intn(len(wordlist))]
		parts[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(parts, "")
}

// godocComment emits a 3-line godoc comment with `wordsPerComment` random words per line.
// Caller passes `name` so the first comment line begins with the canonical "Name does ..."
// godoc form.
func godocComment(r *rand.Rand, name string) string {
	var sb strings.Builder
	sb.WriteString("// ")
	sb.WriteString(name)
	sb.WriteString(" performs ")
	for i := 0; i < wordsPerComment; i++ {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(wordlist[r.Intn(len(wordlist))])
	}
	sb.WriteString(".\n")
	for line := 1; line < commentLines; line++ {
		sb.WriteString("//")
		for i := 0; i < wordsPerComment; i++ {
			sb.WriteString(" ")
			sb.WriteString(wordlist[r.Intn(len(wordlist))])
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// emitFile writes one .go file with the configured mix of funcs and types. The file is
// purely declarative: no bodies that import anything, no transitive package deps. This
// keeps the fixture self-contained and trivially parseable by tree-sitter and bleve.
func emitFile(r *rand.Rand, pkgIdx, fileIdx int) (path string, content string, funcCount int) {
	pkgName := fmt.Sprintf("pkg%d", pkgIdx)
	dir := filepath.Join(outputDir, pkgName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		panic(err)
	}
	path = filepath.Join(dir, fmt.Sprintf("file%d.go", fileIdx))

	var sb strings.Builder
	// Build tag keeps fixture .go files out of the Go module compile graph
	// (`go vet ./...`, `go build ./...`). Bench tests read these files as
	// raw bytes via filepath.WalkDir, not via go/build.
	sb.WriteString("//go:build neverbuild\n\n")
	sb.WriteString(fmt.Sprintf("package %s\n\n", pkgName))

	// Emit funcsPerFile exported funcs.
	for i := 0; i < funcsPerFile; i++ {
		name := pascalCase(r, wordsPerName)
		sb.WriteString(godocComment(r, name))
		sb.WriteString(fmt.Sprintf("func %s() string { return %q }\n\n", name, camelCase(r, 2)))
	}

	// Emit typesPerFile exported types.
	for i := 0; i < typesPerFile; i++ {
		name := pascalCase(r, wordsPerName)
		sb.WriteString(godocComment(r, name))
		sb.WriteString(fmt.Sprintf("type %s struct{ %s string }\n\n", name, camelCase(r, 2)))
	}

	content = sb.String()
	return path, content, funcsPerFile
}

func main() {
	// Wipe ./out so re-runs are clean and deterministic.
	if err := os.RemoveAll(outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "cleanup failed: %v\n", err)
		os.Exit(1)
	}

	r := rand.New(rand.NewSource(seed))
	totalFuncs := 0
	totalTypes := 0
	for pkg := 0; pkg < numPkgs; pkg++ {
		for f := 0; f < filesPerPkg; f++ {
			path, content, fc := emitFile(r, pkg, f)
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "write %s: %v\n", path, err)
				os.Exit(1)
			}
			totalFuncs += fc
			totalTypes += typesPerFile
		}
	}

	fmt.Printf("generated %d files under %s/\n", totalFiles, outputDir)
	fmt.Printf("  funcs: %d  types: %d  total: %d\n", totalFuncs, totalTypes, totalFuncs+totalTypes)
}
