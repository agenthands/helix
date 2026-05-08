// Stub package used only by analysistest fixtures. The package path
// `internal/semantic/integ_evil` shares the bare-`HasPrefix` collision
// surface with the allowed `internal/semantic/integ` — but lacks the
// slash boundary, so the amended analyzer MUST still flag kernel imports
// of this path. Regression guard for A6 (RESEARCH.md §Common Pitfalls 1).
package integ_evil
