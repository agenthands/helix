.PHONY: build clean proto test vet fmt docs clean-jdtls-cache bench-jdtls-warm bench bench-baseline release-snapshot release-smoke update-trust-root

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
VETTOOL_NOKERNEL2SEMANTIC=$(shell go env GOPATH)/bin/vet-nokernel2semantic
VETTOOL_NOSEMANTIC2KERNEL=$(shell go env GOPATH)/bin/vet-nosemantic2kernel
VETTOOL_COMPACT_USES_STORE=$(shell go env GOPATH)/bin/vet-compact-uses-store

# Phase 61 ENRICH-01: enforce semantic does not import kernel (carve-out:
# internal/kernel/lspool). The vet-nosemantic2kernel singlechecker is the
# symmetric sibling of vet-nokernel2semantic; together the pair pin the
# kernel↔semantic boundary in BOTH directions on every `make vet` run.
#
# Phase 63 P63-02 Task 3: vet-compact-uses-store enforces the
# compact→store boundary — internal/semantic/compact MUST NOT import
# duckdb-go directly. Belt-and-braces over vet-noduckdb.
vet: $(VETTOOL) $(VETTOOL_NOKERNEL2SEMANTIC) $(VETTOOL_NOSEMANTIC2KERNEL) $(VETTOOL_COMPACT_USES_STORE)
	$(GO) vet ./...
	$(GO) vet -vettool=$(VETTOOL) ./...
	$(GO) vet -vettool=$(VETTOOL_NOKERNEL2SEMANTIC) ./...
	$(GO) vet -vettool=$(VETTOOL_NOSEMANTIC2KERNEL) ./...
	$(GO) vet -vettool=$(VETTOOL_COMPACT_USES_STORE) ./...

