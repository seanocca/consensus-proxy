package config

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

// Config represents the main configuration structure
type Config struct {
	Server      ServerConfig      `toml:"server"`
	Beacons     BeaconsConfig     `toml:"beacons"`
	Failover    FailoverConfig    `toml:"failover"`
	Metrics     MetricsConfig     `toml:"metrics"`
	Logger      LoggerConfig      `toml:"logger"`
	RateLimit   RateLimitConfig   `toml:"ratelimit"`
	DNS         DNSConfig         `toml:"dns"`
	Proxy       ProxyConfig       `toml:"proxy"`
	WebSocket   WebSocketConfig   `toml:"websocket"`
	HealthCheck HealthCheckConfig `toml:"health"`
}

// ServerConfig contains server-specific configuration
type ServerConfig struct {
	Port              int           `toml:"port"`
	ReadTimeout       time.Duration `toml:"read_timeout"`
	WriteTimeout      time.Duration `toml:"write_timeout"`
	MaxRetries        int           `toml:"max_retries"`
	RequestTimeout    time.Duration `toml:"request_timeout"`
	IdleTimeout       time.Duration `toml:"idle_timeout"`
	ReadHeaderTimeout time.Duration `toml:"read_header_timeout"`
}

// RateLimitConfig contains rate limiting configuration
type RateLimitConfig struct {
	Enabled           bool          `toml:"enabled"`
	RequestsPerSecond int           `toml:"requests_per_second"`
	Window            time.Duration `toml:"window"`
	CleanupInterval   time.Duration `toml:"cleanup_interval"`
	ClientExpiry      time.Duration `toml:"client_expiry"`
}

// DNSConfig contains DNS caching configuration
type DNSConfig struct {
	CacheTTL          time.Duration `toml:"cache_ttl"`
	ConnectionTimeout time.Duration `toml:"connection_timeout"`
}

// ProxyConfig contains HTTP proxy configuration
type ProxyConfig struct {
	UserAgent             string        `toml:"user_agent"`
	MaxIdleConns          int           `toml:"max_idle_connections"`
	IdleConnTimeout       time.Duration `toml:"idle_connection_timeout"`
	MaxIdleConnsPerHost   int           `toml:"max_idle_connections_per_host"`
	MaxConnsPerHost       int           `toml:"max_connections_per_host"`
	ResponseHeaderTimeout time.Duration `toml:"response_header_timeout"`
	TLSHandshakeTimeout   time.Duration `toml:"tls_handshake_timeout"`
	ExpectContinueTimeout time.Duration `toml:"expect_continue_timeout"`
}

// WebSocketConfig contains WebSocket configuration
type WebSocketConfig struct {
	ReadBufferSize     int `toml:"read_buffer_size"`
	WriteBufferSize    int `toml:"write_buffer_size"`
	ErrorChannelBuffer int `toml:"error_channel_buffer"`
}

// NodeConfig represents a beacon node configuration
type NodeConfig struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
	Type string `toml:"type"` // beacon client type: lighthouse, prysm, nimbus, teku, etc.
}

// BeaconsConfig contains all beacon node configurations
type BeaconsConfig struct {
	Nodes       []string     `toml:"nodes"` // List of beacon names in priority order
	parsedNodes []NodeConfig // Parsed individual beacon configurations (not in TOML)
}

// SetParsedNodes sets the parsed nodes for testing purposes
func (bc *BeaconsConfig) SetParsedNodes(nodes []NodeConfig) {
	bc.parsedNodes = nodes
}

// FailoverConfig contains failover configuration
type FailoverConfig struct {
	ErrorThreshold int `toml:"error_threshold"` // Number of consecutive errors before failover
}

// MetricsConfig contains metrics/monitoring configuration
type MetricsConfig struct {
	Enabled    bool   `toml:"enabled"`
	StatsdAddr string `toml:"statsd_addr"`
	Namespace  string `toml:"namespace"`
}

// LoggerConfig contains logging configuration
type LoggerConfig struct {
	Level  string `toml:"level"`  // "debug", "info", "warn", "error"
	Format string `toml:"format"` // "json" or "text"
	Output string `toml:"output"` // "stdout", "stderr", or file path
}

