package terminalbench

import (
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

func ptrInt(i int) *int { return &i }

// TestIngest proves the load-bearing wiring: a parsed per-trial result → a
// bench/runtime.ResultInput with task_success ← the per-trial `is_resolved` bool
// (the AUTHORITATIVE gate — NEVER a count of trial rows), the container_id +
// exit_code carried verbatim (exit_code 0 PRESERVED), benchmark "terminal-bench".
// A missing/empty TaskID is an ingest error with a zero-value ResultInput — never
// fabricated success.
func TestIngest(t *testing.T) {
	t.Run("is_resolved true -> task_success true", func(t *testing.T) {
		trial := TBTrial{TaskID: "hello-world", IsResolved: true}
		in, err := Ingest(trial, "tb-ctr-abc123", ptrInt(0))
		if err != nil {
			t.Fatalf("Ingest: %v", err)
		}
		if in.Benchmark != "terminal-bench" {
			t.Errorf("benchmark = %q, want terminal-bench", in.Benchmark)
		}
		if in.TaskID != "hello-world" {
			t.Errorf("task_id = %q, want hello-world", in.TaskID)
		}
		if in.Metrics.TaskSuccess == nil || *in.Metrics.TaskSuccess != true {
			t.Errorf("task_success = %v, want true (from is_resolved)", in.Metrics.TaskSuccess)
		}
		if in.Metrics.VerifiedCorrectness != nil {
			t.Errorf("verified_correctness = %v, want nil", in.Metrics.VerifiedCorrectness)
		}
		if in.ContainerID != "tb-ctr-abc123" {
			t.Errorf("container_id = %q, want tb-ctr-abc123", in.ContainerID)
		}
		if in.ExitCode == nil || *in.ExitCode != 0 {
			t.Errorf("exit_code = %v, want ptr(0) PRESERVED", in.ExitCode)
		}
	})

	t.Run("is_resolved false -> task_success false", func(t *testing.T) {
		trial := TBTrial{TaskID: "broken-task", IsResolved: false}
		in, err := Ingest(trial, "tb-ctr-xyz", ptrInt(1))
		if err != nil {
			t.Fatalf("Ingest: %v", err)
		}
		if in.Metrics.TaskSuccess == nil || *in.Metrics.TaskSuccess != false {
			t.Errorf("task_success = %v, want false (from is_resolved)", in.Metrics.TaskSuccess)
		}
		if in.ExitCode == nil || *in.ExitCode != 1 {
			t.Errorf("exit_code = %v, want ptr(1)", in.ExitCode)
		}
	})

	t.Run("nil exit_code carried through as nil", func(t *testing.T) {
		trial := TBTrial{TaskID: "hello-world", IsResolved: true}
		in, err := Ingest(trial, "", nil)
		if err != nil {
			t.Fatalf("Ingest: %v", err)
		}
		if in.ExitCode != nil {
			t.Errorf("exit_code = %v, want nil (no exit captured)", in.ExitCode)
		}
		if in.ContainerID != "" {
			t.Errorf("container_id = %q, want empty", in.ContainerID)
		}
	})

	t.Run("empty task_id -> error, never fabricated success", func(t *testing.T) {
		trial := TBTrial{TaskID: "", IsResolved: true}
		in, err := Ingest(trial, "tb-ctr", ptrInt(0))
		if err == nil {
			t.Fatalf("Ingest with empty task_id: want error, got nil (in=%+v)", in)
		}
		if in.Metrics.TaskSuccess != nil {
			t.Errorf("on error task_success must be nil, got %v (no fabricated success)", in.Metrics.TaskSuccess)
		}
		if in.TaskID != "" {
			t.Errorf("on error the ResultInput must be the zero value, got task_id %q", in.TaskID)
		}
	})

	t.Run("task_success and verified are distinct pointers", func(t *testing.T) {
		trial := TBTrial{TaskID: "hello-world", IsResolved: true}
		in, err := Ingest(trial, "tb-ctr", ptrInt(0))
		if err != nil {
			t.Fatalf("Ingest: %v", err)
		}
		if in.Metrics.TaskSuccess != nil && in.Metrics.VerifiedCorrectness != nil &&
			in.Metrics.TaskSuccess == in.Metrics.VerifiedCorrectness {
			t.Error("task_success and verified_correctness must be distinct pointers")
		}
	})
}

// forbiddenIsolationImports are the packages this adapter must NOT reach — even
// transitively — because routing container isolation through bench/container
// would re-implement what tb's per-task DockerComposeManager already owns (SC#2,
// Pitfall 2). The import-level gate proves cross-task filesystem leakage cannot be
// introduced by this adapter.
var forbiddenIsolationImports = []string{
	"github.com/agenthands/helix/bench/container",
}

// TestContainerIsolationNoBenchContainerImport (SC#2, T-88-02-05): loads the FULL
// TRANSITIVE import set of bench/evaluators/terminalbench and asserts it never
// reaches bench/container. The adapter relies on tb's own per-task fresh-container
// isolation; it does NOT route through bench/container.Engine.Run. A
// direct-imports-only check would be a false pass — a helper could re-introduce
// the edge transitively — so this walks the full dependency graph (mirrors the
// cmd/helix-bench-rag leakage-gate idiom).
func TestContainerIsolationNoBenchContainerImport(t *testing.T) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedImports | packages.NeedDeps,
	}
	pkgs, err := packages.Load(cfg, "github.com/agenthands/helix/bench/evaluators/terminalbench")
	if err != nil {
		t.Fatalf("packages.Load: %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("expected exactly 1 root package, got %d", len(pkgs))
	}
	for _, p := range pkgs {
		for _, e := range p.Errors {
			t.Fatalf("package load error: %v", e)
		}
	}

	seen := map[string]bool{}
	var visit func(p *packages.Package)
	visit = func(p *packages.Package) {
		if seen[p.PkgPath] {
			return
		}
		seen[p.PkgPath] = true
		for _, forbidden := range forbiddenIsolationImports {
			if p.PkgPath == forbidden || strings.HasPrefix(p.PkgPath, forbidden+"/") {
				t.Errorf("terminalbench transitively imports forbidden package %s — SC#2 isolation is owned by tb, NOT bench/container", p.PkgPath)
			}
		}
		for _, imp := range p.Imports {
			visit(imp)
		}
	}
	for _, p := range pkgs {
		visit(p)
	}
}
