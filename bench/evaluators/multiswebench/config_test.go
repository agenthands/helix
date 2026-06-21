package multiswebench

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// goldenConfig is the fully-populated Config the committed testdata/config.golden.json
// fixture pins. Every path field is clean+absolute so it passes the validator; the
// struct-field declaration order is the JSON key order json.MarshalIndent emits.
func goldenConfig() Config {
	return Config{
		Mode:    "evaluation",
		Workdir: "/cache/helix/multiswebench-runs/workdir",
		PatchFiles: []string{
			"/cache/helix/multiswebench-runs/patches/go.jsonl",
			"/cache/helix/multiswebench-runs/patches/java.jsonl",
		},
		DatasetFiles: []string{
			"/cache/helix/multiswebench-mini/go/dataset.jsonl",
			"/cache/helix/multiswebench-mini/java/dataset.jsonl",
		},
		ForceBuild:            false,
		OutputDir:             "/cache/helix/multiswebench-runs/output",
		Specifics:             []string{},
		Skips:                 []string{},
		RepoDir:               "/cache/helix/multiswebench-runs/repos",
		NeedClone:             true,
		GlobalEnv:             []string{"GOFLAGS=-mod=mod"},
		ClearEnv:              true,
		StopOnError:           false,
		MaxWorkers:            4,
		MaxWorkersBuildImage:  2,
		MaxWorkersRunInstance: 2,
		LogDir:                "/cache/helix/multiswebench-runs/logs",
		LogLevel:              "INFO",
	}
}

// TestWriteConfigGoldenStable (Task 1, ADAPTER-MULTI-01 / A1): WriteConfig over a
// fully-populated Config writes a file byte-identical to testdata/config.golden.json
// (the producer contract Plan 04's human-verify confirms against the live README),
// and re-running it is byte-identical (deterministic — sorted key order via the
// struct declaration order, no map iteration). This is the hermetic golden — no
// Docker, no python.
func TestWriteConfigGoldenStable(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "config.json")

	if err := WriteConfig(dst, goldenConfig()); err != nil {
		t.Fatalf("WriteConfig on a valid Config returned error: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "config.golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("WriteConfig output mismatch with golden:\n got = %s\nwant = %s", got, want)
	}

	// Determinism: a second write is byte-identical to the first.
	dst2 := filepath.Join(dir, "config2.json")
	if err := WriteConfig(dst2, goldenConfig()); err != nil {
		t.Fatalf("second WriteConfig error: %v", err)
	}
	got2, err := os.ReadFile(dst2)
	if err != nil {
		t.Fatalf("read second config: %v", err)
	}
	if !bytes.Equal(got, got2) {
		t.Errorf("WriteConfig is not deterministic: run1 != run2")
	}
}

// TestWriteConfigPathRefusal (Task 1, T-88-01-01): WriteConfig fail-closes with
// errBadConfig — and writes NOTHING — when ANY path field (Workdir/OutputDir/
// RepoDir/LogDir or any PatchFiles[]/DatasetFiles[] entry) is empty, relative,
// contains '..', contains ':', or is flag-shaped (leading '-'). Each refused case
// leaves no file at the target.
func TestWriteConfigPathRefusal(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
	}{
		{"empty workdir", func(c *Config) { c.Workdir = "" }},
		{"relative workdir", func(c *Config) { c.Workdir = "workdir" }},
		{"dotdot workdir", func(c *Config) { c.Workdir = "/cache/../etc/workdir" }},
		{"colon workdir", func(c *Config) { c.Workdir = "/cache/a:b/workdir" }},
		{"flag-shaped output_dir", func(c *Config) { c.OutputDir = "--output=/tmp/x" }},
		{"empty output_dir", func(c *Config) { c.OutputDir = "" }},
		{"relative repo_dir", func(c *Config) { c.RepoDir = "repos" }},
		{"dotdot log_dir", func(c *Config) { c.LogDir = "/cache/../logs" }},
		{"colon log_dir", func(c *Config) { c.LogDir = "/cache/a:b/logs" }},
		{"relative patch file", func(c *Config) { c.PatchFiles = []string{"patches/go.jsonl"} }},
		{"dotdot patch file", func(c *Config) { c.PatchFiles = []string{"/cache/../etc/go.jsonl"} }},
		{"empty patch file entry", func(c *Config) { c.PatchFiles = []string{"/cache/ok.jsonl", ""} }},
		{"flag-shaped dataset file", func(c *Config) { c.DatasetFiles = []string{"--config=/tmp/x"} }},
		{"colon dataset file", func(c *Config) { c.DatasetFiles = []string{"/cache/a:b/dataset.jsonl"} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			dst := filepath.Join(dir, "config.json")
			c := goldenConfig()
			tc.mutate(&c)
			err := WriteConfig(dst, c)
			if err == nil {
				t.Fatalf("expected errBadConfig for %s, got nil", tc.name)
			}
			if !errors.Is(err, errBadConfig) {
				t.Errorf("expected errBadConfig, got %v", err)
			}
			if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
				t.Errorf("refused WriteConfig must leave NO file at %q (stat err=%v)", dst, statErr)
			}
		})
	}
}

// TestWriteConfigAtomicNoTemp (Task 1, T-88-01-05): a successful WriteConfig leaves
// no leftover .tmp-* file in the target directory — the temp+rename completed, so a
// crash mid-write never leaves a torn config a harness reads.
func TestWriteConfigAtomicNoTemp(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "config.json")
	if err := WriteConfig(dst, goldenConfig()); err != nil {
		t.Fatalf("WriteConfig error: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if len(e.Name()) >= 5 && e.Name()[:5] == ".tmp-" {
			t.Errorf("found leftover temp file %q — atomic temp+rename did not complete", e.Name())
		}
	}
	if len(entries) != 1 {
		t.Errorf("expected exactly 1 file (config.json), got %d: %v", len(entries), entries)
	}
}
