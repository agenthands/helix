// Package rss exposes a CGO-free reader for the current process resident set size.
//
// Platform implementations live in build-tagged files:
//   - rss_linux.go   parses /proc/self/status VmRSS
//   - rss_darwin.go  shells out to `ps -o rss= -p <pid>`
//   - rss_other.go   returns ErrUnsupported for every other GOOS
//
// The public API is the parameterless CurrentRSS function, which dispatches to
// the build-tagged currentRSS implementation. Benchmarks in the parent
// test/bench package use this to capture the Phase 9 v1.1 memory baseline
// (D-09) without pulling in gopsutil or any cgo dependency.
package rss

import "errors"

// ErrUnsupported is returned by CurrentRSS on GOOS values without a platform
// implementation (i.e. anything other than linux or darwin).
var ErrUnsupported = errors.New("rss: unsupported GOOS")

// CurrentRSS returns the resident set size of the current process in bytes.
// On Linux it reads /proc/self/status VmRSS. On darwin it shells out to
// `ps -o rss= -p <pid>`. On other GOOS it returns (0, ErrUnsupported).
func CurrentRSS() (uint64, error) { return currentRSS() }
