package upgrade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"golang.org/x/mod/semver"

	serr "github.com/agenthands/helix/internal/errors"
)

// GitHub Releases API constants. The X-GitHub-Api-Version header pins the
// GitHub REST API surface so an upstream change cannot silently break our
// JSON parsing — see RESEARCH.md Pattern 1.
const (
	defaultAPIBase  = "https://api.github.com"
	releasesOwner   = "agenthands"
	releasesRepo    = "helix"
	acceptHeader    = "application/vnd.github+json"
	apiVersionHeader = "2026-03-10"
	defaultPageSize = 30
)

// userAgent is the User-Agent header value sent with every GitHub API
// request. GitHub recommends a UA per RESEARCH.md Pitfall 6; without one
// anonymous requests are rate-limited harder. Built lazily so the
// goreleaser-injected version is picked up after cli.SetVersion runs.
func userAgent() string {
	v := currentVersion
	if v == "" {
		v = "dev"
	}
	return "helix-upgrade/" + v
}

// currentVersion reflects the binary version threaded by cmd/helix/main.go.
// internal/cli is the source-of-truth setter; this package keeps a small
// shadow so the GitHub User-Agent string can include the version without
// dragging the cli package into the upgrade import graph (which would
// create a cli↔upgrade cycle once internal/cli/upgrade.go imports this
// package). The shadow is set by the cli upgrade subcommand wiring in
// Task 3 via SetCurrentVersion.
var currentVersion = "dev"

// SetCurrentVersion records the binary version string used to build the
// User-Agent header for GitHub API requests. Called by internal/cli/upgrade.go
// (Task 3) before invoking Upgrade or FetchReleaseInfo.
func SetCurrentVersion(v string) {
	if v == "" {
		return
	}
	currentVersion = v
}

// Release matches the GitHub Releases JSON shape we care about. Fields
// not enumerated here are ignored. The struct mirrors the testdata
// fixture at internal/upgrade/testdata/release_latest.json.
type Release struct {
	TagName    string  `json:"tag_name"`
	Name       string  `json:"name"`
	Body       string  `json:"body"`
	Prerelease bool    `json:"prerelease"`
	Draft      bool    `json:"draft"`
	Assets     []Asset `json:"assets"`
}

// Asset describes a single archive attached to a Release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// FindAsset returns the first asset matching the given name, or nil if
// no match is found. Used by Upgrade() to locate the per-platform
// archive and signature pair from a Release's asset list.
func (r *Release) FindAsset(name string) *Asset {
	for i := range r.Assets {
		if r.Assets[i].Name == name {
			return &r.Assets[i]
		}
	}
	return nil
}

// httpClient is the package-level HTTP client used for all GitHub API
// requests and asset downloads. Configured with a sensible timeout so a
// hung GitHub call cannot wedge the upgrade flow.
//
// CheckRedirect strips the Authorization header on cross-host hops so a
// GITHUB_TOKEN bearer credential is not leaked when GitHub redirects an
// asset download from api.github.com to objects.githubusercontent.com
// (or any other CDN). Go's default redirect policy forwards Authorization
// to subdomains under some matching rules; an explicit cross-host strip
// is the only credential-safe behavior. See REVIEW.md CR-01.
var httpClient = &http.Client{
	Timeout: 60 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("too many redirects")
		}
		// Drop Authorization on cross-host redirects.
		if len(via) > 0 && req.URL.Host != via[0].URL.Host {
			req.Header.Del("Authorization")
		}
		return nil
	},
}

