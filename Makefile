.PHONY: build clean proto test vet fmt test-stress bench bench-stat

BINARY=serena
GO=go

build:
	$(GO) build -o $(BINARY) ./cmd/serena

clean:
	rm -f $(BINARY)

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		api/proto/serena/v1/*.proto

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w .

test-stress: ## Run concurrency stress tests with elevated count under -race
	$(GO) test -tags integration ./test/integration/... \
		-run '^TestConcurrency' \
		-race -count=5 -timeout=10m

# bench runs the full benchmark suite once for a quick local sanity check.
# Uses -benchtime=1x for speed; this is NOT suitable for statistical
# comparison — use `make bench-stat` for that.
bench:
	$(GO) test -bench=. -benchmem -benchtime=1x -run=^$$ -count=1 ./test/bench/...

# bench-stat runs the full benchmark suite with -count=10 (D-03 PR
# policy), compares against the committed v1.1 baseline via benchstat
# (for human inspection), and then invokes the benchgate regression
# gate. Fails the command if any benchmark regresses beyond the D-01 PR
# tier thresholds (15% time, 25% allocs) at p<0.05.
bench-stat:
	@command -v benchstat >/dev/null || { echo "benchstat not installed: go install golang.org/x/perf/cmd/benchstat@latest" >&2; exit 1; }
	$(GO) test -bench=. -benchmem -count=10 -run=^$$ ./test/bench/... | tee /tmp/bench-new.txt
	benchstat test/bench/baselines/v1.1-github-hosted.txt /tmp/bench-new.txt
	$(GO) run ./test/bench/cmd/benchgate --baseline test/bench/baselines/v1.1-github-hosted.txt --new /tmp/bench-new.txt
