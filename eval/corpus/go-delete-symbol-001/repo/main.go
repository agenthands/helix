package main

import (
	"fmt"
	"strings"
)

// LegacyParser parses input using the old format. Deprecated: use NewParser.
func LegacyParser(input string) []string {
	return strings.Fields(input)
}

// NewParser parses input using the current format.
func NewParser(input string) []string {
	return strings.Split(strings.TrimSpace(input), ",")
}

func main() {
	result := NewParser("alpha,beta,gamma")
	fmt.Println(result)
}
