.PHONY: build clean proto test vet fmt docs clean-jdtls-cache bench-jdtls-warm

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

clean-jdtls-cache: ## Wipe warm jdtls workspaces under $XDG_CACHE_HOME/serena-test/jdtls/
	@rm -rf "$${XDG_CACHE_HOME:-$$HOME/.cache}/serena-test/jdtls"
	@echo "cleared jdtls warm cache at $${XDG_CACHE_HOME:-$$HOME/.cache}/serena-test/jdtls"

bench-jdtls-warm: ## Run Java integration suite cold then warm; print both wall-clocks
	@$(MAKE) clean-jdtls-cache
	@echo "=== jdtls COLD run ==="
	@time $(GO) test -run 'Java' ./test/integration/... -count=1
	@echo "=== jdtls WARM run ==="
	@time $(GO) test -run 'Java' ./test/integration/... -count=1
