// Package stress — caller-side file for the ENRICH-05 stress fixture.
//
// Files.go defines top-level functions that call into lib.go (Greeter,
// Greet, Farewell, NewGreeter) so the cascade has cross-symbol callHierarchy
// edges to emit. Two files are sufficient for the stress test — the test
// drives pressure via repeated enqueue events, not file count.
package stress

import "fmt"

// SayHello calls Greet on a default Greeter and prints the result.
// callHierarchy on SayHello produces a CALLS edge to Greeter.Greet.
func SayHello(name string) {
	g := NewGreeter("Hello,")
	out := g.Greet(name)
	fmt.Println(out)
}

// SayBye calls Farewell on a default Greeter and prints the result.
// callHierarchy on SayBye produces a CALLS edge to Greeter.Farewell.
func SayBye(name string) {
	g := NewGreeter("So")
	out := g.Farewell(name)
	fmt.Println(out)
}

// RunBoth exercises both greet and bye paths in one call so the per-symbol
// reference loop has at least two non-definition references to chase per
// SayHello / SayBye on a real-LSP run.
func RunBoth(name string) {
	SayHello(name)
	SayBye(name)
}

// FormatLine is a small leaf utility — present so the cascade hover step on
// any of the three above produces a non-trivial type signature.
func FormatLine(prefix, body string) string {
	return fmt.Sprintf("[%s] %s", prefix, body)
}
