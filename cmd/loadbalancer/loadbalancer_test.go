package loadbalancer

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zircuit-labs/consensus-proxy/cmd/config"
)

func TestLoadBalancerFailover(t *testing.T) {
	// Create mock beacon nodes
	var server1Requests, server2Requests int

	// Server 1 - returns error for regular requests but OK for health checks
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server1Requests++
		if r.URL.Path == "/eth/v1/node/syncing" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"0"}}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server1.Close()

	// Server 2 - returns success
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server2Requests++
		if r.URL.Path == "/eth/v1/node/syncing" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"0"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": {"head_slot": "12345"}}`))
	}))
	defer server2.Close()

	// Load the standard config.toml and modify for testing
	cfg := config.LoadOrDefault("../../config.toml")

	// Override settings for failover testing
	cfg.Server.MaxRetries = 3
	cfg.Server.RequestTimeout = 100 * time.Millisecond
	cfg.Metrics.Enabled = false

	// Replace nodes with test servers using beacons configuration
	cfg.Beacons.Nodes = []string{"primary", "backup"}
	cfg.Beacons.SetParsedNodes([]config.NodeConfig{
		{Name: "primary", URL: server1.URL, Type: "lighthouse"},
		{Name: "backup", URL: server2.URL, Type: "lighthouse"},
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

	// Create test request
	req := httptest.NewRequest("GET", "/eth/v1/beacon/headers/head", nil)
	w := httptest.NewRecorder()

	// Measure response time
	start := time.Now()
	lb.ServeHTTP(w, req)
	duration := time.Since(start)

	// Validate response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "head_slot") {
		t.Errorf("Expected response from server2, got: %s", w.Body.String())
	}

	// Validate failover occurred
	if server1Requests == 0 {
		t.Errorf("Expected server1 to receive request (for failover test)")
	}

	if server2Requests == 0 {
		t.Errorf("Expected server2 to receive request (successful response)")
	}

	// Validate response time
	if duration > 100*time.Millisecond {
		t.Errorf("Response took too long: %v (expected < 100ms)", duration)
	}

	t.Logf("Failover completed in %v", duration)
}

func TestLoadBalancerHealthyNodeFirst(t *testing.T) {
	var server1Requests, server2Requests int

	// Server 1 - healthy, returns success immediately
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server1Requests++
		if r.URL.Path == "/eth/v1/node/syncing" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"0"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": {"head_slot": "12345"}}`))
	}))
	defer server1.Close()

	// Server 2 - should not receive requests if server1 is healthy
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server2Requests++
		if r.URL.Path == "/eth/v1/node/syncing" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"0"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": {"head_slot": "67890"}}`))
	}))
	defer server2.Close()

	// Load the standard config.toml and modify for testing
	cfg := config.LoadOrDefault("../../config.toml")

	// Override settings for healthy node testing
	cfg.Server.MaxRetries = 3
	cfg.Server.RequestTimeout = 30 * time.Millisecond // Sub-50ms target
	cfg.Metrics.Enabled = false

	// Replace nodes with test servers using beacons configuration
	cfg.Beacons.Nodes = []string{"primary", "backup"}
	cfg.Beacons.SetParsedNodes([]config.NodeConfig{
		{Name: "primary", URL: server1.URL, Type: "lighthouse"},
		{Name: "backup", URL: server2.URL, Type: "lighthouse"},
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

	// Make multiple requests
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/eth/v1/beacon/headers/head", nil)
		w := httptest.NewRecorder()

		start := time.Now()
		lb.ServeHTTP(w, req)
		duration := time.Since(start)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i, w.Code)
		}

		if duration > 50*time.Millisecond {
			t.Errorf("Request %d took too long: %v", i, duration)
		}
	}

	// Validate that only server1 received requests
	// server1 should receive 1 health check + 5 regular requests = 6 total
	if server1Requests != 6 {
		t.Errorf("Expected server1 to handle 6 requests (1 health check + 5 regular), got %d", server1Requests)
	}

	// server2 should only receive health check(s)
	if server2Requests > 2 { // Allow for health checks
		t.Errorf("Expected server2 to receive minimal requests (health checks only), got %d", server2Requests)
	}

	t.Logf("All requests routed to primary server, server1=%d, server2=%d", server1Requests, server2Requests)
}

