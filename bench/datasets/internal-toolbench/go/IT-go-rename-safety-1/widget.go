// Package rename is the internal-toolbench/go rename_safety fixture
// (IT-go-rename-safety-1).
//
// Foo is defined here and referenced from consumer.go. rename_test.go expects
// the symbol to be named Bar, so the package does not compile until Foo is
// renamed EVERYWHERE. A missed reference leaves a dangling Foo (or a dangling
// Bar caller) and breaks the build — so the test requires the FULL
// find_references output to be acted on (D-05).
package rename

// Foo returns the widget label. It must be renamed to Bar across all references.
func Foo() string {
	return "widget"
}
