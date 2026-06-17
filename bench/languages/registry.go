package languages

import "sync"

// The registry maps (benchmark, language) → LanguageRunner. It is a Caddy-style
// registry: language adapter packages call Register from init(), and the harness
// resolves via RunnerFor. A nil result from RunnerFor is the D-10 fallback signal
// ("no structured runner registered — fall back to verify.sh").
//
// Modeled on internal/skill/registry.go (RWMutex + map + Register/Get).
var (
	registryMu sync.RWMutex
	runners    = make(map[string]LanguageRunner)
)

// key composes the registry key from a benchmark and language.
func key(benchmark, lang string) string {
	return benchmark + "/" + lang
}

// Register adds a runner for (benchmark, lang). Intended to be called from init()
// in language adapter packages (Caddy-style registration). Duplicate keys
// overwrite silently (last wins).
func Register(benchmark, lang string, r LanguageRunner) {
	registryMu.Lock()
	defer registryMu.Unlock()
	runners[key(benchmark, lang)] = r
}

// RunnerFor returns the runner registered for (benchmark, lang), or nil if none
// is registered. A nil return is the D-10 verify.sh-fallback signal.
func RunnerFor(benchmark, lang string) LanguageRunner {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return runners[key(benchmark, lang)]
}
