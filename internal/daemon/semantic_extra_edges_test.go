package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/extract"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
)

// testSym builds a minimal extract.SymbolFact sufficient for factsFromExtracted
// to stamp a non-zero NodeID (= SymbolID) and emit edges.
func testSym(id semantic.SymbolID, lang, kind, name string) extract.SymbolFact {
	return extract.SymbolFact{
		ID:               id,
		Language:         lang,
		Kind:             extract.SymbolKind(kind),
		Name:             name,
		QualifiedName:    name,
		Visibility:       "exported",
		Confidence:       1.0,
		ExtractionSource: "tree_sitter",
	}
}

func TestIsTestSymbol(t *testing.T) {
	cases := []struct {
		lang, name, file string
		want             bool
	}{
		{"go", "TestFoo", "foo_test.go", true},
		{"go", "BenchmarkBar", "bar_test.go", true},
		{"go", "ExampleBaz", "baz_test.go", true},
		{"go", "FuzzQuux", "quux_test.go", true},
		{"go", "TestFoo", "foo.go", false},     // not a test file
		{"go", "Helper", "foo_test.go", false}, // test file, non-test name
		{"python", "test_login", "test_auth.py", true},
		{"python", "TestLogin", "auth_test.py", true},
		{"python", "login", "test_auth.py", false},
		{"typescript", "UserSpec", "user.spec.ts", true},
		{"typescript", "userService", "user.spec.ts", false},
		{"javascript", "runTest", "a.test.js", true},
		// Java / Kotlin / C# — JUnit/NUnit/xUnit class + method conventions.
		{"java", "UserServiceTest", "UserServiceTest.java", true},
		{"java", "testCreateUser", "UserServiceTest.java", true},
		{"java", "helper", "UserServiceTest.java", false},
		{"java", "UserService", "UserService.java", false},
		{"kotlin", "FooTest", "FooTest.kt", true},
		{"c_sharp", "AccountTests", "AccountTests.cs", true},
		{"c_sharp", "testWithdraw", "AccountTests.cs", true},
		// Rust — test_ prefix or tests/ dir.
		{"rust", "test_parse", "src/lib.rs", true},
		{"rust", "integrates_all", "tests/integration.rs", true},
		{"rust", "parse", "src/lib.rs", false},
		// PHP — PHPUnit.
		{"php", "UserTest", "UserTest.php", true},
		{"php", "testStore", "UserTest.php", true},
		// Ruby — Minitest.
		// C / C++ — test fixtures + test-named helpers (macros aren't symbol defs).
		{"cpp", "UserServiceTest", "user_service_test.cpp", true},
		{"c", "test_parse", "test_parser.c", true},
		{"cpp", "Helper", "user_service_test.cpp", false},
		{"ruby", "test_login", "user_test.rb", true},
		{"ruby", "login", "user_test.rb", false},
		{"ruby", "GET", "user_spec.rb", false},
	}
	for _, c := range cases {
		if got := isTestSymbol(c.lang, c.name, c.file); got != c.want {
			t.Errorf("isTestSymbol(%q,%q,%q) = %v, want %v", c.lang, c.name, c.file, got, c.want)
		}
	}
}

func TestTestTargetCandidates(t *testing.T) {
	cases := []struct {
		lang, name string
		want       []string
	}{
		{"go", "TestFoo", []string{"Foo"}},
		{"go", "TestUserRepo_Create", []string{"UserRepo_Create", "UserRepo"}},
		{"go", "Helper", nil},
		{"python", "test_login", []string{"login"}},
		{"python", "TestLogin", []string{"Login"}},
		{"typescript", "UserSpec", []string{"User"}},
		{"javascript", "storeTest", []string{"store"}},
		{"java", "UserServiceTest", []string{"UserService"}},
		{"java", "testCreateUser", []string{"CreateUser"}},
		{"kotlin", "FooTest", []string{"Foo"}},
		{"c_sharp", "AccountTests", []string{"Account"}},
		{"rust", "test_parse", []string{"parse"}},
		{"php", "UserTest", []string{"User"}},
		{"cpp", "UserServiceTest", []string{"UserService"}},
		{"c", "test_parse", []string{"parse"}},
		{"php", "testStore", []string{"Store"}},
		{"ruby", "test_login", []string{"login"}},
	}
	for _, c := range cases {
		got := testTargetCandidates(c.lang, c.name)
		if !equalStrSlices(got, c.want) {
			t.Errorf("testTargetCandidates(%q,%q) = %v, want %v", c.lang, c.name, got, c.want)
		}
	}
}

func equalStrSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestFactsFromExtracted_MemberOfInverseOfDefines: a file with a container
// symbol + one member must emit BOTH a DEFINES (container→member) and its
// inverse MEMBER_OF (member→container).
func TestFactsFromExtracted_MemberOfInverseOfDefines(t *testing.T) {
	ef := &extract.ExtractedFile{
		File: extract.FileFact{Path: "user.go", Language: "go"},
		Symbols: []extract.SymbolFact{
			testSym(100, "go", "struct", "User"),
			testSym(101, "go", "method", "Save"),
		},
	}
	got := factsFromExtracted([]*extract.ExtractedFile{ef}, "r", "", nil, nil)

	var defines, memberOf *semanticstore.EdgeFact
	for i := range got.Edges {
		e := &got.Edges[i]
		switch {
		case e.EdgeKind == "DEFINES" && defines == nil:
			defines = e
		case e.EdgeKind == "MEMBER_OF" && memberOf == nil:
			memberOf = e
		}
	}
	if defines == nil {
		t.Fatal("no DEFINES edge emitted")
	}
	if memberOf == nil {
		t.Fatal("no MEMBER_OF edge emitted")
	}
	if defines.SrcNodeID != 100 || defines.DstNodeID != 101 {
		t.Errorf("DEFINES %d→%d, want 100→101", defines.SrcNodeID, defines.DstNodeID)
	}
	if memberOf.SrcNodeID != 101 || memberOf.DstNodeID != 100 {
		t.Errorf("MEMBER_OF %d→%d, want 101→100 (inverse of DEFINES)", memberOf.SrcNodeID, memberOf.DstNodeID)
	}
}

// TestFactsFromExtracted_TestsEdge: a Test* symbol in a *_test.go file must be
// linked by a TESTS edge to the same-named production symbol in another file
// within the same extraction batch.
func TestFactsFromExtracted_TestsEdge(t *testing.T) {
	prod := &extract.ExtractedFile{
		File:    extract.FileFact{Path: "foo.go", Language: "go"},
		Symbols: []extract.SymbolFact{testSym(100, "go", "function", "Foo")},
	}
	tf := &extract.ExtractedFile{
		File:    extract.FileFact{Path: "foo_test.go", Language: "go"},
		Symbols: []extract.SymbolFact{testSym(200, "go", "function", "TestFoo")},
	}
	got := factsFromExtracted([]*extract.ExtractedFile{prod, tf}, "r", "", nil, nil)

	var testsEdge *semanticstore.EdgeFact
	for i := range got.Edges {
		e := &got.Edges[i]
		if e.EdgeKind == "TESTS" && testsEdge == nil {
			testsEdge = e
		}
	}
	if testsEdge == nil {
		t.Fatal("no TESTS edge emitted for TestFoo → Foo")
	}
	if testsEdge.SrcNodeID != 200 || testsEdge.DstNodeID != 100 {
		t.Errorf("TESTS %d→%d, want TestFoo(200)→Foo(100)", testsEdge.SrcNodeID, testsEdge.DstNodeID)
	}
}

// TestFactsFromExtracted_TestsEdge_NoFalseMatch: a Test* symbol whose stripped
// name has no production counterpart must NOT emit a dangling TESTS edge.
func TestFactsFromExtracted_TestsEdge_NoFalseMatch(t *testing.T) {
	tf := &extract.ExtractedFile{
		File:    extract.FileFact{Path: "lonely_test.go", Language: "go"},
		Symbols: []extract.SymbolFact{testSym(300, "go", "function", "TestOrphan")},
	}
	got := factsFromExtracted([]*extract.ExtractedFile{tf}, "r", "", nil, nil)
	for _, e := range got.Edges {
		if e.EdgeKind == "TESTS" {
			t.Fatalf("unexpected TESTS edge %d→%d for orphan test", e.SrcNodeID, e.DstNodeID)
		}
	}
}

