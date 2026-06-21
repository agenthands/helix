// Package profile defines agent profiles and operational modes for Helix.
// Profiles are curated tool subsets with prompt overrides targeting specific
// agent environments (Claude Code, Codex, IDE assistants, CI bots).
// Modes define behavioral patterns (read, edit, review, admin) with tool
// inclusion/exclusion rules and prompt fragments.
package profile

import (
	"sort"

	"github.com/agenthands/helix/internal/skill"
)

// ProfileGuardrailsConfig carries the guardrails block decoded from a
// profile YAML file. Phase 66 Plan 02 — D-11 profile-level enforcement defaults.
//
// The Enforcement field is the only profile-level guardrail override this phase.
// The full D-20 precedence resolver (ResolveGuardrailEnforcement) consults this
// value at the profile layer (CLI > per-tool > per-rule > profile > global).
type ProfileGuardrailsConfig struct {
	// Enforcement is the profile-level default enforcement level.
	// Accepted values: "off" | "warn" | "enforce" | "require_force".
	// D-11 defaults: ci-bot → "enforce"; all other profiles → "warn".
	Enforcement string `yaml:"enforcement"`
}

// Profile extends ContextSpec with agent-specific fields.
// Each profile targets a specific agent environment and controls which tools
// are exposed, how they are described, and what prompt guidance is provided.
type Profile struct {
	skill.ContextSpec `yaml:",inline"`

	// Prompt is a system prompt fragment providing agent-specific guidance.
	Prompt string `yaml:"prompt"`

	// ToolDescriptionOverrides rewrites tool descriptions for this profile.
	// Key is tool name, value is the replacement description.
	ToolDescriptionOverrides map[string]string `yaml:"tool_description_overrides"`

	// DefaultMode is the initial operational mode for new sessions.
	DefaultMode string `yaml:"default_mode"`

	// SingleProject restricts sessions to a single project when true.
	SingleProject bool `yaml:"single_project"`

	// DisableLSPSubsystem requests the daemon disable the LSP subsystem for this
	// profile (Phase 76 ABLATE-05). First-class field so a bench ablation arm's
	// YAML fully describes the arm (tool surface + subsystem flags, D-01).
	// Zero-value false = subsystem ENABLED; the flag is an opt-in disable (D-02).
	// Read at the daemon composition root (Plan 76-04), OR'd with any CLI override.
	DisableLSPSubsystem bool `yaml:"disable_lsp_subsystem"`

	// DisableSemanticSubsystem requests the daemon disable the semantic-store
	// read seam for this profile (Phase 81 ABLATE-06). First-class field so a
	// bench ablation arm's YAML fully describes the arm (tool surface +
	// subsystem flags, D-01). Zero-value false = subsystem ENABLED; the flag is
	// an opt-in disable (D-02). Read at the daemon composition root (Plan 81-04),
	// OR'd with any CLI override; it resolves to the distinct
	// `semantic_index.bench_disabled` koanf gate (build-but-block, D-03/D-04).
	DisableSemanticSubsystem bool `yaml:"disable_semantic_subsystem"`

	// DisableStructuredEditSubsystem requests the daemon disable structured edits
	// (replace_symbol_body / fuzzy_edit / insert_*) for this profile
	// (Phase 76 ABLATE-07). First-class field per D-01; zero-value false =
	// ENABLED, opt-in disable per D-02. Read at the daemon composition root
	// (Plan 76-04), OR'd with any CLI override.
	DisableStructuredEditSubsystem bool `yaml:"disable_structured_edit_subsystem"`

	// AllowedModeTransitions defines the state machine for mode switching.
	// Key is the source mode, value is the list of allowed target modes.
	AllowedModeTransitions map[string][]string `yaml:"allowed_mode_transitions"`

	// Guardrails carries the per-profile guardrail enforcement default.
	// Phase 66 Plan 02 — D-11 / D-22 LOCKED per-profile defaults.
	Guardrails ProfileGuardrailsConfig `yaml:"guardrails"`
}

// Mode extends ModeSpec with behavioral policies.
// Modes control tool availability and prompt fragments within a session.
type Mode struct {
	skill.ModeSpec `yaml:",inline"`

	// Prompt is a mode-specific prompt fragment providing behavioral guidance.
	Prompt string `yaml:"prompt"`
}

// ProfileStore holds loaded profiles and modes, keyed by name.
type ProfileStore struct {
	profiles map[string]*Profile
	modes    map[string]*Mode
}

// NewProfileStore creates an empty ProfileStore.
func NewProfileStore() *ProfileStore {
	return &ProfileStore{
		profiles: make(map[string]*Profile),
		modes:    make(map[string]*Mode),
	}
}

// Profile returns a profile by name and whether it was found.
func (s *ProfileStore) Profile(name string) (*Profile, bool) {
	p, ok := s.profiles[name]
	return p, ok
}

// Mode returns a mode by name and whether it was found.
func (s *ProfileStore) Mode(name string) (*Mode, bool) {
	m, ok := s.modes[name]
	return m, ok
}

// ProfileNames returns a sorted list of all profile names.
func (s *ProfileStore) ProfileNames() []string {
	names := make([]string, 0, len(s.profiles))
	for name := range s.profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ModeNames returns a sorted list of all mode names.
func (s *ProfileStore) ModeNames() []string {
	names := make([]string, 0, len(s.modes))
	for name := range s.modes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DefaultProfile returns the "full" profile as a fallback.
// If no "full" profile is loaded, returns nil.
func (s *ProfileStore) DefaultProfile() *Profile {
	return s.profiles["full"]
}

// ToolDescriptionOverrides returns the description override map for the named profile.
// Returns nil if the profile has no overrides or is not found.
// This method satisfies the mcp.ProfileResolver interface.
func (s *ProfileStore) ToolDescriptionOverrides(profileName string) map[string]string {
	p, ok := s.profiles[profileName]
	if !ok {
		return nil
	}
	return p.ToolDescriptionOverrides
}

// SetProfile adds or replaces a profile in the store.
// Intended for testing and dynamic profile injection.
func (s *ProfileStore) SetProfile(name string, p *Profile) {
	s.profiles[name] = p
}

// SetMode adds or replaces a mode in the store.
// Intended for testing and dynamic mode injection.
func (s *ProfileStore) SetMode(name string, m *Mode) {
	s.modes[name] = m
}
