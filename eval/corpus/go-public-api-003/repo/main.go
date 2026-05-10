package main

import (
	"fmt"
)

// Validate validates input and returns an error. Will be widened to return a typed *ValidationError instead of error.
func Validate(input string) error {
	if input == "" {
		return fmt.Errorf("empty input")
	}
	return nil
}

// RunValidate calls Validate.
func RunValidate(input string) string {
	if err := Validate(input); err != nil {
		return "invalid: " + err.Error()
	}
	return "ok"
}

func main() {
	result := RunValidate("hi")
	fmt.Println(result)
}
