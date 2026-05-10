package main

import (
	"fmt"
)

// MaxRetries is the maximum retry count (struct field on Config).
type Config struct {
	MaxRetries int
}

// ShouldRetry calls MaxRetries.
func ShouldRetry(c Config, attempt int) bool {
	return attempt < c.MaxRetries
}

func main() {
	result := ShouldRetry(Config{MaxRetries: 3}, 1)
	fmt.Println(result)
}
