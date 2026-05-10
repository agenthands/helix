package budget

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBudgetDefaults verifies the D-08 default budget values.
func TestBudgetDefaults(t *testing.T) {
	assert.Equal(t, 200000, Default.MaxInputTokens)
	assert.Equal(t, 32000, Default.MaxOutputTokens)
	assert.Equal(t, 300, Default.MaxSeconds)
	assert.Equal(t, 100, Default.MaxToolCalls)
}

// TestBudgetParseYAML verifies that LoadFromFile reads per-task overrides and
// falls back to defaults for unset keys. Unknown keys cause an error.
func TestBudgetParseYAML(t *testing.T) {
	t.Run("partial override", func(t *testing.T) {
		yaml := `
max_input_tokens: 50000
max_seconds: 60
`
		path := writeYAML(t, yaml)
		b, err := LoadFromFile(path)
		require.NoError(t, err)

		assert.Equal(t, 50000, b.MaxInputTokens, "max_input_tokens should be overridden")
		assert.Equal(t, Default.MaxOutputTokens, b.MaxOutputTokens, "max_output_tokens should fallback to default")
		assert.Equal(t, 60, b.MaxSeconds, "max_seconds should be overridden")
		assert.Equal(t, Default.MaxToolCalls, b.MaxToolCalls, "max_tool_calls should fallback to default")
	})

	t.Run("unknown key causes error", func(t *testing.T) {
		yaml := `
max_input_tokens: 50000
invalid_key: oops
`
		path := writeYAML(t, yaml)
		_, err := LoadFromFile(path)
		assert.Error(t, err, "unknown YAML keys should cause an error")
	})

	t.Run("empty file uses defaults", func(t *testing.T) {
		path := writeYAML(t, "")
		b, err := LoadFromFile(path)
		require.NoError(t, err)
		assert.Equal(t, Default, b)
	})
}

// TestWatchdogSecondsBreach verifies that the watchdog cancels after MaxSeconds.
func TestWatchdogSecondsBreach(t *testing.T) {
	b := Budget{
		MaxInputTokens:  Default.MaxInputTokens,
		MaxOutputTokens: Default.MaxOutputTokens,
		MaxSeconds:      1, // 1 second limit
		MaxToolCalls:    Default.MaxToolCalls,
	}

	ctx := context.Background()
	wdog, cancel := NewWatchdog(ctx, b)
	defer cancel()

	select {
	case <-wdog.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("watchdog did not fire within 3s")
	}

	br := wdog.Breach()
	require.NotNil(t, br, "breach reason must be set")
	assert.Equal(t, "seconds", br.Axis)
	assert.Equal(t, int64(1), br.Limit)
	assert.GreaterOrEqual(t, br.Observed, int64(1), "observed must be >= 1")
}

// TestWatchdogToolCallsBreach verifies that exceeding MaxToolCalls triggers cancel.
func TestWatchdogToolCallsBreach(t *testing.T) {
	b := Budget{
		MaxInputTokens:  Default.MaxInputTokens,
		MaxOutputTokens: Default.MaxOutputTokens,
		MaxSeconds:      300,
		MaxToolCalls:    3,
	}

	ctx := context.Background()
	wdog, cancel := NewWatchdog(ctx, b)
	defer cancel()

	wdog.RecordToolCall() // 1
	wdog.RecordToolCall() // 2
	wdog.RecordToolCall() // 3 — should breach at 4th
	wdog.RecordToolCall() // 4 = breach

	select {
	case <-wdog.Done():
	case <-time.After(500 * time.Millisecond):
		t.Fatal("tool_calls breach did not fire")
	}

	br := wdog.Breach()
	require.NotNil(t, br, "breach reason must be set")
	assert.Equal(t, "tool_calls", br.Axis)
	assert.Equal(t, int64(3), br.Limit)
}

// TestWatchdogTokensBreach verifies that exceeding MaxInputTokens or MaxOutputTokens triggers cancel.
func TestWatchdogTokensBreach(t *testing.T) {
	t.Run("input_tokens breach", func(t *testing.T) {
		b := Budget{MaxInputTokens: 100, MaxOutputTokens: 99999, MaxSeconds: 300, MaxToolCalls: 999}
		ctx := context.Background()
		wdog, cancel := NewWatchdog(ctx, b)
		defer cancel()

		wdog.RecordTokens(101, 0)

		select {
		case <-wdog.Done():
		case <-time.After(500 * time.Millisecond):
			t.Fatal("input_tokens breach did not fire")
		}

		br := wdog.Breach()
		require.NotNil(t, br)
		assert.Equal(t, "input_tokens", br.Axis)
		assert.Equal(t, int64(100), br.Limit)
		assert.Equal(t, int64(101), br.Observed)
	})

	t.Run("output_tokens breach", func(t *testing.T) {
		b := Budget{MaxInputTokens: 99999, MaxOutputTokens: 50, MaxSeconds: 300, MaxToolCalls: 999}
		ctx := context.Background()
		wdog, cancel := NewWatchdog(ctx, b)
		defer cancel()

		wdog.RecordTokens(0, 51)

		select {
		case <-wdog.Done():
		case <-time.After(500 * time.Millisecond):
			t.Fatal("output_tokens breach did not fire")
		}

		br := wdog.Breach()
		require.NotNil(t, br)
		assert.Equal(t, "output_tokens", br.Axis)
		assert.Equal(t, int64(50), br.Limit)
		assert.Equal(t, int64(51), br.Observed)
	})
}

// TestWatchdogReasonStringConstruction verifies that the watchdog populates
// the BreachReason (type from Plan 01) with the correct Axis value on breach.
// BreachReason.String() is tested in the types_test.go of Plan 01.
func TestWatchdogReasonStringConstruction(t *testing.T) {
	axes := []struct {
		name  string
		setup func(*Watchdog)
		want  string
	}{
		{
			"seconds",
			func(w *Watchdog) { /* timer fires on its own after MaxSeconds=1 */ },
			"seconds",
		},
		{
			"tool_calls",
			func(w *Watchdog) {
				for i := 0; i < 4; i++ {
					w.RecordToolCall()
				}
			},
			"tool_calls",
		},
	}

	for _, tc := range axes {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var b Budget
			switch tc.name {
			case "seconds":
				b = Budget{MaxInputTokens: 999999, MaxOutputTokens: 999999, MaxSeconds: 1, MaxToolCalls: 999}
			case "tool_calls":
				b = Budget{MaxInputTokens: 999999, MaxOutputTokens: 999999, MaxSeconds: 300, MaxToolCalls: 3}
			}

			ctx := context.Background()
			wdog, cancel := NewWatchdog(ctx, b)
			defer cancel()

			tc.setup(wdog)

			select {
			case <-wdog.Done():
			case <-time.After(3 * time.Second):
				t.Fatal("breach did not fire")
			}

			br := wdog.Breach()
			require.NotNil(t, br)
			assert.Equal(t, tc.want, br.Axis)
			assert.Equal(t, "failed-with-cause: budget_"+tc.want, br.String())
		})
	}
}

// ---- helpers ----

func writeYAML(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "budget.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}
