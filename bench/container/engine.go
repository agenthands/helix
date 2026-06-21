package container

import (
	"context"
	"errors"
)

// Engine is a resolved container engine (docker or podman) located on PATH.
type Engine struct {
	bin string
}

// errEngineUnavailable is the sentinel returned by Detect when neither docker
// nor podman is on PATH. Live container tests t.Skip on this.
var errEngineUnavailable = errors.New("bench/container: no docker or podman on PATH")

// Detect — STUB (RED).
func Detect() (*Engine, error) {
	return nil, errors.New("not implemented")
}

// pullArgs — STUB (RED).
func (e *Engine) pullArgs(repo, digest string) ([]string, error) {
	return nil, errors.New("not implemented")
}

// PullByDigest — STUB (RED).
func (e *Engine) PullByDigest(ctx context.Context, repo, digest string) error {
	return errors.New("not implemented")
}
