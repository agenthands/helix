// Package broken is the internal-toolbench/go lsp_diagnostics fixture
// (IT-go-lsp-diagnostics-1).
//
// This file has THREE DELIBERATE compile errors that get_diagnostics enumerates:
//  1. an unused import ("strings")
//  2. a wrong return type on Greeting (declared bool, returns a string)
//  3. an undefined variable (prefix) used in the body
//
// The package does not build until ALL three are fixed, so broken_test.go runs
// only if the agent acted on the full diagnostics output (D-05).
package broken

import (
	"fmt"
	"strings"
)

// Greeting returns "Hello, <name>!".
func Greeting(name string) bool {
	return prefix + fmt.Sprintf("Hello, %s!", name)
}
