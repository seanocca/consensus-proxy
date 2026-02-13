package config

import (
	"os"
	"testing"
	"time"
)

func TestConfigLoad(t *testing.T) {
	// Create temporary config file with current configuration structure
	configContent := `
[server]
port = 8080
max_retries = 5
request_timeout = "25ms"
read_timeout = "30s"
write_timeout = "30s"
idle_timeout = "90s"
read_header_timeout = "10s"

[failover]
error_threshold = 3

[metrics]
enabled = true
statsd_addr = "localhost:8125"
namespace = "test_proxy"

[logger]
level = "debug"
format = "json"
output = "stdout"

[ratelimit]
enabled = false
requests_per_second = 100
window = "1m"
cleanup_interval = "5m"
client_expiry = "10m"

[dns]
cache_ttl = "5m"
connection_timeout = "10s"

[proxy]
user_agent = "test-proxy/1.0"
max_idle_connections = 100
idle_connection_timeout = "90s"
max_idle_connections_per_host = 10
max_connections_per_host = 100
response_header_timeout = "10s"
tls_handshake_timeout = "10s"
expect_continue_timeout = "1s"

[websocket]
read_buffer_size = 4096
write_buffer_size = 4096
error_channel_buffer = 100

[beacons]
nodes = ["test-node-1", "test-node-2"]

[beacons.test-node-1]
url = "http://localhost:5052"
type = "lighthouse"

[beacons.test-node-2]
url = "http://localhost:5053"
type = "prysm"
`

	tmpFile, err := os.CreateTemp("", "test-config-*.toml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// Test loading config
	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Validate server settings
	if cfg.Server.MaxRetries != 5 {
		t.Errorf("Expected max_retries=5, got %d", cfg.Server.MaxRetries)
	}

	if cfg.Server.RequestTimeout != 25*time.Millisecond {
		t.Errorf("Expected request_timeout=25ms, got %v", cfg.Server.RequestTimeout)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("Expected port=8080, got %d", cfg.Server.Port)
	}

	if cfg.Server.IdleTimeout != 90*time.Second {
		t.Errorf("Expected idle_timeout=90s, got %v", cfg.Server.IdleTimeout)
	}

	if cfg.Server.ReadHeaderTimeout != 10*time.Second {
		t.Errorf("Expected read_header_timeout=10s, got %v", cfg.Server.ReadHeaderTimeout)
	}

	// Validate failover settings
	if cfg.Failover.ErrorThreshold != 3 {
		t.Errorf("Expected error_threshold=3, got %d", cfg.Failover.ErrorThreshold)
	}

	// Validate metrics settings
	if !cfg.Metrics.Enabled {
		t.Error("Expected metrics to be enabled")
	}

	if cfg.Metrics.StatsdAddr != "localhost:8125" {
		t.Errorf("Expected statsd_addr=localhost:8125, got %s", cfg.Metrics.StatsdAddr)
	}

	if cfg.Metrics.Namespace != "test_proxy" {
		t.Errorf("Expected namespace=test_proxy, got %s", cfg.Metrics.Namespace)
	}

	// Validate logger settings
	if cfg.Logger.Level != "debug" {
		t.Errorf("Expected log level=debug, got %s", cfg.Logger.Level)
	}

	if cfg.Logger.Format != "json" {
		t.Errorf("Expected log format=json, got %s", cfg.Logger.Format)
	}

	// Validate rate limiting settings
	if cfg.RateLimit.Enabled {
		t.Error("Expected rate limiting to be disabled")
	}

	if cfg.RateLimit.RequestsPerSecond != 100 {
		t.Errorf("Expected requests_per_second=100, got %d", cfg.RateLimit.RequestsPerSecond)
	}

	if cfg.RateLimit.CleanupInterval != 5*time.Minute {
		t.Errorf("Expected cleanup_interval=5m, got %v", cfg.RateLimit.CleanupInterval)
	}

	if cfg.RateLimit.ClientExpiry != 10*time.Minute {
		t.Errorf("Expected client_expiry=10m, got %v", cfg.RateLimit.ClientExpiry)
	}

	// Validate DNS settings
	if cfg.DNS.CacheTTL != 5*time.Minute {
		t.Errorf("Expected dns cache_ttl=5m, got %v", cfg.DNS.CacheTTL)
	}

	if cfg.DNS.ConnectionTimeout != 10*time.Second {
		t.Errorf("Expected dns connection_timeout=10s, got %v", cfg.DNS.ConnectionTimeout)
	}

	// Validate proxy settings
	if cfg.Proxy.UserAgent != "test-proxy/1.0" {
		t.Errorf("Expected user_agent=test-proxy/1.0, got %s", cfg.Proxy.UserAgent)
	}

	if cfg.Proxy.MaxIdleConns != 100 {
		t.Errorf("Expected max_idle_connections=100, got %d", cfg.Proxy.MaxIdleConns)
	}

	if cfg.Proxy.ResponseHeaderTimeout != 10*time.Second {
		t.Errorf("Expected response_header_timeout=10s, got %v", cfg.Proxy.ResponseHeaderTimeout)
	}

	// Validate WebSocket settings
	if cfg.WebSocket.ReadBufferSize != 4096 {
		t.Errorf("Expected read_buffer_size=4096, got %d", cfg.WebSocket.ReadBufferSize)
	}

	if cfg.WebSocket.WriteBufferSize != 4096 {
		t.Errorf("Expected write_buffer_size=4096, got %d", cfg.WebSocket.WriteBufferSize)
	}

	if cfg.WebSocket.ErrorChannelBuffer != 100 {
		t.Errorf("Expected error_channel_buffer=100, got %d", cfg.WebSocket.ErrorChannelBuffer)
	}

	// Validate beacon configuration
	if len(cfg.Beacons.Nodes) != 2 {
		t.Errorf("Expected 2 beacon nodes, got %d", len(cfg.Beacons.Nodes))
	}

	if cfg.Beacons.Nodes[0] != "test-node-1" {
		t.Errorf("Expected first node=test-node-1, got %s", cfg.Beacons.Nodes[0])
	}

	if cfg.Beacons.Nodes[1] != "test-node-2" {
		t.Errorf("Expected second node=test-node-2, got %s", cfg.Beacons.Nodes[1])
	}

	// Test beacon node parsing
	allNodes := cfg.GetAllNodes()
	if len(allNodes) != 2 {
		t.Errorf("Expected 2 parsed nodes, got %d", len(allNodes))
	}

	if allNodes[0].Name != "test-node-1" {
		t.Errorf("Expected first node name=test-node-1, got %s", allNodes[0].Name)
	}

	if allNodes[0].URL != "http://localhost:5052" {
		t.Errorf("Expected first node URL=http://localhost:5052, got %s", allNodes[0].URL)
	}

	if allNodes[0].Type != "lighthouse" {
		t.Errorf("Expected first node type=lighthouse, got %s", allNodes[0].Type)
	}

	if allNodes[1].Name != "test-node-2" {
		t.Errorf("Expected second node name=test-node-2, got %s", allNodes[1].Name)
	}

	if allNodes[1].URL != "http://localhost:5053" {
		t.Errorf("Expected second node URL=http://localhost:5053, got %s", allNodes[1].URL)
	}

	if allNodes[1].Type != "prysm" {
		t.Errorf("Expected second node type=prysm, got %s", allNodes[1].Type)
	}
}

func TestConfigLoadOrDefault(t *testing.T) {
	// Test with non-existent file
	cfg := LoadOrDefault("nonexistent.toml")

	// Should return default configuration
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected default port=8080, got %d", cfg.Server.Port)
	}

	if cfg.Server.MaxRetries != 3 {
		t.Errorf("Expected default max_retries=3, got %d", cfg.Server.MaxRetries)
	}

	if cfg.Failover.ErrorThreshold != 5 {
		t.Errorf("Expected default error_threshold=5, got %d", cfg.Failover.ErrorThreshold)
	}

	if cfg.DNS.CacheTTL != 5*time.Minute {
		t.Errorf("Expected default dns cache_ttl=5m, got %v", cfg.DNS.CacheTTL)
	}

	if cfg.Proxy.UserAgent != "consensus-proxy/1.0" {
		t.Errorf("Expected default user_agent=consensus-proxy/1.0, got %s", cfg.Proxy.UserAgent)
	}

	if cfg.WebSocket.ReadBufferSize != 4096 {
		t.Errorf("Expected default read_buffer_size=4096, got %d", cfg.WebSocket.ReadBufferSize)
	}
}

