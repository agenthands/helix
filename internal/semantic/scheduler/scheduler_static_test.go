// Package scheduler_test exercises invariant gates against
// internal/semantic/scheduler/scheduler.go. The 2026-05-08 update D-07
// requires the body of ScheduleInitialExtraction to remain state-only:
// Phase 65 owns the workspace walk, NOT the scheduler. This file is the
// regression gate that fails the build at test-time if a future commit
// silently reintroduces file-walk logic into the scheduler.
//
// The static test is a pure source-grep: it reads scheduler.go off disk,
// extracts the byte-range of ScheduleInitialExtraction's function body,
// and refuses any of the forbidden tokens documented in
// 59-CONTEXT.md §"2026-05-08 update" D-07.
package scheduler_test

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestScheduleInitialExtraction_BodyRemainsStateOnly is the D-07 invariant
// gate. It re-asserts the 2026-05-04 "STATIC" data-flow finding from
// 59-VERIFICATION.md (line 65). Phase 65 carries the workspace-walk
// responsibility; the scheduler must remain state-only.
func TestScheduleInitialExtraction_BodyRemainsStateOnly(t *testing.T) {
	src, err := os.ReadFile("scheduler.go")
	if err != nil {
		t.Fatalf("read scheduler.go: %v", err)
	}

	sig := []byte("func (s *Scheduler) ScheduleInitialExtraction(")
	idx := bytes.Index(src, sig)
	if idx < 0 {
		t.Fatalf("ScheduleInitialExtraction signature not found in scheduler.go")
	}
	bodyStart := bytes.IndexByte(src[idx:], '{')
	if bodyStart < 0 {
		t.Fatalf("opening brace for ScheduleInitialExtraction not found")
	}
	bodyStart += idx
	depth := 0
	bodyEnd := -1
	for i := bodyStart; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				bodyEnd = i + 1
			}
		}
		if bodyEnd > 0 {
			break
		}
	}
	if bodyEnd < 0 {
		t.Fatalf("closing brace for ScheduleInitialExtraction not found")
	}
	body := string(src[bodyStart:bodyEnd])

	forbidden := []string{
		"os.Open", "os.ReadFile", "os.ReadDir", "os.Stat",
		"filepath.Walk", "filepath.WalkDir", "filepath.Glob",
		"ioutil.ReadFile", "ioutil.ReadDir",
		"provider.Extract", ".Provider(",
	}
	for _, tok := range forbidden {
		if strings.Contains(body, tok) {
			t.Fatalf("D-07 invariant violated: ScheduleInitialExtraction body "+
				"contains forbidden token %q — Phase 65 owns the workspace "+
				"walk per 59-CONTEXT.md §2026-05-08 update D-07", tok)
		}
	}
}

// TestD07_IdempotencyAnchor is a discoverability anchor — a future grep
// for "D-07" should land here. The actual idempotency invariant is
// covered by TestScheduler_Idempotent in scheduler_test.go (existing
// from Phase 59 P03).
func TestD07_IdempotencyAnchor(t *testing.T) {
	t.Log("D-07: idempotency invariant covered by scheduler_test.go:TestScheduler_Idempotent")
}
