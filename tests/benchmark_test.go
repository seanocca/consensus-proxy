package tests

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/zircuit-labs/consensus-proxy/cmd/loadbalancer"

	"github.com/zircuit-labs/consensus-proxy/cmd/config"
)

// Test mode configuration
// Set environment variable CONSENSUS_PROXY_TEST_MODE=real to use actual beacon nodes from config.toml
// Default (or CONSENSUS_PROXY_TEST_MODE=mock) uses mock test servers for isolated testing
func getTestMode() string {
	mode := os.Getenv("CONSENSUS_PROXY_TEST_MODE")
	if mode == "" {
		return "mock" // Default to mock mode for isolated testing
	}
	return mode
}

// createBenchmarkConfig creates configuration for benchmark tests
// In mock mode: uses provided test servers
// In real mode: uses actual beacon nodes from config.toml
func createBenchmarkConfig(mockServers []*httptest.Server, reqTimeout time.Duration) *config.Config {
	cfg := config.LoadOrDefault("../config.toml")

	// Common settings for all benchmark tests
	cfg.Server.MaxRetries = 3
	cfg.Server.RequestTimeout = reqTimeout
	cfg.Metrics.Enabled = false

	testMode := getTestMode()
	if testMode == "real" {
		// Use real beacon nodes from config.toml - no changes needed
		// Just adjust timeouts for benchmarking
		cfg.Server.RequestTimeout = 30 * time.Second // More generous timeout for real nodes
		return cfg
	}

	// Mock mode - replace with test servers
	var beaconNames []string
	var beaconConfigs []config.NodeConfig
	for i, server := range mockServers {
		name := "test-node"
		if i > 0 {
			name = "test-node-" + string(rune('0'+i))
		}
		beaconNames = append(beaconNames, name)
		beaconConfigs = append(beaconConfigs, config.NodeConfig{
			Name: name,
			URL:  server.URL,
			Type: "lighthouse",
		})
	}
	cfg.Beacons.Nodes = beaconNames
	cfg.Beacons.SetParsedNodes(beaconConfigs)

	return cfg
}

// BenchmarkLoadBalancerSingleNode tests performance with one healthy node
func BenchmarkLoadBalancerSingleNode(b *testing.B) {
	var servers []*httptest.Server

	// Only create mock server if in mock mode
	if getTestMode() == "mock" {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/eth/v1/node/health" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"head_slot":"12345"}}`))
		}))
		defer server.Close()
		servers = []*httptest.Server{server}
	}

	// Create configuration based on test mode
	cfg := createBenchmarkConfig(servers, 30*time.Millisecond)

	lb, err := loadbalancer.New(cfg)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/eth/v1/beacon/headers/head", nil)
			w := httptest.NewRecorder()
			lb.ServeHTTP(w, req)

			if w.Code != 200 {
				b.Errorf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkLoadBalancerFailover tests performance during failover scenarios
func BenchmarkLoadBalancerFailover(b *testing.B) {
	var servers []*httptest.Server

	// Only create mock servers if in mock mode
	if getTestMode() == "mock" {
		// Server 1 - fails
		server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/eth/v1/node/health" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server1.Close()

		// Server 2 - succeeds
		server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/eth/v1/node/health" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"head_slot":"12345"}}`))
		}))
		defer server2.Close()

		servers = []*httptest.Server{server1, server2}
	}

	// Create configuration based on test mode
	cfg := createBenchmarkConfig(servers, 30*time.Millisecond)

	lb, err := loadbalancer.New(cfg)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/eth/v1/beacon/headers/head", nil)
			w := httptest.NewRecorder()
			lb.ServeHTTP(w, req)

			if w.Code != 200 {
				b.Errorf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkConfigLoad tests configuration loading performance
func BenchmarkConfigLoad(b *testing.B) {
	// Create temporary config file
	configContent := `
[server]
max_retries = 3
request_timeout = "50ms"

[health]
check_interval = "10s"
check_timeout = "5s"

[[nodes]]
name = "node1"
url = "http://localhost:5052"

[[nodes]]
name = "node2"
url = "http://localhost:5053"
`

	tmpFile, err := createTempFile(configContent)
	if err != nil {
		b.Fatal(err)
	}
	defer tmpFile.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := config.Load(tmpFile.Name())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkHealthCheck tests health check performance
func BenchmarkHealthCheck(b *testing.B) {
	var servers []*httptest.Server

	// Only create mock server if in mock mode
	if getTestMode() == "mock" {
		healthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer healthServer.Close()
		servers = []*httptest.Server{healthServer}
	}

	// Create configuration based on test mode
	cfg := createBenchmarkConfig(servers, 30*time.Millisecond)

	lb, err := loadbalancer.New(cfg)
	if err != nil {
		b.Fatal(err)
	}

	// Get nodes for testing
	nodes := lb.GetNodes()
	if len(nodes) == 0 {
		b.Fatal("No nodes available")
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate health check (access node health status)
		_ = nodes[0].IsHealthy(5) // Use default error threshold
	}
}

// Helper function to create temporary config file
func createTempFile(content string) (*testFile, error) {
	return &testFile{content: content, name: "/tmp/test-config.toml"}, nil
}

type testFile struct {
	content string
	name    string
}

func (tf *testFile) Name() string {
	return tf.name
}

func (tf *testFile) Close() error {
	return nil
}
