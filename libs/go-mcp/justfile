# Build the library
build:
    go build ./...

# Run tests
test:
    go test -v ./...

# Run tests with coverage
test-coverage:
    go test -v -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out

# Lint code
lint:
    golangci-lint run

# Format code
fmt:
    go fmt ./...

# Generate documentation
docs:
    go doc -all

# Run example
example:
    go run examples/simple/main.go

# Clean build artifacts
clean:
    rm -f coverage.out
    go clean

# Run all checks (fmt, lint, test)
check: fmt lint test
