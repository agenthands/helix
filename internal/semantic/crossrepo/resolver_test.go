package crossrepo

import "testing"

func TestClassifyImport(t *testing.T) {
	cases := []struct {
		name, repo, lang, src string
		want                  Relation
	}{
		// Go
		{"go internal sub", "github.com/me/app", "go", "github.com/me/app/internal/db", Internal},
		{"go external dep", "github.com/me/app", "go", "github.com/lib/cool/pkg", External},
		{"go stdlib", "github.com/me/app", "go", "fmt", Internal},
		// Python
		{"py internal pkg", "mypkg", "python", "mypkg.sub.mod", Internal},
		{"py relative", "mypkg", "python", ".sub", Internal},
		{"py external", "mypkg", "python", "fastapi.routing", External},
		// TS/JS
		{"ts relative", "", "typescript", "./util", Internal},
		{"ts alias", "", "typescript", "@/components/Button", Internal},
		{"ts bare external", "", "typescript", "react", External},
		// Rust
		{"rust crate internal", "", "rust", "crate::internal::db", Internal},
		{"rust self internal", "", "rust", "self::Foo", Internal},
		{"rust super internal", "", "rust", "super::Bar", Internal},
		{"rust std external", "", "rust", "std::collections::HashMap", External},
		{"rust third-party external", "", "rust", "serde::Deserialize", External},
		// Java / Kotlin
		{"java stdlib", "", "java", "java.util.List", Internal},
		{"java javax stdlib", "", "java", "javax.servlet.http", Internal},
		{"java external", "com.example", "java", "org.springframework.web.Rest", External},
		{"java internal pkg", "com.example", "java", "com.example.svc.UserService", Internal},
		{"kotlin stdlib", "", "kotlin", "kotlin.collections.List", Internal},
		{"kotlin external", "com.example", "kotlin", "io.ktor.server.routing", External},
		// C#
		{"csharp system bcl", "", "c_sharp", "System.IO.File", Internal},
		{"csharp external", "MyApp", "c_sharp", "Newtonsoft.Json.Converters", External},
		{"csharp internal ns", "MyApp", "c_sharp", "MyApp.Services.Email", Internal},
		// PHP
		{"php internal ns", "App\\Models", "php", "App\\Models\\User", Internal},
		{"php external", "App\\Models", "php", "Symfony\\Component\\HttpFoundation", External},
		// Ruby
		{"ruby relative", "", "ruby", "./lib/helper", Internal},
		{"ruby gem external", "", "ruby", "active_record/base", External},
		// C / C++ — never cross-repo
		{"c include internal", "", "c", "stdio.h", Internal},
		{"cpp include internal", "", "cpp", "vector", Internal},
		// Edge
		{"empty source", "x", "go", "", Internal},
		{"unknown bare", "", "scala", "cats.App", External},
	}
	for _, c := range cases {
		if got := ClassifyImport(c.repo, c.lang, c.src); got != c.want {
			t.Errorf("ClassifyImport(%q,%q,%q) = %v, want %v [%s]", c.repo, c.lang, c.src, got, c.want, c.name)
		}
	}
}

func TestModulePathPrefix(t *testing.T) {
	cases := []struct {
		lang, src, want string
	}{
		{"go", "github.com/org/repo/pkg/sub", "github.com/org/repo"},
		{"python", "package.sub.mod", "package"},
		{"typescript", "@scope/pkg/sub", "@scope/pkg"},
		{"typescript", "react", "react"},
		{"typescript", "./local", ""},
		{"rust", "serde::de::Deserialize", "serde"},
		{"rust", "crate::internal::x", ""},
		{"rust", "std::collections::HashMap", "std"},
		{"java", "org.springframework.web.Rest", "org.springframework"},
		{"kotlin", "com.example.svc.UserService", "com.example"},
		{"c_sharp", "Newtonsoft.Json.Converters", "Newtonsoft.Json"},
		{"php", "Symfony\\Component\\HttpFoundation", "Symfony\\Component"},
		{"ruby", "active_record/base", "active_record"},
		{"ruby", "./lib/x", ""},
		{"c", "stdio.h", ""},
	}
	for _, c := range cases {
		if got := ModulePathPrefix(c.lang, c.src); got != c.want {
			t.Errorf("ModulePathPrefix(%q,%q) = %q, want %q", c.lang, c.src, got, c.want)
		}
	}
}

func TestResolve(t *testing.T) {
	// known keys use the same module-root segmentation ModulePathPrefix emits.
	known := map[string]string{
		"github.com/org/lib-a": "repo-lib-a",
		"github.com/org":       "repo-org-root",
		"org.springframework":  "repo-spring",
		"com.example":          "repo-example-lib",
		"serde":                "repo-serde",
		"Symfony\\Component":   "repo-symfony",
	}
	cases := []struct {
		name, lang, src, wantID string
		wantOK                  bool
	}{
		{"go resolved deep", "go", "github.com/org/lib-a/internal/db", "repo-lib-a", true},
		{"go longest-prefix wins", "go", "github.com/org/lib-a", "repo-lib-a", true},
		{"java spring resolved", "java", "org.springframework.web.bind.RestController", "repo-spring", true},
		{"java own-lib resolved", "java", "com.example.lib.api.Client", "repo-example-lib", true},
		{"rust crate resolved", "rust", "serde::de::Deserialize", "repo-serde", true},
		{"php namespace resolved", "php", "Symfony\\Component\\HttpFoundation\\Request", "repo-symfony", true},
		{"unresolved external", "go", "github.com/unknown/dep", "", false},
		{"rust crate-local unresolved", "rust", "crate::internal::x", "", false},
		{"relative no target", "typescript", "./local", "", false},
	}
	for _, c := range cases {
		id, ok := Resolve(c.lang, c.src, known)
		if id != c.wantID || ok != c.wantOK {
			t.Errorf("Resolve(%q,%q) = (%q,%v), want (%q,%v) [%s]", c.lang, c.src, id, ok, c.wantID, c.wantOK, c.name)
		}
	}
}
