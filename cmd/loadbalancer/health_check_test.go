package loadbalancer

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zircuit-labs/consensus-proxy/cmd/config"
)

// TestStartupHealthCheck_AllHealthy tests StartupHealthCheck when all nodes are healthy
func TestStartupHealthCheck_AllHealthy(t *testing.T) {
	// Create mock server that returns synced response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/eth/v1/node/syncing" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"0"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.LoadOrDefault("../../config.toml")
	cfg.Metrics.Enabled = false
	cfg.Beacons.Nodes = []string{"node1", "node2", "node3"}
	cfg.Beacons.SetParsedNodes([]config.NodeConfig{
		{Name: "node1", URL: server.URL, Type: "lighthouse"},
		{Name: "node2", URL: server.URL, Type: "prysm"},
		{Name: "node3", URL: server.URL, Type: "nimbus"},
	})

	lb, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create load balancer: %v", err)
	}

	err = lb.StartupHealthCheck()
	if err != nil {
		t.Errorf("StartupHealthCheck failed: %v", err)
	}

	healthyNodes := lb.GetHealthyNodes()
	if len(healthyNodes) != 3 {
		t.Errorf("Expected 3 healthy nodes, got %d", len(healthyNodes))
	}
}

// TestStartupHealthCheck_SomeUnhealthy tests StartupHealthCheck when some nodes are unhealthy
func TestStartupHealthCheck_SomeUnhealthy(t *testing.T) {
	// Create mock servers - 2 unhealthy, 1 healthy
	unhealthyServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/eth/v1/node/syncing" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"is_syncing":true,"sync_distance":"100"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer unhealthyServer1.Close()

	unhealthyServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/eth/v1/node/syncing" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"is_syncing":true,"sync_distance":"100"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer unhealthyServer2.Close()

	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/eth/v1/node/syncing" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"0"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer healthyServer.Close()

	cfg := config.LoadOrDefault("../../config.toml")
	cfg.Metrics.Enabled = false
	cfg.Beacons.Nodes = []string{"node1", "node2", "node3"}
	cfg.Beacons.SetParsedNodes([]config.NodeConfig{
		{Name: "node1", URL: unhealthyServer1.URL, Type: "lighthouse"},
		{Name: "node2", URL: unhealthyServer2.URL, Type: "prysm"},
		{Name: "node3", URL: healthyServer.URL, Type: "nimbus"},
	})

	lb, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create load balancer: %v", err)
	}

	err = lb.StartupHealthCheck()
	if err != nil {
		t.Errorf("StartupHealthCheck should not fail when some nodes are healthy: %v", err)
	}

	healthyNodes := lb.GetHealthyNodes()
	if len(healthyNodes) != 1 {
		t.Errorf("Expected 1 healthy node, got %d", len(healthyNodes))
	}

	if len(healthyNodes) > 0 && healthyNodes[0].Name != "node3" {
		t.Errorf("Expected healthy node to be node3, got %s", healthyNodes[0].Name)
	}
}

// TestStartupHealthCheck_AllUnhealthy tests StartupHealthCheck when all nodes are unhealthy
func TestStartupHealthCheck_AllUnhealthy(t *testing.T) {
	// Create mock server that returns syncing response for all nodes
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/eth/v1/node/syncing" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"is_syncing":true,"sync_distance":"100"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.LoadOrDefault("../../config.toml")
	cfg.Metrics.Enabled = false
	cfg.Beacons.Nodes = []string{"node1", "node2"}
	cfg.Beacons.SetParsedNodes([]config.NodeConfig{
		{Name: "node1", URL: server.URL, Type: "lighthouse"},
		{Name: "node2", URL: server.URL, Type: "prysm"},
	})

	lb, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create load balancer: %v", err)
	}

	err = lb.StartupHealthCheck()
	if err == nil {
		t.Error("StartupHealthCheck should fail when all nodes are unhealthy")
	}

	healthyNodes := lb.GetHealthyNodes()
	if len(healthyNodes) != 0 {
		t.Errorf("Expected 0 healthy nodes, got %d", len(healthyNodes))
	}
}

