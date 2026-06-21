package main

import (
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
