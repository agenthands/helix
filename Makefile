.PHONY: build clean proto test vet fmt docs clean-jdtls-cache bench-jdtls-warm bench bench-baseline release-snapshot embed-pubkey verify-embed-pubkey

BINARY=serena
GO=go

build: embed-pubkey
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

docs: ## Regenerate tool and language tables in README.md
	$(GO) run ./cmd/docgen

clean-jdtls-cache: ## Wipe warm jdtls workspaces under the platform user cache dir
	@case "$$(uname -s)" in \
	  Darwin) DIR="$$HOME/Library/Caches/serena-test/jdtls" ;; \
	  *)      DIR="$${XDG_CACHE_HOME:-$$HOME/.cache}/serena-test/jdtls" ;; \
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

embed-pubkey: ## Sync repo-root minisign.pub into internal/upgrade/minisign.pub before build
	@cp minisign.pub internal/upgrade/minisign.pub

verify-embed-pubkey: ## CI gate: assert internal/upgrade/minisign.pub matches repo-root copy byte-for-byte
	@cmp -s minisign.pub internal/upgrade/minisign.pub || { \
	  echo "internal/upgrade/minisign.pub drift; run: make embed-pubkey"; exit 1; }