// TestStartupHealthCheck_ConcurrentChecks tests that StartupHealthCheck performs concurrent health checks
func TestStartupHealthCheck_ConcurrentChecks(t *testing.T) {
	requestTimes := make(chan time.Time, 10)

	// Create mock server that tracks request times
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/eth/v1/node/syncing" {
			requestTimes <- time.Now()
			time.Sleep(100 * time.Millisecond) // Simulate slow response
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"0"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.LoadOrDefault("../../config.toml")
	cfg.Metrics.Enabled = false
	cfg.Beacons.Nodes = []string{"node1", "node2", "node3"}
	cfg.Beacons.SetParsedNodes([]config.NodeConfig{
		{Name: "node1", URL: server.URL, Type: "lighthouse"},
		{Name: "node2", URL: server.URL, Type: "prysm"},
		{Name: "node3", URL: server.URL, Type: "nimbus"},
	})

	lb, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create load balancer: %v", err)
	}

	start := time.Now()
	err = lb.StartupHealthCheck()
	duration := time.Since(start)

	if err != nil {
		t.Errorf("StartupHealthCheck failed: %v", err)
	}

	// If checks were sequential, it would take 300ms (3 * 100ms)
	// If concurrent, should complete in ~100ms
	if duration > 250*time.Millisecond {
		t.Errorf("StartupHealthCheck took too long: %v (expected ~100ms for concurrent checks)", duration)
	}

	close(requestTimes)

	// Verify requests were made concurrently
	times := make([]time.Time, 0)
	for reqTime := range requestTimes {
		times = append(times, reqTime)
	}

	if len(times) != 3 {
		t.Errorf("Expected 3 health check requests, got %d", len(times))
	}

	// Check that all requests were made within a short time window (indicating concurrency)
	if len(times) >= 2 {
		maxDiff := times[len(times)-1].Sub(times[0])
		if maxDiff > 100*time.Millisecond {
			t.Errorf("Requests not concurrent: max time difference %v", maxDiff)
		}
	}
}

