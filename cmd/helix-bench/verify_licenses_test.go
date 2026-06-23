package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTempAudit writes content to a temp LICENSE-AUDIT.md and returns its path.
func writeTempAudit(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "LICENSE-AUDIT.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp audit: %v", err)
	}
	return path
}

// goodAudit is a minimal, strict-decodable two-track audit fixture.
const goodAudit = `# Aider Polyglot — Per-Track License Audit

Prose preamble that is NOT a track block (no top-level track: key).

---
track: python
source_repo: github.com/exercism/python
license: MIT
license_sha256: 0000000000000000000000000000000000000000000000000000000000000001
redistribution_clause_excerpt: "Permission is hereby granted, free of charge, ..."
---
track: rust
source_repo: github.com/exercism/rust
license: MIT
license_sha256: 0000000000000000000000000000000000000000000000000000000000000002
redistribution_clause_excerpt: "Permission is hereby granted, free of charge, ..."
---
`

func TestVerifyLicenses_GoodFileReturnsCount(t *testing.T) {
	path := writeTempAudit(t, goodAudit)
	n, err := verifyLicensesCount(path)
	if err != nil {
		t.Fatalf("expected nil error on good audit, got: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 track entries, got %d", n)
	}
}

func TestVerifyLicenses_EmptyLicenseFailsClosed(t *testing.T) {
	content := `---
track: python
source_repo: github.com/exercism/python
license: ""
license_sha256: 0000000000000000000000000000000000000000000000000000000000000001
redistribution_clause_excerpt: "x"
---
`
	path := writeTempAudit(t, content)
	if _, err := verifyLicensesCount(path); err == nil {
		t.Fatalf("expected error on empty license, got nil")
	}
}

func TestVerifyLicenses_EmptySHA256FailsClosed(t *testing.T) {
	content := `---
track: python
source_repo: github.com/exercism/python
license: MIT
license_sha256: ""
redistribution_clause_excerpt: "x"
---
`
	path := writeTempAudit(t, content)
	if _, err := verifyLicensesCount(path); err == nil {
		t.Fatalf("expected error on empty license_sha256, got nil")
	}
}

func TestVerifyLicenses_UnknownKeyFailsStrictDecode(t *testing.T) {
	content := `---
track: python
source_repo: github.com/exercism/python
license: MIT
license_sha256: 0000000000000000000000000000000000000000000000000000000000000001
redistribution_clause_excerpt: "x"
bogus_unknown_key: surprise
---
`
	path := writeTempAudit(t, content)
	_, err := verifyLicensesCount(path)
	if err == nil {
		t.Fatalf("expected strict-decode error on unknown key, got nil")
	}
	if !strings.Contains(err.Error(), "strict decode") {
		t.Fatalf("expected strict-decode error, got: %v", err)
	}
}

func TestVerifyLicenses_ZeroTracksIsError(t *testing.T) {
	content := "# An audit file with no track blocks at all\n\nJust prose.\n"
	path := writeTempAudit(t, content)
	_, err := verifyLicensesCount(path)
	if err == nil {
		t.Fatalf("expected error on zero-track file, got nil")
	}
}

func TestVerifyLicenses_CommittedAuditPasses(t *testing.T) {
	// The committed audit must itself pass the gate with >0 tracks.
	path := "../../bench/datasets/aider-polyglot/LICENSE-AUDIT.md"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("committed audit not present yet: %v", err)
	}
	n, err := verifyLicensesCount(path)
	if err != nil {
		t.Fatalf("committed LICENSE-AUDIT.md failed the gate: %v", err)
	}
	if n == 0 {
		t.Fatalf("committed LICENSE-AUDIT.md has zero tracks")
	}
}

// ----------------------------------------------------------------------------
// Plan 99-02: dual-disposition + manifest-vs-disk walk + anti-vacuity tamper
// ----------------------------------------------------------------------------

// vendorTree is the materialized paths of a good MIT+Apache temp tree.
type vendorTree struct {
	dir      string // dataset root (holds LICENSE-AUDIT.md, VENDOR-MANIFEST.md)
	audit    string // path to LICENSE-AUDIT.md
	manifest string // path to VENDOR-MANIFEST.md
	tree     string // path to fixtures/ (the --tree root)
}

