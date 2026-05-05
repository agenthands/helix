// Command save_atomic reproduces VS Code's atomic-rename save mode
// (the path triggered when the user sets `files.atomicSave: true`,
// or by remote-FS extensions). Source-of-truth: 60-RESEARCH.md
// "Editor Save-Pattern Dossier — VS Code — atomic-rename".
//
// File ops on save:
//
//  1. write new content to a sibling temp file (.vsctmp~XXXXXX)
//  2. rename the temp file → target
//
// fsnotify event sequence (directory watch):
//
//	CREATE  .vsctmp~XXXXXX
//	WRITE   .vsctmp~XXXXXX  (one or more, depending on flush boundary)
//	RENAME  .vsctmp~XXXXXX → target
//
// The directory-level watch survives the rename (CONTEXT D-01); the
// final pending path-set is {target} after debounce. This is the
// canonical "Pitfall 1" mitigation evidence — watching the directory
// rather than the file.
//
// Usage: go run save_atomic.go <target-file> <new-content>
//
// Pure-Go program; runs on every platform Go itself supports.
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: save_atomic <target> <content>")
		os.Exit(2)
	}
	target, content := os.Args[1], os.Args[2]

	tmp, err := os.CreateTemp(filepath.Dir(target), ".vsctmp~*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create temp: %v\n", err)
		os.Exit(1)
	}
	tmpName := tmp.Name()

	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		os.Exit(1)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		fmt.Fprintf(os.Stderr, "close: %v\n", err)
		os.Exit(1)
	}

	if err := os.Rename(tmpName, target); err != nil {
		_ = os.Remove(tmpName)
		fmt.Fprintf(os.Stderr, "rename: %v\n", err)
		os.Exit(1)
	}
}
