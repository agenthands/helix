package relatedidx

// Boilerplate down-weighting for Random Indexing.
//
// Real function bodies across the 11 supported languages are saturated with
// high-frequency plumbing vocabulary — err, ctx, nil, req, resp, return, if,
// self, this, value, result — plus camelCase/snake_case fragments like get,
// set, new. If these dominate the context vector, the cosine between two
// UNRELATED functions clears any moderate threshold and SEMANTICALLY_RELATED
// degenerates toward a near-complete graph (the central vacuity risk). Dropping
// them at token-extraction time is the cheap, deterministic down-weighting that
// keeps the edge a real domain-vocabulary signal rather than a boilerplate
// echo. (MinHash has no analogue — it normalizes all identifiers to "I", so RI
// builds this from scratch.)
//
// This is a curated stop-list, not IDF: it needs no batch pre-pass, stays
// deterministic, and targets exactly the cross-language keyword + ultra-common
// identifier set. IDF over the batch is a possible future refinement (it would
// adapt to a repo's own idioms) but is out of scope here.

// minSubtokenLen drops 1-character fragments (loop indices i/j/k/n, single
// letters) that carry no domain signal regardless of the stop set.
const minSubtokenLen = 2

// stopTokens is the set of lowercased subtokens excluded from context vectors.
// Covers (a) keywords across Go/TS/JS/Python/Java/C#/Rust/C/C++/Kotlin/PHP/Ruby,
// (b) ubiquitous plumbing identifiers, (c) common camelCase/snake_case verbs
// and type words that fragment out of ordinary names. Membership is a fixed
// compile-time map → deterministic, O(1).
var stopTokens = func() map[string]struct{} {
	words := []string{
		// --- control / declaration keywords (cross-language) ---
		"if", "else", "elif", "for", "foreach", "while", "do", "return", "yield",
		"switch", "case", "default", "break", "continue", "goto", "range",
		"func", "function", "fn", "def", "fun", "lambda", "proc", "sub",
		"var", "val", "let", "const", "static", "final", "mut", "auto", "dim",
		"type", "struct", "class", "interface", "enum", "trait", "impl", "object",
		"record", "union", "namespace", "module", "package", "import", "use",
		"using", "include", "require", "from", "export", "extends", "implements",
		"public", "private", "protected", "internal", "abstract", "virtual",
		"override", "async", "await", "go", "defer", "select", "chan", "map",
		"new", "delete", "make", "throw", "throws", "try", "catch", "finally",
		"raise", "except", "with", "as", "in", "is", "not", "and", "or",
		"begin", "end", "then", "elsif", "ensure", "unless", "until", "when",
		// --- primitive / common type words ---
		"int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16",
		"uint32", "uint64", "float", "float32", "float64", "double", "long",
		"short", "byte", "char", "bool", "boolean", "string", "str", "void",
		"object", "any", "dynamic", "var", "list", "array", "slice", "vec",
		"dict", "set", "tuple",
		// --- literals / sentinels ---
		"nil", "null", "none", "nullptr", "true", "false", "this", "self",
		"super", "base", "undefined", "nan",
		// --- ubiquitous plumbing identifiers ---
		"err", "error", "errors", "ctx", "context", "cancel", "req", "request",
		"resp", "response", "res", "result", "ret", "out", "in", "input",
		"output", "val", "value", "values", "tmp", "temp", "buf", "buffer",
		"ok", "args", "arg", "argv", "params", "param", "opts", "opt", "options",
		"cfg", "config", "conf", "data", "item", "items", "elem", "element",
		"key", "keys", "idx", "index", "len", "size", "count", "num", "obj",
		"ptr", "ref", "ptr", "ptrs", "msg", "message", "name", "names",
		"wg", "mu", "mutex", "lock", "once", "done", "next", "prev", "cur",
		"current", "iter", "node", "head", "tail", "left", "right", "root",
		// --- common camelCase/snake verbs that fragment out ---
		"get", "set", "is", "has", "add", "remove", "del", "put", "post",
		"new", "init", "make", "build", "create", "update", "fetch", "load",
		"save", "read", "write", "parse", "format", "to", "of", "by", "with",
		"on", "off", "do", "run", "call", "apply", "handle", "process",
		// --- single common letters as words ---
		"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m",
		"n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z",
	}
	m := make(map[string]struct{}, len(words))
	for _, w := range words {
		m[w] = struct{}{}
	}
	return m
}()

// keepToken reports whether a lowercased subtoken contributes domain vocabulary
// (i.e. survives stop-list + length + pure-digit filtering).
func keepToken(tok string) bool {
	if len(tok) < minSubtokenLen {
		return false
	}
	if _, stop := stopTokens[tok]; stop {
		return false
	}
	allDigit := true
	for _, r := range tok {
		if r < '0' || r > '9' {
			allDigit = false
			break
		}
	}
	return !allDigit
}
