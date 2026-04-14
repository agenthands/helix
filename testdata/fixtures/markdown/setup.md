# Setup Guide

Instructions for setting up the project.

## Installation

Clone the repository and build:

```bash
git clone https://example.com/demo.git
cd demo
go build ./...
```

## Configuration

Create a config file at `~/.demo/config.yml`:

```yaml
debug: false
port: 8080
```

## Architecture Details

The project follows a layered architecture:

- **Transport**: HTTP and gRPC endpoints
- **Kernel**: Business logic and orchestration
- **Storage**: SQLite for persistence

## Troubleshooting

If the build fails, check your Go version with `go version`.