$(VETTOOL): cmd/vet-noduckdb/main.go internal/lint/noduckdb/*.go
	$(GO) install ./cmd/vet-noduckdb

$(VETTOOL_NOKERNEL2SEMANTIC): cmd/vet-nokernel2semantic/main.go internal/lint/nokernel2semantic/*.go
	$(GO) install ./cmd/vet-nokernel2semantic

$(VETTOOL_NOSEMANTIC2KERNEL): cmd/vet-nosemantic2kernel/main.go internal/lint/nosemantic2kernel/*.go
	$(GO) install ./cmd/vet-nosemantic2kernel

$(VETTOOL_COMPACT_USES_STORE): cmd/vet-compact-uses-store/main.go internal/lint/compactusesstore/*.go
	$(GO) install ./cmd/vet-compact-uses-store

fmt:
	gofmt -w .

docs: ## Regenerate tool and language tables in README.md
	$(GO) run ./cmd/docgen

sync-docs: ## Copy root GUARDRAILS.md / DoD.md into internal/kernel/help/docs/ (Phase 66 IN-05)
	@cp GUARDRAILS.md internal/kernel/help/docs/guardrails.md.tmp
	@printf '<!-- Synced from /GUARDRAILS.md; do not edit directly — edit the root copy and re-run `make sync-docs` -->\n\n' \
	  | cat - internal/kernel/help/docs/guardrails.md.tmp > internal/kernel/help/docs/guardrails.md
	@rm internal/kernel/help/docs/guardrails.md.tmp
	@cp DoD.md internal/kernel/help/docs/dod.md.tmp
	@printf '<!-- Synced from /DoD.md; do not edit directly — edit the root copy and re-run `make sync-docs` -->\n\n' \
	  | cat - internal/kernel/help/docs/dod.md.tmp > internal/kernel/help/docs/dod.md
	@rm internal/kernel/help/docs/dod.md.tmp
	@echo "synced GUARDRAILS.md and DoD.md to internal/kernel/help/docs/"

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
	@command -v zig >/dev/null 2>&1 || { \
	  echo "zig not installed (required for CGO=1 cross-compile of linux/windows targets); see CONTRIBUTING.md (Releasing). brew install zig (macOS) or apt install zig (Ubuntu 22.04+). Note: under split-runner architecture (D-15), the full 6-archive matrix only assembles in CI."; exit 1; }
	goreleaser release --snapshot --clean --skip=sign

release-smoke: ## Smoke-test the linux-amd64 release archive: extract, run daemon, hit MCP tools/list (D-04)
	@test -f dist/helix_v*_linux_amd64.tar.gz || { \
	  echo "dist/helix_v*_linux_amd64.tar.gz not found; run \`make release-snapshot\` first"; exit 1; }
	@# D-04 functional validation gate: linux-amd64 archive smoke harness.
	@# Extract the archive, boot the daemon over Streamable HTTP transport,
	@# poll /readyz for readiness, POST MCP `tools/list` to /mcp, assert
	@# tool count >= 41 (Helix profile per CLAUDE.md: 41+ MCP tools, including
	@# tree-sitter-backed semantic ops over 23 registered grammars). The
	@# daemon does NOT log per-grammar registration ("Registered grammar:"
	@# style messages); the substantive D-04 check is therefore the MCP
	@# tools/list response (a daemon that fails to register the 23
	@# tree-sitter grammars cannot serve the 41+ tools that depend on them).
	@# Note: the helix daemon process is launched here via `helix --serve`
	@# (the CLI is flat-flag-based per D-02; `helix daemon` is NOT a
	@# subcommand — `--serve` is the canonical "run as daemon directly"
	@# flag). Streamable HTTP listener is selected via `--http-addr`. Admin
	@# listener (`--admin-addr`) provides /readyz for boot synchronization.
	@set -e; \
	  TMPDIR_SMOKE="$$(mktemp -d -t helix-smoke-XXXXXX)"; \
	  PIDFILE="$$TMPDIR_SMOKE/daemon.pid"; \
	  LOGFILE="$$TMPDIR_SMOKE/daemon.log"; \
	  PORT=$$(awk 'BEGIN{srand(); print 38000 + int(rand()*2000)}'); \
	  ADMIN_PORT=$$(awk 'BEGIN{srand(); print 39000 + int(rand()*2000)}'); \
	  cleanup() { \
	    if [ -f "$$PIDFILE" ]; then \
	      kill "$$(cat $$PIDFILE)" 2>/dev/null || true; \
	      sleep 1; \
	      kill -9 "$$(cat $$PIDFILE)" 2>/dev/null || true; \
	    fi; \
	    rm -rf "$$TMPDIR_SMOKE"; \
	  }; \
	  trap cleanup EXIT INT TERM; \
	  ARCHIVE="$$(ls -1 dist/helix_v*_linux_amd64.tar.gz | head -1)"; \
	  echo "release-smoke: extracting $$ARCHIVE into $$TMPDIR_SMOKE"; \
	  tar -xzf "$$ARCHIVE" -C "$$TMPDIR_SMOKE"; \
	  BIN="$$TMPDIR_SMOKE/helix"; \
	  test -x "$$BIN" || { echo "release-smoke: FAIL — extracted helix binary not executable at $$BIN"; exit 1; }; \
	  echo "release-smoke: starting daemon on http=127.0.0.1:$$PORT admin=127.0.0.1:$$ADMIN_PORT"; \
	  ( "$$BIN" --serve --http-addr "127.0.0.1:$$PORT" --admin-addr "127.0.0.1:$$ADMIN_PORT" > "$$LOGFILE" 2>&1 & echo $$! > "$$PIDFILE" ); \
	  READY=0; \
	  for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do \
	    if ! kill -0 "$$(cat $$PIDFILE)" 2>/dev/null; then \
	      echo "release-smoke: FAIL — daemon exited before becoming ready; log:"; cat "$$LOGFILE"; exit 1; \
	    fi; \
	    if curl -sf "http://127.0.0.1:$$ADMIN_PORT/readyz" >/dev/null 2>&1; then READY=1; break; fi; \
	    sleep 1; \
	  done; \
	  if [ "$$READY" != "1" ]; then \
	    echo "release-smoke: FAIL — daemon /readyz never returned 200; last 100 log lines:"; tail -100 "$$LOGFILE"; exit 1; \
	  fi; \
	  echo "release-smoke: daemon ready; calling MCP initialize + tools/list"; \
	  INIT_REQ='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"release-smoke","version":"1"}}}'; \
	  INIT_RESP=$$(curl -sS -D "$$TMPDIR_SMOKE/init.headers" \
	    -H 'Content-Type: application/json' \
	    -H 'Accept: application/json, text/event-stream' \
	    -X POST "http://127.0.0.1:$$PORT/mcp" -d "$$INIT_REQ"); \
	  SESSION=$$(awk -F': ' 'tolower($$1)=="mcp-session-id"{gsub(/[\r\n]/,"",$$2); print $$2}' "$$TMPDIR_SMOKE/init.headers"); \
	  if [ -z "$$SESSION" ]; then \
	    echo "release-smoke: FAIL — no Mcp-Session-Id header from initialize; response:"; echo "$$INIT_RESP"; exit 1; \
	  fi; \
	  curl -sS -H 'Content-Type: application/json' \
	    -H 'Accept: application/json, text/event-stream' \
	    -H "Mcp-Session-Id: $$SESSION" \
	    -X POST "http://127.0.0.1:$$PORT/mcp" \
	    -d '{"jsonrpc":"2.0","method":"notifications/initialized"}' >/dev/null 2>&1 || true; \
	  TOOLS_REQ='{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'; \
	  TOOLS_RAW=$$(curl -sS -H 'Content-Type: application/json' \
	    -H 'Accept: application/json, text/event-stream' \
	    -H "Mcp-Session-Id: $$SESSION" \
	    -X POST "http://127.0.0.1:$$PORT/mcp" -d "$$TOOLS_REQ"); \
	  TOOLS_JSON=$$(printf '%s' "$$TOOLS_RAW" | awk '/^data: /{sub(/^data: /,""); print; exit} END{if(NR==0) exit} /^[[:space:]]*\{/{print; exit}'); \
	  if [ -z "$$TOOLS_JSON" ]; then TOOLS_JSON="$$TOOLS_RAW"; fi; \
	  TOOL_COUNT=$$(printf '%s' "$$TOOLS_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(len(d.get("result",{}).get("tools",[])))' 2>/dev/null || echo 0); \
	  echo "release-smoke: tools/list returned $$TOOL_COUNT tools (expecting >= 41 for Helix profile)"; \
	  if [ "$$TOOL_COUNT" -lt 41 ]; then \
	    echo "release-smoke: FAIL — expected >= 41 tools (which require all 23 tree-sitter grammars to be registered), got $$TOOL_COUNT"; \
	    echo "=== daemon log (last 80 lines) ==="; tail -80 "$$LOGFILE"; \
	    echo "=== tools/list response (first 4KB) ==="; printf '%s' "$$TOOLS_JSON" | head -c 4096; echo; \
	    exit 1; \
	  fi; \
	  echo "release-smoke: PASS — $$TOOL_COUNT tools registered, daemon healthy (D-04 gate satisfied; 23 tree-sitter grammars implicitly verified via tool registration)"

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
