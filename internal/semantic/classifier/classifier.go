// Package classifier provides per-language name-based edge classification
// for HTTP_CALLS, ASYNC_CALLS, EMITS, LISTENS_ON, HANDLES, CONFIGURES, WRITES,
// and AST profiling for DATA_FLOWS.
// Ported from codebase-memory-mcp/src/discover/ (pattern tables) and
// src/semantic/ast_profile.c (AST structural features).
package classifier

import tree_sitter "github.com/tree-sitter/go-tree-sitter"

// CallKind classifies a function/method call by its name into a semantic edge kind.
type CallKind string

const (
	KindHTTPCall   CallKind = "HTTP_CALLS"
	KindAsyncCall  CallKind = "ASYNC_CALLS"
	KindEmit       CallKind = "EMITS"
	KindListenOn   CallKind = "LISTENS_ON"
	KindHandles    CallKind = "HANDLES"
	KindConfigures CallKind = "CONFIGURES"
	KindWrites     CallKind = "WRITES"
)

// ClassifyCall returns the edge kind for a function call name, or empty string.
// Language-specific pattern tables are checked first, then cross-language patterns.
func ClassifyCall(lang, name string) CallKind {
	if k := httpPatterns[lang][name]; k != "" {
		return k
	}
	if k := asyncPatterns[lang][name]; k != "" {
		return k
	}
	if k := emitPatterns[lang][name]; k != "" {
		return k
	}
	if k := listenPatterns[lang][name]; k != "" {
		return k
	}
	if k := handlesPatterns[lang][name]; k != "" {
		return k
	}
	if k := configuresPatterns[lang][name]; k != "" {
		return k
	}
	if k := writesPatterns[lang][name]; k != "" {
		return k
	}
	// Cross-language patterns (framework-agnostic names)
	if k := crossLangHTTP[name]; k != "" {
		return k
	}
	if k := crossLangAsync[name]; k != "" {
		return k
	}
	if k := crossLangEmit[name]; k != "" {
		return k
	}
	if k := crossLangListen[name]; k != "" {
		return k
	}
	if k := crossLangHandles[name]; k != "" {
		return k
	}
	if k := crossLangConfigures[name]; k != "" {
		return k
	}
	if k := crossLangWrites[name]; k != "" {
		return k
	}
	return ""
}

// httpPatterns: per-language HTTP client function/method names.
var httpPatterns = map[string]map[string]CallKind{
	"go": {
		"Get": KindHTTPCall, "Post": KindHTTPCall, "Put": KindHTTPCall,
		"Delete": KindHTTPCall, "Head": KindHTTPCall, "Patch": KindHTTPCall,
		"NewRequest": KindHTTPCall, "NewRequestWithContext": KindHTTPCall,
		"Do": KindHTTPCall, "Client.Do": KindHTTPCall,
	},
	"typescript": {
		"fetch": KindHTTPCall, "get": KindHTTPCall, "post": KindHTTPCall,
		"put": KindHTTPCall, "delete": KindHTTPCall, "patch": KindHTTPCall,
		"request": KindHTTPCall, "axios": KindHTTPCall,
	},
	"javascript": {
		"fetch": KindHTTPCall, "get": KindHTTPCall, "post": KindHTTPCall,
	},
	"python": {
		"get": KindHTTPCall, "post": KindHTTPCall, "put": KindHTTPCall,
		"delete": KindHTTPCall, "request": KindHTTPCall,
		"urlopen": KindHTTPCall, "Request": KindHTTPCall,
	},
	"java": {
		"send": KindHTTPCall, "execute": KindHTTPCall,
		"getForObject": KindHTTPCall, "postForObject": KindHTTPCall,
		"exchange": KindHTTPCall, "get": KindHTTPCall,
	},
	"c_sharp": {
		"GetAsync": KindHTTPCall, "PostAsync": KindHTTPCall,
		"PutAsync": KindHTTPCall, "DeleteAsync": KindHTTPCall,
		"SendAsync": KindHTTPCall, "GetStringAsync": KindHTTPCall,
		"Create": KindHTTPCall,
	},
	"rust": {
		"get": KindHTTPCall, "post": KindHTTPCall,
		"request": KindHTTPCall, "send": KindHTTPCall,
	},
	"php": {
		"curl_exec": KindHTTPCall, "curl_init": KindHTTPCall,
		"file_get_contents": KindHTTPCall, "get": KindHTTPCall,
		"post": KindHTTPCall, "request": KindHTTPCall,
	},
	"ruby": {
		"get": KindHTTPCall, "post": KindHTTPCall, "put": KindHTTPCall,
		"delete": KindHTTPCall, "get_response": KindHTTPCall,
	},
	"c": {
		"curl_easy_perform": KindHTTPCall, "curl_easy_init": KindHTTPCall,
		"curl_easy_setopt": KindHTTPCall,
	},
	"cpp": {
		"curl_easy_perform": KindHTTPCall, "curl_easy_init": KindHTTPCall,
		"SendRequest": KindHTTPCall, "Get": KindHTTPCall,
	},
	"kotlin": {
		"get": KindHTTPCall, "post": KindHTTPCall,
		"request": KindHTTPCall, "execute": KindHTTPCall,
	},
}

