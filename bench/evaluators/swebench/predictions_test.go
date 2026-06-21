package swebench

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestPredictions proves the predictions.jsonl producer is a pure, deterministic
// transform: its output is byte-identical to the committed golden, and two calls
// over the SAME (shuffled) input emit identical bytes (Pitfall 4 reproducibility).
func TestPredictions(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("testdata", "predictions.golden.jsonl"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	// Rows are supplied SHUFFLED (not in instance_id order) on purpose: the
	// producer must sort before emit so the output is order-independent.
	rows := []Prediction{
		{InstanceID: "sympy__sympy-20590", ModelNameOrPath: "helix", ModelPatch: "diff --git a/sympy/core/sympify.py b/sympy/core/sympify.py\n@@\n-old\n+new\n"},
		{InstanceID: "astropy__astropy-12345", ModelNameOrPath: "helix", ModelPatch: "diff --git a/astropy/x.py b/astropy/x.py\n@@\n-a\n+b\n"},
		{InstanceID: "django__django-11099", ModelNameOrPath: "helix", ModelPatch: "diff --git a/django/y.py b/django/y.py\n@@\n-c\n+d\n"},
	}

	t.Run("golden byte-match", func(t *testing.T) {
		var buf bytes.Buffer
		if err := WritePredictions(&buf, rows); err != nil {
			t.Fatalf("WritePredictions: %v", err)
		}
		if !bytes.Equal(buf.Bytes(), golden) {
			t.Fatalf("predictions bytes != golden\n--- got ---\n%s\n--- want ---\n%s", buf.Bytes(), golden)
		}
	})

	t.Run("determinism across shuffled input", func(t *testing.T) {
		var a bytes.Buffer
		if err := WritePredictions(&a, rows); err != nil {
			t.Fatalf("WritePredictions a: %v", err)
		}
		// Reverse the input order; output must be identical.
		rev := make([]Prediction, len(rows))
		for i := range rows {
			rev[len(rows)-1-i] = rows[i]
		}
		var b bytes.Buffer
		if err := WritePredictions(&b, rev); err != nil {
			t.Fatalf("WritePredictions b: %v", err)
		}
		if !bytes.Equal(a.Bytes(), b.Bytes()) {
			t.Fatalf("output not order-independent:\n--- a ---\n%s\n--- b ---\n%s", a.Bytes(), b.Bytes())
		}
	})

	t.Run("empty rows emit zero lines", func(t *testing.T) {
		var buf bytes.Buffer
		if err := WritePredictions(&buf, nil); err != nil {
			t.Fatalf("WritePredictions empty: %v", err)
		}
		if buf.Len() != 0 {
			t.Fatalf("empty input must emit zero bytes, got %q", buf.String())
		}
	})
}