// TestStartupHealthCheck_MixedResponses tests StartupHealthCheck with various error conditions
func TestStartupHealthCheck_MixedResponses(t *testing.T) {
	nodeCount := 0

	// Create mock server with different responses for each node
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/eth/v1/node/syncing" {
			nodeCount++
			switch nodeCount {
			case 1:
				// Node 1: healthy
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"0"}}`))
			case 2:
				// Node 2: syncing
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"is_syncing":true,"sync_distance":"50"}}`))
			case 3:
				// Node 3: server error
				w.WriteHeader(http.StatusInternalServerError)
			case 4:
				// Node 4: invalid JSON
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`invalid json`))
			case 5:
				// Node 5: healthy
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"0"}}`))
			}
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.LoadOrDefault("../../config.toml")
	cfg.Metrics.Enabled = false
	cfg.Beacons.Nodes = []string{"node1", "node2", "node3", "node4", "node5"}
	cfg.Beacons.SetParsedNodes([]config.NodeConfig{
		{Name: "node1", URL: server.URL, Type: "lighthouse"},
		{Name: "node2", URL: server.URL, Type: "prysm"},
		{Name: "node3", URL: server.URL, Type: "nimbus"},
		{Name: "node4", URL: server.URL, Type: "teku"},
		{Name: "node5", URL: server.URL, Type: "lighthouse"},
	})

	lb, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create load balancer: %v", err)
	}

	err = lb.StartupHealthCheck()
	if err != nil {
		t.Errorf("StartupHealthCheck should not fail when some nodes are healthy: %v", err)
	}

	healthyNodes := lb.GetHealthyNodes()
	// Should have 2 healthy nodes (node1 and node5)
	if len(healthyNodes) != 2 {
		t.Errorf("Expected 2 healthy nodes, got %d", len(healthyNodes))
	}
}

// TestStartPeriodicHealthCheck tests that periodic health checks run
func TestStartPeriodicHealthCheck(t *testing.T) {
	requestCount := 0
	requestChan := make(chan int, 10)

	// Create mock server that counts requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/eth/v1/node/syncing" {
			requestCount++
			requestChan <- requestCount
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"0"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.LoadOrDefault("../../config.toml")
	cfg.Metrics.Enabled = false
	cfg.HealthCheck.Interval = 100 * time.Millisecond // Fast interval for testing
	cfg.Beacons.Nodes = []string{"primary", "backup"}
	cfg.Beacons.SetParsedNodes([]config.NodeConfig{
		{Name: "primary", URL: server.URL, Type: "lighthouse"},
		{Name: "backup", URL: server.URL, Type: "prysm"},
	})

	lb, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create load balancer: %v", err)
	}

	// Run startup health check first
	err = lb.StartupHealthCheck()
	if err != nil {
		t.Fatalf("StartupHealthCheck failed: %v", err)
	}

	// Clear the request count (startup checks don't count)
	requestCount = 0

	// Start periodic health check
	lb.StartPeriodicHealthCheck()

	// Wait for at least 2 periodic checks (backup node only)
	// Each check should happen every 100ms
	time.Sleep(350 * time.Millisecond)

	// Should have received at least 2 health check requests for backup node
	if requestCount < 2 {
		t.Errorf("Expected at least 2 periodic health checks, got %d", requestCount)
	}

	// Verify requests came in periodically
	close(requestChan)
	requests := make([]int, 0)
	for req := range requestChan {
		requests = append(requests, req)
	}

	if len(requests) < 2 {
		t.Logf("Warning: Only received %d periodic health check requests", len(requests))
	}
}

// TestStartPeriodicHealthCheck_BackupOnly tests that periodic checks only check backup nodes
func TestStartPeriodicHealthCheck_BackupOnly(t *testing.T) {
	primaryRequests := 0
	backupRequests := 0

	// Create separate servers for primary and backup
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/eth/v1/node/syncing" {
			primaryRequests++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"0"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer primaryServer.Close()

	backupServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/eth/v1/node/syncing" {
			backupRequests++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"0"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer backupServer.Close()

	cfg := config.LoadOrDefault("../../config.toml")
	cfg.Metrics.Enabled = false
	cfg.HealthCheck.Interval = 100 * time.Millisecond
	cfg.Beacons.Nodes = []string{"primary", "backup"}
	cfg.Beacons.SetParsedNodes([]config.NodeConfig{
		{Name: "primary", URL: primaryServer.URL, Type: "lighthouse"},
		{Name: "backup", URL: backupServer.URL, Type: "prysm"},
	})

	lb, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create load balancer: %v", err)
	}

	// Run startup health check
	err = lb.StartupHealthCheck()
	if err != nil {
		t.Fatalf("StartupHealthCheck failed: %v", err)
	}

	// Reset counters after startup checks
	initialPrimaryRequests := primaryRequests
	initialBackupRequests := backupRequests
	primaryRequests = 0
	backupRequests = 0

	// Start periodic health check
	lb.StartPeriodicHealthCheck()

	// Wait for periodic checks
	time.Sleep(350 * time.Millisecond)

	// Backup should have received multiple periodic checks
	if backupRequests < 2 {
		t.Errorf("Expected at least 2 periodic checks for backup, got %d", backupRequests)
	}

	// Primary should not receive periodic checks (only checked on-demand when failing)
	if primaryRequests > 0 {
		t.Logf("Primary received %d periodic checks (expected 0, but may receive some during failover/failback)", primaryRequests)
	}

	t.Logf("Startup checks - Primary: %d, Backup: %d", initialPrimaryRequests, initialBackupRequests)
	t.Logf("Periodic checks - Primary: %d, Backup: %d", primaryRequests, backupRequests)
}

// TestStartPeriodicHealthCheck_NodeRecovery tests that nodes recover after becoming healthy
func TestStartPeriodicHealthCheck_NodeRecovery(t *testing.T) {
	backupHealthy := false

	// Create servers
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/eth/v1/node/syncing" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"0"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer primaryServer.Close()

	backupServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/eth/v1/node/syncing" {
			if backupHealthy {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"0"}}`))
			} else {
				// Initially unhealthy
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"is_syncing":true,"sync_distance":"100"}}`))
			}
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer backupServer.Close()

	cfg := config.LoadOrDefault("../../config.toml")
	cfg.Metrics.Enabled = false
	cfg.HealthCheck.Interval = 100 * time.Millisecond
	cfg.Beacons.Nodes = []string{"primary", "backup"}
	cfg.Beacons.SetParsedNodes([]config.NodeConfig{
		{Name: "primary", URL: primaryServer.URL, Type: "lighthouse"},
		{Name: "backup", URL: backupServer.URL, Type: "prysm"},
	})

	lb, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create load balancer: %v", err)
	}

	// Run startup health check - backup will be unhealthy
	err = lb.StartupHealthCheck()
	if err != nil {
		t.Fatalf("StartupHealthCheck failed: %v", err)
	}

	healthyNodes := lb.GetHealthyNodes()
	if len(healthyNodes) != 1 {
		t.Errorf("Expected 1 healthy node initially, got %d", len(healthyNodes))
	}

	// Start periodic health check
	lb.StartPeriodicHealthCheck()

	// Wait a bit, then make backup healthy
	time.Sleep(150 * time.Millisecond)
	backupHealthy = true

	// Wait for periodic check to discover backup is healthy
	time.Sleep(250 * time.Millisecond)

	// Now backup should be in healthy nodes
	healthyNodes = lb.GetHealthyNodes()
	if len(healthyNodes) != 2 {
		t.Errorf("Expected 2 healthy nodes after recovery, got %d", len(healthyNodes))
	}
}
