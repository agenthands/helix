package guardrails

import "fmt"

// EnforcementLevel is the closed enum for guardrail enforcement modes.
// D-21 (LOCKED): off | warn | enforce | require_force.
type EnforcementLevel string

const (
	// LevelOff skips guardrail evaluation entirely.
	LevelOff EnforcementLevel = "off"
	// LevelWarn evaluates and emits a warning but allows the tool to proceed.
	LevelWarn EnforcementLevel = "warn"
	// LevelEnforce refuses unless required receipts are satisfied.
	LevelEnforce EnforcementLevel = "enforce"
	// LevelRequireForce refuses unless receipts are satisfied OR explicit force
	// override is present. The force-override surface is out of scope for Phase 66;
	// until implemented, LevelRequireForce behaves identically to LevelEnforce.
	LevelRequireForce EnforcementLevel = "require_force"
)

// ErrInvalidEnforcementLevel is returned by ParseEnforcementLevel on unknown input.
type ErrInvalidEnforcementLevel struct {
	Input string
}

func (e *ErrInvalidEnforcementLevel) Error() string {
	return fmt.Sprintf("invalid enforcement level %q: must be one of off|warn|enforce|require_force", e.Input)
}

// ParseEnforcementLevel parses a string into an EnforcementLevel.
// Returns ErrInvalidEnforcementLevel for unknown values.
func ParseEnforcementLevel(s string) (EnforcementLevel, error) {
	switch EnforcementLevel(s) {
	case LevelOff, LevelWarn, LevelEnforce, LevelRequireForce:
		return EnforcementLevel(s), nil
	default:
		return "", &ErrInvalidEnforcementLevel{Input: s}
	}
}

// EnforcementLayer carries an EnforcementLevel and a Set flag.
// Set==false means this layer was not explicitly configured; first Set layer wins.
type EnforcementLayer struct {
	Level EnforcementLevel
	Set   bool
}

// ResolveGuardrailEnforcement resolves the enforcement level using 5-layer
// highest-precedence-set-wins logic (D-20 LOCKED):
//
//	CLI > per-tool > per-rule > profile > global default
//
// The FIRST layer where Set==true wins. NOT most-restrictive: an operator can
// set global=enforce and rule.G-001=warn; the result for G-001 is warn.
// If no layer is Set, returns global.Level (defaulting to LevelWarn if global also unset).
func ResolveGuardrailEnforcement(cli, perTool, perRule, profile, global EnforcementLayer) EnforcementLevel {
	for _, layer := range []EnforcementLayer{cli, perTool, perRule, profile} {
		if layer.Set {
			return layer.Level
		}
	}
	// Fall through to global (always "set" conceptually — return its Level even if !Set).
	if global.Set {
		return global.Level
	}
	// No layer set at all: return the safe default.
	return LevelWarn
}
