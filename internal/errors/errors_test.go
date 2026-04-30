package errors_test

import (
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	serr "github.com/agenthands/helix/internal/errors"
)

func TestKinds(t *testing.T) {
	tests := []struct {
		kind serr.Kind
		str  string
	}{
		{serr.NotFound, "not_found"},
		{serr.InvalidArgs, "invalid_args"},
		{serr.NoWorkspace, "no_workspace"},
		{serr.Unsupported, "unsupported"},
		{serr.Internal, "internal"},
		{serr.CircuitOpen, "circuit_open"},
		{serr.Timeout, "timeout"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.str, string(tt.kind), "Kind %q should have string value %q", tt.kind, tt.str)
	}
}

func TestNew(t *testing.T) {
	err := serr.New(serr.NotFound, "msg")
	require.NotNil(t, err)
	assert.Equal(t, serr.NotFound, err.Kind)
	assert.Equal(t, "msg", err.Message)
	assert.Empty(t, err.Tool)
	assert.Empty(t, err.Detail)
}

func TestWrap(t *testing.T) {
	cause := io.EOF
	err := serr.Wrap(serr.Internal, "msg", cause)
	require.NotNil(t, err)
	assert.Equal(t, serr.Internal, err.Kind)
	assert.Equal(t, "msg", err.Message)
	assert.Equal(t, cause, err.Unwrap())
}

func TestWithTool(t *testing.T) {
	err := serr.New(serr.NotFound, "x").WithTool("find_references")
	assert.Equal(t, "find_references", err.Tool)
}

func TestWithDetail(t *testing.T) {
	err := serr.New(serr.NotFound, "x").WithDetail("detail text")
	assert.Equal(t, "detail text", err.Detail)
}

func TestErrorString(t *testing.T) {
	t.Run("without detail", func(t *testing.T) {
		err := serr.New(serr.NotFound, "msg")
		assert.Equal(t, "not_found: msg", err.Error())
	})
	t.Run("with detail", func(t *testing.T) {
		err := serr.New(serr.NotFound, "msg").WithDetail("detail")
		assert.Equal(t, "not_found: msg (detail)", err.Error())
	})
}

func TestIsKindMatch(t *testing.T) {
	err := serr.New(serr.NotFound, "x")
	assert.ErrorIs(t, err, serr.ErrNotFound)
}

func TestIsKindMismatch(t *testing.T) {
	err := serr.New(serr.NotFound, "x")
	assert.NotErrorIs(t, err, serr.ErrTimeout)
}

func TestIsChainWrapped(t *testing.T) {
	inner := serr.New(serr.NotFound, "x")
	outer := fmt.Errorf("outer: %w", inner)
	assert.ErrorIs(t, outer, serr.ErrNotFound)
}

func TestIsWrappedCause(t *testing.T) {
	err := serr.Wrap(serr.Internal, "msg", io.EOF)
	assert.ErrorIs(t, err, io.EOF)
}

func TestAs(t *testing.T) {
	inner := serr.New(serr.NotFound, "x").WithTool("t")
	outer := fmt.Errorf("wrap: %w", inner)

	var target *serr.Error
	require.ErrorAs(t, outer, &target)
	assert.Equal(t, "t", target.Tool)
	assert.Equal(t, serr.NotFound, target.Kind)
}

func TestMarshalJSON(t *testing.T) {
	t.Run("all fields", func(t *testing.T) {
		err := serr.New(serr.NotFound, "msg").WithTool("t").WithDetail("d")
		data, jsonErr := json.Marshal(err)
		require.NoError(t, jsonErr)

		var m map[string]string
		require.NoError(t, json.Unmarshal(data, &m))
		assert.Equal(t, "not_found", m["kind"])
		assert.Equal(t, "msg", m["message"])
		assert.Equal(t, "t", m["tool"])
		assert.Equal(t, "d", m["detail"])
	})
	t.Run("omit empty", func(t *testing.T) {
		err := serr.New(serr.Internal, "msg")
		data, jsonErr := json.Marshal(err)
		require.NoError(t, jsonErr)

		var m map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &m))
		_, hasTool := m["tool"]
		_, hasDetail := m["detail"]
		assert.False(t, hasTool, "empty tool should be omitted")
		assert.False(t, hasDetail, "empty detail should be omitted")
	})
}

func TestNilError(t *testing.T) {
	// Ensure that returning nil directly from a function with error return
	// does not produce a false positive when using *Error.
	fn := func() error {
		// CORRECT: return nil directly, NOT a typed nil *Error.
		return nil
	}
	assert.NoError(t, fn())
}

func TestSentinels(t *testing.T) {
	sentinels := []struct {
		sentinel *serr.Error
		kind     serr.Kind
	}{
		{serr.ErrNotFound, serr.NotFound},
		{serr.ErrInvalidArgs, serr.InvalidArgs},
		{serr.ErrNoWorkspace, serr.NoWorkspace},
		{serr.ErrUnsupported, serr.Unsupported},
		{serr.ErrInternal, serr.Internal},
		{serr.ErrCircuitOpen, serr.CircuitOpen},
		{serr.ErrTimeout, serr.Timeout},
	}
	for _, s := range sentinels {
		require.NotNil(t, s.sentinel, "sentinel for %s should not be nil", s.kind)
		assert.Equal(t, s.kind, s.sentinel.Kind, "sentinel Kind mismatch for %s", s.kind)
	}
}

func TestBuilderChaining(t *testing.T) {
	err := serr.New(serr.NotFound, "msg").WithTool("tool1").WithDetail("detail1")
	assert.Equal(t, "tool1", err.Tool)
	assert.Equal(t, "detail1", err.Detail)
	assert.Equal(t, serr.NotFound, err.Kind)
	assert.Equal(t, "msg", err.Message)
}
