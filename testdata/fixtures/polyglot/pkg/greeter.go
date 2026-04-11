package pkg

import "fmt"

// Greeter provides greeting functionality for cross-package reference tests.
type Greeter struct{}

// Greet returns a greeting string.
func (g *Greeter) Greet(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}
