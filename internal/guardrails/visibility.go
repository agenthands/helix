// Package guardrails provides the Phase 66 agent guardrail policy engine:
// rule predicates (G-001..G-005), receipt validation, enforcement-level
// resolution, and the GuardrailMiddleware that gates destructive MCP tools
// on prior evidence collected through read-side tools.
//
// Phase 66 Plan 02 ships the Visibility type alias and per-language coverage
// table so that Wave-2 rule predicates can import guardrails without also
// importing internal/semantic/integ everywhere.
package guardrails

import (
	"github.com/agenthands/helix/internal/semantic/integ"
)

// Visibility is a type alias for integ.Visibility. Rule predicates in this
// package (G-002, G-003) import guardrails rather than internal/semantic/integ
// to keep the dependency graph shallow.
//
// All six constants and the ParseVisibility constructor are re-exported below
// so callers need only one import.
//
// Per-language Visibility coverage (Phase 66 Plan 02):
//
//	Language      Extractor source            Coverage
//	──────────────────────────────────────────────────────────
//	Go            golang/provider.go          Full (exported/package via uppercase heuristic)
//	TypeScript    typescript/provider.go      Full (exported/private via export/private keyword)
//	JavaScript    typescript/provider.go      Full (exported via export keyword)
//	Python        python/provider.go          Full (exported/private via leading-underscore convention)
//	Java          (future Phase 66 Wave-3)    VisUnknown — D-19 conservative-warn fallback
//	Rust          (future Phase 66 Wave-3)    VisUnknown — D-19 conservative-warn fallback
//	C#            (future Phase 66 Wave-3)    VisUnknown — D-19 conservative-warn fallback
//	Other langs   extractor not implemented   VisUnknown — D-19 conservative-warn fallback
//
// When the extractor returns "" or an unrecognised string, ParseVisibility maps
// it to VisUnknown, and IsPublicLike() returns false (T-66-10 mitigation).
type Visibility = integ.Visibility

// Re-exported constants — use these in guardrails rule predicates.
const (
	VisPublic    = integ.VisPublic
	VisExported  = integ.VisExported
	VisProtected = integ.VisProtected
	VisPackage   = integ.VisPackage
	VisPrivate   = integ.VisPrivate
	VisUnknown   = integ.VisUnknown
)

// ParseVisibility translates a raw extractor visibility string into the typed
// Visibility enum. Delegates to integ.ParseVisibility; see that function's
// documentation for the canonical mapping table.
var ParseVisibility = integ.ParseVisibility
