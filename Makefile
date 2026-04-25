.PHONY: build clean proto test vet fmt docs clean-jdtls-cache bench-jdtls-warm bench-capture bench-compare

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

bench-capture: ## Capture a local pre-release benchmark baseline (override OUT=test/bench/baselines/v<version>-local-<goos>-<goarch>.txt)
	@OUT="$${OUT:-test/bench/baselines/v-local-$$($(GO) env GOOS)-$$($(GO) env GOARCH).txt}"; \
	echo "Capturing baseline -> $$OUT"; \
	$(GO) test -short -bench=. -benchmem -count=10 -run=^$$ ./test/bench/... | tee "$$OUT"

bench-compare: ## Diff two baselines via benchstat (usage: make bench-compare OLD=<path> NEW=<path>)
	@if [ -z "$$OLD" ] || [ -z "$$NEW" ]; then \
	  echo "usage: make bench-compare OLD=<path> NEW=<path>" >&2; exit 2; \
	fi
	@command -v benchstat >/dev/null 2>&1 || { echo "benchstat not on PATH; install via: go install golang.org/x/perf/cmd/benchstat@latest" >&2; exit 2; }
	benchstat "$$OLD" "$$NEW"
