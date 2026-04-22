package profile

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	serr "github.com/postfix/serena/internal/errors"
	"github.com/postfix/serena/internal/mcp"
	"github.com/postfix/serena/internal/skill"
)

// SessionProvider gives the profile skill access to the current session state.
// The MCP server wires this up during startup.
type SessionProvider interface {
	CurrentSession() *mcp.SessionInfo
}

// profileSkill exposes switch_mode and get_token_budget as MCP tools (D-04).
type profileSkill struct {
	store   *ProfileStore
	logger  *slog.Logger
	session SessionProvider
}

// SetSessionProvider injects the session provider after skill init.
// Called by the daemon/server wiring layer.
func (s *profileSkill) SetSessionProvider(sp SessionProvider) {
	s.session = sp
}

func init() {
	skill.Register(&profileSkill{})
}

func (s *profileSkill) Name() string        { return "profile" }
func (s *profileSkill) Description() string  { return "Agent profiles and mode switching (D-04)" }

// Init loads embedded profiles and applies overrides from global/project dirs.
func (s *profileSkill) Init(deps skill.SkillDeps) error {
	s.logger = deps.Logger
	store, err := LoadEmbedded()
	if err != nil {
		return fmt.Errorf("loading embedded profiles: %w", err)
	}

	// Apply overrides from global and project directories if present.
	globalProfiles := filepath.Join(deps.GlobalDir, "profiles")
	globalModes := filepath.Join(deps.GlobalDir, "modes")
	if err := LoadOverrides(store, globalProfiles, globalModes); err != nil {
		return fmt.Errorf("loading global overrides: %w", err)
	}

	projectProfiles := filepath.Join(deps.ProjectDir, "profiles")
	projectModes := filepath.Join(deps.ProjectDir, "modes")
	if err := LoadOverrides(store, projectProfiles, projectModes); err != nil {
		return fmt.Errorf("loading project overrides: %w", err)
	}

	s.store = store
	return nil
}

// Tools returns switch_mode and get_token_budget tool definitions.
func (s *profileSkill) Tools() []*mcp.ToolDef {
	return []*mcp.ToolDef{
		{
			Name:             "switch_mode",
			Description:      "Switch the current session's operational mode (read/edit/review/admin)",
			BriefDescription: "Switch operational mode (read/edit/review/admin)",
		},
		{
			Name:             "get_token_budget",
			Description:      "Get token budget breakdown for the current or specified profile/mode",
			BriefDescription: "Get token budget for current profile and mode",
		},
	}
}

// validateModeTransition checks whether switching from currentMode to targetMode
// is allowed by the given profile's AllowedModeTransitions.
func validateModeTransition(store *ProfileStore, profileName, currentMode, targetMode string) error {
	// Check target mode exists.
	if _, ok := store.Mode(targetMode); !ok {
		return serr.New(serr.InvalidArgs, "unknown mode").
			WithDetail(fmt.Sprintf("%s; available modes: %s", targetMode, strings.Join(store.ModeNames(), ", ")))
	}

	// Look up profile.
	prof, ok := store.Profile(profileName)
	if !ok {
		return serr.New(serr.InvalidArgs, "unknown profile").WithDetail(profileName)
	}

	// Check allowed transitions.
	allowed, exists := prof.AllowedModeTransitions[currentMode]
	if !exists {
		return serr.New(serr.InvalidArgs, "mode has no transitions defined").
			WithDetail(fmt.Sprintf("mode %s in profile %s", currentMode, profileName))
	}

	for _, a := range allowed {
		if a == targetMode {
			return nil
		}
	}

	return serr.New(serr.InvalidArgs, "mode transition not allowed").
		WithDetail(fmt.Sprintf("from %s to %s in profile %s; allowed: %s",
			currentMode, targetMode, profileName, strings.Join(allowed, ", ")))
}

