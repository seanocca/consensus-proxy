# consensus Proxy Makefile
# Automates building and testing

.PHONY: all build test stress benchmark clean install docker-local docker-registry docker-run help

# Default target
all: build test

# Build the application
build:
	@echo "🔨 Building consensus proxy..."
	@go build -o bin/consensus-proxy -ldflags="-s -w" .
	@echo "✅ Build complete: bin/consensus-proxy"

# Install dependencies
install:
	@echo "📦 Installing dependencies..."
	@go mod download
	@go mod tidy
	@echo "✅ Dependencies installed"

# Run all tests (follows standard Go conventions)
test:
	@echo "🧪 Running test suite..."
	@go test -v ./...

# Run unit tests only (package-level tests)
test-unit:
	@echo "🧪 Running unit tests..."
	@go test -v ./cmd/...

# Run integration tests (system-level tests)
test-integration:
	@echo "🧪 Running integration tests..."
	@go test -v ./tests/

# Run benchmark tests
benchmark:
	@echo "⚡ Running benchmarks..."
	@go test -v ./tests/ -bench=. -benchmem -run="^$$"
	@echo "📊 Benchmark results saved to test-results/"

# Run benchmark tests with real consensus nodes
benchmark-real:
	@echo "⚡ Running benchmarks against real consensus nodes..."
	@CONSENSUS_PROXY_TEST_MODE=real go test -v ./tests/ -bench=. -benchmem -run="^$$"
	@echo "📊 Benchmark results with real nodes complete"

# Run stress tests
stress:
	@echo "💪 Running stress tests..."
	@go test -v ./tests/ -run "TestStressSuite"

# Run stress tests with real consensus nodes
stress-real:
	@echo "💪 Running stress tests against real consensus nodes..."
	@CONSENSUS_PROXY_TEST_MODE=real go test -v ./tests/ -run "TestStressSuite"

# Build the Registry Docker image
docker-registry:
	@echo "🐳 Building Docker image..."
	@docker build -t ghcr.io/seanocca/consensus-proxy:latest .
	@echo "✅ Docker image built: consensus-proxy:latest"

# Build the local Docker image
docker-local:
	@echo "🐳 Building Docker image..."
	@docker build -t consensus-proxy:latest .
	@echo "✅ Docker image built: consensus-proxy:latest"

# Run with Docker
docker-run:
	@echo "🐳 Running with Docker..."
	@docker run -p 8080:8080 -v $(PWD)/config.toml:/app/config.toml consensus-proxy:latest

# Clean build artifacts
clean:
	@echo "🧹 Cleaning up..."
	@rm -rf bin/
	@rm -rf test-results/
	@rm -f stress-test
	@docker rmi consensus-proxy:latest 2>/dev/null || true
	@echo "✅ Cleanup complete"

# Development server with hot reload
dev:
	@echo "🔄 Starting development server..."
	@go run main.go --config config.toml

# Validate configuration files
validate:
	@echo "✅ Validating configurations..."
	@go run -c "import('consensus-proxy/config'); cfg, err := config.Load('config.toml'); if err != nil { panic(err) }; println('TOML config valid')"
	@echo "✅ Configuration files are valid"

# Show help
help:
	@echo "consensus Proxy Makefile Commands:"
	@echo ""
	@echo "Building:"
	@echo "  make build        		Build the application"
	@echo "  make install      		Install dependencies"
	@echo "  make docker-local  	Build the local Docker image"
	@echo "  make docker-registry  	Build the Registry Docker image"
	@echo "  make docker-run   		Run the application with Docker"
	@echo ""
	@echo "Testing:"
	@echo "  make test              Run full test suite (all packages)"
	@echo "  make test-unit         Run unit tests only (package-level)"
	@echo "  make test-integration  Run integration tests (system-level)"
	@echo "  make benchmark         Run benchmark tests (mock servers)"
	@echo "  make benchmark-real    Run benchmark tests (real consensus nodes)"
	@echo "  make stress            Run stress tests (mock servers)"
	@echo "  make stress-real       Run stress tests (real consensus nodes)"
	@echo ""
	@echo "Development:"
	@echo "  make dev          Start development server"
	@echo "  make validate     Validate config files"
	@echo ""
	@echo "Utilities:"
	@echo "  make clean        Clean build artifacts"
	@echo "  make help         Show this help message"
	@echo ""
	@echo "Test Modes:"
	@echo "  By default, benchmark and stress tests use mock servers for isolation."
	@echo "  Use -real targets to test against actual consensus nodes from config.toml:"
	@echo "    CONSENSUS_PROXY_TEST_MODE=mock     # Use mock servers (default)"
	@echo "    CONSENSUS_PROXY_TEST_MODE=real     # Use real consensus nodes"
	@echo ""
	@echo "Examples:"
	@echo "  make build test                   # Build and test"
	@echo "  make benchmark                    # Benchmark with mock servers"
	@echo "  make benchmark-real               # Benchmark with real consensus nodes"
	@echo "  make stress-real                  # Stress test with real consensus nodes"
	@echo "  make docker-local docker-run      # Build and run container"
	@echo "  make install dev                  # Setup and start development"