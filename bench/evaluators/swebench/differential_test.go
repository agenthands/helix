package swebench

import (
	"math"
	"testing"
)

const goldPatch = `diff --git a/sympy/core/sympify.py b/sympy/core/sympify.py
index 1111..2222 100644
--- a/sympy/core/sympify.py
+++ b/sympy/core/sympify.py
@@ -1,3 +1,4 @@
-old
+new
diff --git a/sympy/core/basic.py b/sympy/core/basic.py
index 3333..4444 100644
--- a/sympy/core/basic.py
+++ b/sympy/core/basic.py
@@ -10,2 +10,3 @@
+more
`

// TestDifferential exercises the gold-vs-agent diff-overlap signal (SC#4): a pure
// text transform over the `diff --git a/ b/` headers of two unified diffs. Empty/
// nil gold is an undefined denominator → (nil, *MetricError), never a fabricated
// 0 or 1 (clone of regression_checker's null+MetricError discipline).
func TestDifferential(t *testing.T) {
	approx := func(got *float64, want float64) bool {
		return got != nil && math.Abs(*got-want) < 1e-9
	}

	t.Run("full overlap -> 1.0", func(t *testing.T) {
		ov, me := DiffOverlap(goldPatch, goldPatch)
		if me != nil {
			t.Fatalf("unexpected MetricError: %+v", me)
		}
		if !approx(ov, 1.0) {
			t.Fatalf("overlap = %v, want 1.0", ov)
		}
	})

	t.Run("partial overlap = |gold ∩ agent| / |gold|", func(t *testing.T) {
		// agent touches one gold file + one extra (extra does NOT inflate the rate).
		agent := `diff --git a/sympy/core/sympify.py b/sympy/core/sympify.py
@@ -1,1 +1,1 @@
-x
+y
diff --git a/sympy/unrelated/extra.py b/sympy/unrelated/extra.py
@@ -1,1 +1,1 @@
-p
+q
`
		ov, me := DiffOverlap(goldPatch, agent)
		if me != nil {
			t.Fatalf("unexpected MetricError: %+v", me)
		}
		// gold = {sympify.py, basic.py} (2); intersection = {sympify.py} (1) → 0.5.
		if !approx(ov, 0.5) {
			t.Fatalf("overlap = %v, want 0.5", ov)
		}
	})

	t.Run("zero overlap -> 0.0 (not a MetricError)", func(t *testing.T) {
		agent := `diff --git a/other/x.py b/other/x.py
@@ -1,1 +1,1 @@
-a
+b
`
		ov, me := DiffOverlap(goldPatch, agent)
		if me != nil {
			t.Fatalf("unexpected MetricError: %+v", me)
		}
		if !approx(ov, 0.0) {
			t.Fatalf("overlap = %v, want 0.0", ov)
		}
	})

	t.Run("empty gold -> nil + MetricError (no fabricated 0/1)", func(t *testing.T) {
		ov, me := DiffOverlap("", goldPatch)
		if ov != nil {
			t.Fatalf("overlap = %v, want nil on empty gold", ov)
		}
		if me == nil {
			t.Fatal("want MetricError on empty gold (undefined denominator), got nil")
		}
		if me.Grader != "swebench_differential" {
			t.Errorf("grader = %q, want swebench_differential", me.Grader)
		}
		if me.Metric != "diff_overlap" {
			t.Errorf("metric = %q, want diff_overlap", me.Metric)
		}
	})

	t.Run("gold with no diff --git headers -> nil + MetricError", func(t *testing.T) {
		ov, me := DiffOverlap("not a patch at all\njust text\n", goldPatch)
		if ov != nil || me == nil {
			t.Fatalf("want (nil, MetricError) for headerless gold, got (%v, %+v)", ov, me)
		}
	})
}
