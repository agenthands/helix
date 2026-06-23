.PHONY: build clean proto test vet fmt docs verify-cligen verify-docs clean-jdtls-cache bench-jdtls-warm bench-micro bench-baseline bench bench-quick bench-aider-edit release-snapshot release-smoke update-trust-root eval eval-quick eval-no-network eval-attestation-check validate-cost-table verify-tos verify-licenses verify-no-docker-sdk verify-verified-md

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
VETTOOL_ABLATION_LEAKAGE=$(shell go env GOPATH)/bin/vet-ablation-leakage
VETTOOL_BENCH_RAG_LEAKAGE=$(shell go env GOPATH)/bin/vet-bench-rag-leakage

# Phase 61 ENRICH-01: enforce semantic does not import kernel (carve-out:
# internal/kernel/lspool). The vet-nosemantic2kernel singlechecker is the
# symmetric sibling of vet-nokernel2semantic; together the pair pin the
# kernel↔semantic boundary in BOTH directions on every `make vet` run.
#
# Phase 63 P63-02 Task 3: vet-compact-uses-store enforces the
# compact→store boundary — internal/semantic/compact MUST NOT import
# duckdb-go directly. Belt-and-braces over vet-noduckdb.
#
# Phase 76 ABLATE-08: vet-ablation-leakage enforces the ablation import
# boundary — the bench-runner namespace (github.com/agenthands/helix/bench/
# runners) MUST NOT import the disabled-subsystem packages
# internal/kernel/lspool or internal/semantic/store. Static, compile-time
# complement to the kernel Unsupported runtime guard (Plan 76-01).
#
# Phase 83 ABLATE-04 #1c: vet-bench-rag-leakage enforces the standalone
# baseline_rag control-arm boundary — cmd/helix-bench-rag MUST NOT import
# internal/kernel or internal/semantic (transitively the same as not importing
# internal/mcp). Static, compile-time complement to the dynamic transitive
# import-set test in cmd/helix-bench-rag/leakage_test.go.
vet: verify-no-docker-sdk $(VETTOOL) $(VETTOOL_NOKERNEL2SEMANTIC) $(VETTOOL_NOSEMANTIC2KERNEL) $(VETTOOL_COMPACT_USES_STORE) $(VETTOOL_ABLATION_LEAKAGE) $(VETTOOL_BENCH_RAG_LEAKAGE)
	$(GO) vet ./...
	$(GO) vet -vettool=$(VETTOOL) ./...
	$(GO) vet -vettool=$(VETTOOL_NOKERNEL2SEMANTIC) ./...
	$(GO) vet -vettool=$(VETTOOL_NOSEMANTIC2KERNEL) ./...
	$(GO) vet -vettool=$(VETTOOL_COMPACT_USES_STORE) ./...
	$(GO) vet -vettool=$(VETTOOL_ABLATION_LEAKAGE) ./...
	$(GO) vet -vettool=$(VETTOOL_BENCH_RAG_LEAKAGE) ./...

