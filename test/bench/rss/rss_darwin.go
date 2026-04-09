//go:build darwin

package rss

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// currentRSS shells out to `ps -o rss= -p <pid>` and returns RSS in bytes.
// ps reports RSS in KiB on darwin; the trailing `=` on `-o rss=` suppresses
// the header so we receive a single integer.
func currentRSS() (uint64, error) {
	pid := os.Getpid()
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, fmt.Errorf("rss: ps -o rss= -p %d: %w", pid, err)
	}
	s := strings.TrimSpace(string(out))
	kib, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("rss: parse ps output %q: %w", s, err)
	}
	return kib * 1024, nil
}
