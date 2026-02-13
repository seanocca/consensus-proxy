package beaconnode_test

import (
	"sync"
	"testing"
	"time"

	"github.com/zircuit-labs/consensus-proxy/cmd/beaconnode"
	"github.com/zircuit-labs/consensus-proxy/cmd/config"
)

func TestBeaconNode_IsHealthy(t *testing.T) {
	cfg := config.LoadOrDefault("../../config.toml")
	nodeConfig := config.NodeConfig{
		Name: "test-node",
		URL:  "http://localhost:5052",
		Type: "lighthouse",
	}

	node, err := beaconnode.NewBeaconNode(nodeConfig, cfg)
	if err != nil {
		t.Fatalf("Failed to create beacon node: %v", err)
	}

	// Initially should be healthy
	if !node.IsHealthy(5) {
		t.Error("New node should be healthy")
	}

	// Increment errors but stay below threshold
	node.IncrementError()
	node.IncrementError()
	if !node.IsHealthy(5) {
		t.Error("Node should still be healthy with 2 errors (threshold 5)")
	}

	// Reach threshold
	node.IncrementError()
	node.IncrementError()
	node.IncrementError()
	if node.IsHealthy(5) {
		t.Error("Node should be unhealthy after reaching threshold")
	}

	// Reset should make it healthy again
	node.ResetErrors()
	if !node.IsHealthy(5) {
		t.Error("Node should be healthy after reset")
	}
}

func TestBeaconNode_IncrementError(t *testing.T) {
	cfg := config.LoadOrDefault("../../config.toml")
	nodeConfig := config.NodeConfig{
		Name: "test-node",
		URL:  "http://localhost:5052",
		Type: "lighthouse",
	}

	node, err := beaconnode.NewBeaconNode(nodeConfig, cfg)
	if err != nil {
		t.Fatalf("Failed to create beacon node: %v", err)
	}

	consecutiveErrors, totalFailures, _ := node.GetStats()
	if consecutiveErrors != 0 || totalFailures != 0 {
		t.Errorf("Expected 0 errors initially, got consecutive=%d, total=%d", consecutiveErrors, totalFailures)
	}

	// Increment error
	node.IncrementError()
	consecutiveErrors, totalFailures, _ = node.GetStats()
	if consecutiveErrors != 1 || totalFailures != 1 {
		t.Errorf("Expected 1 error, got consecutive=%d, total=%d", consecutiveErrors, totalFailures)
	}

	// Increment again
	node.IncrementError()
	consecutiveErrors, totalFailures, _ = node.GetStats()
	if consecutiveErrors != 2 || totalFailures != 2 {
		t.Errorf("Expected 2 errors, got consecutive=%d, total=%d", consecutiveErrors, totalFailures)
	}
}

func TestBeaconNode_ResetErrors(t *testing.T) {
	cfg := config.LoadOrDefault("../../config.toml")
	nodeConfig := config.NodeConfig{
		Name: "test-node",
		URL:  "http://localhost:5052",
		Type: "lighthouse",
	}

	node, err := beaconnode.NewBeaconNode(nodeConfig, cfg)
	if err != nil {
		t.Fatalf("Failed to create beacon node: %v", err)
	}

	// Add some errors
	node.IncrementError()
	node.IncrementError()
	node.IncrementError()

	consecutiveErrors, totalFailures, _ := node.GetStats()
	if consecutiveErrors != 3 {
		t.Errorf("Expected 3 consecutive errors, got %d", consecutiveErrors)
	}

	// Reset consecutive errors
	node.ResetErrors()
	consecutiveErrors, totalFailures, _ = node.GetStats()
	if consecutiveErrors != 0 {
		t.Errorf("Expected 0 consecutive errors after reset, got %d", consecutiveErrors)
	}

	// Total failures should remain unchanged
	if totalFailures != 3 {
		t.Errorf("Expected total failures to remain at 3, got %d", totalFailures)
	}
}

