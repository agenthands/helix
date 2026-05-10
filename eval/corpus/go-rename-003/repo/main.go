package main

import (
	"fmt"
)

// Dispatch dispatches a message to a single channel (method on Router).
type Router struct{ name string }

func (r *Router) Dispatch(msg string) string {
	return r.name + ": " + msg
}

// Broadcast calls Dispatch.
func Broadcast(r *Router, msgs []string) []string {
	out := make([]string, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, r.Dispatch(m))
	}
	return out
}

func main() {
	result := Broadcast(&Router{name: "main"}, []string{"hi"})
	fmt.Println(result)
}