$(VETTOOL): cmd/vet-noduckdb/main.go internal/lint/noduckdb/*.go
	$(GO) install ./cmd/vet-noduckdb

$(VETTOOL_NOKERNEL2SEMANTIC): cmd/vet-nokernel2semantic/main.go internal/lint/nokernel2semantic/*.go
	$(GO) install ./cmd/vet-nokernel2semantic

$(VETTOOL_NOSEMANTIC2KERNEL): cmd/vet-nosemantic2kernel/main.go internal/lint/nosemantic2kernel/*.go
	$(GO) install ./cmd/vet-nosemantic2kernel

$(VETTOOL_COMPACT_USES_STORE): cmd/vet-compact-uses-store/main.go internal/lint/compactusesstore/*.go
	$(GO) install ./cmd/vet-compact-uses-store

$(VETTOOL_ABLATION_LEAKAGE): cmd/vet-ablation-leakage/main.go internal/lint/ablationleakage/*.go
	$(GO) install ./cmd/vet-ablation-leakage

$(VETTOOL_BENCH_RAG_LEAKAGE): cmd/vet-bench-rag-leakage/main.go internal/lint/benchragleakage/*.go
	$(GO) install ./cmd/vet-bench-rag-leakage

fmt:
	gofmt -w .

docs: ## Regenerate tool and language tables in README.md
	$(GO) run ./cmd/docgen

reference: ## Regenerate internal/cli/skills/helix/reference.md from the live tool registry
	$(GO) run ./cmd/helix-refgen

verify-cligen: ## HARD-FAIL drift gate: internal/cli/verbs_gen.go must match the live tool registry (VERB-02)
	$(GO) run ./cmd/helix-cligen --check

verify-docs: ## HARD-FAIL drift gate: README.md tool table must match the live registry (DOCS-02)
	$(GO) run ./cmd/docgen --check

verify-reference: ## HARD-FAIL drift gate: internal/cli/skills/helix/reference.md must match the live registry (REF-03)
	$(GO) run ./cmd/helix-refgen --check

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

# bench-micro: the Phase 64 Go microbenchmark suite (formerly `make bench`).
# RENAMED in Phase 77 (BENCH-05, RESEARCH Pitfall 1): the `bench` target name was
# reclaimed by the v1.12 milestone bench stack (`cmd/helix-bench run`). The original
# microbench recipe is preserved verbatim here under the unambiguous `bench-micro`
# name. `make bench-baseline` still captures a local baseline from this same recipe.
bench-micro: ## Run the Go microbenchmark suite once and print results to stdout (formerly `make bench`)
	$(GO) test -short -bench=. -benchmem -count=10 -run=^$$ ./test/bench/...

bench-baseline: ## Capture a local microbench baseline into test/bench/baselines/local.txt (gitignored, overwrites)
	$(GO) test -short -bench=. -benchmem -count=10 -run=^$$ ./test/bench/... | tee test/bench/baselines/local.txt

# ─── v1.12 milestone bench stack (BENCH-05) ────────────────────────────────────
# `bench`, `bench-quick`, and `bench SUITE=<suite>` invoke `cmd/helix-bench run`
# (NOT the Go microbench — that is now `bench-micro`). These mirror the eval-quick /
# eval local-only, no-network discipline: the scripted agent makes zero external
# network calls and uses no API key (D-01). See bench/BENCH.md for the operator
# contract and the result.v2 provenance key names.
#
# SUITE selects the benchmark suite (the `bench-<suite>` parameterization, RESEARCH
# Open Question 1): `make bench SUITE=internal-toolbench`. Defaults to internal-toolbench.
SUITE ?= internal-toolbench

bench: ## Run the milestone bench suite via cmd/helix-bench (use SUITE=<suite>; default internal-toolbench)
	$(GO) run ./cmd/helix-bench run --benchmarks=$(SUITE)

# bench-quick: the hermetic scripted CI smoke gate (BENCH-05 criterion #2, <=90s,
# >=1 task succeeds). Builds the helix daemon binary FIRST (RESEARCH build-sequencing
# note: the subprocess-daemon / forwarder-drive path SKIPs if `helix` is absent),
# then runs the scripted `your_agent_full` smoke on the single IT-go-patch-apply-1
# seed task. Local-only, no network, no API key (scripted agent, D-01). The absolute
# --helix-bin ensures the daemon resolves from the per-cell ephemeral scratch cwd.
bench-quick: ## Build helix, then run the hermetic scripted bench smoke (<=90s CI gate)
	$(GO) build -o $(BINARY) ./cmd/helix
	$(GO) run ./cmd/helix-bench run \
		--benchmarks=$(SUITE) \
		--languages=go \
		--modes=your_agent_full \
		--tasks=IT-go-patch-apply-1 \
		--agent=scripted \
		--helix-bin=$(CURDIR)/$(BINARY) \
		--out bench/reports

bench-aider-edit: ## Build helix, then regenerate the committed byte-reproducible polyglot-edit baseline (BASELINE-01)
	# Local-only regenerate path (no CI benchstat gate — STATE.md local-only-benches).
	# cmd/helix-bench run does not route the aider-polyglot fixtures through the matrix
	# (the aider fixtures use a different on-disk layout than cellSeedDir expects), so
	# the regenerator drives the smallest entrypoint that exercises runAiderEditCell
	# directly: the //go:build ignore script bench/runtime/aider_edit_baseline_regen.go.
	# It drives the DETERMINISTIC scripted arm (applies the exercism reference solution
	# via replace_in_file → guaranteed PASS) over go/wordy against the warm daemon and
	# writes the deterministic-metrics-only result.v2.json + BENCH-RESULTS.md into the
	# force-tracked bench/reports/aider-edit-baseline/ dir. The committed bytes are the
	# CI contract; re-running this regenerates them byte-identically.
	$(GO) build -o $(BINARY) ./cmd/helix
	HELIX_BIN="$(CURDIR)/$(BINARY)" $(GO) run bench/runtime/aider_edit_baseline_regen.go

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

# eval-quick: in-process scripted-agent harness validation. <30s wall-time for
# the full 9-fixture set (D-05 target "~10"; 06a ships 2, 06b adds 7). Runs on
# every PR as a CI gate. inprocess_fixtures_test.go asserts >= 9.
#
# ╔══════════════════════════════════════════════════════════════════════════╗
# ║  PITFALL 6 — CRITICAL WARNING (T-67-Pitfall-6, four-layer mitigation)   ║
# ║                                                                          ║
# ║  eval-quick uses a SCRIPTED AGENT that replays hard-coded MCP call       ║
# ║  sequences. It does NOT measure real Claude Code agent behavior.         ║
# ║                                                                          ║
# ║  What eval-quick validates:                                              ║
# ║    ✓ Harness wiring (daemon startup, profile loading, tool registry)     ║
# ║    ✓ Scorer rule parsing and application                                 ║
# ║    ✓ Report generation (5 EVAL-04 report files)                          ║
# ║                                                                          ║
# ║  What eval-quick does NOT validate:                                      ║
# ║    ✗ Whether a real LLM agent chooses the right tools                    ║
# ║    ✗ Agent cost, reasoning, or error recovery behavior                   ║
# ║    ✗ Tool output quality or semantic correctness                         ║
# ║                                                                          ║
# ║  Use 'make eval' for real agent behavior measurements (nightly/release). ║
# ║  Never report eval-quick results as agent-behavior evidence.             ║
# ║                                                                          ║
# ║  D-05 BUDGET: 9-fixture set target <30s wall-time on CI runners          ║
# ║  (D-05 narrative says "~10"; 06a ships 2 + 06b adds 7 = 9 actual).       ║
# ╚══════════════════════════════════════════════════════════════════════════╝
eval-quick:
	go run ./cmd/helix-eval run --quick --corpus eval/fixtures --out eval/reports

# eval-no-network: alias for eval-quick; use this when you want to be explicit
# that no external network calls are made (scripted agent, no real Anthropic API).
# Identical to eval-quick; both are the only PR-gating eval targets.
eval-no-network: eval-quick

# eval: full out-of-process matrix. Local-only; nightly CI only.
# NEVER add this to PR-gating workflows (project rule: benchmarks local-only).
eval:
	go run ./cmd/helix-eval run --corpus eval/corpus --mode baseline --mode native --mode semantic --mode semantic_guarded --out eval/reports

# eval-attestation-check: warn-only date-staleness check for eval/EVAL.md.
# Per project rule "benchmarks local-only", this is hygiene, not a build gate.
# Default mode (no --strict) ALWAYS exits 0; prints WARNING/ERROR to stderr if
# the attestation date is >180 days old or unparseable. Wired into CI as a
# warn-only step (continue-on-error: true) — never blocks merges.
eval-attestation-check:
	go run ./cmd/eval-attestation-check eval/EVAL.md

# validate-cost-table: HARD-FAIL strict validator for bench/datasets/cost-table.yaml
# (COST-01/D-13/D-16). Exits NON-ZERO on an unknown key, an unparseable date, a
# past valid_until, or a last_verified more than 90 days stale. This is a build
# gate — NO continue-on-error (the deliberate inversion of eval-attestation-check).
validate-cost-table:
	go run ./cmd/helix-bench validate-cost-table bench/datasets/cost-table.yaml

# verify-tos: HARD-FAIL freshness gate for bench/PROVIDERS.md TOS attestations
# (D-14/D-16). Exits NON-ZERO on a malformed/unknown-key frontmatter block, a
# missing required field, or an attested_on more than 90 days stale. Build gate —
# NO continue-on-error. Checks freshness/parse validity only, not legal accuracy.
verify-tos:
	go run ./cmd/helix-bench verify-tos bench/PROVIDERS.md

# verify-licenses: HARD-FAIL dual-disposition license gate for the Aider-Polyglot
# vendored tree (SC#4 / ADAPTER-AIDER-01 / VENDOR-03). Strict-decodes every MIT
# `track:` block AND every Apache-2.0 `fixture:` block in
# bench/datasets/aider-polyglot/LICENSE-AUDIT.md (KnownFields(true) — unknown key
# → non-zero exit), asserts a non-empty SPDX license + sha256 per track, requires
# >=1 Apache-2.0 fixture block (dual disposition — a regression to single
# disposition fails closed), and runs a BIDIRECTIONAL manifest-vs-disk
# crypto/sha256 walk over the full vendored fixtures/ tree: every on-disk file
# must have a VENDOR-MANIFEST.md row with a matching digest, and every manifest
# row must exist on disk (a swapped byte, a dropped/extra entry, or a removed
# NOTICE/LICENSE sidecar all hard-fail). Build gate — NO continue-on-error.
# Checks structural validity + digest integrity only, not legal accuracy.
verify-licenses:
	go run ./cmd/helix-bench verify-licenses bench/datasets/aider-polyglot/LICENSE-AUDIT.md --manifest bench/datasets/aider-polyglot/VENDOR-MANIFEST.md --tree bench/datasets/aider-polyglot/fixtures

# verify-no-docker-sdk: HARD-FAIL supply-chain gate (CONTAINER-01 / SC#1). The
# bench container stack MUST drive docker/podman purely via os/exec — the Docker
# Go SDK (github.com/docker/docker) must never appear in go.mod. On a match this
# prints a CI ::error:: and exits NON-ZERO; on no match the recipe succeeds (the
# grep exit-1 is swallowed by the `if` so the absence case is the pass).
#
# The pattern is anchored to `github.com/docker/docker` followed by whitespace
# (the go.mod module/version separator) so it matches ONLY the Engine SDK module
# line, not legitimately-distinct sibling modules whose path is a superstring —
# e.g. github.com/docker/docker-credential-helpers, which go-containerregistry's
# default registry-auth keychain pulls in transitively (84-03). Those are
# registry-credential helpers, not the Docker Engine SDK; banning them would be
# a false positive. The Engine SDK line is always `github.com/docker/docker vX`.
verify-no-docker-sdk:
	@if grep -Eq 'github\.com/docker/docker[[:space:]]' go.mod; then \
		echo "::error::SC#1 violation: github.com/docker/docker present in go.mod"; \
		exit 1; \
	fi

# verify-verified-md: HARD-FAIL doc gate for BOTH VERIFIED.md acceptance
# artifacts. It asserts each file exists and carries every required `## ` section
# header documenting that gate's contract, exiting NON-ZERO when a file is missing
# or any required header is absent — keeping the documented contracts honest
# (T-86-02-03 / T-87-03-04). Build gate — NO continue-on-error. Each header match
# is anchored to `^## <name>$` so only true section headers count, not prose
# mentions of the same word elsewhere in the doc.
#
#   * bench/evaluators/VERIFIED.md            (VERIFIED-03 completion gate):
#       Oracles Threshold Abstain Tokenizer EditSimilarity Proof
#   * bench/evaluators/swebench/VERIFIED.md   (VERIFIED-01/02 SWE-bench gate):
#       Oracles Conditions Abstain Differential Proof
verify-verified-md:
	@set -e; \
	check() { \
		f="$$1"; shift; \
		if [ ! -f "$$f" ]; then \
			echo "::error::VERIFIED.md violation: $$f is missing"; \
			exit 1; \
		fi; \
		for h in "$$@"; do \
			if ! grep -Eq "^## $$h$$" "$$f"; then \
				echo "::error::VERIFIED.md violation: $$f is missing required section '## $$h'"; \
				exit 1; \
			fi; \
		done; \
		echo "verify-verified-md: $$f — all required sections present"; \
	}; \
	check bench/evaluators/VERIFIED.md Oracles Threshold Abstain Tokenizer EditSimilarity Proof; \
	check bench/evaluators/swebench/VERIFIED.md Oracles Conditions Abstain Differential Proof
