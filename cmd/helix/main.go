package main

import (
	"fmt"
	"os"

	"github.com/agenthands/helix/internal/cli"
)

// version is set via -ldflags="-X main.version=$VERSION" by goreleaser; see .goreleaser.yaml.
var version = "dev"

func main() {
	cli.SetVersion(version)
	cmd := cli.NewRootCommand()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
