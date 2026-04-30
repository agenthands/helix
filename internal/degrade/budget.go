// Package degrade provides tool classification and timeout budget lookup
// for graceful degradation (Phase 13). Every registered MCP tool maps to
// a ToolClass, and each class has a configurable timeout budget.
package degrade

import (
	"time"

	"github.com/agenthands/helix/internal/config"
)

// ToolClass categorizes tools by their expected latency profile.
type ToolClass string

const (
	ClassRead        ToolClass = "read"
	ClassSearch      ToolClass = "search"
	ClassEdit        ToolClass = "edit"
	ClassIndex       ToolClass = "index"
	ClassDiagnostics ToolClass = "diagnostics"
)

// toolClassMap maps every registered tool name to its timeout class.
// Unmapped tools default to ClassRead (tightest budget) as a safety net.
var toolClassMap = map[string]ToolClass{
	// Symbol retrieval (LS-backed reads)
	"go_to_definition":     ClassRead,
	"find_references":      ClassSearch, // can be slow on large repos
	"get_hover_info":       ClassRead,
	"find_implementations": ClassSearch,
	"get_call_hierarchy":   ClassSearch,
	"get_type_hierarchy":   ClassSearch,
	"analyze_blast_radius": ClassSearch,
	"search_symbols":       ClassSearch,
	"get_symbol_overview":  ClassRead,

	// Symbol editing (LS-backed writes)
	"replace_symbol_body":  ClassEdit,
	"insert_before_symbol": ClassEdit,
	"insert_after_symbol":  ClassEdit,
	"rename_symbol":        ClassEdit,
	"safe_delete_symbol":   ClassEdit,
	"verify_edit":          ClassEdit,

	// File ops (no LS, fast)
	"read_file":       ClassRead,
	"write_file":      ClassEdit,
	"create_file":     ClassEdit,
	"delete_file":     ClassEdit,
	"list_directory":  ClassRead,
	"find_files":      ClassSearch,
	"search_in_files": ClassSearch,
	"replace_in_file": ClassEdit,

	// Diagnostics (LS-backed)
	"get_diagnostics":  ClassDiagnostics,
	"get_code_actions": ClassDiagnostics,
	"format_code":      ClassDiagnostics,

	// Memory (in-process, fast)
	"write_memory":    ClassRead,
	"read_memory":     ClassRead,
	"list_memories":   ClassRead,
	"search_memories": ClassSearch,
	"edit_memory":     ClassRead,
	"delete_memory":   ClassRead,
	"rename_memory":   ClassRead,

	// Workflow (in-process)
	"onboard_project":              ClassIndex, // may trigger LS indexing
	"prepare_for_new_conversation": ClassRead,

	// Misc
	"ping":    ClassRead,
	"body":    ClassRead,
	"special": ClassRead,
}

// defaultBudgets maps each ToolClass to its hardcoded default timeout (D-02).
var defaultBudgets = map[ToolClass]time.Duration{
	ClassRead:        5 * time.Second,
	ClassSearch:      15 * time.Second,
	ClassEdit:        10 * time.Second,
	ClassIndex:       120 * time.Second,
	ClassDiagnostics: 20 * time.Second,
}

// ToolClassFor returns the ToolClass for the given tool name.
// Unknown tools default to ClassRead (tightest budget).
func ToolClassFor(toolName string) ToolClass {
	if tc, ok := toolClassMap[toolName]; ok {
		return tc
	}
	return ClassRead
}

// BudgetFor returns the timeout budget for the given tool name using the
// supplied DegradationConfig. If the config has a positive override for the
// tool's class, that value is used; otherwise the hardcoded default applies.
//
// T-13-01: negative/zero config values are clamped to defaults; this function
// never returns a zero or negative duration.
func BudgetFor(toolName string, cfg config.DegradationConfig) time.Duration {
	tc := ToolClassFor(toolName)

	// Check for a positive config override for this class.
	if override := configTimeout(tc, cfg); override > 0 {
		return time.Duration(override) * time.Second
	}

	// Fall back to hardcoded default. Every class has an entry, so this
	// always returns a positive duration.
	return defaultBudgets[tc]
}

// configTimeout returns the config-supplied timeout (in seconds) for the
// given class, or 0 if not set / non-positive (triggering default fallback).
func configTimeout(tc ToolClass, cfg config.DegradationConfig) int {
	switch tc {
	case ClassRead:
		return clampPositive(cfg.TimeoutRead)
	case ClassSearch:
		return clampPositive(cfg.TimeoutSearch)
	case ClassEdit:
		return clampPositive(cfg.TimeoutEdit)
	case ClassIndex:
		return clampPositive(cfg.TimeoutIndex)
	case ClassDiagnostics:
		return clampPositive(cfg.TimeoutDiagnostics)
	default:
		return 0
	}
}

// clampPositive returns v if positive, otherwise 0 (signaling "use default").
func clampPositive(v int) int {
	if v > 0 {
		return v
	}
	return 0
}