func TestLoadBalancerWebSocketUpgrade(t *testing.T) {
	// Create mock WebSocket server
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") == "websocket" {
			// Mock WebSocket upgrade response
			w.Header().Set("Upgrade", "websocket")
			w.Header().Set("Connection", "Upgrade")
			w.WriteHeader(http.StatusSwitchingProtocols)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer wsServer.Close()

	// Load the standard config.toml and modify for testing
	cfg := config.LoadOrDefault("../../config.toml")

	// Override settings for WebSocket testing
	cfg.Server.MaxRetries = 3
	cfg.Server.RequestTimeout = 100 * time.Millisecond
	cfg.Metrics.Enabled = false

	// Replace nodes with WebSocket test server (clear beacons config)
	cfg.Beacons.Nodes = []string{"primary"}
	cfg.Beacons.SetParsedNodes([]config.NodeConfig{
		{Name: "primary", URL: wsServer.URL, Type: "lighthouse"},
	})

	lb, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create load balancer: %v", err)
	}

	// Test WebSocket upgrade detection
	req := httptest.NewRequest("GET", "/eth/v1/events", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	w := httptest.NewRecorder()

	start := time.Now()
	lb.ServeHTTP(w, req)
	duration := time.Since(start)

	t.Logf("WebSocket upgrade attempt completed in %v", duration)

	// Note: Full WebSocket testing requires more complex setup
	// This test validates that WebSocket upgrade requests are detected and handled
}

