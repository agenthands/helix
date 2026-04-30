package upgrade

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProbeWritable returns nil iff the directory containing installPath is
// writable by the current user. Implementation creates and immediately
// removes a temp file via `os.CreateTemp`, which avoids the stat-time
// TOCTOU window (a stat-only check could lie about effective permissions
// when ACLs, mount options, or capability filters are in play).
//
// Called BEFORE any network I/O per CONTEXT.md D-09 — failing fast on an
// unwritable install directory saves a multi-MB download and lets the
// upgrade flow surface the SudoHint message before the user invests
// time in a download that cannot land.
func ProbeWritable(installPath string) error {
	dir := filepath.Dir(installPath)
	f, err := os.CreateTemp(dir, ".helix-perm-probe-*")
	if err != nil {
		return fmt.Errorf("install path %s not writable by current user: %w", installPath, err)
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return nil
}

// SudoHint formats the actionable re-invocation message printed by the
// upgrade subcommand when ProbeWritable fails. The message tells the user
// where the binary lives and how to re-run with elevated privileges.
//
// Per CONTEXT.md D-09 the upgrade flow does NOT prompt for sudo
// internally — the user must explicitly retype the command. The args
// parameter typically receives `os.Args[2:]` so the hint preserves the
// flag set the user originally invoked (e.g., `--prerelease --version vX`).
func SudoHint(installPath string, args []string) string {
	return fmt.Sprintf(
		"helix is installed at %s and your user cannot write there.\nRe-run with elevated privileges, e.g.:\n  sudo helix upgrade %s",
		installPath, strings.Join(args, " "),
	)
}
