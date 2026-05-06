// Package main is the cascade integration-test fixture for Phase 61 P02.
//
// Shape: a small Go program with one caller and one callee so the cascade
// produces at least one CALLS edge via callHierarchy and at least one
// RESOLVES_TO edge via definition. The package compiles cleanly so gopls
// indexes it without diagnostics noise.
package main

import "fmt"

// Greet returns a greeting for name. It is the callee target of main, so
// callHierarchy on main.main yields a CALLS edge (main -> Greet).
func Greet(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}

// Farewell returns a farewell for name. Distinct from Greet so the cascade
// emits multiple symbols and we can assert MaxSymbolsPerFile honors them.
func Farewell(name string) string {
	return fmt.Sprintf("Goodbye, %s!", name)
}

func main() {
	greeting := Greet("World")
	farewell := Farewell("World")
	fmt.Println(greeting)
	fmt.Println(farewell)
}
