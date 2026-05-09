package catalogs_test

import (
	"testing"

	"github.com/agenthands/helix/internal/guardrails/catalogs"
	"github.com/agenthands/helix/internal/semantic"
)

func emptyCfg() semantic.GuardrailsConfig {
	return semantic.GuardrailsConfig{}
}

// TestLoad_AllLanguages ensures all four embedded catalogs decode with non-empty ImportPatterns.
func TestLoad_AllLanguages(t *testing.T) {
	langs := []string{"go", "typescript", "javascript", "python"}
	for _, lang := range langs {
		t.Run(lang, func(t *testing.T) {
			cat, err := catalogs.Load(lang, emptyCfg())
			if err != nil {
				t.Fatalf("Load(%q) error: %v", lang, err)
			}
			if len(cat.ImportPatterns) == 0 {
				t.Errorf("Load(%q): expected non-empty ImportPatterns", lang)
			}
		})
	}
}

// TestLoad_UnknownLanguage verifies unknown languages return empty Catalog + nil error.
func TestLoad_UnknownLanguage(t *testing.T) {
	cat, err := catalogs.Load("rust", emptyCfg())
	if err != nil {
		t.Fatalf("Load(rust) unexpected error: %v", err)
	}
	if len(cat.ImportPatterns) != 0 {
		t.Errorf("expected empty ImportPatterns for unknown language, got %v", cat.ImportPatterns)
	}
}

// TestMatchesImportPattern_GoSuffix: "crypto/aes" matches "crypto/*"; "crypto" does not.
func TestMatchesImportPattern_GoSuffix(t *testing.T) {
	patterns := []string{"crypto/*"}
	if !catalogs.MatchesImportPattern("crypto/aes", patterns) {
		t.Error("expected crypto/aes to match crypto/*")
	}
	if catalogs.MatchesImportPattern("crypto", patterns) {
		t.Error("expected crypto (no slash) to NOT match crypto/*")
	}
	if catalogs.MatchesImportPattern("crypto/", patterns) {
		t.Error("expected crypto/ (empty remainder) to NOT match crypto/*")
	}
}

// TestMatchesImportPattern_NPMScoped: "@auth/core" matches "@auth/*".
func TestMatchesImportPattern_NPMScoped(t *testing.T) {
	patterns := []string{"@auth/*"}
	if !catalogs.MatchesImportPattern("@auth/core", patterns) {
		t.Error("expected @auth/core to match @auth/*")
	}
	if !catalogs.MatchesImportPattern("@auth/sveltekit", patterns) {
		t.Error("expected @auth/sveltekit to match @auth/*")
	}
	if catalogs.MatchesImportPattern("@other/core", patterns) {
		t.Error("expected @other/core to NOT match @auth/*")
	}
}

// TestMatchesImportPattern_PythonDotted: "cryptography.fernet" matches "cryptography.*".
func TestMatchesImportPattern_PythonDotted(t *testing.T) {
	patterns := []string{"cryptography.*"}
	if !catalogs.MatchesImportPattern("cryptography.fernet", patterns) {
		t.Error("expected cryptography.fernet to match cryptography.*")
	}
	if !catalogs.MatchesImportPattern("cryptography.hazmat.primitives.ciphers", patterns) {
		t.Error("expected deep dotted path to match cryptography.*")
	}
	if catalogs.MatchesImportPattern("cryptography2.fernet", patterns) {
		t.Error("expected cryptography2.fernet to NOT match cryptography.*")
	}
}

// TestMatchesImportPattern_PrefixWildcard: "passport-local" matches "passport-*".
func TestMatchesImportPattern_PrefixWildcard(t *testing.T) {
	patterns := []string{"passport-*"}
	if !catalogs.MatchesImportPattern("passport-local", patterns) {
		t.Error("expected passport-local to match passport-*")
	}
	if !catalogs.MatchesImportPattern("passport-jwt", patterns) {
		t.Error("expected passport-jwt to match passport-*")
	}
	if catalogs.MatchesImportPattern("passport", patterns) {
		t.Error("expected passport (no dash) to NOT match passport-*")
	}
}