// SwitchModeResult is the response payload for the switch_mode tool.
type SwitchModeResult struct {
	PreviousMode   string `json:"previous_mode"`
	CurrentMode    string `json:"current_mode"`
	ToolsAvailable int    `json:"tools_available"`
}

// ExecuteSwitchMode performs mode switching with validation.
// Exported for direct testing; the MCP tool handler delegates here.
func (s *profileSkill) ExecuteSwitchMode(targetMode string) (*SwitchModeResult, error) {
	if s.session == nil {
		return nil, serr.New(serr.Internal, "no session provider configured")
	}
	sess := s.session.CurrentSession()
	if sess == nil {
		return nil, serr.New(serr.Internal, "no active session")
	}

	snapshot := sess.Snapshot()
	currentMode := snapshot.Mode
	profileName := snapshot.Profile

	if err := validateModeTransition(s.store, profileName, currentMode, targetMode); err != nil {
		return nil, err
	}

	// Resolve the new tool set from mode + profile merged specs.
	mode, _ := s.store.Mode(targetMode)
	prof, _ := s.store.Profile(profileName)

	// Merge mode skills with profile skills.
	skillSet := make(map[string]bool)
	for _, sk := range prof.Skills {
		skillSet[sk] = true
	}
	for _, sk := range mode.Skills {
		skillSet[sk] = true
	}
	skillNames := make([]string, 0, len(skillSet))
	for sk := range skillSet {
		skillNames = append(skillNames, sk)
	}

	// Merge include tools.
	includeTools := make([]string, 0)
	includeTools = append(includeTools, prof.Tools...)
	includeTools = append(includeTools, mode.Tools...)

	// Merge exclude tools.
	excludeTools := make([]string, 0)
	excludeTools = append(excludeTools, prof.ExcludeTools...)
	excludeTools = append(excludeTools, mode.ExcludeTools...)

	resolved := skill.ResolveTools(skillNames, includeTools, excludeTools)

	// Update session state.
	toolNames := make([]string, len(resolved))
	for i, t := range resolved {
		toolNames[i] = t.Name
	}
	// Order matters: set the new tool whitelist before advertising the mode
	// transition so any concurrent reader that sees Mode=targetMode also sees
	// the matching AllowedTools (threat T-08-08 mitigation).
	sess.SetAllowedTools(toolNames)
	sess.RecordModeTransition(currentMode, targetMode)

	return &SwitchModeResult{
		PreviousMode:   currentMode,
		CurrentMode:    targetMode,
		ToolsAvailable: len(resolved),
	}, nil
}

// TokenBudgetResult is the response payload for the get_token_budget tool.
type TokenBudgetResult struct {
	TotalTokens int              `json:"total_tokens"`
	ToolCount   int              `json:"tool_count"`
	PerTool     []ToolTokenInfo  `json:"per_tool,omitempty"`
}

// ToolTokenInfo holds per-tool token estimates.
type ToolTokenInfo struct {
	Name              string `json:"name"`
	SchemaTokens      int    `json:"schema_tokens"`
	DescriptionTokens int    `json:"description_tokens"`
}