// TestFactsFromExtracted_RouteSymbolAndHandlesEdge: a RouteFact must promote
// into a Route symbol plus a HANDLES edge from the named handler to the route.
func TestFactsFromExtracted_RouteSymbolAndHandlesEdge(t *testing.T) {
	ef := &extract.ExtractedFile{
		File: extract.FileFact{Path: "main.go", Language: "go"},
		Symbols: []extract.SymbolFact{
			testSym(100, "go", "function", "getItems"),
		},
		Routes: []extract.RouteFact{
			{Language: "go", Method: "GET", Path: "/items", Handler: "getItems", File: "main.go"},
		},
	}
	got := factsFromExtracted([]*extract.ExtractedFile{ef}, "r", "", nil, nil)

	var route *semanticstore.SymbolFact
	for i := range got.Symbols {
		if got.Symbols[i].Kind == string(extract.KindRoute) {
			route = &got.Symbols[i]
		}
	}
	if route == nil {
		t.Fatal("no Route symbol synthesized")
	}
	if route.Name != "GET /items" {
		t.Errorf("route name = %q, want %q", route.Name, "GET /items")
	}
	if route.NodeID == 0 {
		t.Errorf("route NodeID = 0; want a stable non-zero hash")
	}

	var handles *semanticstore.EdgeFact
	for i := range got.Edges {
		if got.Edges[i].EdgeKind == "HANDLES" && got.Edges[i].DstNodeID == route.NodeID {
			handles = &got.Edges[i]
		}
	}
	if handles == nil {
		t.Fatal("no HANDLES edge from getItems → route")
	}
	if handles.SrcNodeID != 100 {
		t.Errorf("HANDLES src = %d, want handler node 100", handles.SrcNodeID)
	}
}

// TestFactsFromExtracted_CrossImports: with the repo module path set, imports
// outside the module promote to external-module nodes + CROSS_IMPORTS edges;
// intra-module imports do not.
func TestFactsFromExtracted_CrossImports(t *testing.T) {
	ef := &extract.ExtractedFile{
		File: extract.FileFact{Path: "main.go", Language: "go"},
		Symbols: []extract.SymbolFact{
			testSym(100, "go", "function", "main"),
		},
		Imports: []extract.ImportFact{
			{Language: "go", Source: "github.com/me/app/internal/db", File: "main.go"}, // internal
			{Language: "go", Source: "github.com/lib/cool", File: "main.go"},           // external
			{Language: "go", Source: "github.com/other/dep/pkg", File: "main.go"},      // external
		},
	}
	// knownRepos registers a sibling repo for github.com/lib/cool → that import
	// resolves to "repo-lib-cool"; github.com/other/dep stays module-level.
	known := map[string]string{"github.com/lib/cool": "repo-lib-cool"}
	got := factsFromExtracted([]*extract.ExtractedFile{ef}, "r", "github.com/me/app", known, nil)

	extNodes := map[string]bool{}
	for _, s := range got.Symbols {
		if s.Kind == string(extract.KindExternalModule) {
			extNodes[s.Name] = true
		}
	}
	if !extNodes["github.com/lib/cool"] {
		t.Errorf("missing external-module node github.com/lib/cool; got %v", extNodes)
	}
	if !extNodes["github.com/other/dep"] {
		t.Errorf("missing external-module node github.com/other/dep; got %v", extNodes)
	}
	if extNodes["github.com/me/app/internal/db"] {
		t.Errorf("intra-module import must not become an external-module node")
	}

	var crossCount int
	for _, e := range got.Edges {
		if e.EdgeKind == "CROSS_IMPORTS" {
			crossCount++
		}
	}
	if crossCount != 2 {
		t.Errorf("CROSS_IMPORTS edge count = %d, want 2", crossCount)
	}

	// Resolution: the lib/cool node carries the sibling repoID + high conf;
	// the other/dep node stays unresolved.
	var resolved, unresolved *semanticstore.SymbolFact
	for i := range got.Symbols {
		s := got.Symbols[i]
		if s.Kind != string(extract.KindExternalModule) {
			continue
		}
		switch s.Name {
		case "github.com/lib/cool":
			resolved = &got.Symbols[i]
		case "github.com/other/dep":
			unresolved = &got.Symbols[i]
		}
	}
	if resolved == nil || resolved.LSPIdentity != "repo-lib-cool" {
		t.Errorf("lib/cool node not resolved to sibling repo; got %+v", resolved)
	}
	if unresolved != nil && unresolved.LSPIdentity != "" {
		t.Errorf("other/dep node should be unresolved; LSPIdentity=%q", unresolved.LSPIdentity)
	}
	var resolvedEdge bool
	for _, e := range got.Edges {
		if e.EdgeKind == "CROSS_IMPORTS" && e.Source == "crossrepo.resolved" {
			resolvedEdge = true
		}
	}
	if !resolvedEdge {
		t.Errorf("no crossrepo.resolved CROSS_IMPORTS edge for the sibling-repo import")
	}
}