// TestMatchesImportPattern_Exact: "bcrypt" matches "bcrypt"; "bcryptjs" does not.
func TestMatchesImportPattern_Exact(t *testing.T) {
	patterns := []string{"bcrypt"}
	if !catalogs.MatchesImportPattern("bcrypt", patterns) {
		t.Error("expected bcrypt to match bcrypt exactly")
	}
	if catalogs.MatchesImportPattern("bcryptjs", patterns) {
		t.Error("expected bcryptjs to NOT match bcrypt (no wildcard)")
	}
}

// TestMatchesPathGlob_Doublestar: doublestar "**" matches zero or more segments.
// "**/auth*.go" matches files named auth*.go anywhere in the directory tree.
func TestMatchesPathGlob_Doublestar(t *testing.T) {
	globs := []string{"**/auth*.go"}
	// auth_handler.go nested under pkg/
	if !catalogs.MatchesPathGlob("pkg/auth_handler.go", globs) {
		t.Error("expected pkg/auth_handler.go to match **/auth*.go")
	}
	// auth_service.go at root
	if !catalogs.MatchesPathGlob("auth_service.go", globs) {
		t.Error("expected root-level auth_service.go to match **/auth*.go")
	}
	// deeply nested auth*.go
	if !catalogs.MatchesPathGlob("internal/api/auth_middleware.go", globs) {
		t.Error("expected deeply nested auth_middleware.go to match **/auth*.go")
	}
	// service.go should NOT match
	if catalogs.MatchesPathGlob("pkg/service.go", globs) {
		t.Error("expected pkg/service.go to NOT match **/auth*.go")
	}
}

// TestMatchesIdentifier_Anchored: "AuthMiddleware" matches ".*Auth.*"; "unknown" does not.
func TestMatchesIdentifier_Anchored(t *testing.T) {
	patterns := []string{".*Auth.*"}
	if !catalogs.MatchesIdentifier("AuthMiddleware", patterns) {
		t.Error("expected AuthMiddleware to match .*Auth.*")
	}
	if !catalogs.MatchesIdentifier("MyAuthService", patterns) {
		t.Error("expected MyAuthService to match .*Auth.*")
	}
	if catalogs.MatchesIdentifier("unknown", patterns) {
		t.Error("expected 'unknown' (no Auth substring) to NOT match .*Auth.*")
	}
}

// TestLoad_OverrideViaConfig: cfg.G005.ImportPatterns["go"] replaces default import_patterns.
func TestLoad_OverrideViaConfig(t *testing.T) {
	cfg := semantic.GuardrailsConfig{
		G005: semantic.G005Config{
			ImportPatterns: map[string][]string{
				"go": {"custom/*"},
			},
		},
	}
	cat, err := catalogs.Load("go", cfg)
	if err != nil {
		t.Fatalf("Load(go, override) error: %v", err)
	}
	if len(cat.ImportPatterns) != 1 || cat.ImportPatterns[0] != "custom/*" {
		t.Errorf("expected ImportPatterns to be replaced with [custom/*], got %v", cat.ImportPatterns)
	}
}

// TestLoad_MergePathGlobs: cfg.G005.PathGlobs are merged (not replaced) with defaults.
func TestLoad_MergePathGlobs(t *testing.T) {
	cfg := semantic.GuardrailsConfig{
		G005: semantic.G005Config{
			PathGlobs: []string{"**/custom_secret*.go"},
		},
	}
	cat, err := catalogs.Load("go", cfg)
	if err != nil {
		t.Fatalf("Load(go, pathGlobs) error: %v", err)
	}
	// Default go.yaml has path_globs; we expect merged result.
	found := false
	for _, g := range cat.PathGlobs {
		if g == "**/custom_secret*.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected cfg PathGlob to be merged into catalog, got %v", cat.PathGlobs)
	}
}
