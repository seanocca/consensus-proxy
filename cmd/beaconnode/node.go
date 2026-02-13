package beaconnode

import (
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zircuit-labs/consensus-proxy/cmd/config"
)

// BeaconNode represents a single beacon node with its proxy and status
type BeaconNode struct {
	Name                 string
	URL                  string
	Proxy                *httputil.ReverseProxy
	ConsecutiveErrors    int64 // atomic consecutive error counter
	ConsecutiveSuccesses int64 // atomic consecutive success counter (for failback)
	TotalFailures        int64 // atomic total failure counter
	Requests             int64 // atomic request counter
	LastCheck            time.Time
	mu                   sync.RWMutex
	Priority             int
	OriginalPriority     int // The node's initial configured priority (never changes)
}

// NewBeaconNode creates a new beacon node with reverse proxy
func NewBeaconNode(nodeConfig config.NodeConfig, cfg *config.Config) (*BeaconNode, error) {
	parsedURL, err := url.Parse(nodeConfig.URL)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(parsedURL)

	// Configure proxy Director with custom header handling
	proxy.Director = createProxyDirector(proxy.Director, parsedURL, cfg.Proxy.UserAgent, nodeConfig.URL)

	// Configure proxy with optimized transport settings
	proxy.Transport = createProxyTransport(cfg, nodeConfig.URL)

	node := &BeaconNode{
		Name:              nodeConfig.Name,
		URL:               nodeConfig.URL,
		Proxy:             proxy,
		ConsecutiveErrors: 0, // start with no errors
		LastCheck:         time.Now(),
	}

	return node, nil
}

// IsHealthy returns whether the node is currently healthy based on error threshold
func (bn *BeaconNode) IsHealthy(errorThreshold int) bool {
	return atomic.LoadInt64(&bn.ConsecutiveErrors) < int64(errorThreshold)
}

// ResetErrors resets the consecutive error count on successful request
func (bn *BeaconNode) ResetErrors() {
	atomic.StoreInt64(&bn.ConsecutiveErrors, 0)
}

// IncrementError increments both consecutive and total error counts
func (bn *BeaconNode) IncrementError() {
	atomic.AddInt64(&bn.ConsecutiveErrors, 1)
	atomic.AddInt64(&bn.TotalFailures, 1)
	// Reset consecutive successes when an error occurs
	atomic.StoreInt64(&bn.ConsecutiveSuccesses, 0)
}

// IncrementSuccess increments the consecutive success count (for failback tracking)
func (bn *BeaconNode) IncrementSuccess() {
	atomic.AddInt64(&bn.ConsecutiveSuccesses, 1)
}

// ResetSuccesses resets the consecutive success count
func (bn *BeaconNode) ResetSuccesses() {
	atomic.StoreInt64(&bn.ConsecutiveSuccesses, 0)
}

// GetConsecutiveSuccesses returns the current consecutive success count
func (bn *BeaconNode) GetConsecutiveSuccesses() int64 {
	return atomic.LoadInt64(&bn.ConsecutiveSuccesses)
}

// IncrementRequests increments the request counter
func (bn *BeaconNode) IncrementRequests() {
	atomic.AddInt64(&bn.Requests, 1)
}

// GetStats returns current node statistics
func (bn *BeaconNode) GetStats() (consecutiveErrors int64, totalFailures int64, requests int64) {
	return atomic.LoadInt64(&bn.ConsecutiveErrors), atomic.LoadInt64(&bn.TotalFailures), atomic.LoadInt64(&bn.Requests)
}

// SetPriority sets the priority of the node (0 = primary, 1+ = backup)
func (bn *BeaconNode) SetPriority(priority int) {
	bn.mu.Lock()
	defer bn.mu.Unlock()
	bn.Priority = priority
}

// GetPriority returns the current priority of the node
func (bn *BeaconNode) GetPriority() int {
	bn.mu.RLock()
	defer bn.mu.RUnlock()
	return bn.Priority
}

// IsPrimary returns true if this is the primary node (priority 0)
func (bn *BeaconNode) IsPrimary() bool {
	return bn.GetPriority() == 0
}

// IsBackup returns true if this is a backup node (priority > 0)
func (bn *BeaconNode) IsBackup() bool {
	return bn.GetPriority() > 0
}
