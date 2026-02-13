package beaconnode_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zircuit-labs/consensus-proxy/cmd/beaconnode"
	"github.com/zircuit-labs/consensus-proxy/cmd/config"
)

// TestCheckSyncStatus_Synced tests CheckSyncStatus with a fully synced node
func TestCheckSyncStatus_Synced(t *testing.T) {
	// Create mock server that returns synced response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/eth/v1/node/syncing" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"0"}}`))
	}))
	defer server.Close()

	cfg := config.LoadOrDefault("../../config.toml")
	nodeConfig := config.NodeConfig{
		Name: "test-node",
		URL:  server.URL,
		Type: "lighthouse",
	}

	node, err := beaconnode.NewBeaconNode(nodeConfig, cfg)
	if err != nil {
		t.Fatalf("Failed to create beacon node: %v", err)
	}

	healthCfg := config.HealthCheckConfig{
		Timeout: 5 * time.Second,
	}

	isHealthy, err := node.CheckSyncStatus(healthCfg)
	if err != nil {
		t.Errorf("CheckSyncStatus failed: %v", err)
	}
	if !isHealthy {
		t.Error("Expected node to be healthy when synced")
	}
}

// TestCheckSyncStatus_Syncing tests CheckSyncStatus with a syncing node
func TestCheckSyncStatus_Syncing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"is_syncing":true,"sync_distance":"100"}}`))
	}))
	defer server.Close()

	cfg := config.LoadOrDefault("../../config.toml")
	nodeConfig := config.NodeConfig{
		Name: "test-node",
		URL:  server.URL,
		Type: "lighthouse",
	}

	node, err := beaconnode.NewBeaconNode(nodeConfig, cfg)
	if err != nil {
		t.Fatalf("Failed to create beacon node: %v", err)
	}

	healthCfg := config.HealthCheckConfig{
		Timeout: 5 * time.Second,
	}

	isHealthy, err := node.CheckSyncStatus(healthCfg)
	if err == nil {
		t.Error("Expected error when node is syncing")
	}
	if isHealthy {
		t.Error("Expected node to be unhealthy when syncing")
	}
}

// TestCheckSyncStatus_SyncDistanceNonZero tests CheckSyncStatus with sync_distance > 0
func TestCheckSyncStatus_SyncDistanceNonZero(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"5"}}`))
	}))
	defer server.Close()

	cfg := config.LoadOrDefault("../../config.toml")
	nodeConfig := config.NodeConfig{
		Name: "test-node",
		URL:  server.URL,
		Type: "lighthouse",
	}

	node, err := beaconnode.NewBeaconNode(nodeConfig, cfg)
	if err != nil {
		t.Fatalf("Failed to create beacon node: %v", err)
	}

	healthCfg := config.HealthCheckConfig{
		Timeout: 5 * time.Second,
	}

	isHealthy, err := node.CheckSyncStatus(healthCfg)
	if err == nil {
		t.Error("Expected error when sync_distance is not 0")
	}
	if isHealthy {
		t.Error("Expected node to be unhealthy when sync_distance > 0")
	}
}

// TestCheckSyncStatus_RequestFailed tests CheckSyncStatus when request fails
func TestCheckSyncStatus_RequestFailed(t *testing.T) {
	cfg := config.LoadOrDefault("../../config.toml")
	nodeConfig := config.NodeConfig{
		Name: "test-node",
		URL:  "http://localhost:1", // Invalid port
		Type: "lighthouse",
	}

	node, err := beaconnode.NewBeaconNode(nodeConfig, cfg)
	if err != nil {
		t.Fatalf("Failed to create beacon node: %v", err)
	}

	healthCfg := config.HealthCheckConfig{
		Timeout: 100 * time.Millisecond,
	}

	isHealthy, err := node.CheckSyncStatus(healthCfg)
	if err == nil {
		t.Error("Expected error when request fails")
	}
	if isHealthy {
		t.Error("Expected node to be unhealthy when request fails")
	}
}

// TestCheckSyncStatus_Non200Status tests CheckSyncStatus with non-200 status
func TestCheckSyncStatus_Non200Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	cfg := config.LoadOrDefault("../../config.toml")
	nodeConfig := config.NodeConfig{
		Name: "test-node",
		URL:  server.URL,
		Type: "lighthouse",
	}

	node, err := beaconnode.NewBeaconNode(nodeConfig, cfg)
	if err != nil {
		t.Fatalf("Failed to create beacon node: %v", err)
	}

	healthCfg := config.HealthCheckConfig{
		Timeout: 5 * time.Second,
	}

	isHealthy, err := node.CheckSyncStatus(healthCfg)
	if err == nil {
		t.Error("Expected error when status is not 200")
	}
	if isHealthy {
		t.Error("Expected node to be unhealthy with non-200 status")
	}
}

