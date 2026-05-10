// Package runner implements the Phase 67 evaluation mode dispatch loop.
// For each (task, mode) pair it spawns an isolated daemon subprocess, launches
// the agent (claude CLI or scripted agent for eval-quick), waits for completion,
// and hands off to the trace collector. Real Run bodies land in Wave 1+; this
// Wave 0 package skeleton locks the import boundary.
package runner
