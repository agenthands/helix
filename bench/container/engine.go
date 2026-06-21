package container

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// Engine is a resolved container engine (docker or podman) located on PATH. All
// invocation crosses the os/exec boundary with FIXED argv and a strict env
// allowlist — never a shell, never the full inherited env (T-84-01-01, mirrors
// bench/runtime/subprocess/ragserver.go discipline).
type Engine struct {
	bin string
}

// errEngineUnavailable is the sentinel returned by Detect when neither docker
// nor podman is on PATH. Live container tests t.Skip on this.
var errEngineUnavailable = errors.New("bench/container: no docker or podman on PATH")

// engineCandidates is the fixed probe order. docker is probed first, so when
// both are present docker wins (TestDetectPrefersDocker).
var engineCandidates = []string{"docker", "podman"}

// Detect resolves the first available container engine on PATH, probing docker
// then podman. Returns errEngineUnavailable when neither is found.
func Detect() (*Engine, error) {
	for _, name := range engineCandidates {
		if p, err := exec.LookPath(name); err == nil {
			return &Engine{bin: p}, nil
		}
	}
	return nil, errEngineUnavailable
}

// pullArgs builds the FIXED argv for a digest-pinned pull: ["pull",
// "<repo>@sha256:<hex>"]. It fail-closes via isHexSHA256 BEFORE any argv is
// produced, so a tag-shaped or malformed ref can never reach the engine
// (T-84-01-02 / CONTAINER-02 / Pitfall 2). This is the hermetically-assertable
// seam the unit tests drive without a live engine.
func (e *Engine) pullArgs(repo, digest string) ([]string, error) {
	if !isHexSHA256(digest) {
		return nil, errBadDigest
	}
	if !isValidRepo(repo) {
		return nil, errBadRepo
	}
	ref := fmt.Sprintf("%s@sha256:%s", repo, digest)
	// "--" terminates docker/podman option parsing so a crafted ref can never be
	// smuggled as a flag (argv flag-smuggling, T-84-01-03). isValidRepo already
	// rejects a leading '-'; the separator is belt-and-suspenders.
	return []string{"pull", "--", ref}, nil
}

// PullByDigest pulls repo@sha256:<digest> via the resolved engine. It rejects a
// non-64-hex digest with errBadDigest before crossing the os/exec boundary,
// runs the fixed argv with a strict env allowlist (PATH/HOME/HELIX_CACHE_DIR
// only), and owns its process group (Setpgid) so a context cancel can group-kill
// the engine and any descendants.
func (e *Engine) PullByDigest(ctx context.Context, repo, digest string) error {
	args, err := e.pullArgs(repo, digest)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, e.bin, args...)
	// procGroupAttr is build-tag-split: on unix it sets Setpgid so a context
	// cancel can group-kill the engine and its descendants; on windows it is nil
	// (POSIX process groups have no windows analog), keeping the package
	// cross-compilable for the windows release archive (Pitfall 6).
	cmd.SysProcAttr = procGroupAttr()
	cmd.Env = allowlistEnv()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bench/container: %s pull %s@sha256:%s: %w", e.bin, repo, digest, err)
	}
	return nil
}

// allowlistEnv builds the strict env passed to the engine subprocess: only
// PATH, HOME, and HELIX_CACHE_DIR are forwarded (set keys only), never the full
// parent environment. Mirrors ragserver.go's env discipline (T-84-01-01).
func allowlistEnv() []string {
	env := make([]string, 0, 3)
	for _, k := range []string{"PATH", "HOME", "HELIX_CACHE_DIR"} {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	return env
}