func TestBeaconNode_IncrementRequests(t *testing.T) {
	cfg := config.LoadOrDefault("../../config.toml")
	nodeConfig := config.NodeConfig{
		Name: "test-node",
		URL:  "http://localhost:5052",
		Type: "lighthouse",
	}

	node, err := beaconnode.NewBeaconNode(nodeConfig, cfg)
	if err != nil {
		t.Fatalf("Failed to create beacon node: %v", err)
	}

	_, _, requests := node.GetStats()
	if requests != 0 {
		t.Errorf("Expected 0 requests initially, got %d", requests)
	}

	// Increment requests
	node.IncrementRequests()
	_, _, requests = node.GetStats()
	if requests != 1 {
		t.Errorf("Expected 1 request, got %d", requests)
	}

	// Increment multiple times
	for i := 0; i < 10; i++ {
		node.IncrementRequests()
	}
	_, _, requests = node.GetStats()
	if requests != 11 {
		t.Errorf("Expected 11 requests, got %d", requests)
	}
}

func TestBeaconNode_GetStats(t *testing.T) {
	cfg := config.LoadOrDefault("../../config.toml")
	nodeConfig := config.NodeConfig{
		Name: "test-node",
		URL:  "http://localhost:5052",
		Type: "lighthouse",
	}

	node, err := beaconnode.NewBeaconNode(nodeConfig, cfg)
	if err != nil {
		t.Fatalf("Failed to create beacon node: %v", err)
	}

	// Test initial state
	consecutiveErrors, totalFailures, requests := node.GetStats()
	if consecutiveErrors != 0 || totalFailures != 0 || requests != 0 {
		t.Errorf("Expected all stats to be 0, got consecutive=%d, total=%d, requests=%d",
			consecutiveErrors, totalFailures, requests)
	}

	// Mix of operations
	node.IncrementRequests()
	node.IncrementRequests()
	node.IncrementError()
	node.ResetErrors()
	node.IncrementError()
	node.IncrementRequests()

	consecutiveErrors, totalFailures, requests = node.GetStats()
	if consecutiveErrors != 1 {
		t.Errorf("Expected consecutive errors=1, got %d", consecutiveErrors)
	}
	if totalFailures != 2 {
		t.Errorf("Expected total failures=2, got %d", totalFailures)
	}
	if requests != 3 {
		t.Errorf("Expected requests=3, got %d", requests)
	}
}

func TestBeaconNode_ThreadSafety(t *testing.T) {
	cfg := config.LoadOrDefault("../../config.toml")
	nodeConfig := config.NodeConfig{
		Name: "test-node",
		URL:  "http://localhost:5052",
		Type: "lighthouse",
	}

	node, err := beaconnode.NewBeaconNode(nodeConfig, cfg)
	if err != nil {
		t.Fatalf("Failed to create beacon node: %v", err)
	}

	// Concurrent increments
	var wg sync.WaitGroup
	iterations := 100

	// Increment requests concurrently
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			node.IncrementRequests()
		}()
	}

	// Increment errors concurrently
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			node.IncrementError()
		}()
	}

	wg.Wait()

	_, totalFailures, requests := node.GetStats()
	if requests != int64(iterations) {
		t.Errorf("Expected %d requests, got %d", iterations, requests)
	}
	if totalFailures != int64(iterations) {
		t.Errorf("Expected %d total failures, got %d", iterations, totalFailures)
	}
}

func TestNewBeaconNode_InvalidURL(t *testing.T) {
	cfg := config.LoadOrDefault("../../config.toml")
	nodeConfig := config.NodeConfig{
		Name: "test-node",
		URL:  "://invalid-url",
		Type: "lighthouse",
	}

	_, err := beaconnode.NewBeaconNode(nodeConfig, cfg)
	if err == nil {
		t.Error("Expected error for invalid URL")
	}
}

func TestNewBeaconNode_Configuration(t *testing.T) {
	cfg := config.LoadOrDefault("../../config.toml")
	nodeConfig := config.NodeConfig{
		Name: "test-node",
		URL:  "http://localhost:5052",
		Type: "lighthouse",
	}

	node, err := beaconnode.NewBeaconNode(nodeConfig, cfg)
	if err != nil {
		t.Fatalf("Failed to create beacon node: %v", err)
	}

	if node.Name != "test-node" {
		t.Errorf("Expected name=test-node, got %s", node.Name)
	}

	if node.URL != "http://localhost:5052" {
		t.Errorf("Expected URL=http://localhost:5052, got %s", node.URL)
	}

	if node.Proxy == nil {
		t.Error("Expected proxy to be initialized")
	}

	if node.LastCheck.IsZero() {
		t.Error("Expected LastCheck to be set")
	}

	// Verify it's recent
	if time.Since(node.LastCheck) > 1*time.Second {
		t.Error("Expected LastCheck to be recent")
	}
}
