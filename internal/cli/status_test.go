package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/postfix/serena/internal/kernel/lspool"
)

func TestStatusPrinter_PrintReport_AllHealthy(t *testing.T) {
	var buf bytes.Buffer
	printer := &StatusPrinter{Writer: &buf}

	report := &lspool.HealthReport{
		Summary: "All 2 language servers healthy",
	}

	printer.PrintReport(report, false)

	assert.Contains(t, buf.String(), "All 2 language servers healthy")
}

func TestStatusPrinter_PrintReport_AllHealthy_VerboseIgnoresSummary(t *testing.T) {
	var buf bytes.Buffer
	printer := &StatusPrinter{Writer: &buf}

	report := &lspool.HealthReport{
		Summary: "All 2 language servers healthy",
		Workspaces: []lspool.WorkspaceHealth{
			{
				Root:      "/tmp/project",
				Languages: []string{"go", "python"},
				Workers: []lspool.WorkerHealth{
					{Language: "go", Command: "gopls", State: "healthy", Capabilities: []string{"hover", "definition"}},
					{Language: "python", Command: "pylsp", State: "healthy", Capabilities: []string{"hover"}},
				},
			},
		},
	}

	printer.PrintReport(report, true)

	output := buf.String()
	assert.Contains(t, output, "Workspace: /tmp/project")
	assert.Contains(t, output, "Languages: go, python")
	assert.Contains(t, output, "go (gopls)")
	assert.Contains(t, output, "python (pylsp)")
	assert.NotContains(t, output, "All 2 language servers healthy")
}

func TestStatusPrinter_PrintReport_WithFailures(t *testing.T) {
	var buf bytes.Buffer
	printer := &StatusPrinter{Writer: &buf}

	report := &lspool.HealthReport{
		Workspaces: []lspool.WorkspaceHealth{
			{
				Root: "/tmp/project",
				Workers: []lspool.WorkerHealth{
					{Language: "python", Command: "pylsp", State: "failed"},
				},
			},
		},
	}

	printer.PrintReport(report, false)

	output := buf.String()
	assert.Contains(t, output, "python (pylsp) failed")
}

func TestStatusPrinter_PrintReport_WithDegraded(t *testing.T) {
	var buf bytes.Buffer
	printer := &StatusPrinter{Writer: &buf}

	report := &lspool.HealthReport{
		Workspaces: []lspool.WorkspaceHealth{
			{
				Root: "/tmp/project",
				Workers: []lspool.WorkerHealth{
					{Language: "rust", Command: "rust-analyzer", State: "degraded"},
				},
			},
		},
	}

	printer.PrintReport(report, false)

	output := buf.String()
	assert.Contains(t, output, "rust (rust-analyzer) degraded")
}

func TestStatusPrinter_PrintReport_CircuitBreaker(t *testing.T) {
	var buf bytes.Buffer
	printer := &StatusPrinter{Writer: &buf}

	report := &lspool.HealthReport{
		Workspaces: []lspool.WorkspaceHealth{
			{
				Root: "/tmp/project",
				Circuits: []lspool.CircuitHealth{
					{Language: "java", State: "open", Failures: 5},
				},
			},
		},
	}

	printer.PrintReport(report, false)

	output := buf.String()
	assert.Contains(t, output, "java circuit breaker open (5 failures)")
}

func TestStatusPrinter_HasFailures_True_FailedWorker(t *testing.T) {
	printer := &StatusPrinter{}

	report := &lspool.HealthReport{
		Workspaces: []lspool.WorkspaceHealth{
			{
				Workers: []lspool.WorkerHealth{
					{State: "healthy"},
					{State: "failed"},
				},
			},
		},
	}

	assert.True(t, printer.HasFailures(report))
}

func TestStatusPrinter_HasFailures_True_OpenCircuit(t *testing.T) {
	printer := &StatusPrinter{}

	report := &lspool.HealthReport{
		Workspaces: []lspool.WorkspaceHealth{
			{
				Workers: []lspool.WorkerHealth{
					{State: "healthy"},
				},
				Circuits: []lspool.CircuitHealth{
					{State: "open", Failures: 3},
				},
			},
		},
	}

	assert.True(t, printer.HasFailures(report))
}

func TestStatusPrinter_HasFailures_False(t *testing.T) {
	printer := &StatusPrinter{}

	report := &lspool.HealthReport{
		Workspaces: []lspool.WorkspaceHealth{
			{
				Workers: []lspool.WorkerHealth{
					{State: "healthy"},
					{State: "degraded"},
				},
				Circuits: []lspool.CircuitHealth{
					{State: "closed"},
					{State: "half_open"},
				},
			},
		},
	}

	assert.False(t, printer.HasFailures(report))
}

func TestStatusPrinter_HasFailures_Empty(t *testing.T) {
	printer := &StatusPrinter{}

	report := &lspool.HealthReport{}
	assert.False(t, printer.HasFailures(report))
}

func TestNewStatusCommand_Flags(t *testing.T) {
	cmd := newStatusCommand()

	assert.Equal(t, "status", cmd.Use)
	assert.NotNil(t, cmd.RunE)

	jsonFlag := cmd.Flags().Lookup("json")
	assert.NotNil(t, jsonFlag, "json flag should exist")
	assert.Equal(t, "false", jsonFlag.DefValue)

	verboseFlag := cmd.Flags().Lookup("verbose")
	assert.NotNil(t, verboseFlag, "verbose flag should exist")
	assert.Equal(t, "false", verboseFlag.DefValue)
	assert.Equal(t, "v", verboseFlag.Shorthand)
}
