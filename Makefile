# consensus Proxy Makefile
# Automates building and testing

.PHONY: all build test stress benchmark clean install docker help \
	testnet-setup testnet-up testnet-down testnet-logs \
	testnet-lighthouse testnet-prysm testnet-nimbus testnet-teku testnet-erigon

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

# Build Docker image
docker:
	@echo "🐳 Building Docker image..."
	@docker build -t consensus-proxy:latest .
	@echo "✅ Docker image built: consensus-proxy:latest"

# Run with Docker
docker-run:
	@echo "🐳 Running with Docker..."
	@docker run -p 8080:8080 -v $(PWD)/config.toml:/app/config.toml consensus-proxy:latest

# ── Testnet: beacon node test environment ─────────────────────────────

NETWORK ?= holesky

# Generate JWT secret required by beacon nodes
testnet-setup:
	@./tests/setup.sh

# Start all beacon nodes and Geth execution client
testnet-up: testnet-setup
	@echo "🚀 Starting beacon node testnet ($(NETWORK))..."
	@NETWORK=$(NETWORK) docker compose -f tests/docker-compose.yaml up -d
	@echo "✅ Testnet running. Beacon API endpoints:"
	@echo "   Lighthouse  http://localhost:5052"
	@echo "   Prysm       http://localhost:3500"
	@echo "   Nimbus      http://localhost:5053"
	@echo "   Teku        http://localhost:5051"
	@echo "   Erigon      http://localhost:5555"
	@echo "   Geth RPC    http://localhost:8545"

# Start a single beacon node (+ Geth if needed). Erigon is self-contained.
testnet-lighthouse: testnet-setup
	@echo "🚀 Starting Lighthouse + Geth ($(NETWORK))..."
	@NETWORK=$(NETWORK) docker compose -f tests/docker-compose.yaml up -d lighthouse
	@echo "✅ Lighthouse  http://localhost:5052"

testnet-prysm: testnet-setup
	@echo "🚀 Starting Prysm + Geth ($(NETWORK))..."
	@NETWORK=$(NETWORK) docker compose -f tests/docker-compose.yaml up -d prysm
	@echo "✅ Prysm       http://localhost:3500"

testnet-nimbus: testnet-setup
	@echo "🚀 Starting Nimbus + Geth ($(NETWORK))..."
	@NETWORK=$(NETWORK) docker compose -f tests/docker-compose.yaml up -d nimbus
	@echo "✅ Nimbus      http://localhost:5053"

testnet-teku: testnet-setup
	@echo "🚀 Starting Teku + Geth ($(NETWORK))..."
	@NETWORK=$(NETWORK) docker compose -f tests/docker-compose.yaml up -d teku
	@echo "✅ Teku        http://localhost:5051"

testnet-erigon: testnet-setup
	@echo "🚀 Starting Erigon ($(NETWORK))..."
	@NETWORK=$(NETWORK) docker compose -f tests/docker-compose.yaml up -d erigon
	@echo "✅ Erigon      http://localhost:5555"

# Stop and remove all testnet containers and volumes
testnet-down:
	@echo "🛑 Stopping beacon node testnet..."
	@docker compose -f tests/docker-compose.yaml down -v
	@echo "✅ Testnet stopped"

# Tail logs for all testnet services (or a single service via SVC=)
testnet-logs:
	@docker compose -f tests/docker-compose.yaml logs -f $(SVC)

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
	@echo "  make build        Build the application"
	@echo "  make install      Install dependencies"
	@echo "  make docker       Build Docker image"
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
	@echo "Testnet (beacon nodes):"
	@echo "  make testnet-up              Start all beacon nodes + Geth"
	@echo "  make testnet-down            Stop and remove all testnet containers"
	@echo "  make testnet-logs            Tail logs for all services"
	@echo "  make testnet-logs SVC=prysm  Tail logs for a single service"
	@echo "  make testnet-setup           Generate JWT secret only"
	@echo ""
	@echo "  Single node (starts Geth automatically where needed):"
	@echo "  make testnet-lighthouse      Lighthouse  http://localhost:5052"
	@echo "  make testnet-prysm           Prysm       http://localhost:3500"
	@echo "  make testnet-nimbus          Nimbus      http://localhost:5053"
	@echo "  make testnet-teku            Teku        http://localhost:5051"
	@echo "  make testnet-erigon          Erigon      http://localhost:5555"
	@echo ""
	@echo "  Network (default: holesky):"
	@echo "  make testnet-up NETWORK=sepolia"
	@echo "  make testnet-lighthouse NETWORK=sepolia"
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
	@echo "  make build test                    # Build and test"
	@echo "  make benchmark                     # Benchmark with mock servers"
	@echo "  make benchmark-real                # Benchmark with real consensus nodes"
	@echo "  make stress-real                   # Stress test with real consensus nodes"
	@echo "  make docker docker-run            # Build and run container"
	@echo "  make install dev                  # Setup and start development"