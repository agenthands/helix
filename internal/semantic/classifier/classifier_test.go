package classifier

import (
	"testing"
)

func TestClassifyCall_HTTP(t *testing.T) {
	tests := []struct {
		lang, name string
		want       CallKind
	}{
		{"go", "Get", KindHTTPCall},
		{"typescript", "fetch", KindHTTPCall},
		{"python", "get", KindHTTPCall},
		{"java", "send", KindHTTPCall},
		{"c_sharp", "GetAsync", KindHTTPCall},
		{"rust", "get", KindHTTPCall},
		{"php", "curl_exec", KindHTTPCall},
		{"c", "curl_easy_perform", KindHTTPCall},
		{"cpp", "curl_easy_perform", KindHTTPCall},
		{"ruby", "get", KindHTTPCall},
		{"kotlin", "get", KindHTTPCall},
		{"go", "unknownFunc", ""},
		{"typescript", "unknownFunc", ""},
	}
	for _, tt := range tests {
		got := ClassifyCall(tt.lang, tt.name)
		if got != tt.want {
			t.Errorf("ClassifyCall(%q, %q) = %q, want %q", tt.lang, tt.name, got, tt.want)
		}
	}
}

func TestClassifyCall_Async(t *testing.T) {
	tests := []struct {
		lang, name string
		want       CallKind
	}{
		{"go", "Go", KindAsyncCall},
		{"typescript", "then", KindAsyncCall},
		{"python", "create_task", KindAsyncCall},
		{"java", "thenApply", KindAsyncCall},
		{"c_sharp", "Run", KindAsyncCall},
		{"kotlin", "launch", KindAsyncCall},
	}
	for _, tt := range tests {
		got := ClassifyCall(tt.lang, tt.name)
		if got != tt.want {
			t.Errorf("ClassifyCall(%q, %q) = %q, want %q", tt.lang, tt.name, got, tt.want)
		}
	}
}

func TestClassifyCall_Emit(t *testing.T) {
	cases := map[CallKind][]struct{ lang, name string }{
		KindEmit:     {{"typescript", "emit"}, {"javascript", "emit"}, {"python", "emit"}},
		KindListenOn: {{"typescript", "on"}, {"javascript", "addEventListener"}, {"python", "subscribe"}},
	}
	for want, tests := range cases {
		for _, tt := range tests {
			got := ClassifyCall(tt.lang, tt.name)
			if got != want {
				t.Errorf("ClassifyCall(%q, %q) = %q, want %q", tt.lang, tt.name, got, want)
			}
		}
	}
}

func TestComputeProfile(t *testing.T) {
	// Verify AST profile computation doesn't panic on nil
	p := ComputeProfile(nil, nil)
	for i, v := range p {
		if v != 0 {
			t.Errorf("nil profile[%d] = %f, want 0", i, v)
		}
	}
}

func TestClassifyCall_Handles(t *testing.T) {
	tests := []struct {
		lang, name string
		want       CallKind
	}{
		{"go", "ServeHTTP", KindHandles},
		{"go", "Process", KindHandles},
		{"python", "handle", KindHandles},
		{"java", "doGet", KindHandles},
		{"c_sharp", "Handle", KindHandles},
		{"rust", "serve", KindHandles},
		{"typescript", "handle", KindHandles},
		{"cpp", "Process", KindHandles},
		{"c", "dispatch", KindHandles},
		// unknown language falls through to cross-language table
		{"elixir", "process", KindHandles},
	}
	for _, tt := range tests {
		got := ClassifyCall(tt.lang, tt.name)
		if got != tt.want {
			t.Errorf("ClassifyCall(%q, %q) = %q, want %q", tt.lang, tt.name, got, tt.want)
		}
	}
}

func TestClassifyCall_Configures(t *testing.T) {
	tests := []struct {
		lang, name string
		want       CallKind
	}{
		{"go", "Configure", KindConfigures},
		{"go", "Register", KindConfigures},
		{"python", "setup", KindConfigures},
		{"typescript", "use", KindConfigures},
		{"java", "initialize", KindConfigures},
		{"c_sharp", "AddSingleton", KindConfigures},
		{"kotlin", "install", KindConfigures},
		{"rust", "register", KindConfigures},
		{"cpp", "Setup", KindConfigures},
		{"c", "init", KindConfigures},
		{"elixir", "configure", KindConfigures},
	}
	for _, tt := range tests {
		got := ClassifyCall(tt.lang, tt.name)
		if got != tt.want {
			t.Errorf("ClassifyCall(%q, %q) = %q, want %q", tt.lang, tt.name, got, tt.want)
		}
	}
}

func TestClassifyCall_Writes(t *testing.T) {
	tests := []struct {
		lang, name string
		want       CallKind
	}{
		{"go", "Save", KindWrites},
		{"go", "WriteFile", KindWrites},
		{"go", "ExecContext", KindWrites},
		{"c", "fwrite", KindWrites},
		{"cpp", "Save", KindWrites},
		{"python", "commit", KindWrites},
		{"typescript", "insert", KindWrites},
		{"java", "persist", KindWrites},
		{"c_sharp", "SaveChanges", KindWrites},
		{"rust", "push", KindWrites},
		{"ruby", "destroy", KindWrites},
		{"c", "fwrite", KindWrites},
		{"elixir", "save", KindWrites},
	}
	for _, tt := range tests {
		got := ClassifyCall(tt.lang, tt.name)
		if got != tt.want {
			t.Errorf("ClassifyCall(%q, %q) = %q, want %q", tt.lang, tt.name, got, tt.want)
		}
	}
}

// TestClassifyCall_PrecedenceGuardsExistingKinds: names that appear in BOTH
// the new tables and an existing table MUST resolve to the existing kind
// (http/async/emit/listen are checked before handles/configures/writes).
func TestClassifyCall_PrecedenceGuardsExistingKinds(t *testing.T) {
	tests := []struct {
		lang, name string
		want       CallKind
	}{
		// Go HTTP verbs (httpPatterns) must win over WRITES.
		{"go", "Delete", KindHTTPCall},
		{"go", "Put", KindHTTPCall},
		{"go", "Get", KindHTTPCall},
		// Go listener registration (listenPatterns) wins over HANDLES.
		{"go", "Handle", KindListenOn},
		{"go", "HandleFunc", KindListenOn},
		// Python "create" is in writesPatterns; ensure no earlier table claims it.
		{"python", "create", KindWrites},
		// Unknown name returns empty.
		{"go", "totallyUnknownThing", ""},
	}
	for _, tt := range tests {
		got := ClassifyCall(tt.lang, tt.name)
		if got != tt.want {
			t.Errorf("ClassifyCall(%q, %q) = %q, want %q", tt.lang, tt.name, got, tt.want)
		}
	}
}
