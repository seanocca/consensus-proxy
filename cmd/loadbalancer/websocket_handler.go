package loadbalancer

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/zircuit-labs/consensus-proxy/cmd/beaconnode"
	"github.com/zircuit-labs/consensus-proxy/cmd/logger"
)

// handleWebSocket handles WebSocket connections with failover
func (lb *LoadBalancer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Validate endpoint
	if !lb.validator.IsValidBeaconEndpoint(r.URL.Path) {
		logger.Warn("invalid beacon endpoint attempted via websocket",
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
		)
		http.Error(w, "Invalid Beacon Chain API endpoint", http.StatusForbidden)
		if lb.metrics != nil {
			lb.metrics.Incr("request.invalid_endpoint", []string{"protocol:websocket"}, 1)
		}
		return
	}

	// Get healthy nodes with proper locking to avoid race conditions
	healthyNodes := lb.GetHealthyNodes()

	if len(healthyNodes) == 0 {
		http.Error(w, "No healthy beacon nodes available", http.StatusServiceUnavailable)
		return
	}

	// Try to establish WebSocket connection to a healthy node
	upstreamConn, selectedNode := lb.connectToUpstreamWebSocket(healthyNodes, r)
	if upstreamConn == nil {
		http.Error(w, "Failed to establish WebSocket connection to any node", http.StatusBadGateway)
		return
	}
	defer upstreamConn.Close()

	// Upgrade client connection
	clientConn, err := lb.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("failed to upgrade client websocket connection",
			"error", err,
			"remote_addr", r.RemoteAddr,
		)
		return
	}
	defer clientConn.Close()

	selectedNode.IncrementRequests()

	// Send metrics
	if lb.metrics != nil {
		lb.metrics.Incr("websocket.connected", []string{
			fmt.Sprintf("node:%s", selectedNode.Name),
		}, 1)
	}

	logger.Info("websocket proxy established",
		"node_name", selectedNode.Name,
		"node_url", selectedNode.URL,
		"client_addr", r.RemoteAddr,
		"path", r.URL.Path,
	)

	// Proxy WebSocket messages bidirectionally
	lb.proxyWebSocketMessages(clientConn, upstreamConn, selectedNode, r.RemoteAddr)
}

// connectToUpstreamWebSocket attempts to establish a WebSocket connection to a healthy node
func (lb *LoadBalancer) connectToUpstreamWebSocket(healthyNodes []*beaconnode.BeaconNode, r *http.Request) (*websocket.Conn, *beaconnode.BeaconNode) {
	for _, node := range healthyNodes {
		// Convert HTTP URL to WebSocket URL
		wsURL := strings.Replace(node.URL, "http://", "ws://", 1)
		wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
		wsURL += r.URL.Path + "?" + r.URL.RawQuery

		upstreamConn, _, err := websocket.DefaultDialer.Dial(wsURL, r.Header)
		if err != nil {
			logger.Warn("websocket connection failed",
				"node_name", node.Name,
				"url", wsURL,
				"error", err,
			)
			node.IncrementError()

			// Check if primary node has exceeded error threshold
			if node.IsPrimary() {
				consecutiveErrors := node.ConsecutiveErrors
				if consecutiveErrors >= int64(lb.config.Failover.ErrorThreshold) {
					logger.Warn("primary node websocket failover - demoting to backup priority",
						"node_name", node.Name,
						"consecutive_errors", consecutiveErrors,
						"threshold", lb.config.Failover.ErrorThreshold,
					)

					// Demote primary to backup priority
					maxPriority := len(lb.nodes)
					node.SetPriority(maxPriority)
					logger.Info("primary node demoted to backup (websocket)",
						"node_name", node.Name,
						"new_priority", maxPriority,
					)

					// Remove from healthy nodes
					lb.mu.Lock()
					updatedHealthy := make([]*beaconnode.BeaconNode, 0)
					for _, n := range lb.healthyNodes {
						if n.Name != node.Name {
							updatedHealthy = append(updatedHealthy, n)
						}
					}
					lb.healthyNodes = updatedHealthy
					lb.mu.Unlock()

					if lb.metrics != nil {
						lb.metrics.Incr("node.primary_demoted", []string{
							fmt.Sprintf("node:%s", node.Name),
							"protocol:websocket",
						}, 1)
					}
				}
			}

			continue
		}

		return upstreamConn, node
	}

	return nil, nil
}

// proxyWebSocketMessages handles bidirectional message forwarding between client and upstream
func (lb *LoadBalancer) proxyWebSocketMessages(clientConn, upstreamConn *websocket.Conn, node *beaconnode.BeaconNode, clientAddr string) {
	errChan := make(chan error, 2)

	// Forward messages from client to upstream
	go func() {
		for {
			messageType, data, err := clientConn.ReadMessage()
			if err != nil {
				errChan <- fmt.Errorf("client read error: %v", err)
				return
			}

			if err := upstreamConn.WriteMessage(messageType, data); err != nil {
				errChan <- fmt.Errorf("upstream write error: %v", err)
				return
			}
		}
	}()

	// Forward messages from upstream to client
	go func() {
		for {
			messageType, data, err := upstreamConn.ReadMessage()
			if err != nil {
				errChan <- fmt.Errorf("upstream read error: %v", err)
				return
			}

			if err := clientConn.WriteMessage(messageType, data); err != nil {
				errChan <- fmt.Errorf("client write error: %v", err)
				return
			}
		}
	}()

	// Wait for connection to close
	err := <-errChan
	logger.Info("websocket connection closed",
		"node_name", node.Name,
		"client_addr", clientAddr,
		"reason", err.Error(),
	)

	if lb.metrics != nil {
		lb.metrics.Incr("websocket.disconnected", []string{
			fmt.Sprintf("node:%s", node.Name),
		}, 1)
	}
}