// TestFactsFromExtracted_ResourceSymbol: a ResourceFact (ORM entity) promotes
// into a Resource symbol node.
func TestFactsFromExtracted_ResourceSymbol(t *testing.T) {
	ef := &extract.ExtractedFile{
		File: extract.FileFact{Path: "user.go", Language: "go"},
		Resources: []extract.ResourceFact{
			{Language: "go", Name: "User", Table: "users", ORM: "gorm", File: "user.go"},
		},
	}
	got := factsFromExtracted([]*extract.ExtractedFile{ef}, "r", "", nil, nil)
	var res *semanticstore.SymbolFact
	for i := range got.Symbols {
		if got.Symbols[i].Kind == string(extract.KindResource) {
			res = &got.Symbols[i]
		}
	}
	if res == nil {
		t.Fatal("no Resource symbol synthesized")
	}
	if res.Name != "User" {
		t.Errorf("Resource name = %q, want User", res.Name)
	}
	if res.NodeID == 0 {
		t.Errorf("Resource NodeID = 0; want stable non-zero")
	}
}

// TestFactsFromExtracted_CrossCalls: a member-call whose receiver matches an
// external import's local binding (cool.DoThing() with import
// github.com/lib/cool) emits a CROSS_CALLS edge to the external-module node.
func TestFactsFromExtracted_CrossCalls(t *testing.T) {
	ef := &extract.ExtractedFile{
		File: extract.FileFact{Path: "main.go", Language: "go"},
		Symbols: []extract.SymbolFact{
			testSym(100, "go", "function", "main"),
		},
		Imports: []extract.ImportFact{
			{Language: "go", Source: "github.com/lib/cool", File: "main.go"}, // external; local binding "cool"
		},
		References: []extract.ReferenceFact{
			{Language: "go", Kind: extract.ReferenceKind("call"), Name: "DoThing", ReceiverText: "cool", File: "main.go"},
		},
	}
	got := factsFromExtracted([]*extract.ExtractedFile{ef}, "r", "github.com/me/app", nil, nil)

	var extNode *semanticstore.SymbolFact
	for i := range got.Symbols {
		if got.Symbols[i].Kind == string(extract.KindExternalModule) && got.Symbols[i].Name == "github.com/lib/cool" {
			extNode = &got.Symbols[i]
		}
	}
	if extNode == nil {
		t.Fatal("no external-module node for github.com/lib/cool")
	}
	var crossCall *semanticstore.EdgeFact
	for i := range got.Edges {
		if got.Edges[i].EdgeKind == "CROSS_CALLS" {
			crossCall = &got.Edges[i]
		}
	}
	if crossCall == nil {
		t.Fatal("no CROSS_CALLS edge emitted for cool.DoThing() → external module")
	}
	if crossCall.DstNodeID != extNode.NodeID {
		t.Errorf("CROSS_CALLS dst = %d, want external-module node %d", crossCall.DstNodeID, extNode.NodeID)
	}
}

func TestMultiRepoRegistry_RegisterAndKnownRepos(t *testing.T) {
	r := newMultiRepoRegistry()
	r.Register("repo-a", "github.com/me/app", "/a")
	r.Register("repo-b", "github.com/lib/cool", "/b")

	// Self is excluded; repos without a module path are skipped.
	r.Register("repo-c", "", "/c")
	known := r.KnownRepos("repo-a")
	if got := known["github.com/lib/cool"]; got != "repo-b" {
		t.Errorf("KnownRepos(repo-a)[lib/cool] = %q, want repo-b", got)
	}
	if _, present := known["github.com/me/app"]; present {
		t.Errorf("self repo must be excluded from KnownRepos")
	}
	if len(known) != 1 {
		t.Errorf("KnownRepos len = %d, want 1 (self + empty-module excluded); got %v", len(known), known)
	}

	// Re-registering refreshes; nil registry is safe.
	r.Register("repo-b", "github.com/lib/cool/v2", "/b")
	if got := r.KnownRepos("repo-a")["github.com/lib/cool/v2"]; got != "repo-b" {
		t.Errorf("refreshed module path not reflected: got %q", got)
	}
	var nilReg *multiRepoRegistry
	nilReg.Register("x", "m", "/") // must not panic
	if k := nilReg.KnownRepos("x"); k != nil {
		t.Errorf("nil registry KnownRepos = %v, want nil", k)
	}
}

func TestDetectRepoModulePath(t *testing.T) {
	// Go module from go.mod.
	goDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(goDir, "go.mod"), []byte("module github.com/me/app\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := detectRepoModulePath(goDir); got != "github.com/me/app" {
		t.Errorf("go.mod: got %q, want github.com/me/app", got)
	}
	// Python: single top-level package dir with __init__.py.
	pyDir := t.TempDir()
	pkgDir := filepath.Join(pyDir, "mypkg")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "__init__.py"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := detectRepoModulePath(pyDir); got != "mypkg" {
		t.Errorf("python package: got %q, want mypkg", got)
	}
	// Empty / unknown → "".
	if got := detectRepoModulePath(t.TempDir()); got != "" {
		t.Errorf("unknown repo: got %q, want empty", got)
	}
}
