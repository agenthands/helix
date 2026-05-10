package main

import (
	"fmt"
)

// unusedHelper is an unused legacy helper (deprecated).
func unusedHelper(s string) string {
	return "[" + s + "]"
}

// RealWork calls unusedHelper.
func RealWork(s string) string {
	return s + "!"
}

func main() {
	result := RealWork("hi")
	fmt.Println(result)
}
