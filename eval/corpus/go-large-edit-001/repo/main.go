package main

import (
	"fmt"
	"strings"
)

// ReportEntry holds a single record to include in the report.
type ReportEntry struct {
	Name  string
	Value int
}

// BuildReport constructs a plain-text report from the given entries.
// Rewrite this function body to produce a JSON report instead.
func BuildReport(entries []ReportEntry) string {
	var sb strings.Builder
	sb.WriteString("=== Report ===\n")
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("  %s: %d\n", e.Name, e.Value))
	}
	sb.WriteString("=== End Report ===\n")
	return sb.String()
}

func main() {
	entries := []ReportEntry{
		{Name: "requests", Value: 1024},
		{Name: "errors", Value: 3},
		{Name: "latency_ms", Value: 42},
	}
	fmt.Print(BuildReport(entries))
}
