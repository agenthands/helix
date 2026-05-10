package main

import "fmt"

// unusedHelper is no longer called from anywhere.
func unusedHelper() string {
	return "unused"
}

func main() {
	fmt.Println("hello")
}
