//go:build windows

package upgrade

import "os"

// chmod is a no-op on Windows because Windows ACLs make POSIX chmod
// modes ineffective for the file owner. The unwritable-dir test in
// permission_test.go skips on Windows, so this stub is only here to
// satisfy the build-tag pair invariant (every signature in the unix
// file must have a windows counterpart).
func chmod(path string, mode os.FileMode) error {
	_ = path
	_ = mode
	return nil
}
