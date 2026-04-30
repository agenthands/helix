//go:build !windows

package upgrade

import "os"

// chmod is a thin os.Chmod wrapper used by permission_test.go on Unix
// to simulate an unwritable install directory. Windows builds use the
// no-op stub in permission_chmod_windows.go because Windows ACLs make
// chmod(0o555) a no-op for the file owner.
func chmod(path string, mode os.FileMode) error {
	return os.Chmod(path, mode)
}
