package upgrade

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// loadFixtureRelease reads testdata/release_latest.json into a Release
// struct, ensuring tests stay in sync with the canned fixture from
// Plan 01.
func loadFixtureRelease(t *testing.T) Release {
	t.Helper()
	bytes, err := os.ReadFile("testdata/release_latest.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	var r Release
	if err := json.Unmarshal(bytes, &r); err != nil {
		t.Fatalf("decoding fixture: %v", err)
	}
	return r
}

func TestApiFetchLatestHappyPath(t *testing.T) {
	t.Parallel()
	rel := loadFixtureRelease(t)

	var lastReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastReq = r
		if r.URL.Path != "/repos/agenthands/helix/releases/latest" {
			t.Errorf("unexpected path: %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rel)
	}))
	defer server.Close()

	got, err := fetchLatest(context.Background(), server.URL, "agenthands", "helix")
	if err != nil {
		t.Fatalf("fetchLatest: %v", err)
	}
	if got.TagName != "v1.9.0" {
		t.Errorf("TagName = %q, want v1.9.0", got.TagName)
	}
	if len(got.Assets) != 4 {
		t.Errorf("Assets count = %d, want 4", len(got.Assets))
	}
	// Header invariants.
	if !strings.Contains(lastReq.Header.Get("User-Agent"), "helix-upgrade/") {
		t.Errorf("User-Agent missing: %q", lastReq.Header.Get("User-Agent"))
	}
	if !strings.Contains(lastReq.Header.Get("Accept"), "application/vnd.github+json") {
		t.Errorf("Accept missing: %q", lastReq.Header.Get("Accept"))
	}
	if lastReq.Header.Get("X-GitHub-Api-Version") != apiVersionHeader {
		t.Errorf("X-GitHub-Api-Version = %q, want %q",
			lastReq.Header.Get("X-GitHub-Api-Version"), apiVersionHeader)
	}
}

func TestApiFetchByTag(t *testing.T) {
	t.Parallel()
	rel := loadFixtureRelease(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/agenthands/helix/releases/tags/v1.9.0" {
			t.Errorf("unexpected path: %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(rel)
	}))
	defer server.Close()

	got, err := fetchByTag(context.Background(), server.URL, "agenthands", "helix", "v1.9.0")
	if err != nil {
		t.Fatalf("fetchByTag: %v", err)
	}
	if got.TagName != "v1.9.0" {
		t.Errorf("TagName = %q, want v1.9.0", got.TagName)
	}
}

func TestApiFetchAll(t *testing.T) {
	t.Parallel()
	rel := loadFixtureRelease(t)
	prerel := rel
	prerel.TagName = "v1.10.0-rc1"
	prerel.Prerelease = true

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/repos/agenthands/helix/releases") {
			t.Errorf("unexpected path: %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]Release{prerel, rel})
	}))
	defer server.Close()

	got, err := fetchAllReleases(context.Background(), server.URL, "agenthands", "helix")
	if err != nil {
		t.Fatalf("fetchAll: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d releases, want 2", len(got))
	}
}

func TestApiRateLimitError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	_, err := fetchLatest(context.Background(), server.URL, "agenthands", "helix")
	if err == nil {
		t.Fatal("expected rate-limit error, got nil")
	}
	if !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Errorf("rate-limit err missing GITHUB_TOKEN hint: %q", err.Error())
	}
}

func TestApiRateLimit429(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	_, err := fetchLatest(context.Background(), server.URL, "agenthands", "helix")
	if err == nil {
		t.Fatal("expected rate-limit error on 429, got nil")
	}
	if !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Errorf("err missing GITHUB_TOKEN hint: %q", err.Error())
	}
}

func TestApiFetchReleaseInfoPrerelease(t *testing.T) {
	t.Parallel()
	stable := loadFixtureRelease(t)
	prerel := stable
	prerel.TagName = "v1.10.0-rc1"
	prerel.Prerelease = true

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only the all-releases endpoint should be hit on prerelease=true.
		if !strings.HasSuffix(r.URL.Path, "/releases") {
			t.Errorf("unexpected path: %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]Release{stable, prerel})
	}))
	defer server.Close()

	got, err := fetchReleaseInfoFrom(context.Background(), server.URL, true)
	if err != nil {
		t.Fatalf("fetchReleaseInfoFrom: %v", err)
	}
	if got.TagName != "v1.10.0-rc1" {
		t.Errorf("TagName = %q, want v1.10.0-rc1 (highest semver)", got.TagName)
	}
}

func TestApiUserAgentVersionWiring(t *testing.T) {
	prev := currentVersion
	t.Cleanup(func() { currentVersion = prev })
	SetCurrentVersion("v1.9.0")

	if !strings.Contains(userAgent(), "v1.9.0") {
		t.Errorf("userAgent = %q, want contains v1.9.0", userAgent())
	}

	// Empty SetCurrentVersion is ignored.
	SetCurrentVersion("")
	if !strings.Contains(userAgent(), "v1.9.0") {
		t.Errorf("userAgent after empty SetCurrentVersion = %q, want still v1.9.0", userAgent())
	}
}
