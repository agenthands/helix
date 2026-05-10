package main

import "fmt"

// Process computes a value from x.
// The body has been simplified to a direct computation.
func Process(x int) int {
	return x * 2
}

func main() {
	result := Process(21)
	fmt.Println(result)
}