func TestLoadBalancerPerformance(t *testing.T) {
	// Create fast mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/eth/v1/node/syncing" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"0"}}`))
			return
		}
		// Simulate fast beacon API response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"head_slot":"12345","finalized_slot":"12300"}}`))
	}))
	defer server.Close()

	// Load the standard config.toml and modify for testing
	cfg := config.LoadOrDefault("../../config.toml")

	// Override settings for performance testing
	cfg.Server.MaxRetries = 3
	cfg.Server.RequestTimeout = 30 * time.Millisecond // Sub-50ms target
	cfg.Metrics.Enabled = false

	// Replace nodes with performance test server using beacons configuration
	cfg.Beacons.Nodes = []string{"fast-node"}
	cfg.Beacons.SetParsedNodes([]config.NodeConfig{
		{Name: "fast-node", URL: server.URL, Type: "lighthouse"},
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

	// Performance test: measure response times
	const numRequests = 100
	var totalDuration time.Duration
	var maxDuration time.Duration

	for i := 0; i < numRequests; i++ {
		req := httptest.NewRequest("GET", "/eth/v1/beacon/states/head/finality_checkpoints", nil)
		w := httptest.NewRecorder()

		start := time.Now()
		lb.ServeHTTP(w, req)
		duration := time.Since(start)

		totalDuration += duration
		if duration > maxDuration {
			maxDuration = duration
		}

		if w.Code != http.StatusOK {
			t.Errorf("Request %d failed with status %d", i, w.Code)
		}
	}

	avgDuration := totalDuration / numRequests

	t.Logf("Performance Results:")
	t.Logf("  Requests: %d", numRequests)
	t.Logf("  Average: %v", avgDuration)
	t.Logf("  Maximum: %v", maxDuration)
	t.Logf("  Total: %v", totalDuration)

	// Validate performance requirements
	if avgDuration > 10*time.Millisecond {
		t.Errorf("Average response time too slow: %v (expected < 10ms for local test)", avgDuration)
	}

	if maxDuration > 50*time.Millisecond {
		t.Errorf("Maximum response time too slow: %v (expected < 50ms)", maxDuration)
	}
}

func TestErrorBasedFailover(t *testing.T) {
	var primaryRequestCount, backupRequestCount int

	// Primary server - responds to health checks but fails regular requests 5 times
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Health check endpoint - always succeeds
		if r.URL.Path == "/eth/v1/node/syncing" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"0"}}`))
			return
		}

		primaryRequestCount++
		if primaryRequestCount <= 5 {
			w.WriteHeader(http.StatusInternalServerError) // Force server error
			return
		}
		// After 5 failures, start succeeding
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": {"head_slot": "12345"}}`))
	}))
	defer primaryServer.Close()

	// Backup server - always succeeds
	backupServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Health check endpoint
		if r.URL.Path == "/eth/v1/node/syncing" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"0"}}`))
			return
		}

		backupRequestCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": {"head_slot": "67890"}}`))
	}))
	defer backupServer.Close()

	// Load config and override with test servers
	cfg := config.LoadOrDefault("../../config.toml")
	cfg.Failover.ErrorThreshold = 3 // Set threshold to 3 for faster testing
	cfg.Server.MaxRetries = 10      // Allow more retries to test failover
	cfg.Metrics.Enabled = false

	// Override nodes with test servers (primary first, backup second)
	cfg.Beacons.Nodes = []string{"primary", "backup"}
	cfg.Beacons.SetParsedNodes([]config.NodeConfig{
		{Name: "primary", URL: primaryServer.URL, Type: "lighthouse"},
		{Name: "backup", URL: backupServer.URL, Type: "lighthouse"},
	})

	// Create load balancer
	lb, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create load balancer: %v", err)
	}

	// Run startup health check to populate healthy nodes
	err = lb.StartupHealthCheck()
	if err != nil {
		t.Fatalf("StartupHealthCheck failed: %v", err)
	}

	nodes := lb.GetNodes()
	if len(nodes) != 2 {
		t.Fatalf("Expected 2 nodes, got %d", len(nodes))
	}

	// First request should fail on primary (1 error)
	req1 := httptest.NewRequest("GET", "/eth/v1/beacon/headers/head", nil)
	w1 := httptest.NewRecorder()
	lb.ServeHTTP(w1, req1)

	// Check primary node has 1 consecutive error, should still be healthy
	consecutiveErrors, _, _ := nodes[0].GetStats()
	if consecutiveErrors != 1 {
		t.Errorf("Expected primary to have 1 consecutive error, got %d", consecutiveErrors)
	}
	if !nodes[0].IsHealthy(cfg.Failover.ErrorThreshold) {
		t.Errorf("Primary should still be healthy after 1 error")
	}

	// Second request should fail on primary (2 errors)
	req2 := httptest.NewRequest("GET", "/eth/v1/beacon/headers/head", nil)
	w2 := httptest.NewRecorder()
	lb.ServeHTTP(w2, req2)

	consecutiveErrors, _, _ = nodes[0].GetStats()
	if consecutiveErrors != 2 {
		t.Errorf("Expected primary to have 2 consecutive errors, got %d", consecutiveErrors)
	}

	// Third request should fail on primary and trigger failover (3 errors = threshold)
	req3 := httptest.NewRequest("GET", "/eth/v1/beacon/headers/head", nil)
	w3 := httptest.NewRecorder()
	lb.ServeHTTP(w3, req3)

	consecutiveErrors, _, _ = nodes[0].GetStats()
	if consecutiveErrors != 3 {
		t.Errorf("Expected primary to have 3 consecutive errors, got %d", consecutiveErrors)
	}
	if nodes[0].IsHealthy(cfg.Failover.ErrorThreshold) {
		t.Errorf("Primary should be unhealthy after reaching error threshold")
	}

	// Fourth request should now use backup server (primary unhealthy)
	req4 := httptest.NewRequest("GET", "/eth/v1/beacon/headers/head", nil)
	w4 := httptest.NewRecorder()
	lb.ServeHTTP(w4, req4)

	if w4.Code != http.StatusOK {
		t.Errorf("Expected backup to return 200, got %d", w4.Code)
	}
	if !strings.Contains(w4.Body.String(), "67890") {
		t.Errorf("Expected response from backup server, got: %s", w4.Body.String())
	}

	// Test recovery: fifth request should succeed on primary (if we had enough retries)
	// But since primary is unhealthy, it won't be tried

	t.Logf("Error-based failover test completed")
	t.Logf("Primary requests: %d, Backup requests: %d", primaryRequestCount, backupRequestCount)
	t.Logf("Primary consecutive errors: %d", consecutiveErrors)
}