// asyncPatterns: per-language async/concurrent function/method names.
var asyncPatterns = map[string]map[string]CallKind{
	"go": {
		"Go": KindAsyncCall, "Add": KindAsyncCall, "Done": KindAsyncCall,
	},
	"typescript": {
		"then": KindAsyncCall, "catch": KindAsyncCall, "finally": KindAsyncCall,
		"all": KindAsyncCall, "race": KindAsyncCall, "resolve": KindAsyncCall,
	},
	"javascript": {
		"then": KindAsyncCall, "catch": KindAsyncCall,
	},
	"python": {
		"ensure_future": KindAsyncCall, "create_task": KindAsyncCall,
		"gather": KindAsyncCall, "run": KindAsyncCall,
		"submit": KindAsyncCall, "map": KindAsyncCall,
	},
	"java": {
		"thenApply": KindAsyncCall, "thenAccept": KindAsyncCall,
		"thenCompose": KindAsyncCall, "supplyAsync": KindAsyncCall,
		"runAsync": KindAsyncCall, "join": KindAsyncCall, "get": KindAsyncCall,
	},
	"c_sharp": {
		"ContinueWith": KindAsyncCall, "Wait": KindAsyncCall,
		"Run": KindAsyncCall, "StartNew": KindAsyncCall,
		"WhenAll": KindAsyncCall, "WhenAny": KindAsyncCall,
	},
	"rust": {
		"spawn": KindAsyncCall, "block_on": KindAsyncCall,
		"join": KindAsyncCall, "select": KindAsyncCall,
	},
	"cpp": {
		"async": KindAsyncCall, "future": KindAsyncCall,
		"promise": KindAsyncCall, "co_await": KindAsyncCall,
	},
	"kotlin": {
		"launch": KindAsyncCall, "async": KindAsyncCall,
		"withContext": KindAsyncCall, "await": KindAsyncCall,
		"runBlocking": KindAsyncCall,
	},
	"ruby": {
		"async": KindAsyncCall, "await": KindAsyncCall,
	},
	"php": {
		"then": KindAsyncCall, "catch": KindAsyncCall,
		"all": KindAsyncCall, "resolve": KindAsyncCall,
	},
	"c": {
		"pthread_create": KindAsyncCall, "fork": KindAsyncCall,
	},
}

// emitPatterns: per-language event emission patterns.
var emitPatterns = map[string]map[string]CallKind{
	"typescript": {
		"emit": KindEmit, "dispatch": KindEmit, "publish": KindEmit,
		"send": KindEmit, "next": KindEmit, "broadcast": KindEmit,
	},
	"javascript": {
		"emit": KindEmit, "dispatch": KindEmit, "publish": KindEmit,
		"send": KindEmit, "trigger": KindEmit,
	},
	"python": {
		"emit": KindEmit, "send": KindEmit, "dispatch": KindEmit,
		"publish": KindEmit, "notify": KindEmit,
	},
	"go": {
		"Emit": KindEmit, "Publish": KindEmit, "Send": KindEmit,
		"Notify": KindEmit, "Broadcast": KindEmit,
	},
	"java": {
		"fireEvent": KindEmit, "publishEvent": KindEmit,
		"post": KindEmit, "send": KindEmit, "emit": KindEmit,
	},
	"c_sharp": {
		"Invoke": KindEmit, "Raise": KindEmit, "Publish": KindEmit,
		"OnNext": KindEmit, "OnCompleted": KindEmit,
		"SendAsync": KindEmit,
	},
	"rust": {
		"send": KindEmit, "emit": KindEmit, "dispatch": KindEmit,
	},
	"kotlin": {
		"emit": KindEmit, "send": KindEmit, "postValue": KindEmit,
		"offer": KindEmit,
	},
}

