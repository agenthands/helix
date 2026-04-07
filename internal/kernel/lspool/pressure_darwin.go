//go:build darwin

package lspool

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// DarwinMemoryPressure detects memory pressure on macOS.
// Uses vm_stat and ps for a CGO-free approach with graceful degradation.
type DarwinMemoryPressure struct{}

// NewDarwinMemoryPressure creates a macOS-specific memory pressure detector.
func NewDarwinMemoryPressure() *DarwinMemoryPressure {
	return &DarwinMemoryPressure{}
}

// Level returns the current memory pressure level on macOS.
// Parses vm_stat to compute the ratio of free+inactive pages to total pages.
func (dp *DarwinMemoryPressure) Level() PressureLevel {
	out, err := exec.Command("vm_stat").Output()
	if err != nil {
		return PressureNone // Degrade gracefully.
	}

	stats := parseVMStat(string(out))
	free := stats["Pages free"]
	inactive := stats["Pages inactive"]
	speculative := stats["Pages speculative"]
	wiredDown := stats["Pages wired down"]
	active := stats["Pages active"]
	compressed := stats["Pages occupied by compressor"]

	total := free + inactive + speculative + wiredDown + active + compressed
	if total == 0 {
		return PressureNone
	}

	available := free + inactive + speculative
	ratio := float64(available) / float64(total)

	switch {
	case ratio < 0.05:
		return PressureCritical
	case ratio < 0.10:
		return PressureHigh
	case ratio < 0.20:
		return PressureMedium
	case ratio < 0.35:
		return PressureLow
	default:
		return PressureNone
	}
}

// WorkerRSS returns the resident set size in bytes for a given process ID.
// Uses `ps -o rss= -p {pid}` for a CGO-free approach.
func (dp *DarwinMemoryPressure) WorkerRSS(pid int) (uint64, error) {
	out, err := exec.Command("ps", "-o", "rss=", "-p", fmt.Sprintf("%d", pid)).Output()
	if err != nil {
		return 0, fmt.Errorf("reading RSS for pid %d: %w", pid, err)
	}

	rssStr := strings.TrimSpace(string(out))
	if rssStr == "" {
		return 0, fmt.Errorf("no RSS output for pid %d", pid)
	}

	kb, err := strconv.ParseUint(rssStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing RSS for pid %d: %w", pid, err)
	}

	return kb * 1024, nil // Convert kB to bytes.
}

// parseVMStat parses macOS vm_stat output into a map of stat names to page counts.
func parseVMStat(output string) map[string]uint64 {
	result := make(map[string]uint64)
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		valStr := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(parts[1]), "."))
		val, err := strconv.ParseUint(valStr, 10, 64)
		if err != nil {
			continue
		}
		result[name] = val
	}
	return result
}