// HealthCheckConfig contains health check configuration
type HealthCheckConfig struct {
	Interval                    time.Duration `toml:"interval"`                       // How often to run health checks
	Timeout                     time.Duration `toml:"timeout"`                        // Timeout for individual health check requests
	SuccessfulChecksForFailback int           `toml:"successful_checks_for_failback"` // Number of consecutive successful checks before failing back to original primary
}

// Load loads configuration from a TOML file with sensible defaults
func Load(configPath string) (*Config, error) {
	config := getDefaultConfig()

	// Check if config file exists
	if _, err := os.Stat(configPath); err == nil {
		// First decode the config to get the beacon names
		var rawConfig map[string]interface{}
		if _, err := toml.DecodeFile(configPath, &rawConfig); err != nil {
			return nil, fmt.Errorf("failed to decode config file %s: %v", configPath, err)
		}

		// Decode the main config structure
		if _, err := toml.DecodeFile(configPath, config); err != nil {
			return nil, fmt.Errorf("failed to decode config file %s: %v", configPath, err)
		}

		// Parse individual beacon configurations
		if err := config.parseBeaconConfigs(rawConfig); err != nil {
			return nil, fmt.Errorf("failed to parse beacon configurations: %v", err)
		}
	} else {
		return nil, fmt.Errorf("config file not found: %s", configPath)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %v", err)
	}

	return config, nil
}

// parseBeaconConfigs parses individual beacon configurations from the raw TOML data
func (c *Config) parseBeaconConfigs(rawConfig map[string]interface{}) error {
	// If no beacon names are configured, skip parsing
	if len(c.Beacons.Nodes) == 0 {
		return nil
	}

	c.Beacons.parsedNodes = make([]NodeConfig, 0, len(c.Beacons.Nodes))

	// Get the beacons section - it's optional
	beaconsSection, ok := rawConfig["beacons"].(map[string]interface{})
	if !ok {
		// If beacons section doesn't exist but we have beacon names, return error
		if len(c.Beacons.Nodes) > 0 {
			return fmt.Errorf("beacons section not found but beacon names are configured")
		}
		return nil
	}

	// Parse each beacon configuration
	for _, beaconName := range c.Beacons.Nodes {
		beaconConfig, ok := beaconsSection[beaconName].(map[string]interface{})
		if !ok {
			return fmt.Errorf("beacon configuration not found for: %s", beaconName)
		}

		// Extract URL (required)
		url, ok := beaconConfig["url"].(string)
		if !ok || url == "" {
			return fmt.Errorf("beacon %s: url is required", beaconName)
		}

		// Extract type (optional)
		beaconType := ""
		if t, ok := beaconConfig["type"].(string); ok {
			beaconType = t
		}

		c.Beacons.parsedNodes = append(c.Beacons.parsedNodes, NodeConfig{
			Name: beaconName,
			URL:  url,
			Type: beaconType,
		})
	}

	return nil
}

// LoadOrDefault loads configuration from file, or returns defaults if file doesn't exist
func LoadOrDefault(configPath string) *Config {
	config, err := Load(configPath)
	if err != nil {
		// Return default configuration
		config = getDefaultConfig()
	}
	return config
}

func getDefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:              8080,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			MaxRetries:        3,
			RequestTimeout:    30 * time.Millisecond,
			IdleTimeout:       90 * time.Second,
			ReadHeaderTimeout: 10 * time.Second,
		},
		Failover: FailoverConfig{
			ErrorThreshold: 5,
		},
		Metrics: MetricsConfig{
			Enabled:    false,
			StatsdAddr: "localhost:8125",
			Namespace:  "consensus_proxy",
		},
		Logger: LoggerConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
		RateLimit: RateLimitConfig{
			Enabled:           false,
			RequestsPerSecond: 100,
			Window:            1 * time.Minute,
			CleanupInterval:   5 * time.Minute,
			ClientExpiry:      10 * time.Minute,
		},
		DNS: DNSConfig{
			CacheTTL:          5 * time.Minute,
			ConnectionTimeout: 10 * time.Second,
		},
		Proxy: ProxyConfig{
			UserAgent:             "consensus-proxy/1.0",
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			MaxIdleConnsPerHost:   10,
			MaxConnsPerHost:       100,
			ResponseHeaderTimeout: 10 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		WebSocket: WebSocketConfig{
			ReadBufferSize:     4096,
			WriteBufferSize:    4096,
			ErrorChannelBuffer: 100,
		},
		HealthCheck: HealthCheckConfig{
			Interval:                    30 * time.Second,
			Timeout:                     5 * time.Second,
			SuccessfulChecksForFailback: 3,
		},
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	allNodes := c.GetAllNodes()
	if len(allNodes) == 0 {
		return fmt.Errorf("at least one beacon node must be configured in [beacons] section")
	}

	// Validate beacon list consistency
	if len(c.Beacons.Nodes) == 0 {
		return fmt.Errorf("beacon names list is empty - must specify nodes in [beacons] section")
	}

	if len(c.Beacons.parsedNodes) == 0 {
		return fmt.Errorf("beacon names are configured but no individual beacon configurations found")
	}

	if len(c.Beacons.parsedNodes) != len(c.Beacons.Nodes) {
		return fmt.Errorf("mismatch between beacon name list and individual configurations")
	}

	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Server.MaxRetries < 1 {
		return fmt.Errorf("max_retries must be at least 1")
	}

	if c.Server.RequestTimeout <= 0 {
		return fmt.Errorf("request_timeout must be positive")
	}

	if c.Failover.ErrorThreshold < 1 {
		return fmt.Errorf("failover error_threshold must be at least 1")
	}

	// Validate beacon node configurations
	for i, node := range allNodes {
		if node.Name == "" {
			return fmt.Errorf("node %d: name cannot be empty", i)
		}
		if node.URL == "" {
			return fmt.Errorf("node %d (%s): URL cannot be empty", i, node.Name)
		}

		// Validate beacon type if specified
		if node.Type != "" {
			validTypes := map[string]bool{"lighthouse": true, "prysm": true, "nimbus": true, "teku": true, "erigon": true, "infura": true, "alchemy": true}
			if !validTypes[node.Type] {
				return fmt.Errorf("node %d (%s): invalid beacon type '%s' (valid types: lighthouse, prysm, nimbus, teku, erigon, infura, alchemy)", i, node.Name, node.Type)
			}
		}
	}

	// Validate logger configuration
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Logger.Level] {
		return fmt.Errorf("invalid logger level: %s (must be debug, info, warn, or error)", c.Logger.Level)
	}

	validFormats := map[string]bool{"json": true, "text": true}
	if !validFormats[c.Logger.Format] {
		return fmt.Errorf("invalid logger format: %s (must be json or text)", c.Logger.Format)
	}

	// Validate rate limiting configuration
	if c.RateLimit.Enabled {
		if c.RateLimit.RequestsPerSecond < 1 {
			return fmt.Errorf("rate limit requests_per_second must be at least 1")
		}
		if c.RateLimit.Window <= 0 {
			return fmt.Errorf("rate limit window must be positive")
		}
	}

	// Validate health check configuration
	if c.HealthCheck.Interval <= 0 {
		return fmt.Errorf("health check interval must be positive")
	}
	if c.HealthCheck.Timeout <= 0 {
		return fmt.Errorf("health check timeout must be positive")
	}
	if c.HealthCheck.Timeout >= c.HealthCheck.Interval {
		return fmt.Errorf("health check timeout must be less than interval")
	}
	if c.HealthCheck.SuccessfulChecksForFailback < 1 {
		return fmt.Errorf("health check successful_checks_for_failback must be at least 1")
	}

	return nil
}

// GetAllNodes returns all configured beacon nodes
// The first beacon node is always treated as primary, followed by other beacon nodes in order
func (c *Config) GetAllNodes() []NodeConfig {
	return c.Beacons.parsedNodes
}

// GetListenAddr returns the formatted listen address
func (c *Config) GetListenAddr() string {
	return fmt.Sprintf(":%d", c.Server.Port)
}
