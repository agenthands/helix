package cli

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// installedBundleNames installs the skill into a fresh temp dir and returns the
// sorted names of the non-directory files actually written to disk. It asserts
// against the LITERAL on-disk listing — never a ReadDir of the embed SOURCE dir
// (that would be a set-compared-to-itself tautology that can never go RED;
// PITFALLS.md / 97-RESEARCH Pitfall 2).
func installedBundleNames(t *testing.T) []string {
	t.Helper()
	dir := skillTargetDir(t.TempDir())
	require.NoError(t, installSkill(dir), "installSkill must succeed")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err, "reading installed skill dir")
	var got []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		got = append(got, e.Name())
	}
	sort.Strings(got)
	return got
}

// TestInstallSkillInstallsExactlyBundle is the CLOSED-SET install contract
// (BUNDLE-01, SC-1): after installSkill, the target dir must contain EXACTLY the
// two bundle files {SKILL.md, reference.md} and nothing else. A positive-only
// bundle test (TestInstallSkillWritesBundle) cannot catch a stray embedded asset
// shipped to the user's disk; this closed-set assertion goes RED on ANY extra
// installed file (the recurring Phase 87/89 CR-01 vacuous-gate class).
func TestInstallSkillInstallsExactlyBundle(t *testing.T) {
	want := []string{"SKILL.md", "reference.md"}
	sort.Strings(want)
	got := installedBundleNames(t)
	assert.Equal(t, want, got,
		"installed skill dir must contain EXACTLY %v; a stray installed file is a supply-chain leak (BUNDLE-01)", want)
}

// TestUninstallSkillRemovesBundleSymmetric proves uninstallSkill is driven by the
// SAME closed-set allowlist as install (BUNDLE-01, T-103-04): after a clean
// install, uninstallSkill removes every allowlisted bundle file and prunes the
// now-empty skills/helix dir. Asymmetric filtering would orphan a now-un-shipped
// file on upgrade; this keeps install and uninstall symmetric.
func TestUninstallSkillRemovesBundleSymmetric(t *testing.T) {
	dir := skillTargetDir(t.TempDir())
	require.NoError(t, installSkill(dir), "installSkill must succeed")
	// Precondition: the bundle files are present.
	for _, name := range []string{"SKILL.md", "reference.md"} {
		_, err := os.Stat(filepath.Join(dir, name))
		require.NoError(t, err, "precondition: %s must exist before uninstall", name)
	}

	require.NoError(t, uninstallSkill(dir), "uninstallSkill must succeed")

	// No allowlisted bundle file is left behind.
	for _, name := range []string{"SKILL.md", "reference.md"} {
		_, err := os.Stat(filepath.Join(dir, name))
		assert.Truef(t, os.IsNotExist(err),
			"uninstallSkill must remove the allowlisted bundle file %s", name)
	}
	// The now-empty skills/helix dir is pruned (uninstallSkill contract).
	_, err := os.Stat(dir)
	assert.True(t, os.IsNotExist(err),
		"uninstallSkill must prune the now-empty skills/helix dir")
}
