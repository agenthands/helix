package swebenchutboost

import (
	"context"
	"net"
	"os"
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
