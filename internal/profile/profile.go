// Package profile defines agent profiles and operational modes for Serena.
// Profiles are curated tool subsets with prompt overrides targeting specific
// agent environments (Claude Code, Codex, IDE assistants, CI bots).
// Modes define behavioral patterns (read, edit, review, admin) with tool
// inclusion/exclusion rules and prompt fragments.
package profile

import (
	"sort"

	"github.com/postfix/serena/internal/skill"
)

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

	// AllowedModeTransitions defines the state machine for mode switching.
	// Key is the source mode, value is the list of allowed target modes.
	AllowedModeTransitions map[string][]string `yaml:"allowed_mode_transitions"`
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
	p, _ := s.profiles["full"]
	return p
}

// SetProfile adds or replaces a profile in the store.
// Intended for testing and dynamic profile injection.
func (s *ProfileStore) SetProfile(name string, p *Profile) {
	s.profiles[name] = p
}
