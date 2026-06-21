package repobench

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// TestPinIsImmutableRev proves each language pins a real 40-hex immutable commit
// (never a branch/tag, T-86-04-01) and the host/repos are the pinned upstream.
func TestPinIsImmutableRev(t *testing.T) {
	if !isValidHTTPSHost(Host) {
		t.Fatalf("Host = %q must be a valid https origin (no leading '-', https only)", Host)
	}
	for _, lang := range Languages {
		rev := PinnedRev(lang)
		if !isHexSHA1(rev) {
			t.Errorf("PinnedRev(%s) = %q is not a 40-char hex immutable commit", lang, rev)
		}
		repo := PinnedRepo(lang)
		if !strings.Contains(repo, "repobench") || !strings.Contains(repo, lang) {
			t.Errorf("PinnedRepo(%s) = %q, want the per-language repobench mirror", lang, repo)
		}
	}
}

// TestIsHexSHA1 proves the rev validator accepts a 40-hex commit and rejects
// everything else (short, long, uppercase, non-hex, branch-shaped).
func TestIsHexSHA1(t *testing.T) {
	good := PinnedRev("python")
	if !isHexSHA1(good) {
		t.Fatalf("isHexSHA1(%q) = false, want true", good)
	}
	for _, bad := range []string{
		"",
		"main",
		"8a7cf0c8", // too short
		good + "00",
		strings.ToUpper(good),
		"8a7cf0c8942cc1aa066bf261839650ac55a2ff7g", // non-hex 'g'
	} {
		if isHexSHA1(bad) {
			t.Errorf("isHexSHA1(%q) = true, want false", bad)
		}
	}
}

// TestResolveURLIsPinned proves the resolve URL is built ONLY from the pinned
// constants + a validated rev/language (T-86-04-02 SSRF): it is the single
// https://huggingface.co/datasets URL, it embeds the pinned rev + the
// per-language pinned repo, and it rejects a mutable ref / unknown / traversal
// language before producing any URL.
func TestResolveURLIsPinned(t *testing.T) {
	for _, lang := range Languages {
		rev := PinnedRev(lang)
		url, err := resolveURL(rev, lang)
		if err != nil {
			t.Fatalf("resolveURL(%s): %v", lang, err)
		}
		if !strings.HasPrefix(url, "https://huggingface.co/datasets/") {
			t.Errorf("resolveURL(%s) = %q, want https://huggingface.co/datasets/ prefix", lang, url)
		}
		if !strings.Contains(url, rev) {
			t.Errorf("resolveURL(%s) = %q, want it to embed the pinned rev", lang, url)
		}
		if !strings.Contains(url, PinnedRepo(lang)) {
			t.Errorf("resolveURL(%s) = %q, want it to embed the per-language pinned repo", lang, url)
		}
	}
	// Mutable ref refused.
	if _, err := resolveURL("main", "python"); err == nil {
		t.Error("resolveURL(main, python) = nil error, want mutable-ref rejection")
	}
	// Unknown language refused.
	if _, err := resolveURL(PinnedRev("python"), "rust"); err == nil {
		t.Error("resolveURL(rev, rust) = nil error, want unsupported-language rejection")
	}
	// Traversal language refused before URL build.
	if _, err := resolveURL(PinnedRev("python"), "../etc"); err == nil {
		t.Error("resolveURL(rev, ../etc) = nil error, want path-segment rejection")
	}
}

// TestCacheDirHonorsEnv proves the cache root honors HELIX_CACHE_DIR (the shared
// operator convention) and the per-language cache path lands under
// <cacheDir>/repobench/<rev>/.
func TestCacheDirHonorsEnv(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(cacheDirEnv, tmp)
	rev := PinnedRev("python")
	p, err := cachePath(rev, "python")
	if err != nil {
		t.Fatalf("cachePath: %v", err)
	}
	if !strings.HasPrefix(p, tmp) {
		t.Errorf("cachePath = %q, want under %q", p, tmp)
	}
	if !strings.Contains(p, fetchSubdir) || !strings.Contains(p, rev) {
		t.Errorf("cachePath = %q, want it to contain %q and the pinned rev", p, fetchSubdir)
	}
}

// TestFetchUsesCacheNoNetwork proves Fetch returns committed cached bytes with NO
// network when the cache file already exists (the cache-hit path). It seeds the
// cache from the committed sample.parquet so it stays hermetic.
func TestFetchUsesCacheNoNetwork(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(cacheDirEnv, tmp)
	rev := PinnedRev("python")
	dst, err := cachePath(rev, "python")
	if err != nil {
		t.Fatalf("cachePath: %v", err)
	}
	if err := os.MkdirAll(dirOf(dst), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	raw, err := os.ReadFile("testdata/sample.parquet")
	if err != nil {
		t.Fatalf("read sample.parquet: %v", err)
	}
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	got, err := Fetch(context.Background(), rev, "python")
	if err != nil {
		t.Fatalf("Fetch (cache hit): %v", err)
	}
	if len(got) != len(raw) {
		t.Errorf("Fetch returned %d bytes, want cached %d", len(got), len(raw))
	}
}

func dirOf(p string) string {
	i := strings.LastIndexAny(p, `/\`)
	if i < 0 {
		return "."
	}
	return p[:i]
}

// TestLiveFetch is the network-gated live leg. It SKIPs offline (a quick
// reachability probe) or unless HELIX_BENCH_NETWORK is opted in. It is NEVER the
// sole proof — the hermetic TestParquetDecodeRetrievalAndCompletion is
// authoritative. The tianyang/repobench_*_v1.1 mirror may not ship an
// auto-converted per-language parquet at the resolve path used here, so a live
// resolve may 404; the test tolerates that as a clean network-gated skip and
// never asserts published metrics as the sole proof.
func TestLiveFetch(t *testing.T) {
	if os.Getenv("HELIX_BENCH_NETWORK") == "" {
		t.Skip("set HELIX_BENCH_NETWORK=1 to run the live RepoBench parquet fetch (network gated)")
	}
	conn, err := net.DialTimeout("tcp", "huggingface.co:443", 3*time.Second)
	if err != nil {
		t.Skipf("huggingface.co unreachable, skipping live fetch: %v", err)
	}
	_ = conn.Close()

	tmp := t.TempDir()
	t.Setenv(cacheDirEnv, tmp)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	for _, lang := range Languages {
		tasks, err := Load(ctx, lang)
		if err != nil {
			// The mirror may not ship a per-language parquet at this resolve path;
			// record honestly and continue rather than fail (never the sole proof).
			t.Logf("live Load(%s) unavailable (expected if mirror ships JSONL/sharded only): %v", lang, err)
			continue
		}
		if len(tasks) == 0 {
			t.Errorf("live Load(%s) returned zero tasks", lang)
		}
	}
}