// listenPatterns: per-language event listener registration patterns.
var listenPatterns = map[string]map[string]CallKind{
	"typescript": {
		"on": KindListenOn, "addListener": KindListenOn,
		"addEventListener": KindListenOn, "once": KindListenOn,
		"subscribe": KindListenOn, "listen": KindListenOn,
	},
	"javascript": {
		"on": KindListenOn, "addEventListener": KindListenOn,
		"addListener": KindListenOn, "subscribe": KindListenOn,
		"listen": KindListenOn, "once": KindListenOn,
	},
	"python": {
		"on": KindListenOn, "subscribe": KindListenOn,
		"add_listener": KindListenOn, "listen": KindListenOn,
		"connect": KindListenOn, "register": KindListenOn,
	},
	"go": {
		"On": KindListenOn, "Subscribe": KindListenOn,
		"Listen": KindListenOn, "Handle": KindListenOn,
		"HandleFunc": KindListenOn, "AddEventListener": KindListenOn,
	},
	"java": {
		"addListener": KindListenOn, "addEventListener": KindListenOn,
		"subscribe": KindListenOn, "on": KindListenOn,
		"register": KindListenOn,
	},
	"c_sharp": {
		"Subscribe": KindListenOn, "add": KindListenOn,
		"Attach": KindListenOn,
	},
	"rust": {
		"on": KindListenOn, "subscribe": KindListenOn,
		"listen": KindListenOn, "recv": KindListenOn,
	},
	"kotlin": {
		"on": KindListenOn, "subscribe": KindListenOn,
		"observe": KindListenOn, "collect": KindListenOn,
	},
	"ruby": {
		"on": KindListenOn, "subscribe": KindListenOn,
		"listen": KindListenOn,
	},
}

// handlesPatterns: per-language request/event/message handler dispatch.
// Names already claimed by http/async/emit/listen tables resolve to those
// kinds first (e.g. Go "Handle"/"HandleFunc" stay LISTENS_ON).
var handlesPatterns = map[string]map[string]CallKind{
	"go": {
		"ServeHTTP": KindHandles, "Serve": KindHandles, "Process": KindHandles,
		"Dispatch": KindHandles, "HandleRequest": KindHandles, "ServeContent": KindHandles,
	},
	"python": {
		"handle": KindHandles, "process": KindHandles,
		"dispatch": KindHandles, "serve": KindHandles,
	},
	"typescript": {
		"handle": KindHandles, "process": KindHandles,
		"dispatch": KindHandles, "serve": KindHandles,
	},
	"javascript": {
		"handle": KindHandles, "process": KindHandles, "dispatch": KindHandles,
	},
	"java": {
		"handle": KindHandles, "process": KindHandles, "service": KindHandles,
		"doGet": KindHandles, "doPost": KindHandles, "doPut": KindHandles, "doDelete": KindHandles,
	},
	"c_sharp": {
		"Handle": KindHandles, "Process": KindHandles, "Serve": KindHandles,
	},
	"rust": {
		"handle": KindHandles, "process": KindHandles, "serve": KindHandles,
	},
	"kotlin": {
		"handle": KindHandles, "process": KindHandles,
	},
	"ruby": {
		"handle": KindHandles, "process": KindHandles,
	},
	"php": {
		"handle": KindHandles, "process": KindHandles,
	},
	"c": {
		"process": KindHandles, "dispatch": KindHandles, "serve": KindHandles,
	},
	"cpp": {
		"Handle": KindHandles, "Process": KindHandles, "Dispatch": KindHandles, "Serve": KindHandles,
	},
}