// sha256Hex returns the lowercase hex sha256 of b.
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// vendorTempTree materializes, under t.TempDir(), a GOOD mixed-license vendored
// tree: a fixtures/ root with two MIT files + an Apache-2.0 _aider-edit-format/
// subtree (with its NOTICE + LICENSE sidecars), a VENDOR-MANIFEST.md carrying a
// REAL crypto/sha256 row per on-disk file, and a dual-disposition
// LICENSE-AUDIT.md (one MIT track block + one Apache-2.0 fixture block). The
// good tree passes the gate; each tamper test mutates exactly one thing and
// asserts a non-nil error. Mirrors writeTempAudit's t.TempDir() discipline.
func vendorTempTree(t *testing.T) vendorTree {
	t.Helper()
	dir := t.TempDir()
	tree := filepath.Join(dir, "fixtures")

	// On-disk fixture files: <relpath-under-fixtures> -> content.
	files := map[string]string{
		"go/exercises/practice/bowling/bowling.go":    "package bowling\n",
		"rust/exercises/practice/leap/src/lib.rs":     "pub fn is_leap_year(_y: u64) -> bool { false }\n",
		"_aider-edit-format/NOTICE":                   "Aider\nCopyright ...\n",
		"_aider-edit-format/LICENSE":                  "Apache License 2.0 ...\n",
		"_aider-edit-format/languages/python/test.py": "def test(): pass\n",
	}

	var rows []string
	for rel, content := range files {
		abs := filepath.Join(tree, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", abs, err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", abs, err)
		}
		lic := "MIT"
		if strings.HasPrefix(rel, "_aider-edit-format/") {
			lic = "Apache-2.0"
		}
		rows = append(rows, "| `"+rel+"` | "+sha256Hex([]byte(content))+" | "+lic+" | upstream@deadbeef |")
	}

	manifest := "# Vendor Manifest (test)\n\n" +
		"| path (relative to fixtures/) | sha256 | license | upstream_provenance |\n" +
		"|------------------------------|--------|---------|---------------------|\n" +
		strings.Join(rows, "\n") + "\n"
	manifestPath := filepath.Join(dir, "VENDOR-MANIFEST.md")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	audit := `# Test Audit

Prose preamble that is NOT a block.

---
track: go
source_repo: github.com/exercism/go
license: MIT
license_sha256: 0000000000000000000000000000000000000000000000000000000000000001
redistribution_clause_excerpt: "Permission is hereby granted ..."
---
fixture: _aider-edit-format
source_repo: Aider-AI/aider@5dc9490bb35f9729ef2c95d00a19ccd30c26339c
license: Apache-2.0
notice_path: fixtures/_aider-edit-format/NOTICE
license_path: fixtures/_aider-edit-format/LICENSE
---
`
	auditPath := filepath.Join(dir, "LICENSE-AUDIT.md")
	if err := os.WriteFile(auditPath, []byte(audit), 0o644); err != nil {
		t.Fatalf("write audit: %v", err)
	}

	return vendorTree{dir: dir, audit: auditPath, manifest: manifestPath, tree: tree}
}

// TestVerifyLicensesFull_GoodTreeControl is the anti-vacuity control: a good
// tree must PASS so the gate is not RED-on-everything.
func TestVerifyLicensesFull_GoodTreeControl(t *testing.T) {
	vt := vendorTempTree(t)
	if err := verifyLicensesFull(vt.audit, vt.manifest, vt.tree); err != nil {
		t.Fatalf("good tree control must pass, got: %v", err)
	}
}