// computeTokenBudget calculates token estimates for the tools in a given profile/mode.
func computeTokenBudget(store *ProfileStore, profileName, modeName string, detailed bool) (*TokenBudgetResult, error) {
	prof, ok := store.Profile(profileName)
	if !ok {
		return nil, serr.New(serr.InvalidArgs, "unknown profile").WithDetail(profileName)
	}

	mode, ok := store.Mode(modeName)
	if !ok {
		return nil, serr.New(serr.InvalidArgs, "unknown mode").WithDetail(modeName)
	}

	// Merge mode skills with profile skills.
	skillSet := make(map[string]bool)
	for _, sk := range prof.Skills {
		skillSet[sk] = true
	}
	for _, sk := range mode.Skills {
		skillSet[sk] = true
	}
	skillNames := make([]string, 0, len(skillSet))
	for sk := range skillSet {
		skillNames = append(skillNames, sk)
	}

	includeTools := make([]string, 0)
	includeTools = append(includeTools, prof.Tools...)
	includeTools = append(includeTools, mode.Tools...)

	excludeTools := make([]string, 0)
	excludeTools = append(excludeTools, prof.ExcludeTools...)
	excludeTools = append(excludeTools, mode.ExcludeTools...)

	resolved := skill.ResolveTools(skillNames, includeTools, excludeTools)

	result := &TokenBudgetResult{
		ToolCount: len(resolved),
	}

	totalTokens := 0
	for _, t := range resolved {
		descTokens := len(t.Description) / 4
		// Estimate schema tokens from tool name + description as a baseline.
		// In production this would marshal the actual JSON schema.
		schemaTokens := estimateSchemaTokens(t)
		totalTokens += descTokens + schemaTokens

		if detailed {
			result.PerTool = append(result.PerTool, ToolTokenInfo{
				Name:              t.Name,
				SchemaTokens:      schemaTokens,
				DescriptionTokens: descTokens,
			})
		}
	}
	result.TotalTokens = totalTokens

	return result, nil
}

// estimateSchemaTokens estimates the token cost of a tool's schema.
// Uses a rough heuristic: marshal tool definition to JSON and divide by 4.
func estimateSchemaTokens(t *mcp.ToolDef) int {
	// Marshal the tool definition for a rough size estimate.
	data, err := json.Marshal(struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}{
		Name:        t.Name,
		Description: t.Description,
	})
	if err != nil {
		return 10 // fallback minimum
	}
	tokens := len(data) / 4
	if tokens < 1 {
		tokens = 1
	}
	return tokens
}

// ExecuteGetTokenBudget computes the token budget for a profile/mode combination.
func (s *profileSkill) ExecuteGetTokenBudget(profileName, modeName, format string) (*TokenBudgetResult, error) {
	// Use defaults from session if not specified.
	if profileName == "" || modeName == "" {
		if s.session != nil {
			sess := s.session.CurrentSession()
			if sess != nil {
				snap := sess.Snapshot()
				if profileName == "" {
					profileName = snap.Profile
				}
				if modeName == "" {
					modeName = snap.Mode
				}
			}
		}
	}

	if profileName == "" {
		profileName = "full"
	}
	if modeName == "" {
		modeName = "edit"
	}

	detailed := format == "detailed"
	return computeTokenBudget(s.store, profileName, modeName, detailed)
}

// ExecuteTool dispatches MCP tool calls to the appropriate handler.
func (s *profileSkill) ExecuteTool(name string, params map[string]interface{}) (string, error) {
	switch name {
	case "switch_mode":
		targetMode, _ := params["target_mode"].(string)
		if targetMode == "" {
			return "", serr.New(serr.InvalidArgs, "missing required parameter: target_mode").WithTool("switch_mode")
		}
		result, err := s.ExecuteSwitchMode(targetMode)
		if err != nil {
			return "", err
		}
		data, _ := json.Marshal(result)
		return string(data), nil

	case "get_token_budget":
		profileName, _ := params["profile"].(string)
		modeName, _ := params["mode"].(string)
		format, _ := params["format"].(string)
		result, err := s.ExecuteGetTokenBudget(profileName, modeName, format)
		if err != nil {
			return "", err
		}
		data, _ := json.Marshal(result)
		return string(data), nil

	default:
		return "", serr.New(serr.InvalidArgs, "unknown tool").WithTool(name)
	}
}

// GetProfileSkill returns the registered profile skill instance for wiring.
// Returns nil if the skill has not been registered yet.
func GetProfileSkill() *profileSkill {
	s, ok := skill.Get("profile")
	if !ok {
		return nil
	}
	ps, ok := s.(*profileSkill)
	if !ok {
		return nil
	}
	return ps
}
