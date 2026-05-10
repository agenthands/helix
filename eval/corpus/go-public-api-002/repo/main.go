package main

import (
	"fmt"
)

// Encode encodes the input. Will be widened to support a context.Context first parameter.
type Encoder struct{}

func (e *Encoder) Encode(input string) string {
	return "encoded:" + input
}

// RunEncoder calls Encode.
func RunEncoder(e *Encoder, input string) string {
	return e.Encode(input)
}

func main() {
	result := RunEncoder(&Encoder{}, "hi")
	fmt.Println(result)
}