// TestCheckSyncStatus_InvalidJSON tests CheckSyncStatus with invalid JSON response
func TestCheckSyncStatus_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	cfg := config.LoadOrDefault("../../config.toml")
	nodeConfig := config.NodeConfig{
		Name: "test-node",
		URL:  server.URL,
		Type: "lighthouse",
	}

	node, err := beaconnode.NewBeaconNode(nodeConfig, cfg)
	if err != nil {
		t.Fatalf("Failed to create beacon node: %v", err)
	}

	healthCfg := config.HealthCheckConfig{
		Timeout: 5 * time.Second,
	}

	isHealthy, err := node.CheckSyncStatus(healthCfg)
	if err == nil {
		t.Error("Expected error when JSON is invalid")
	}
	if isHealthy {
		t.Error("Expected node to be unhealthy with invalid JSON")
	}
}

// TestHealthCheck_Synced tests HealthCheck with a fully synced node
func TestHealthCheck_Synced(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/eth/v1/node/syncing" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"0"}}`))
	}))
	defer server.Close()

	cfg := config.LoadOrDefault("../../config.toml")
	nodeConfig := config.NodeConfig{
		Name: "test-node",
		URL:  server.URL,
		Type: "lighthouse",
	}

	node, err := beaconnode.NewBeaconNode(nodeConfig, cfg)
	if err != nil {
		t.Fatalf("Failed to create beacon node: %v", err)
	}

	healthCfg := config.HealthCheckConfig{
		Timeout: 5 * time.Second,
	}

	isHealthy, healthErr := node.HealthCheck(healthCfg)
	if !isHealthy {
		t.Error("Expected node to be healthy when synced")
	}
	if healthErr != nil {
		t.Errorf("Expected no error, got: %v", healthErr)
	}
}

// TestHealthCheck_Syncing tests HealthCheck with a syncing node
func TestHealthCheck_Syncing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"is_syncing":true,"sync_distance":"100"}}`))
	}))
	defer server.Close()

	cfg := config.LoadOrDefault("../../config.toml")
	nodeConfig := config.NodeConfig{
		Name: "test-node",
		URL:  server.URL,
		Type: "lighthouse",
	}

	node, err := beaconnode.NewBeaconNode(nodeConfig, cfg)
	if err != nil {
		t.Fatalf("Failed to create beacon node: %v", err)
	}

	healthCfg := config.HealthCheckConfig{
		Timeout: 5 * time.Second,
	}

	isHealthy, healthErr := node.HealthCheck(healthCfg)
	if isHealthy {
		t.Error("Expected node to be unhealthy when syncing")
	}
	if healthErr == nil {
		t.Error("Expected HealthCheckError when node is syncing")
	}
	if healthErr != nil {
		if healthErr.Reason != "is_syncing" {
			t.Errorf("Expected reason 'is_syncing', got: %s", healthErr.Reason)
		}
		if !healthErr.IsSyncing {
			t.Error("Expected IsSyncing to be true")
		}
		if healthErr.SyncDistance != "100" {
			t.Errorf("Expected SyncDistance '100', got: %s", healthErr.SyncDistance)
		}
	}
}

// TestHealthCheck_SyncDistanceNonZero tests HealthCheck with sync_distance > 0
func TestHealthCheck_SyncDistanceNonZero(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"5"}}`))
	}))
	defer server.Close()

	cfg := config.LoadOrDefault("../../config.toml")
	nodeConfig := config.NodeConfig{
		Name: "test-node",
		URL:  server.URL,
		Type: "lighthouse",
	}

	node, err := beaconnode.NewBeaconNode(nodeConfig, cfg)
	if err != nil {
		t.Fatalf("Failed to create beacon node: %v", err)
	}

	healthCfg := config.HealthCheckConfig{
		Timeout: 5 * time.Second,
	}

	isHealthy, healthErr := node.HealthCheck(healthCfg)
	if isHealthy {
		t.Error("Expected node to be unhealthy when sync_distance > 0")
	}
	if healthErr == nil {
		t.Error("Expected HealthCheckError when sync_distance > 0")
	}
	if healthErr != nil {
		if healthErr.Reason != "sync_distance_not_zero" {
			t.Errorf("Expected reason 'sync_distance_not_zero', got: %s", healthErr.Reason)
		}
		if healthErr.IsSyncing {
			t.Error("Expected IsSyncing to be false")
		}
		if healthErr.SyncDistance != "5" {
			t.Errorf("Expected SyncDistance '5', got: %s", healthErr.SyncDistance)
		}
	}
}

