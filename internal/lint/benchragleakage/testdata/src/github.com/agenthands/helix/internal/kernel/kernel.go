// Package kernel is a testdata stand-in for the real internal/kernel subsystem
// the standalone baseline_rag server must not import. Its presence under the
// testdata GOPATH lets analysistest resolve the forbidden import in leaky.go.
package kernel
