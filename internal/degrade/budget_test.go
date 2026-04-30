package degrade

import (
	"testing"
	"time"

	"github.com/agenthands/helix/internal/config"
)

// allToolNames is the exhaustive list of 38 registered tool names.
var allToolNames = []string{
	// Symbol retrieval (9)
	"go_to_definition",
	"find_references",
	"get_hover_info",
	"find_implementations",
	"get_call_hierarchy",
	"get_type_hierarchy",
	"analyze_blast_radius",
	"search_symbols",
	"get_symbol_overview",
	// Symbol editing (6)
	"replace_symbol_body",
	"insert_before_symbol",
	"insert_after_symbol",
	"rename_symbol",
	"safe_delete_symbol",
	"verify_edit",
	// File ops (8)
	"read_file",
	"write_file",
	"create_file",
	"delete_file",
	"list_directory",
	"find_files",
	"search_in_files",
	"replace_in_file",
	// Diagnostics (3)
	"get_diagnostics",
	"get_code_actions",
	"format_code",
	// Memory (7)
	"write_memory",
	"read_memory",
	"list_memories",
	"search_memories",
	"edit_memory",
	"delete_memory",
	"rename_memory",
	// Workflow (2)
	"onboard_project",
	"prepare_for_new_conversation",
	// Misc (3)
	"ping",
	"body",
	"special",
}

func TestToolClassMap(t *testing.T) {
	for _, name := range allToolNames {
		tc := ToolClassFor(name)
		if tc == "" {
			t.Errorf("tool %q returned empty ToolClass", name)
		}
	}
}

func TestToolClassMap_UnmappedDefault(t *testing.T) {
	tc := ToolClassFor("nonexistent_tool")
	if tc != ClassRead {
		t.Errorf("unmapped tool: got %q, want %q", tc, ClassRead)
	}
}

func TestBudgetFor_Defaults(t *testing.T) {
	cfg := config.DegradationConfig{}
	got := BudgetFor("go_to_definition", cfg)
	if got != 5*time.Second {
		t.Errorf("go_to_definition default: got %v, want 5s", got)
	}
}

func TestBudgetFor_Search(t *testing.T) {
	cfg := config.DegradationConfig{}
	got := BudgetFor("find_references", cfg)
	if got != 15*time.Second {
		t.Errorf("find_references default: got %v, want 15s", got)
	}
}

func TestBudgetFor_Edit(t *testing.T) {
	cfg := config.DegradationConfig{}
	got := BudgetFor("replace_symbol_body", cfg)
	if got != 10*time.Second {
		t.Errorf("replace_symbol_body default: got %v, want 10s", got)
	}
}

func TestBudgetFor_Index(t *testing.T) {
	cfg := config.DegradationConfig{}
	got := BudgetFor("onboard_project", cfg)
	if got != 120*time.Second {
		t.Errorf("onboard_project default: got %v, want 120s", got)
	}
}

func TestBudgetFor_Diagnostics(t *testing.T) {
	cfg := config.DegradationConfig{}
	got := BudgetFor("get_diagnostics", cfg)
	if got != 20*time.Second {
		t.Errorf("get_diagnostics default: got %v, want 20s", got)
	}
}

func TestBudgetFor_ConfigOverride(t *testing.T) {
	cfg := config.DegradationConfig{TimeoutRead: 3}
	got := BudgetFor("go_to_definition", cfg)
	if got != 3*time.Second {
		t.Errorf("config override: got %v, want 3s", got)
	}
}

func TestBudgetFor_NegativeConfigClampedToDefault(t *testing.T) {
	// T-13-01: negative config values must be clamped to defaults
	cfg := config.DegradationConfig{TimeoutRead: -1}
	got := BudgetFor("go_to_definition", cfg)
	if got != 5*time.Second {
		t.Errorf("negative config should clamp to default: got %v, want 5s", got)
	}
}

func TestBudgetFor_NeverReturnsZero(t *testing.T) {
	// T-13-01: BudgetFor must never return 0 duration
	cfg := config.DegradationConfig{}
	for _, name := range allToolNames {
		got := BudgetFor(name, cfg)
		if got <= 0 {
			t.Errorf("tool %q returned non-positive budget %v", name, got)
		}
	}
}
