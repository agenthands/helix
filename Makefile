.PHONY: build clean proto test vet fmt docs clean-jdtls-cache bench-jdtls-warm bench bench-baseline release-snapshot update-trust-root

BINARY=helix
GO=go

build:
	$(GO) build -o $(BINARY) ./cmd/helix

clean:
	rm -f $(BINARY)

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		api/proto/serena/v1/*.proto

test: vet
	$(GO) test ./...

VETTOOL=$(shell go env GOPATH)/bin/vet-noduckdb

vet: $(VETTOOL)
	$(GO) vet ./...
	$(GO) vet -vettool=$(VETTOOL) ./...

$(VETTOOL): cmd/vet-noduckdb/main.go internal/lint/noduckdb/*.go
	$(GO) install ./cmd/vet-noduckdb

fmt:
	gofmt -w .

docs: ## Regenerate tool and language tables in README.md
	$(GO) run ./cmd/docgen

clean-jdtls-cache: ## Wipe warm jdtls workspaces under the platform user cache dir
	@case "$$(uname -s)" in \
	  Darwin) DIR="$$HOME/Library/Caches/helix-test/jdtls" ;; \
	  *)      DIR="$${XDG_CACHE_HOME:-$$HOME/.cache}/helix-test/jdtls" ;; \
	esac; \
	rm -rf "$$DIR"; \
	echo "cleared jdtls warm cache at $$DIR"

bench-jdtls-warm: ## Run Java integration suite cold then warm; print both wall-clocks
	@$(MAKE) clean-jdtls-cache
	@echo "=== jdtls COLD run ==="
	-@time $(GO) test -run 'Java' ./test/integration/... -count=1
	@echo "=== jdtls WARM run ==="
	-@time $(GO) test -run 'Java' ./test/integration/... -count=1

bench: ## Run the bench suite once and print results to stdout
	$(GO) test -short -bench=. -benchmem -count=10 -run=^$$ ./test/bench/...

bench-baseline: ## Capture a local baseline into test/bench/baselines/local.txt (gitignored, overwrites)
	$(GO) test -short -bench=. -benchmem -count=10 -run=^$$ ./test/bench/... | tee test/bench/baselines/local.txt

release-snapshot: ## Run a local goreleaser dry-run; writes archives to dist/ (overwrites; gitignored)
	@command -v goreleaser >/dev/null 2>&1 || { \
	  echo "goreleaser not installed; see CONTRIBUTING.md (Releasing). brew install goreleaser"; exit 1; }
	goreleaser release --snapshot --clean --skip=sign

update-trust-root: ## Refresh internal/upgrade/trusted_root.json from the LIVE sigstore TUF repository
	@# WR-10: source from sigstore's TUF service via cosign's local TUF
	@# state, NOT from sigstore-go's static `examples/trusted-root-public-good.json`.
	@# The static example file had drifted from the live public-good TUF (it still
	@# carried GitHub's internal-services TSA root after sigstore replaced it with
	@# `sigstore-tsa-selfsigned`), causing helix's verifier to reject every CI
	@# bundle with "certificate signed by unknown authority". cosign initialize
	@# fetches and caches the current live trust root under ~/.sigstore/.
	@command -v cosign >/dev/null || { echo "cosign not on PATH; install cosign 2.4+ and retry" >&2; exit 1; }
	@cosign initialize >/dev/null 2>&1 || { echo "cosign initialize failed -- check network access to tuf-repo-cdn.sigstore.dev" >&2; exit 1; }
	@cp ~/.sigstore/root/tuf-repo-cdn.sigstore.dev/targets/trusted_root.json internal/upgrade/trusted_root.json
	@echo "trusted_root.json refreshed from live sigstore TUF; commit and bump per CONTRIBUTING.md"
	@sha256sum internal/upgrade/trusted_root.json 2>/dev/null || shasum -a 256 internal/upgrade/trusted_root.json
