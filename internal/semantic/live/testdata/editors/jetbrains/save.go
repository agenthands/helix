// Command save reproduces JetBrains IDE "safe write" save sequence
// verbatim. Source-of-truth: 60-RESEARCH.md "Editor Save-Pattern
// Dossier — JetBrains — ___jb_tmp___ + ___jb_old___ rename".
//
// File ops on save:
//
//  1. write new content to ${target}___jb_tmp___
//  2. rename original ${target} → ${target}___jb_old___ (if target exists)
//  3. rename ${target}___jb_tmp___ → ${target}
//  4. remove ${target}___jb_old___
//
// fsnotify event sequence (directory watch):
//
//	CREATE  ${target}___jb_tmp___
//	RENAME  ${target}                 (original → ___jb_old___)
//	CREATE  ${target}___jb_old___
//	RENAME  ${target}___jb_tmp___ → ${target}
//	REMOVE  ${target}___jb_old___
//
// The watcher MUST filter the ___jb_tmp___ and ___jb_old___ suffixes
// (handleEvent in watcher.go) so the resulting pending path-set ends
// up {target} after debounce — NOT {target, ${target}___jb_tmp___,
// ${target}___jb_old___}.
//
// Usage: go run save.go <target-file> <new-content>
//
// Pure-Go program; runs on every platform Go itself supports.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: save <target> <content>")
		os.Exit(2)
	}
	target, content := os.Args[1], os.Args[2]
	tmp := target + "___jb_tmp___"
	old := target + "___jb_old___"

	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write tmp: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, old); err != nil {
			fmt.Fprintf(os.Stderr, "rename target->old: %v\n", err)
			os.Exit(1)
		}
	}
	if err := os.Rename(tmp, target); err != nil {
		fmt.Fprintf(os.Stderr, "rename tmp->target: %v\n", err)
		os.Exit(1)
	}
	// Best-effort cleanup; missing file is fine (target didn't exist).
	_ = os.Remove(old)
}