// TestHealthCheck_RequestFailed tests HealthCheck when request fails
func TestHealthCheck_RequestFailed(t *testing.T) {
	cfg := config.LoadOrDefault("../../config.toml")
	nodeConfig := config.NodeConfig{
		Name: "test-node",
		URL:  "http://localhost:1", // Invalid port
		Type: "lighthouse",
	}

	node, err := beaconnode.NewBeaconNode(nodeConfig, cfg)
	if err != nil {
		t.Fatalf("Failed to create beacon node: %v", err)
	}

	healthCfg := config.HealthCheckConfig{
		Timeout: 100 * time.Millisecond,
	}

	isHealthy, healthErr := node.HealthCheck(healthCfg)
	if isHealthy {
		t.Error("Expected node to be unhealthy when request fails")
	}
	if healthErr == nil {
		t.Error("Expected HealthCheckError when request fails")
	}
	if healthErr != nil {
		if healthErr.Reason != "request_failed" {
			t.Errorf("Expected reason 'request_failed', got: %s", healthErr.Reason)
		}
		if healthErr.Err == nil {
			t.Error("Expected underlying error to be set")
		}
	}
}

// TestHealthCheck_Non200Status tests HealthCheck with non-200 status
func TestHealthCheck_Non200Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error":"service unavailable"}`))
	}))
	defer server.Close()

	cfg := config.LoadOrDefault("../../config.toml")
	nodeConfig := config.NodeConfig{
		Name: "test-node",
		URL:  server.URL,
		Type: "lighthouse",
	}

	node, err := beaconnode.NewBeaconNode(nodeConfig, cfg)
	if err != nil {
		t.Fatalf("Failed to create beacon node: %v", err)
	}

	healthCfg := config.HealthCheckConfig{
		Timeout: 5 * time.Second,
	}

	isHealthy, healthErr := node.HealthCheck(healthCfg)
	if isHealthy {
		t.Error("Expected node to be unhealthy with non-200 status")
	}
	if healthErr == nil {
		t.Error("Expected HealthCheckError when status is not 200")
	}
	if healthErr != nil {
		if healthErr.Reason != "non_200_status" {
			t.Errorf("Expected reason 'non_200_status', got: %s", healthErr.Reason)
		}
		if healthErr.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("Expected StatusCode %d, got: %d", http.StatusServiceUnavailable, healthErr.StatusCode)
		}
	}
}

// TestHealthCheck_InvalidJSON tests HealthCheck with invalid JSON response
func TestHealthCheck_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not valid json`))
	}))
	defer server.Close()

	cfg := config.LoadOrDefault("../../config.toml")
	nodeConfig := config.NodeConfig{
		Name: "test-node",
		URL:  server.URL,
		Type: "lighthouse",
	}

	node, err := beaconnode.NewBeaconNode(nodeConfig, cfg)
	if err != nil {
		t.Fatalf("Failed to create beacon node: %v", err)
	}

	healthCfg := config.HealthCheckConfig{
		Timeout: 5 * time.Second,
	}

	isHealthy, healthErr := node.HealthCheck(healthCfg)
	if isHealthy {
		t.Error("Expected node to be unhealthy with invalid JSON")
	}
	if healthErr == nil {
		t.Error("Expected HealthCheckError when JSON is invalid")
	}
	if healthErr != nil {
		if healthErr.Reason != "json_parse_failed" {
			t.Errorf("Expected reason 'json_parse_failed', got: %s", healthErr.Reason)
		}
		if healthErr.Err == nil {
			t.Error("Expected underlying error to be set")
		}
	}
}

// TestHealthCheckError_Error tests the Error() method
func TestHealthCheckError_Error(t *testing.T) {
	// Test with underlying error
	err1 := &beaconnode.HealthCheckError{
		Reason: "request_failed",
		Err:    http.ErrHandlerTimeout,
	}
	errMsg1 := err1.Error()
	if errMsg1 != "request_failed: http: Handler timeout" {
		t.Errorf("Unexpected error message: %s", errMsg1)
	}

	// Test without underlying error
	err2 := &beaconnode.HealthCheckError{
		Reason: "is_syncing",
	}
	errMsg2 := err2.Error()
	if errMsg2 != "is_syncing" {
		t.Errorf("Unexpected error message: %s", errMsg2)
	}
}

// TestHealthCheck_Timeout tests HealthCheck respects timeout
func TestHealthCheck_Timeout(t *testing.T) {
	// Create server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Delay longer than timeout
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"is_syncing":false,"sync_distance":"0"}}`))
	}))
	defer server.Close()

	cfg := config.LoadOrDefault("../../config.toml")
	nodeConfig := config.NodeConfig{
		Name: "test-node",
		URL:  server.URL,
		Type: "lighthouse",
	}

	node, err := beaconnode.NewBeaconNode(nodeConfig, cfg)
	if err != nil {
		t.Fatalf("Failed to create beacon node: %v", err)
	}

	healthCfg := config.HealthCheckConfig{
		Timeout: 100 * time.Millisecond, // Short timeout
	}

	start := time.Now()
	isHealthy, healthErr := node.HealthCheck(healthCfg)
	duration := time.Since(start)

	if isHealthy {
		t.Error("Expected node to be unhealthy on timeout")
	}
	if healthErr == nil {
		t.Error("Expected HealthCheckError on timeout")
	}
	if duration > 500*time.Millisecond {
		t.Errorf("HealthCheck took too long: %v (expected < 500ms due to timeout)", duration)
	}
}
