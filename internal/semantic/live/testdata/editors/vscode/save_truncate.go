// Command save_truncate reproduces VS Code's default save mode —
// open the target file with O_TRUNC|O_WRONLY|O_CREATE, write, close.
// Source-of-truth: 60-RESEARCH.md "Editor Save-Pattern Dossier —
// VS Code — default truncate-write".
//
// File ops on save:
//
//  1. open target with O_TRUNC|O_WRONLY|O_CREATE
//  2. write new content
//  3. close
//
// fsnotify event sequence (directory watch):
//
//	WRITE   target  (one or more, depending on flush boundary; the
//	                 leading O_TRUNC may also surface as a CHMOD on
//	                 some platforms)
//
// This is the simplest possible save path — no rename, no temp file.
// The watcher MUST observe at least one event resolving to target;
// after debounce the pending path-set is {target}.
//
// Usage: go run save_truncate.go <target-file> <new-content>
//
// Pure-Go program; runs on every platform Go itself supports.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: save_truncate <target> <content>")
		os.Exit(2)
	}
	target, content := os.Args[1], os.Args[2]

	f, err := os.OpenFile(target, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open: %v\n", err)
		os.Exit(1)
	}
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		os.Exit(1)
	}
	if err := f.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "close: %v\n", err)
		os.Exit(1)
	}
}