// fetchJSON GETs url, decodes the JSON body into out, and surfaces the
// rate-limit branch as a typed Timeout error per RESEARCH.md Pitfall 6.
// Honors GITHUB_TOKEN if set in the environment (lifts rate limit from
// 60/hr/IP to 5000/hr authenticated).
func fetchJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return serr.Wrap(serr.Internal, "creating github request", err)
	}
	req.Header.Set("User-Agent", userAgent())
	req.Header.Set("Accept", acceptHeader)
	req.Header.Set("X-GitHub-Api-Version", apiVersionHeader)
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return serr.Wrap(serr.Internal, "github request failed", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Rate-limit branch (RESEARCH.md Pitfall 6): 429 explicit; or 403 with
	// X-RateLimit-Remaining=0. Return a typed Timeout error with an
	// actionable hint pointing at GITHUB_TOKEN.
	if resp.StatusCode == http.StatusTooManyRequests ||
		(resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0") {
		retry := resp.Header.Get("Retry-After")
		hint := "github API rate limited; retry later or set GITHUB_TOKEN to lift the 60/hr anon limit"
		if retry != "" {
			hint = fmt.Sprintf("github API rate limited; retry after %ss or set GITHUB_TOKEN", retry)
		}
		return serr.New(serr.Timeout, hint)
	}

	if resp.StatusCode == http.StatusNotFound {
		return serr.New(serr.NotFound, "github release not found").
			WithDetail("url=" + url)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return serr.New(serr.Internal, fmt.Sprintf("github API HTTP %d", resp.StatusCode)).
			WithDetail(strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return serr.Wrap(serr.Internal, "decoding github response", err)
	}
	return nil
}

// fetchLatest queries GET /repos/{owner}/{repo}/releases/latest. The
// /latest endpoint filters drafts and prereleases server-side, so this
// function never returns a prerelease tag — see fetchAllReleases for the
// prerelease-inclusive path.
func fetchLatest(ctx context.Context, baseURL, owner, repo string) (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", baseURL, owner, repo)
	var rel Release
	if err := fetchJSON(ctx, url, &rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// fetchByTag queries GET /repos/{owner}/{repo}/releases/tags/{tag}.
// Used by --version pinning to fetch a specific tag's assets.
func fetchByTag(ctx context.Context, baseURL, owner, repo, tag string) (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", baseURL, owner, repo, tag)
	var rel Release
	if err := fetchJSON(ctx, url, &rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// fetchAllReleases queries GET /repos/{owner}/{repo}/releases (single
// page, default page size). Returns drafts + prereleases + stable
// releases mixed together. The caller filters as needed.
//
// Pagination is intentionally NOT supported in v1.9 (RESEARCH.md
// Pattern 1 lists pagination as a v1.10 polish item). Per_page=30 is
// far more than the Helix release cadence requires.
func fetchAllReleases(ctx context.Context, baseURL, owner, repo string) ([]Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=%d", baseURL, owner, repo, defaultPageSize)
	var rels []Release
	if err := fetchJSON(ctx, url, &rels); err != nil {
		return nil, err
	}
	return rels, nil
}

// FetchReleaseInfo is the public entry point for `helix update` and
// `helix upgrade` (without --version). When prerelease=false it calls
// /releases/latest (which already excludes prereleases server-side).
// When prerelease=true it lists /releases and returns the highest
// semver-comparing tag including prereleases.
//
// The optional baseURL overrides the GitHub API host — tests pass an
// httptest server URL; production callers pass "" to use defaultAPIBase.
func FetchReleaseInfo(ctx context.Context, prerelease bool) (*Release, error) {
	return fetchReleaseInfoFrom(ctx, defaultAPIBase, prerelease)
}

// fetchReleaseInfoFrom is the test-friendly variant that accepts a base
// URL override. Production callers go through FetchReleaseInfo which
// pins to api.github.com.
func fetchReleaseInfoFrom(ctx context.Context, baseURL string, prerelease bool) (*Release, error) {
	if !prerelease {
		return fetchLatest(ctx, baseURL, releasesOwner, releasesRepo)
	}
	all, err := fetchAllReleases(ctx, baseURL, releasesOwner, releasesRepo)
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, serr.New(serr.NotFound, "no github releases available")
	}
	// Skip drafts and pick the highest-semver tag (which may itself be a
	// prerelease since the user opted in).
	var picked *Release
	for i := range all {
		r := &all[i]
		if r.Draft {
			continue
		}
		if picked == nil {
			picked = r
			continue
		}
		if semver.Compare(Canonical(r.TagName), Canonical(picked.TagName)) > 0 {
			picked = r
		}
	}
	if picked == nil {
		return nil, serr.New(serr.NotFound, "no non-draft github releases")
	}
	return picked, nil
}

// FetchByTag exposes fetchByTag publicly for the upgrade orchestrator
// (`--version vX.Y.Z` pinning).
func FetchByTag(ctx context.Context, tag string) (*Release, error) {
	return fetchByTag(ctx, defaultAPIBase, releasesOwner, releasesRepo, tag)
}

// fetchByTagFrom is the test-friendly variant.
func fetchByTagFrom(ctx context.Context, baseURL, tag string) (*Release, error) {
	return fetchByTag(ctx, baseURL, releasesOwner, releasesRepo, tag)
}

// downloadFile streams the URL body to destPath using the package
// httpClient. Used to download archive + signature + checksums files
// into the stage dir. Honors ctx for cancellation.
func downloadFile(ctx context.Context, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return serr.Wrap(serr.Internal, "creating download request", err)
	}
	req.Header.Set("User-Agent", userAgent())
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return serr.Wrap(serr.Internal, "downloading "+url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return serr.New(serr.Internal, fmt.Sprintf("download failed: HTTP %d for %s", resp.StatusCode, url))
	}

	out, err := os.Create(destPath)
	if err != nil {
		return serr.Wrap(serr.Internal, "creating "+destPath, err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		return serr.Wrap(serr.Internal, "writing "+destPath, err)
	}
	if err := out.Close(); err != nil {
		return serr.Wrap(serr.Internal, "closing "+destPath, err)
	}
	return nil
}

// sortReleasesBySemverDesc sorts in-place by descending semver tag.
// Helper exposed for tests; not used by production code paths.
func sortReleasesBySemverDesc(rels []Release) {
	sort.Slice(rels, func(i, j int) bool {
		return semver.Compare(Canonical(rels[i].TagName), Canonical(rels[j].TagName)) > 0
	})
}
