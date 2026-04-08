package pkg

import "fmt"

// Greeter is an interface for cross-file reference and implementation tests.
type Greeter interface {
	Greet(name string) string
}

// SimpleGreeter implements Greeter.
type SimpleGreeter struct{}

// Greet returns a greeting string.
func (s *SimpleGreeter) Greet(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}

// NewGreeter creates a SimpleGreeter. Used for call hierarchy tests.
func NewGreeter() Greeter {
	return &SimpleGreeter{}
}
