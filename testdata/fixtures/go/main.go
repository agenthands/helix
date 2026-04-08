package main

import "fmt"

func main() {
	fmt.Println("Hello, Go!")
	Helper()
}

// Helper is a top-level function used for go_to_definition and find_references tests.
func Helper() {
	fmt.Println("Helper function called")
}

// DemoStruct has a known field and method for symbol retrieval tests.
type DemoStruct struct {
	Field int
}

// Value returns the field value. Used for method resolution tests.
func (d *DemoStruct) Value() int {
	return d.Field
}

// UsingHelper calls Helper to create a cross-reference for find_references tests.
func UsingHelper() {
	Helper()
}