func TestEndpointValidation(t *testing.T) {
	// Create mock beacon server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Health check endpoint
		if r.URL.Path == "/eth/v1/node/syncing" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"0"}}`))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": "test"}`))
	}))
	defer server.Close()

	cfg := config.LoadOrDefault("../../config.toml")
	cfg.Server.MaxRetries = 3
	cfg.Server.RequestTimeout = 100 * time.Millisecond
	cfg.Metrics.Enabled = false

	cfg.Beacons.Nodes = []string{"test"}
	cfg.Beacons.SetParsedNodes([]config.NodeConfig{
		{Name: "test", URL: server.URL, Type: "lighthouse"},
	})

	lb, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create load balancer: %v", err)
	}

	// Run startup health check to populate healthy nodes
	err = lb.StartupHealthCheck()
	if err != nil {
		t.Fatalf("StartupHealthCheck failed: %v", err)
	}

	testCases := []struct {
		name           string
		path           string
		expectedStatus int
	}{
		{"valid beacon endpoint", "/eth/v1/beacon/genesis", http.StatusOK},
		{"valid node endpoint", "/eth/v1/node/health", http.StatusOK},
		{"valid validator endpoint", "/eth/v1/validator/duties/attester/12345", http.StatusOK},
		{"valid events endpoint", "/eth/v1/events", http.StatusOK},
		{"invalid endpoint", "/invalid/path", http.StatusForbidden},
		{"malicious path traversal", "/eth/v1/beacon/../../etc/passwd", http.StatusForbidden},
		{"execution layer endpoint", "/eth/v1/execution/blocks", http.StatusForbidden},
		{"random path", "/admin/config", http.StatusForbidden},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			w := httptest.NewRecorder()

			lb.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d for path %s, got %d", tc.expectedStatus, tc.path, w.Code)
			}

			if tc.expectedStatus == http.StatusForbidden {
				if !strings.Contains(w.Body.String(), "Invalid Beacon Chain API endpoint") {
					t.Errorf("Expected forbidden error message, got: %s", w.Body.String())
				}
			}
		})
	}
}

func TestLoadBalancer_GetNodes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.LoadOrDefault("../../config.toml")
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

	nodes := lb.GetNodes()
	if len(nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(nodes))
	}

	if nodes[0].Name != "node1" {
		t.Errorf("Expected first node to be node1, got %s", nodes[0].Name)
	}

	if nodes[1].Name != "node2" {
		t.Errorf("Expected second node to be node2, got %s", nodes[1].Name)
	}

	if nodes[2].Name != "node3" {
		t.Errorf("Expected third node to be node3, got %s", nodes[2].Name)
	}
}

func TestLoadBalancer_GetHealthyNodes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Health check endpoint
		if r.URL.Path == "/eth/v1/node/syncing" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"0"}}`))
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.LoadOrDefault("../../config.toml")
	cfg.Failover.ErrorThreshold = 3
	cfg.Beacons.Nodes = []string{"node1", "node2"}
	cfg.Beacons.SetParsedNodes([]config.NodeConfig{
		{Name: "node1", URL: server.URL, Type: "lighthouse"},
		{Name: "node2", URL: server.URL, Type: "prysm"},
	})

	lb, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create load balancer: %v", err)
	}

	// Run startup health check to populate healthy nodes
	err = lb.StartupHealthCheck()
	if err != nil {
		t.Fatalf("StartupHealthCheck failed: %v", err)
	}

	// Initially all nodes should be healthy
	healthyNodes := lb.GetHealthyNodes()
	if len(healthyNodes) != 2 {
		t.Errorf("Expected 2 healthy nodes initially, got %d", len(healthyNodes))
	}

	// Test that we can get all nodes
	allNodes := lb.GetNodes()
	if len(allNodes) != 2 {
		t.Errorf("Expected 2 total nodes, got %d", len(allNodes))
	}

	// Verify the healthy nodes list includes the expected nodes
	foundNode1 := false
	foundNode2 := false
	for _, node := range healthyNodes {
		if node.Name == "node1" {
			foundNode1 = true
		}
		if node.Name == "node2" {
			foundNode2 = true
		}
	}

	if !foundNode1 {
		t.Error("Expected node1 to be in healthy nodes list")
	}
	if !foundNode2 {
		t.Error("Expected node2 to be in healthy nodes list")
	}
}
