package main

import (
	"fmt"
)

// computeChecksum computes a checksum over the payload bytes (private helper).
func computeChecksum(payload []byte) int {
	sum := 0
	for _, b := range payload {
		sum += int(b)
	}
	return sum
}

// ProcessPayload calls computeChecksum.
func ProcessPayload(payload []byte) string {
	c := computeChecksum(payload)
	return fmt.Sprintf("checksum=%d", c)
}

func main() {
	result := ProcessPayload([]byte("hello"))
	fmt.Println(result)
}
