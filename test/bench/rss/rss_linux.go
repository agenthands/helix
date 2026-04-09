//go:build linux

package rss

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// currentRSS reads /proc/self/status and returns VmRSS in bytes.
// VmRSS is reported in KiB per the Linux kernel proc(5) man page.
func currentRSS() (uint64, error) {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return 0, fmt.Errorf("rss: malformed VmRSS line: %q", line)
			}
			kib, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0, fmt.Errorf("rss: parse VmRSS value %q: %w", fields[1], err)
			}
			return kib * 1024, nil
		}
	}
	if err := s.Err(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("rss: VmRSS not found in /proc/self/status")
}
