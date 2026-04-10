.PHONY: build clean proto test vet fmt docs

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
