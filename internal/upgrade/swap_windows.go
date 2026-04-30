//go:build windows

package upgrade

import (
	"fmt"
	"os"
	"os/exec"
)

// swap replaces a running .exe on Windows. Windows refuses to rename
// or delete a binary that is currently executing, so the canonical
// dance is:
//  1. Best-effort remove any stale .old leftover from a previous
//     upgrade.
//  2. Rename current → current.old (allowed because nothing has the
//     .old name open, and the move is in-place metadata).
//  3. Rename newPath → current. If this fails, roll back step 2 so
//     the user is never left without a working binary.
//
// The .old file is leaked indefinitely (Pattern 4 + threat T-52-04-13
// `accept` disposition). v1.10 polish may hide it via attrib +H.
func swap(currentPath, newPath string) error {
	oldPath := currentPath + ".old"
	_ = os.Remove(oldPath)
	if err := os.Rename(currentPath, oldPath); err != nil {
		return fmt.Errorf("renaming current to .old: %w", err)
	}
	if err := os.Rename(newPath, currentPath); err != nil {
		// Roll back so the user isn't left without a binary.
		_ = os.Rename(oldPath, currentPath)
		return fmt.Errorf("moving new binary into place: %w", err)
	}
	return nil
}

// relaunch spawns the new binary as a fresh process and exits the
// current process via os.Exit(0). Windows lacks the POSIX exec model,
// so process replacement requires fork-then-exit. Stdio is wired
// through so the child sees the same terminal as the parent —
// forgetting this is Pitfall 5 (Windows users see "broken" output).
func relaunch(binPath string, args, env []string) error {
	// args[0] is the program name; pass the rest as actual args.
	var passArgs []string
	if len(args) > 1 {
		passArgs = args[1:]
	}
	cmd := exec.Command(binPath, passArgs...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		return err
	}
	os.Exit(0)
	return nil // unreachable
}
