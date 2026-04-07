//go:build linux

package lspool

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// LinuxMemoryPressure reads memory pressure from Linux PSI and /proc.
type LinuxMemoryPressure struct{}

// NewLinuxMemoryPressure creates a Linux-specific memory pressure detector.
func NewLinuxMemoryPressure() *LinuxMemoryPressure {
	return &LinuxMemoryPressure{}
}

// Level reads /proc/pressure/memory and maps PSI avg10 to a PressureLevel.
func (lp *LinuxMemoryPressure) Level() PressureLevel {
	data, err := os.ReadFile("/proc/pressure/memory")
	if err != nil {
		return PressureNone // Degrade gracefully if PSI not available.
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, "some ") {
			continue
		}
		// Parse: some avg10=X.XX avg60=X.XX avg300=X.XX total=XXXXX
		parts := strings.Fields(line)
		for _, part := range parts {
			if !strings.HasPrefix(part, "avg10=") {
				continue
			}
			valStr := strings.TrimPrefix(part, "avg10=")
			val, err := strconv.ParseFloat(valStr, 64)
			if err != nil {
				return PressureNone
			}
			switch {
			case val >= 50.0:
				return PressureCritical
			case val >= 25.0:
				return PressureHigh
			case val >= 10.0:
				return PressureMedium
			case val >= 2.0:
				return PressureLow
			default:
				return PressureNone
			}
		}
	}
	return PressureNone
}

// WorkerRSS reads the VmRSS field from /proc/{pid}/status.
func (lp *LinuxMemoryPressure) WorkerRSS(pid int) (uint64, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, fmt.Errorf("reading proc status for pid %d: %w", pid, err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, fmt.Errorf("unexpected VmRSS format: %s", line)
		}
		kb, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parsing VmRSS: %w", err)
		}
		return kb * 1024, nil // Convert kB to bytes.
	}
	return 0, fmt.Errorf("VmRSS not found for pid %d", pid)
}