// configuresPatterns: per-language configuration / setup / registration.
var configuresPatterns = map[string]map[string]CallKind{
	"go": {
		"Configure": KindConfigures, "Setup": KindConfigures, "Register": KindConfigures,
		"Init": KindConfigures, "Initialize": KindConfigures, "Use": KindConfigures,
		"Mount": KindConfigures, "Enable": KindConfigures,
	},
	"python": {
		"configure": KindConfigures, "setup": KindConfigures, "register": KindConfigures,
		"init": KindConfigures, "initialize": KindConfigures, "use": KindConfigures,
	},
	"typescript": {
		"configure": KindConfigures, "setup": KindConfigures, "register": KindConfigures,
		"init": KindConfigures, "use": KindConfigures,
	},
	"javascript": {
		"configure": KindConfigures, "setup": KindConfigures, "register": KindConfigures,
		"init": KindConfigures, "use": KindConfigures,
	},
	"java": {
		"configure": KindConfigures, "setup": KindConfigures, "register": KindConfigures,
		"init": KindConfigures, "initialize": KindConfigures,
	},
	"c_sharp": {
		"Configure": KindConfigures, "Setup": KindConfigures, "Register": KindConfigures,
		"AddSingleton": KindConfigures, "AddScoped": KindConfigures, "AddTransient": KindConfigures,
		"Use": KindConfigures, "Init": KindConfigures,
	},
	"rust": {
		"configure": KindConfigures, "setup": KindConfigures, "register": KindConfigures,
		"init": KindConfigures,
	},
	"kotlin": {
		"configure": KindConfigures, "setup": KindConfigures, "register": KindConfigures,
		"init": KindConfigures, "install": KindConfigures,
	},
	"ruby": {
		"configure": KindConfigures, "setup": KindConfigures, "register": KindConfigures,
	},
	"php": {
		"configure": KindConfigures, "setup": KindConfigures, "register": KindConfigures,
	},
	"c": {
		"configure": KindConfigures, "setup": KindConfigures, "init": KindConfigures, "register": KindConfigures,
	},
	"cpp": {
		"Configure": KindConfigures, "Setup": KindConfigures, "Init": KindConfigures, "Register": KindConfigures,
	},
}

// writesPatterns: per-language persistence / storage write calls. HTTP verbs
// (Get/Post/Put/Delete) are claimed by httpPatterns and win first.
var writesPatterns = map[string]map[string]CallKind{
	"go": {
		"Save": KindWrites, "Insert": KindWrites, "Update": KindWrites,
		"Create": KindWrites, "Exec": KindWrites, "ExecContext": KindWrites,
		"WriteFile": KindWrites, "Write": KindWrites, "Upsert": KindWrites,
		"Persist": KindWrites, "Store": KindWrites, "Append": KindWrites,
	},
	"python": {
		"save": KindWrites, "insert": KindWrites, "update": KindWrites,
		"create": KindWrites, "write": KindWrites, "commit": KindWrites,
		"execute": KindWrites, "flush": KindWrites, "persist": KindWrites,
	},
	"typescript": {
		"save": KindWrites, "insert": KindWrites, "update": KindWrites,
		"create": KindWrites, "write": KindWrites, "set": KindWrites,
		"push": KindWrites, "persist": KindWrites, "upsert": KindWrites,
	},
	"javascript": {
		"save": KindWrites, "insert": KindWrites, "update": KindWrites,
		"create": KindWrites, "write": KindWrites, "set": KindWrites, "push": KindWrites,
	},
	"java": {
		"save": KindWrites, "persist": KindWrites, "merge": KindWrites,
		"create": KindWrites, "insert": KindWrites, "update": KindWrites,
		"write": KindWrites, "executeUpdate": KindWrites,
	},
	"c_sharp": {
		"Save": KindWrites, "SaveChanges": KindWrites, "Insert": KindWrites,
		"Update": KindWrites, "Write": KindWrites, "ExecuteNonQuery": KindWrites,
	},
	"rust": {
		"save": KindWrites, "insert": KindWrites, "create": KindWrites,
		"update": KindWrites, "write": KindWrites, "push": KindWrites,
	},
	"kotlin": {
		"save": KindWrites, "insert": KindWrites, "create": KindWrites,
		"update": KindWrites, "write": KindWrites, "execute": KindWrites,
	},
	"ruby": {
		"save": KindWrites, "create": KindWrites, "update": KindWrites,
		"destroy": KindWrites, "write": KindWrites, "insert": KindWrites,
	},
	"php": {
		"save": KindWrites, "insert": KindWrites, "update": KindWrites,
		"create": KindWrites, "write": KindWrites, "execute": KindWrites, "query": KindWrites,
	},
	"c": {
		"fwrite": KindWrites, "fprintf": KindWrites, "fputs": KindWrites,
		"write": KindWrites,
	},
	"cpp": {
		"Write": KindWrites, "Insert": KindWrites, "push_back": KindWrites,
		"ExecuteUpdate": KindWrites, "Save": KindWrites,
	},
}