func TestConfigValidation(t *testing.T) {
	// Start with default config which has all required fields set
	cfg := LoadOrDefault("nonexistent-file-to-get-defaults.toml")
	cfg.Beacons.Nodes = []string{"test"}
	cfg.Beacons.SetParsedNodes([]NodeConfig{{Name: "test", URL: "http://localhost:5052"}})

	// Test invalid port
	cfg.Server.Port = -1
	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation error for invalid port")
	}

	// Test invalid max retries
	cfg.Server.Port = 8080
	cfg.Server.MaxRetries = 0
	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation error for invalid max_retries")
	}

	// Test invalid error threshold
	cfg.Server.MaxRetries = 3
	cfg.Failover.ErrorThreshold = 0
	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation error for invalid error_threshold")
	}

	// Test valid configuration with defaults
	// Reset to valid values (using defaults that were already loaded)
	cfg = LoadOrDefault("nonexistent-file-to-get-defaults.toml")
	cfg.Beacons.Nodes = []string{"test"}
	cfg.Beacons.SetParsedNodes([]NodeConfig{{Name: "test", URL: "http://localhost:5052"}})
	if err := cfg.Validate(); err != nil {
		t.Errorf("Valid configuration with defaults should not produce error: %v", err)
	}
}

func TestGetListenAddr(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port: 8080,
		},
	}

	expected := ":8080"
	if addr := cfg.GetListenAddr(); addr != expected {
		t.Errorf("Expected listen address=%s, got %s", expected, addr)
	}

	// Test with different port
	cfg.Server.Port = 9090
	expected = ":9090"
	if addr := cfg.GetListenAddr(); addr != expected {
		t.Errorf("Expected listen address=%s, got %s", expected, addr)
	}
}

func TestSetParsedNodes(t *testing.T) {
	beacons := &BeaconsConfig{}

	nodes := []NodeConfig{
		{Name: "node1", URL: "http://localhost:5052", Type: "lighthouse"},
		{Name: "node2", URL: "http://localhost:5053", Type: "prysm"},
	}

	beacons.SetParsedNodes(nodes)

	if len(beacons.parsedNodes) != 2 {
		t.Errorf("Expected 2 parsed nodes, got %d", len(beacons.parsedNodes))
	}

	if beacons.parsedNodes[0].Name != "node1" {
		t.Errorf("Expected first node name=node1, got %s", beacons.parsedNodes[0].Name)
	}

	if beacons.parsedNodes[1].URL != "http://localhost:5053" {
		t.Errorf("Expected second node URL=http://localhost:5053, got %s", beacons.parsedNodes[1].URL)
	}
}
