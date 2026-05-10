package main

import (
	"fmt"
	"os/exec"
)

// handleRequest processes a request by running a command.
// The command is constructed safely without shell interpolation.
func handleRequest(name string) error {
	cmd := exec.Command("echo", name)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("handleRequest: %w", err)
	}
	fmt.Printf("result: %s", out)
	return nil
}

func main() {
	if err := handleRequest("world"); err != nil {
		panic(err)
	}
}
