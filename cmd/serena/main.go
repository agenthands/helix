package main

// Build-time version metadata (Version/Commit/Date) is injected into the
// internal/cli package via -ldflags, NOT into main. See
// internal/cli/version.go and .planning/phases/51-packaging-goreleaser/
// 51-CONTEXT.md D-15. cmd/serena is a thin entry point.

import (
	"fmt"
	"os"

	"github.com/postfix/serena/internal/cli"
)

func main() {
	cmd := cli.NewRootCommand()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
