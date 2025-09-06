# Simple Data Processing Engine Makefile

.PHONY: help build test benchmark demo clean install deps fmt lint docker

# Default target
help:
	@echo "🚀 Simple Data Processing Engine"
	@echo "================================="
	@echo ""
	@echo "Available targets:"
	@echo "  build      - Build all binaries"
	@echo "  test       - Run all tests"
	@echo "  benchmark  - Run performance benchmarks"
	@echo "  demo       - Run the demo application"
	@echo "  clean      - Clean build artifacts"
	@echo "  install    - Install dependencies"
	@echo "  deps       - Download dependencies"
	@echo "  fmt        - Format Go code"
	@echo "  lint       - Run linters"
	@echo "  docker     - Build Docker image"
	@echo "  coverage   - Generate test coverage report"
	@echo ""

# Build targets
build:
	@echo "🔨 Building binaries..."
	@go build -o bin/demo ./cmd/demo
	@go build -o bin/benchmark ./cmd/benchmark
	@echo "✅ Build complete!"

# Test targets
test:
	@echo "🧪 Running tests..."
	@go test -v ./...
	@echo "✅ Tests complete!"

benchmark:
	@echo "⚡ Running benchmarks..."
	@go test -bench=. -benchmem ./...
	@echo "✅ Benchmarks complete!"

coverage:
	@echo "📊 Generating coverage report..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report generated: coverage.html"

# Run targets
demo:
	@echo "🚀 Starting demo..."
	@go run ./cmd/demo

benchmark-run:
	@echo "⚡ Starting performance benchmark..."
	@go run ./cmd/benchmark

# Development targets
deps:
	@echo "📦 Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "✅ Dependencies updated!"

install: deps
	@echo "🔧 Installing tools..."
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "✅ Tools installed!"

fmt:
	@echo "🎨 Formatting code..."
	@go fmt ./...
	@goimports -w .
	@echo "✅ Code formatted!"

lint:
	@echo "🔍 Running linters..."
	@golangci-lint run
	@echo "✅ Linting complete!"

# Clean targets
clean:
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@go clean -cache
	@echo "✅ Clean complete!"

# Docker targets
docker:
	@echo "🐳 Building Docker image..."
	@docker build -t simple-data-processing-engine .
	@echo "✅ Docker image built!"

docker-run:
	@echo "🐳 Running Docker container..."
	@docker run --rm -p 8080:8080 simple-data-processing-engine

# Development workflow
dev: clean deps fmt lint test build
	@echo "🎉 Development build complete!"

# Production build
release: clean deps test build
	@echo "🚀 Release build complete!"

# Performance testing
perf: build
	@echo "📈 Running performance tests..."
	@./bin/benchmark
	@echo "✅ Performance tests complete!"

# All-in-one targets
all: deps fmt lint test benchmark build
	@echo "🎊 All tasks complete!"

check: fmt lint test
	@echo "✅ Pre-commit checks passed!"
