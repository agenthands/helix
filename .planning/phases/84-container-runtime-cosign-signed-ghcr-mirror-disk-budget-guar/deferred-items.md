# Deferred Items — Phase 84

Out-of-scope discoveries logged during execution. Do NOT fix in the discovering plan.

- [84-02] gofmt drift in `bench/container/engine_test.go` (Plan-01 file, inline-comment alignment ~L106-110). Not a `make vet` gate. Resolve with `gofmt -w bench/container/engine_test.go` in a future docs/cleanup pass.
