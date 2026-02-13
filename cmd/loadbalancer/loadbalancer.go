package loadbalancer

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/zircuit-labs/consensus-proxy/cmd/beaconnode"
	"github.com/zircuit-labs/consensus-proxy/cmd/config"
	"github.com/zircuit-labs/consensus-proxy/cmd/logger"
	"github.com/zircuit-labs/consensus-proxy/cmd/metrics"
	"github.com/zircuit-labs/consensus-proxy/cmd/validator"

	"github.com/gorilla/websocket"
)

// LoadBalancer manages multiple beacon nodes and handles load balancing
type LoadBalancer struct {
	nodes        []*beaconnode.BeaconNode
	config       *config.Config
	metrics      metrics.Client
	upgrader     websocket.Upgrader
	validator    *validator.BeaconEndpointValidator
	mu           sync.RWMutex
	healthyNodes []*beaconnode.BeaconNode
}

// New creates a new LoadBalancer instance
func New(cfg *config.Config) (*LoadBalancer, error) {
	allNodes := cfg.GetAllNodes()
	if len(allNodes) == 0 {
		return nil, fmt.Errorf("at least one beacon node is required")
	}

	nodes := make([]*beaconnode.BeaconNode, 0, len(allNodes))
	for i, nodeConfig := range allNodes {
		node, err := beaconnode.NewBeaconNode(nodeConfig, cfg)
		if err != nil {
			logger.Error("failed to create beacon node",
				"name", nodeConfig.Name,
				"url", nodeConfig.URL,
				"error", err,
			)
			continue
		}
		// Set priority: first node is primary (0), others are backups (1, 2, 3, ...)
		node.SetPriority(i)
		// Set original priority - this never changes and is used for failback
		node.OriginalPriority = i
		nodes = append(nodes, node)
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no valid beacon nodes configured")
	}

	lb := &LoadBalancer{
		nodes:     nodes,
		config:    cfg,
		validator: validator.NewBeaconEndpointValidator(),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for Web3 apps
			},
			ReadBufferSize:  cfg.WebSocket.ReadBufferSize,
			WriteBufferSize: cfg.WebSocket.WriteBufferSize,
		},
	}

	// Initialize metrics
	var err error
	lb.metrics, err = metrics.NewClient(&cfg.Metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize metrics client: %v", err)
	}

	return lb, nil
}

// GetNodes returns all configured nodes (for health/status endpoints)
func (lb *LoadBalancer) GetNodes() []*beaconnode.BeaconNode {
	return lb.nodes
}

// GetHealthyNodes returns a slice of currently healthy nodes
func (lb *LoadBalancer) GetHealthyNodes() []*beaconnode.BeaconNode {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return lb.healthyNodes
}