// Cross-language patterns (case-insensitive match handled by caller).
var crossLangHTTP = map[string]CallKind{
	"fetch": KindHTTPCall, "request": KindHTTPCall,
}
var crossLangAsync = map[string]CallKind{
	"then": KindAsyncCall, "spawn": KindAsyncCall,
	"await": KindAsyncCall, "async": KindAsyncCall,
}
var crossLangEmit = map[string]CallKind{
	"emit": KindEmit, "fire": KindEmit, "dispatch": KindEmit,
}
var crossLangListen = map[string]CallKind{
	"on": KindListenOn, "subscribe": KindListenOn,
	"listen": KindListenOn,
}
var crossLangHandles = map[string]CallKind{
	"handle": KindHandles, "process": KindHandles,
	"dispatch": KindHandles, "serve": KindHandles,
}
var crossLangConfigures = map[string]CallKind{
	"configure": KindConfigures, "setup": KindConfigures,
	"register": KindConfigures, "init": KindConfigures,
}
var crossLangWrites = map[string]CallKind{
	"save": KindWrites, "insert": KindWrites, "create": KindWrites,
	"update": KindWrites, "write": KindWrites, "persist": KindWrites,
}

// ── DATA_FLOWS: AST structural profile (ported from ast_profile.c) ──

// ASTProfile captures 25 structural features of a function body.
type ASTProfile [25]float32

// ComputeProfile walks a function body AST and computes the structural profile.
// Features: control flow depth (5), nesting (5), expression types (5),
// literals (3), data flow (4), Halstead (3).
func ComputeProfile(body *tree_sitter.Node, source []byte) ASTProfile {
	var p ASTProfile
	if body == nil {
		return p
	}
	// Feature indices
	const (
		fIfDepth     = 0  // max if/else nesting
		fLoopDepth   = 1  // max loop nesting
		fSwitchCount = 2  // switch/match count
		fTryCount    = 3  // try/catch count
		fReturnCount = 4  // return count
		fFuncDepth   = 5  // max function nesting
		fBlockDepth  = 6  // max block nesting
		fExprDepth   = 7  // max expression nesting
		fStmtCount   = 8  // statement count
		fParamCount  = 9  // parameter count
		fCallCount   = 10 // call expression count
		fBinaryCount = 11 // binary expression count
		fUnaryCount  = 12 // unary expression count
		fAssignCount = 13 // assignment count
		fFieldAccess = 14 // field access count
		fStringCount = 15 // string literal count
		fNumberCount = 16 // number literal count
		fBoolCount   = 17 // boolean literal count
		fLocalCount  = 18 // local variable count
		fReadCount   = 19 // variable read count
		fWriteCount  = 20 // variable write count
		fArgCount    = 21 // argument pass count
		fOperatorCnt = 22 // distinct operators
		fOperandCnt  = 23 // distinct operands
		fLength      = 24 // function body length
	)

	var depth, maxDepth int
	walkProfile(body, source, &depth, &maxDepth, &p,
		fIfDepth, fLoopDepth, fSwitchCount, fTryCount, fReturnCount,
		fFuncDepth, fBlockDepth, fExprDepth, fStmtCount, fParamCount,
		fCallCount, fBinaryCount, fUnaryCount, fAssignCount, fFieldAccess,
		fStringCount, fNumberCount, fBoolCount, fLocalCount,
		fReadCount, fWriteCount, fArgCount, fOperatorCnt, fOperandCnt, fLength)
	return p
}