// TestVerifyLicensesFull_TamperFlippedManifestLicense — flipping a manifest
// row's license value is harmless to the sha walk, so the real flip-detector is
// the AUDIT disposition. Here we flip the Apache-2.0 fixture block's license to
// MIT; the fixture block must still assert a recorded Apache-2.0 disposition...
// (covered by ZeroApacheBlocks + FlippedAuditLicense below). This case proves a
// flipped MANIFEST license is still caught when it desyncs from the audit.
func TestVerifyLicensesFull_TamperFlippedManifestLicense(t *testing.T) {
	vt := vendorTempTree(t)
	b, _ := os.ReadFile(vt.manifest)
	flipped := strings.Replace(string(b), "| Apache-2.0 | upstream@deadbeef |", "| GPL-3.0 | upstream@deadbeef |", 1)
	if flipped == string(b) {
		t.Fatal("test setup: nothing flipped")
	}
	if err := os.WriteFile(vt.manifest, []byte(flipped), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyLicensesFull(vt.audit, vt.manifest, vt.tree); err == nil {
		t.Fatal("expected error on a manifest license that desyncs from the audit disposition, got nil")
	}
}

// TestVerifyLicensesFull_TamperFlippedAuditLicense — change the Apache-2.0
// fixture block's license to a non-Apache value: the dual-disposition assertion
// (≥1 Apache-2.0 fixture block) fails closed.
func TestVerifyLicensesFull_TamperFlippedAuditLicense(t *testing.T) {
	vt := vendorTempTree(t)
	b, _ := os.ReadFile(vt.audit)
	flipped := strings.Replace(string(b), "license: Apache-2.0", "license: MIT", 1)
	if err := os.WriteFile(vt.audit, []byte(flipped), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyLicensesFull(vt.audit, vt.manifest, vt.tree); err == nil {
		t.Fatal("expected error when the only Apache-2.0 fixture block is flipped away, got nil")
	}
}

// TestVerifyLicensesFull_TamperDroppedManifestEntry — an on-disk file with NO
// manifest row hard-fails.
func TestVerifyLicensesFull_TamperDroppedManifestEntry(t *testing.T) {
	vt := vendorTempTree(t)
	b, _ := os.ReadFile(vt.manifest)
	// Drop the bowling.go manifest row.
	var kept []string
	for _, line := range strings.Split(string(b), "\n") {
		if strings.Contains(line, "bowling/bowling.go") {
			continue
		}
		kept = append(kept, line)
	}
	if err := os.WriteFile(vt.manifest, []byte(strings.Join(kept, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	err := verifyLicensesFull(vt.audit, vt.manifest, vt.tree)
	if err == nil {
		t.Fatal("expected error on an on-disk file with no manifest entry, got nil")
	}
	if !strings.Contains(err.Error(), "no manifest entry") {
		t.Fatalf("expected 'no manifest entry' error, got: %v", err)
	}
}

// TestVerifyLicensesFull_TamperMutatedByte — flipping one byte of a vendored
// file so its on-disk sha256 != its manifest row hard-fails.
func TestVerifyLicensesFull_TamperMutatedByte(t *testing.T) {
	vt := vendorTempTree(t)
	target := filepath.Join(vt.tree, "go/exercises/practice/bowling/bowling.go")
	if err := os.WriteFile(target, []byte("package bowling // tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := verifyLicensesFull(vt.audit, vt.manifest, vt.tree)
	if err == nil {
		t.Fatal("expected error on a mutated byte (sha mismatch), got nil")
	}
	if !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("expected 'sha256 mismatch' error, got: %v", err)
	}
}

// TestVerifyLicensesFull_TamperExtraManifestEntry — a manifest row whose file is
// absent on disk hard-fails (reverse direction).
func TestVerifyLicensesFull_TamperExtraManifestEntry(t *testing.T) {
	vt := vendorTempTree(t)
	b, _ := os.ReadFile(vt.manifest)
	extra := string(b) + "| `ghost/phantom.txt` | " + sha256Hex([]byte("nope")) + " | MIT | upstream@deadbeef |\n"
	if err := os.WriteFile(vt.manifest, []byte(extra), 0o644); err != nil {
		t.Fatal(err)
	}
	err := verifyLicensesFull(vt.audit, vt.manifest, vt.tree)
	if err == nil {
		t.Fatal("expected error on a manifest entry with no on-disk file, got nil")
	}
}

// TestVerifyLicensesFull_TamperRemovedSidecar — deleting the notice/license path
// named by the Apache-2.0 fixture block hard-fails.
func TestVerifyLicensesFull_TamperRemovedSidecar(t *testing.T) {
	vt := vendorTempTree(t)
	notice := filepath.Join(vt.tree, "_aider-edit-format/NOTICE")
	if err := os.Remove(notice); err != nil {
		t.Fatal(err)
	}
	// The manifest row for the now-removed file would also fail the walk, so
	// drop it too — isolating the missing-sidecar check.
	b, _ := os.ReadFile(vt.manifest)
	var kept []string
	for _, line := range strings.Split(string(b), "\n") {
		if strings.Contains(line, "_aider-edit-format/NOTICE") {
			continue
		}
		kept = append(kept, line)
	}
	if err := os.WriteFile(vt.manifest, []byte(strings.Join(kept, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyLicensesFull(vt.audit, vt.manifest, vt.tree); err == nil {
		t.Fatal("expected error on a removed notice/license sidecar, got nil")
	}
}

// TestVerifyLicensesFull_ZeroApacheBlocksIsError — an audit with only MIT track
// blocks (zero Apache-2.0 fixture blocks) fails closed: dual disposition is
// required, not optional (regression to single disposition).
func TestVerifyLicensesFull_ZeroApacheBlocksIsError(t *testing.T) {
	vt := vendorTempTree(t)
	b, _ := os.ReadFile(vt.audit)
	// Strip the entire fixture block.
	idx := strings.Index(string(b), "fixture: _aider-edit-format")
	if idx < 0 {
		t.Fatal("test setup: no fixture block to strip")
	}
	// Keep everything up to the `---` fence that precedes the fixture block.
	trimmed := string(b)[:strings.LastIndex(string(b)[:idx], "---")] + "---\n"
	if err := os.WriteFile(vt.audit, []byte(trimmed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyLicensesFull(vt.audit, vt.manifest, vt.tree); err == nil {
		t.Fatal("expected error on zero Apache-2.0 fixture blocks, got nil")
	}
}

// TestVerifyLicensesFull_FixtureUnknownKeyFailsStrictDecode — an unknown key in
// the Apache-2.0 fixture block fails the KnownFields(true) strict decode.
func TestVerifyLicensesFull_FixtureUnknownKeyFailsStrictDecode(t *testing.T) {
	vt := vendorTempTree(t)
	b, _ := os.ReadFile(vt.audit)
	poisoned := strings.Replace(string(b),
		"license_path: fixtures/_aider-edit-format/LICENSE",
		"license_path: fixtures/_aider-edit-format/LICENSE\nbogus_unknown_key: surprise", 1)
	if err := os.WriteFile(vt.audit, []byte(poisoned), 0o644); err != nil {
		t.Fatal(err)
	}
	err := verifyLicensesFull(vt.audit, vt.manifest, vt.tree)
	if err == nil {
		t.Fatal("expected strict-decode error on unknown fixture key, got nil")
	}
	if !strings.Contains(err.Error(), "strict decode") {
		t.Fatalf("expected strict-decode error, got: %v", err)
	}
}

// TestVerifyLicensesFull_TamperSymlinkInTree (WR-01) — a symlink inside the
// --tree root is refused: filepath.WalkDir does not follow symlinks, so a
// symlinked directory's subtree would never be required to be manifest-pinned
// and a symlinked file would be hashed by out-of-tree target bytes. The gate
// must hard-fail rather than silently fail to cover what it cannot traverse.
func TestVerifyLicensesFull_TamperSymlinkInTree(t *testing.T) {
	vt := vendorTempTree(t)
	// Replace a real on-disk file with a symlink pointing at an out-of-tree
	// target whose bytes match the manifest digest. Without the WR-01 refusal
	// the dereferenced read would hash the target and PASS — exactly the
	// fail-open this test pins shut.
	rel := "go/exercises/practice/bowling/bowling.go"
	target := filepath.Join(vt.tree, rel)
	content := "package bowling\n" // identical bytes to the original file
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(outside, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, target); err != nil {
		t.Skipf("symlinks not supported on this platform: %v", err)
	}
	err := verifyLicensesFull(vt.audit, vt.manifest, vt.tree)
	if err == nil {
		t.Fatal("expected error on a symlink inside the tree, got nil")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected a 'symlink' refusal error, got: %v", err)
	}
}

// TestVerifyLicensesFull_TamperDuplicateManifestRow (WR-02) — a manifest with a
// wrong-digest row followed by a correct row for the SAME path must hard-fail
// rather than silently collapse last-wins. 3 rows → 2 distinct paths.
func TestVerifyLicensesFull_TamperDuplicateManifestRow(t *testing.T) {
	vt := vendorTempTree(t)
	b, _ := os.ReadFile(vt.manifest)
	rel := "go/exercises/practice/bowling/bowling.go"
	// Inject a wrong-digest DUPLICATE row for an existing path. The genuine
	// (correct) row for the same path remains, so under last-wins the gate
	// would pass; the duplicate-key refusal is what catches it.
	dupRow := "| `" + rel + "` | " + sha256Hex([]byte("WRONG BYTES")) + " | MIT | upstream@deadbeef |\n"
	tampered := string(b) + dupRow
	if err := os.WriteFile(vt.manifest, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}
	err := verifyLicensesFull(vt.audit, vt.manifest, vt.tree)
	if err == nil {
		t.Fatal("expected error on a duplicate manifest row, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate row") {
		t.Fatalf("expected a 'duplicate row' error, got: %v", err)
	}
}

// TestVerifyLicensesFull_TamperEmptyLicenseColumn (WR-03) — a per-file manifest
// row whose license column was blanked must hard-fail. An empty license is an
// unverifiable claim, not an exemption from the audit-disposition cross-check.
func TestVerifyLicensesFull_TamperEmptyLicenseColumn(t *testing.T) {
	vt := vendorTempTree(t)
	b, _ := os.ReadFile(vt.manifest)
	// Blank the license column of the bowling.go row (MIT → empty) while
	// keeping its digest correct, so only the license half is tampered.
	blanked := strings.Replace(string(b),
		"bowling/bowling.go` | "+sha256Hex([]byte("package bowling\n"))+" | MIT |",
		"bowling/bowling.go` | "+sha256Hex([]byte("package bowling\n"))+" |  |", 1)
	if blanked == string(b) {
		t.Fatal("test setup: nothing blanked")
	}
	if err := os.WriteFile(vt.manifest, []byte(blanked), 0o644); err != nil {
		t.Fatal(err)
	}
	err := verifyLicensesFull(vt.audit, vt.manifest, vt.tree)
	if err == nil {
		t.Fatal("expected error on an empty license column, got nil")
	}
	if !strings.Contains(err.Error(), "empty license column") {
		t.Fatalf("expected an 'empty license column' error, got: %v", err)
	}
}

// TestVerifyLicensesFull_TamperMalformedShortRow (WR-04) — a per-file row that
// is missing a column must hard-fail loudly instead of being silently dropped
// from the map (where a digest claim would evaporate with no diagnostic).
func TestVerifyLicensesFull_TamperMalformedShortRow(t *testing.T) {
	vt := vendorTempTree(t)
	b, _ := os.ReadFile(vt.manifest)
	// Append a malformed short row (only path + sha, no license/provenance
	// columns). Its path is NOT on disk, so the old code's silent `continue`
	// would drop it with no error; the column-count check makes it loud.
	shortRow := "| `ghost/short.txt` | " + sha256Hex([]byte("x")) + " |\n"
	tampered := string(b) + shortRow
	if err := os.WriteFile(vt.manifest, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}
	err := verifyLicensesFull(vt.audit, vt.manifest, vt.tree)
	if err == nil {
		t.Fatal("expected error on a malformed short manifest row, got nil")
	}
	if !strings.Contains(err.Error(), "too few columns") {
		t.Fatalf("expected a 'too few columns' error, got: %v", err)
	}
}

// TestVerifyLicensesFull_CommittedTreePasses runs the FULL gate (audit +
// manifest + tree) against the REAL committed Plan 01 tree.
func TestVerifyLicensesFull_CommittedTreePasses(t *testing.T) {
	base := "../../bench/datasets/aider-polyglot"
	audit := filepath.Join(base, "LICENSE-AUDIT.md")
	manifest := filepath.Join(base, "VENDOR-MANIFEST.md")
	tree := filepath.Join(base, "fixtures")
	for _, p := range []string{audit, manifest, tree} {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("committed tree not present yet: %v", err)
		}
	}
	if err := verifyLicensesFull(audit, manifest, tree); err != nil {
		t.Fatalf("committed mixed-license tree failed the full gate: %v", err)
	}
}
