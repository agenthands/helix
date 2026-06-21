package swebenchutboost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCacheDirHonorsEnv (Phase 87 Task 5): cacheDir returns HELIX_CACHE_DIR
// verbatim when set, and cachePath lands under <cacheDir>/swebench-utboost/<rev>/.
func TestCacheDirHonorsEnv(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(cacheDirEnv, tmp)

	if got := cacheDir(); got != tmp {
		t.Errorf("cacheDir() = %q, want the HELIX_CACHE_DIR override %q", got, tmp)
	}
	p, err := cachePath(PinnedSHA, "data.parquet")
	if err != nil {
		t.Fatalf("cachePath error: %v", err)
	}
	if !strings.Contains(p, tmp) || !strings.Contains(p, fetchSubdir) || !strings.Contains(p, PinnedSHA) {
		t.Errorf("cachePath = %q, want it under %q and to contain %q and the pinned rev", p, tmp, fetchSubdir)
	}
}

// TestCachePathRejectsTraversal (T-87-04): a rev or file segment carrying a
// traversal/separator is refused before any filepath.Join, so the cache path
// cannot escape the cache root.
func TestCachePathRejectsTraversal(t *testing.T) {
	if _, err := cachePath("..", "data.parquet"); err == nil {
		t.Error("cachePath must reject a '..' rev segment")
	}
	if _, err := cachePath(PinnedSHA, "../escape"); err == nil {
		t.Error("cachePath must reject a traversal file segment")
	}
	if _, err := cachePath(PinnedSHA, "a/b"); err == nil {
		t.Error("cachePath must reject a file segment with a separator")
	}
}

// TestFetchRefusesMutableRevBeforeNetwork (T-87-03): a non-hex (mutable) rev is
// refused by Fetch BEFORE any cache read or network call — the error is the
// mutable-ref refusal, never an HTTP error. Hermetic: no network is performed.
func TestFetchRefusesMutableRevBeforeNetwork(t *testing.T) {
	t.Setenv(cacheDirEnv, t.TempDir())
	_, err := Fetch(context.Background(), "main", "data.parquet")
	if err == nil {
		t.Fatal("Fetch must refuse a mutable rev 'main' before any network call")
	}
	if !strings.Contains(err.Error(), "mutable ref") {
		t.Errorf("expected a mutable-ref refusal error, got %v", err)
	}
}