func walkProfile(n *tree_sitter.Node, src []byte, depth *int, maxDepth *int,
	p *ASTProfile,
	ifIdx, loopIdx, switchIdx, tryIdx, retIdx,
	funcIdx, blockIdx, exprIdx, stmtIdx, paramIdx,
	callIdx, binIdx, unIdx, assignIdx, fieldIdx,
	strIdx, numIdx, boolIdx, localIdx,
	readIdx, writeIdx, argIdx, opIdx, operandIdx, lenIdx int) {

	*depth++
	if *depth > *maxDepth {
		*maxDepth = *depth
	}
	p[lenIdx]++

	kind := n.Kind()
	switch {
	case kind == "if_statement" || kind == "if_expression" || kind == "ternary_expression":
		p[ifIdx] = maxF(p[ifIdx], float32(*depth))
	case kind == "for_statement" || kind == "while_statement" || kind == "loop" ||
		kind == "do_statement" || kind == "for_each" || kind == "for_in_statement":
		p[loopIdx] = maxF(p[loopIdx], float32(*depth))
	case kind == "switch_statement" || kind == "match_expression" || kind == "switch_expression":
		p[switchIdx]++
		p[ifIdx] = maxF(p[ifIdx], float32(*depth))
	case kind == "try_statement" || kind == "try_expression" || kind == "catch_clause":
		p[tryIdx]++
	case kind == "return_statement" || kind == "return_expression":
		p[retIdx]++
	case kind == "function_declaration" || kind == "function_definition" ||
		kind == "method_declaration" || kind == "function_item" ||
		kind == "arrow_function" || kind == "lambda_expression" ||
		kind == "closure_expression" || kind == "function_expression":
		p[funcIdx] = maxF(p[funcIdx], float32(*depth))
	case kind == "block" || kind == "compound_statement" || kind == "statement_block" ||
		kind == "declaration_list" || kind == "class_body":
		p[blockIdx] = maxF(p[blockIdx], float32(*depth))
	case kind == "binary_expression" || kind == "augmented_assignment_expression":
		p[binIdx]++
		p[exprIdx] = maxF(p[exprIdx], float32(*depth))
	case kind == "unary_expression" || kind == "update_expression":
		p[unIdx]++
	case kind == "assignment_expression" || kind == "variable_declaration" ||
		kind == "let_declaration" || kind == "const_declaration":
		p[assignIdx]++
	case kind == "call_expression" || kind == "method_invocation" ||
		kind == "function_call_expression" || kind == "invocation_expression":
		p[callIdx]++
		p[exprIdx] = maxF(p[exprIdx], float32(*depth))
	case kind == "field_expression" || kind == "member_expression" ||
		kind == "member_access_expression" || kind == "navigation_expression":
		p[fieldIdx]++
	case kind == "string_literal" || kind == "interpreted_string_literal" ||
		kind == "string" || kind == "encapsed_string" || kind == "template_string":
		p[strIdx]++
	case kind == "number_literal" || kind == "int_literal" || kind == "integer" ||
		kind == "float_literal" || kind == "decimal_integer_literal":
		p[numIdx]++
	case kind == "true" || kind == "false" || kind == "boolean_literal":
		p[boolIdx]++
	case kind == "identifier" || kind == "variable_name" ||
		kind == "field_identifier" || kind == "simple_identifier":
		p[localIdx]++
	case kind == "formal_parameters" || kind == "parameter_list" ||
		kind == "function_value_parameters":
		p[paramIdx]++
	}
	p[stmtIdx]++

	for i := uint(0); i < n.ChildCount(); i++ {
		if c := n.Child(i); c != nil {
			walkProfile(c, src, depth, maxDepth, p,
				ifIdx, loopIdx, switchIdx, tryIdx, retIdx,
				funcIdx, blockIdx, exprIdx, stmtIdx, paramIdx,
				callIdx, binIdx, unIdx, assignIdx, fieldIdx,
				strIdx, numIdx, boolIdx, localIdx,
				readIdx, writeIdx, argIdx, opIdx, operandIdx, lenIdx)
		}
	}
	*depth--
}

func maxF(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
