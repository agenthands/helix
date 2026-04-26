.PHONY: build clean proto test vet fmt docs clean-jdtls-cache bench-jdtls-warm bench-capture bench-compare release-snapshot release-verify

BINARY=serena
GO=go

# Per D-16: inject version metadata into internal/cli vars via -ldflags so
# locally-built binaries report a meaningful --version. Falls back to the
# internal/cli defaults (2.0.0-dev / none / unknown) when git is unavailable.
# Ldflag target package matches .goreleaser.yml: github.com/postfix/serena/internal/cli.
VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo "2.0.0-dev")
COMMIT  ?= $(shell git rev-parse HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell git log -1 --format=%cI 2>/dev/null || echo "unknown")

LDFLAGS = -s -w \
	-X github.com/postfix/serena/internal/cli.Version=$(VERSION) \
	-X github.com/postfix/serena/internal/cli.Commit=$(COMMIT) \
	-X github.com/postfix/serena/internal/cli.Date=$(DATE)

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/serena

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

release-snapshot: ## Run goreleaser in snapshot mode (no signing, local validation only)
	@command -v goreleaser >/dev/null 2>&1 || { echo "goreleaser not on PATH; install via: brew install goreleaser OR go install github.com/goreleaser/goreleaser/v2@latest" >&2; exit 2; }
	goreleaser release --snapshot --clean

release-verify: ## Run two snapshots back-to-back and diff for reproducibility (D-07)
	@command -v goreleaser >/dev/null 2>&1 || { echo "goreleaser not on PATH; install via: brew install goreleaser OR go install github.com/goreleaser/goreleaser/v2@latest" >&2; exit 2; }
	@rm -rf /tmp/serena-dist1 /tmp/serena-dist2
	goreleaser release --snapshot --clean
	mv dist /tmp/serena-dist1
	goreleaser release --snapshot --clean
	mv dist /tmp/serena-dist2
	@diff -r \
	  --exclude='*.sig' \
	  --exclude='*.pem' \
	  --exclude='checksums.txt*' \
	  --exclude='artifacts.json' \
	  --exclude='metadata.json' \
	  --exclude='config.yaml' \
	  /tmp/serena-dist1 /tmp/serena-dist2 \
	  && echo "OK: reproducible (modulo metadata + signatures)" \
	  || { echo "FAIL: snapshot diff non-empty" >&2; exit 1; }
