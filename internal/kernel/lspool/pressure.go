package lspool

// PressureLevel represents the system memory pressure level.
type PressureLevel int

const (
	// PressureNone indicates no memory pressure.
	PressureNone PressureLevel = iota
	// PressureLow indicates low memory pressure.
	PressureLow
	// PressureMedium indicates medium memory pressure.
	PressureMedium
	// PressureHigh indicates high memory pressure -- eviction should start.
	PressureHigh
	// PressureCritical indicates critical memory pressure -- aggressive eviction.
	PressureCritical
)

// String returns the string representation of the pressure level.
func (p PressureLevel) String() string {
	switch p {
	case PressureNone:
		return "none"
	case PressureLow:
		return "low"
	case PressureMedium:
		return "medium"
	case PressureHigh:
		return "high"
	case PressureCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// MemoryPressure provides system memory pressure information.
type MemoryPressure interface {
	// Level returns the current system memory pressure level.
	Level() PressureLevel
	// WorkerRSS returns the resident set size in bytes for the given process ID.
	WorkerRSS(pid int) (uint64, error)
}
