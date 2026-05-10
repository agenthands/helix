package rules

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/agenthands/helix/internal/guardrails"
	"github.com/agenthands/helix/internal/guardrails/catalogs"
)

// IsSecuritySensitive is the pure-Go 3-signal classifier for G-005 (D-17).
//
// Returns (matched bool, signals []string) where signals contains any of:
// "path_glob", "identifier", "import_pattern" — the set of triggered detectors.
//
// Signal detection order:
//  1. Path glob match against cat.PathGlobs.
//  2. Identifier match against args.SymbolName / args.Find using cat.IdentifierPatterns.
//  3. Import-pattern match against args.FileImports (semantic index, when available)
//     OR args.FileImportsFromGrep (degraded-mode grep fallback).
func IsSecuritySensitive(args RuleArgs, cat catalogs.Catalog) (bool, []string) {
	var signals []string

	// Signal 1: path glob.
	if catalogs.MatchesPathGlob(args.Path, cat.PathGlobs) {
		signals = append(signals, "path_glob")
	}

	// Signal 2: identifier pattern (SymbolName and Find).
	identToCheck := args.SymbolName
	if identToCheck == "" {
		identToCheck = args.Find
	}
	if identToCheck != "" && catalogs.MatchesIdentifier(identToCheck, cat.IdentifierPatterns) {
		signals = append(signals, "identifier")
	}

	// Signal 3: import pattern.
	imports := args.FileImports
	if len(imports) == 0 {
		imports = args.FileImportsFromGrep
	}
	for _, imp := range imports {
		if catalogs.MatchesImportPattern(imp, cat.ImportPatterns) {
			signals = append(signals, "import_pattern")
			break
		}
	}

	return len(signals) > 0, signals
}

// EvaluateG005 evaluates the security-sensitive rule (G-005, D-17 LOCKED).
//
// It is a 3-signal pre-edit classifier. When the target is security-sensitive
// and no covering receipt (context_gathered or structural_overview) exists,
// returns Warn/Block (per enforcement level).
//
// When a covering receipt IS present, returns Allow with:
//   - Decision.SuggestedTools = ["verify_edit"] (post-edit obligation; D-18)
//   - Decision.Warnings = ["security-sensitive edit; verify_edit required to issue diagnostics_clean receipt"]
//
// The catalog is resolved from sc.Catalogs using the file extension language key.
// When sc.Catalogs is empty or the language is unknown, all 3 signals use empty patterns.
func EvaluateG005(ctx context.Context, args RuleArgs, sc SessionContext) Decision {
	// Resolve catalog by language (best-effort from path extension).
	cat := resolveG005Catalog(args.Path, sc.Catalogs)

	matched, signals := IsSecuritySensitive(args, cat)
	if !matched {
		return Decision{Action: Allow}
	}

	// Security-sensitive: check for covering receipt.
	target := guardrails.Target{
		Path:         args.Path,
		TouchedFiles: []string{args.Path},
		RequiredClass: guardrails.ClassContextGathered,
	}
	coveringClasses := []guardrails.ReceiptClass{
		guardrails.ClassContextGathered,
		guardrails.ClassStructuralOverview,
	}
	if findReceiptCovering(sc, args, coveringClasses, target) {
		// Pre-edit check passed. Return Allow + post-edit obligation.
		return Decision{
			Action:         Allow,
			Rule:           "G-005",
			SuggestedTools: []string{"verify_edit"},
			Warnings:       []string{"security-sensitive edit; verify_edit required to issue diagnostics_clean receipt"},
		}
	}

	// No covering receipt: emit Block/Warn.
	level := resolveLevel(args, sc, "G-005")
	action := ResolveAction(level)
	if action == Allow {
		return Decision{Action: Allow}
	}

	return Decision{
		Action:  action,
		Rule:    "G-005",
		Message: "security-sensitive edit detected (signals: " + joinSignals(signals) + "); gather context before editing security-critical code",
		RequiredReceipts: []RequiredReceipt{
			{Class: guardrails.ClassContextGathered, ScopeHint: "get_context on " + args.Path},
			{Class: guardrails.ClassStructuralOverview, ScopeHint: "get_repo_map for structural overview"},
		},
		SuggestedTools: []string{"get_context", "get_repo_map"},
		SeeAlso: []SeeAlsoRef{
			{Tool: "get_tool_help", Args: map[string]string{"topic": "workflow:security-sensitive-edit"}},
		},
	}
}

// resolveG005Catalog picks the best matching Catalog from sc.Catalogs by file extension.
func resolveG005Catalog(filePath string, cats map[string]catalogs.Catalog) catalogs.Catalog {
	if len(cats) == 0 {
		return catalogs.Catalog{}
	}
	lang := langFromPath(filePath)
	if cat, ok := cats[lang]; ok {
		return cat
	}
	return catalogs.Catalog{}
}

// langFromPath returns a language key from the file extension. Returns ""
// for any extension not in the four catalogs that ship with v1.10
// (go, typescript, javascript, python). The empty result is intentional and
// fail-closed: resolveG005Catalog returns an empty Catalog{} for unknown
// languages, so IsSecuritySensitive cannot fire any signal. Operators with
// repos in Java/Rust/C#/Kotlin/Ruby/PHP/Swift/etc. should treat G-005 as a
// no-op until that language's catalog ships in a later phase. See
// GUARDRAILS.md > "G-005 catalog coverage" for the up-to-date status.
//
// IN-03: uses filepath.Ext rather than a hand-rolled scan.
func langFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".mjs", ".cjs":
		return "javascript"
	case ".py":
		return "python"
	}
	return ""
}

// joinSignals concatenates G-005 trigger signals into a comma-separated list
// for the violation message. WR-05: was a hand-rolled +=; replaced with
// strings.Join.
func joinSignals(signals []string) string {
	return strings.Join(signals, ", ")
}
