# sx justfile (Go)

set positional-arguments

# === Default ===

# List available commands
default:
    @just --list

# === Build ===

# Build debug binary
build:
    go build -o sx .

# Build release binary (stripped, optimized)
build-release:
    go build -ldflags="-s -w" -o sx .

# Fast compile check
check:
    go vet ./...

# === Test ===

# Run tests
test:
    go test ./...

# Run tests with verbose output
test-v:
    go test -v ./...

# Run tests with coverage
test-cover:
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html

# === Lint & Format ===

# Format code
fmt:
    gofmt -w .

# Check formatting
fmt-check:
    @test -z "$(gofmt -l .)" || (echo "Files need formatting:" && gofmt -l . && exit 1)

# Run static analysis
lint:
    go vet ./...

# === Install ===

# Install to $GOPATH/bin
install:
    go install .

# === Clean ===

# Clean build artifacts
clean:
    rm -f sx coverage.out coverage.html
    go clean

# === Dependencies ===

# Update dependencies
update:
    go get -u ./...
    go mod tidy

# Tidy module
tidy:
    go mod tidy

# === Development ===

# Run with arguments
run *args:
    go run . {{args}}

# === Release ===

# Build release with goreleaser (dry run)
release-dry:
    goreleaser release --snapshot --clean

# Build release with goreleaser
release:
    goreleaser release --clean
