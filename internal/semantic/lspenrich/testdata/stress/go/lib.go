// Package stress is the ENRICH-05 stress-test fixture for Phase 61 P04.
//
// Shape: a small Go module with a handful of cross-referenced symbols so a
// single documentSymbol + callHierarchy + definition cascade has real work
// to do. The stress test enqueues many enrichment events against this same
// fixture (the test does NOT need many distinct files — it needs many
// enrichment EVENTS to drive cascade pressure).
package stress

import "fmt"

// Greeter formats a greeting. Callee of Greet and Farewell so callHierarchy
// produces real CALLS edges off the symbols defined in files.go.
type Greeter struct {
	Prefix string
}

// Greet formats a greeting using g.Prefix.
func (g *Greeter) Greet(name string) string {
	return fmt.Sprintf("%s %s!", g.Prefix, name)
}

// Farewell formats a farewell using g.Prefix.
func (g *Greeter) Farewell(name string) string {
	return fmt.Sprintf("%s, goodbye %s.", g.Prefix, name)
}

// NewGreeter constructs a Greeter with the supplied prefix.
func NewGreeter(prefix string) *Greeter {
	return &Greeter{Prefix: prefix}
}