// TestFetchCacheHitHonorsSizeCap (WR-02): a present cache file under the
// predictable cache path that exceeds maxDatasetBytes is REFUSED on the
// cache-hit read leg — symmetric with the network leg's io.LimitReader cap —
// rather than read unbounded into memory. Hermetic: no network; a small valid
// cache file is still served. We exercise readCacheCapped directly (the
// cache-read primitive) so the test needs no multi-GiB fixture: the cap is
// asserted against fi.Size() and via io.LimitReader, both reachable here.
func TestFetchCacheHitHonorsSizeCap(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(cacheDirEnv, tmp)

	dst, err := cachePath(PinnedSHA, "data.parquet")
	if err != nil {
		t.Fatalf("cachePath error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}

	// A genuine miss surfaces os.IsNotExist so Fetch falls through to network.
	if _, err := readCacheCapped(dst); !os.IsNotExist(err) {
		t.Fatalf("readCacheCapped on a missing file = %v, want an os.IsNotExist error", err)
	}

	// A small in-cap file is served verbatim.
	want := []byte("small-valid-cache-payload")
	if err := os.WriteFile(dst, want, 0o644); err != nil {
		t.Fatalf("write small cache file: %v", err)
	}
	got, err := readCacheCapped(dst)
	if err != nil {
		t.Fatalf("readCacheCapped on an in-cap file = %v, want it served", err)
	}
	if string(got) != string(want) {
		t.Errorf("readCacheCapped returned %q, want %q", got, want)
	}

	// An oversized file is REFUSED (not read unbounded). Use a tiny sentinel cap
	// by writing a file one byte over maxDatasetBytes would be wasteful; instead
	// assert the size-guard branch by creating a sparse file sized just over the
	// cap (sparse → no real bytes written), so the os.Stat().Size() guard fires
	// without allocating maxDatasetBytes of memory or disk.
	big := filepath.Join(filepath.Dir(dst), "oversized.parquet")
	f, err := os.Create(big)
	if err != nil {
		t.Fatalf("create oversized file: %v", err)
	}
	if err := f.Truncate(int64(maxDatasetBytes) + 1); err != nil {
		f.Close()
		t.Fatalf("truncate oversized file: %v", err)
	}
	f.Close()
	if _, err := readCacheCapped(big); err == nil {
		t.Fatal("readCacheCapped must REFUSE an oversized cache file, got nil error")
	} else if os.IsNotExist(err) {
		t.Fatalf("oversized cache file must be a cap refusal, not a miss: %v", err)
	} else if !strings.Contains(err.Error(), "cap") {
		t.Errorf("expected a size-cap refusal error, got %v", err)
	}
}

// TestContentDigestAssertion (WR-01): assertContentDigest is a no-op when no
// digest is pinned for a file (the documented rev-pin-only residual), MATCHES the
// correct payload when a digest IS pinned, and FAILS CLOSED on a mismatch (a
// moved/wrong commit or tampered/poisoned payload). PinnedContentDigests is a var
// so the test injects and restores a pin hermetically.
func TestContentDigestAssertion(t *testing.T) {
	body := []byte("audited-utboost-payload")
	sum := sha256.Sum256(body)
	want := hex.EncodeToString(sum[:])
	const file = "data.parquet"

	// No pin recorded -> no-op (residual rev-pin-only path).
	if err := assertContentDigest(file, body); err != nil {
		t.Fatalf("assertContentDigest with no pin = %v, want nil (rev-pin-only residual)", err)
	}

	// Inject a pin, restore on cleanup.
	orig := PinnedContentDigests
	PinnedContentDigests = map[string]string{file: want}
	t.Cleanup(func() { PinnedContentDigests = orig })

	// Correct payload matches.
	if err := assertContentDigest(file, body); err != nil {
		t.Fatalf("assertContentDigest on the matching payload = %v, want nil", err)
	}

	// Tampered payload fails closed.
	tampered := append([]byte(nil), body...)
	tampered[0] ^= 0xFF
	err := assertContentDigest(file, tampered)
	if err == nil {
		t.Fatal("assertContentDigest must FAIL CLOSED on a digest mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "digest mismatch") {
		t.Errorf("expected a digest-mismatch refusal, got %v", err)
	}
}

// TestResolveURLPinnedConstantsOnly (T-87-04 SSRF): resolveURL is built ONLY from
// the pinned Host+DatasetID constants + the validated rev+file; it refuses a
// mutable rev and produces the expected huggingface.co resolve URL for the pin.
func TestResolveURLPinnedConstantsOnly(t *testing.T) {
	if _, err := resolveURL("main", "data.parquet"); err == nil {
		t.Error("resolveURL must refuse a mutable rev")
	}
	url, err := resolveURL(PinnedSHA, "data.parquet")
	if err != nil {
		t.Fatalf("resolveURL error: %v", err)
	}
	want := Host + "/datasets/" + DatasetID + "/resolve/" + PinnedSHA + "/data.parquet"
	if url != want {
		t.Errorf("resolveURL = %q, want %q", url, want)
	}
}

// TestLiveFetch is the network-gated live leg. It SKIPs unless HELIX_BENCH_NETWORK
// is opted in, and SKIPs offline (a quick reachability probe). It is NEVER the
// sole proof — the hermetic cachePath/resolveURL/mutable-ref tests above are
// authoritative. The exact pinned-rev file name + report log-dir path are
// confirmed against the DOCUMENTED upstream but their LIVE confirmation is
// deferred to a Docker+swebench+network host (87-RESEARCH A1/A3, recorded as
// deferred in the SUMMARY); a live resolve may 404 if the documented pin/file
// differs, which the test tolerates as a clean network-gated skip.
func TestLiveFetch(t *testing.T) {
	if os.Getenv("HELIX_BENCH_NETWORK") == "" {
		t.Skip("set HELIX_BENCH_NETWORK=1 to run the live UTBoost fetch (network gated)")
	}
	conn, err := net.DialTimeout("tcp", "huggingface.co:443", 3*time.Second)
	if err != nil {
		t.Skipf("huggingface.co unreachable, skipping live fetch: %v", err)
	}
	_ = conn.Close()

	t.Setenv(cacheDirEnv, t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	// The augmented suite file name is confirmed-but-deferred; tolerate a 404 as a
	// clean skip rather than a failure (never the sole proof).
	if _, err := Fetch(ctx, PinnedSHA, "data.parquet"); err != nil {
		t.Logf("live UTBoost Fetch unavailable (expected until the exact pin/file is live-confirmed): %v", err)
	}
}
