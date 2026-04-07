package lspool

import "os"

// pid returns the current process ID.
func pid() int {
	return os.Getpid()
}
