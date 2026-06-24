// Package dspytune is a testdata stub standing in for the dev-time tools/ tree
// so the leakyruntime fixture's forbidden import resolves under the analysistest
// GOPATH. Its own import path IS rooted under the tools/ prefix, so the
// analyzer's self-import exemption applies to it (tools/ may self-import).
package dspytune

// Marker is an exported symbol so the package is non-empty.
const Marker = "dspytune"
