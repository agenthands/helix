package health_test

import (
	"testing"

	"github.com/agenthands/helix/internal/kernel/health"
	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/stretchr/testify/assert"
)

func makeReport(workers ...lspool.WorkerHealth) *lspool.HealthReport {
	ws := lspool.WorkspaceHealth{
		Root:      "/tmp/test-project",
		Languages: []string{"go"},
		Workers:   workers,
		Circuits: []lspool.CircuitHealth{
			{Language: "go", State: "closed", Failures: 0},
		},
	}
	return &lspool.HealthReport{
		Workspaces: []lspool.WorkspaceHealth{ws},
	}
}

func healthyWorker(id string) lspool.WorkerHealth {
	return lspool.WorkerHealth{
		ID:       id,
		Language: "go",
		WorkDir:  "/tmp/test-project",
		Command:  "gopls",
		State:    "healthy",
	}
}

func failedWorker(id string) lspool.WorkerHealth {
	return lspool.WorkerHealth{
		ID:       id,
		Language: "go",
		WorkDir:  "/tmp/test-project",
		Command:  "gopls",
		State:    "failed",
	}
}

func degradedWorker(id string) lspool.WorkerHealth {
	return lspool.WorkerHealth{
		ID:       id,
		Language: "go",
		WorkDir:  "/tmp/test-project",
		Command:  "gopls",
		State:    "degraded",
	}
}

func TestFilterReport_VerboseShowsAll(t *testing.T) {
	report := makeReport(healthyWorker("w-1"), healthyWorker("w-2"))

	health.FilterReport(report, true)

	assert.Len(t, report.Workspaces[0].Workers, 2)
	assert.Empty(t, report.Summary)
}

func TestFilterReport_DefaultHidesHealthy(t *testing.T) {
	report := makeReport(healthyWorker("w-1"), healthyWorker("w-2"), failedWorker("w-3"))

	health.FilterReport(report, false)

	// Only the failed worker should remain.
	assert.Len(t, report.Workspaces[0].Workers, 1)
	assert.Equal(t, "w-3", report.Workspaces[0].Workers[0].ID)
	assert.Equal(t, "failed", report.Workspaces[0].Workers[0].State)
	assert.Empty(t, report.Summary) // Not all healthy.
}

func TestFilterReport_AllHealthySummary(t *testing.T) {
	report := makeReport(healthyWorker("w-1"), healthyWorker("w-2"), healthyWorker("w-3"))

	health.FilterReport(report, false)

	assert.Empty(t, report.Workspaces[0].Workers)
	assert.Equal(t, "All 3 language servers healthy", report.Summary)
}

func TestFilterReport_DegradedShown(t *testing.T) {
	report := makeReport(healthyWorker("w-1"), degradedWorker("w-2"))

	health.FilterReport(report, false)

	assert.Len(t, report.Workspaces[0].Workers, 1)
	assert.Equal(t, "w-2", report.Workspaces[0].Workers[0].ID)
	assert.Equal(t, "degraded", report.Workspaces[0].Workers[0].State)
	assert.Empty(t, report.Summary) // Not all healthy.
}

func TestFilterReport_NonClosedCircuitsShown(t *testing.T) {
	report := &lspool.HealthReport{
		Workspaces: []lspool.WorkspaceHealth{
			{
				Root:      "/tmp/test-project",
				Languages: []string{"go"},
				Workers:   []lspool.WorkerHealth{healthyWorker("w-1")},
				Circuits: []lspool.CircuitHealth{
					{Language: "go", State: "closed", Failures: 0},
					{Language: "python", State: "open", Failures: 3},
				},
			},
		},
	}

	health.FilterReport(report, false)

	// Closed circuit should be filtered out, open should remain.
	assert.Len(t, report.Workspaces[0].Circuits, 1)
	assert.Equal(t, "open", report.Workspaces[0].Circuits[0].State)
}
