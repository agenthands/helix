.PHONY: build clean proto test vet fmt test-stress

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
