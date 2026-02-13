package loadbalancer

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/zircuit-labs/consensus-proxy/cmd/beaconnode"
	"github.com/zircuit-labs/consensus-proxy/cmd/logger"
)

// ServeHTTP implements the http.Handler interface
func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Validate endpoint before processing
	if !lb.validator.IsValidBeaconEndpoint(r.URL.Path) {
		logger.Warn("invalid beacon endpoint attempted",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)
		http.Error(w, "Invalid Beacon Chain API endpoint", http.StatusForbidden)
		if lb.metrics != nil {
			lb.metrics.Incr("request.invalid_endpoint", []string{"protocol:http"}, 1)
		}
		return
	}

	// Check if this is a WebSocket upgrade request
	if websocket.IsWebSocketUpgrade(r) {
		lb.handleWebSocket(w, r)
		return
	}

	// Regular HTTP request handling
	lb.handleHTTPRequest(w, r, start)
}

// handleHTTPRequest processes regular HTTP requests with retry logic
func (lb *LoadBalancer) handleHTTPRequest(w http.ResponseWriter, r *http.Request, start time.Time) {
	var lastStatusCode int

	// Create overall request timeout context
	overallCtx, overallCancel := context.WithTimeout(r.Context(), lb.config.Server.RequestTimeout)
	defer overallCancel()

	// Get healthy nodes with proper locking to avoid race conditions
	healthyNodes := lb.GetHealthyNodes()

	// Try each node in sequence until success
	for i, node := range healthyNodes {
		if i >= lb.config.Server.MaxRetries {
			break
		}

		// Check if overall timeout has been exceeded
		if lb.checkRequestTimeout(overallCtx, start, r, node.Name) {
			return
		}

		// Calculate remaining timeout for this attempt
		remainingTimeout := lb.config.Server.RequestTimeout - time.Since(start)
		if remainingTimeout <= 0 {
			http.Error(w, "Request timeout", http.StatusGatewayTimeout)
			return
		}

		// Attempt the request
		recorder, attemptDuration := lb.attemptNodeRequest(overallCtx, node, r, remainingTimeout, i)
		lastStatusCode = recorder.statusCode

		// Send metrics for this attempt
		lb.recordAttemptMetrics(node.Name, lastStatusCode, attemptDuration, i)

		// Check if response was successful
		if lastStatusCode >= HTTPStatusSuccessMin && lastStatusCode < HTTPStatusSuccessMax {
			lb.handleSuccessResponse(w, r, node, recorder, start, lastStatusCode)
			return
		}

		// Failed - handle error
		lb.handleNodeError(node, r, lastStatusCode, i)

		// If this wasn't the last attempt, continue to next node
		if i < len(healthyNodes)-1 && i < lb.config.Server.MaxRetries-1 {
			continue
		}
	}

	// All attempts failed
	lb.handleAllNodesFailed(r, start, lastStatusCode, len(healthyNodes))
	http.Error(w, "All beacon nodes unavailable", http.StatusBadGateway)
}

// checkRequestTimeout checks if the overall request timeout has been exceeded
func (lb *LoadBalancer) checkRequestTimeout(ctx context.Context, start time.Time, r *http.Request, nodeName string) bool {
	select {
	case <-ctx.Done():
		totalDuration := time.Since(start)
		logger.Warn("request timeout exceeded before attempting node",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", totalDuration.String(),
			"timeout", lb.config.Server.RequestTimeout.String(),
			"node", nodeName,
		)
		return true
	default:
		return false
	}
}

// attemptNodeRequest attempts to proxy a request to a specific node
func (lb *LoadBalancer) attemptNodeRequest(overallCtx context.Context, node *beaconnode.BeaconNode, r *http.Request, timeout time.Duration, attemptNum int) (*responseRecorder, time.Duration) {
	ctx, cancel := context.WithTimeout(overallCtx, timeout)
	defer cancel()

	reqWithTimeout := r.WithContext(ctx)
	recorder := &responseRecorder{}

	node.IncrementRequests()
	attemptStart := time.Now()

	// Try the request
	node.Proxy.ServeHTTP(recorder, reqWithTimeout)

	return recorder, time.Since(attemptStart)
}

// recordAttemptMetrics sends metrics for a request attempt
func (lb *LoadBalancer) recordAttemptMetrics(nodeName string, statusCode int, duration time.Duration, attemptNum int) {
	if lb.metrics != nil {
		lb.metrics.Timing("request.attempt_duration", duration, []string{
			fmt.Sprintf("node:%s", nodeName),
			fmt.Sprintf("status_code:%d", statusCode),
			fmt.Sprintf("attempt:%d", attemptNum+1),
		}, 1)
	}
}

// handleSuccessResponse handles a successful response from a node
func (lb *LoadBalancer) handleSuccessResponse(w http.ResponseWriter, r *http.Request, node *beaconnode.BeaconNode, recorder *responseRecorder, start time.Time, statusCode int) {
	// Success! Reset consecutive errors and copy response to actual ResponseWriter
	node.ResetErrors()
	recorder.copyToResponseWriter(w)

	totalDuration := time.Since(start)
	// Use the specialized request logging method
	log := logger.Default()
	log.LogRequest(r.Method, r.URL.Path, r.UserAgent(), totalDuration, statusCode, node.Name)

	if lb.metrics != nil {
		lb.metrics.Timing("request.duration", totalDuration, []string{
			fmt.Sprintf("node:%s", node.Name),
			fmt.Sprintf("status_code:%d", statusCode),
			"result:success",
		}, 1)
		lb.metrics.Incr("request.success", []string{
			fmt.Sprintf("node:%s", node.Name),
		}, 1)
	}
}

// handleNodeError handles an error response from a node
func (lb *LoadBalancer) handleNodeError(node *beaconnode.BeaconNode, r *http.Request, statusCode int, attemptNum int) {
	if statusCode >= HTTPStatusServerErrorMin {
		node.IncrementError()
		consecutiveErrors := atomic.LoadInt64(&node.ConsecutiveErrors)

		// Check if this is the primary node and if we've reached threshold
		if node.IsPrimary() && consecutiveErrors >= int64(lb.config.Failover.ErrorThreshold) {
			logger.Warn("primary node failover triggered - demoting to backup priority",
				"node_name", node.Name,
				"node_url", node.URL,
				"consecutive_errors", consecutiveErrors,
				"threshold", lb.config.Failover.ErrorThreshold,
				"status_code", statusCode,
				"attempt", attemptNum+1,
			)

			// Demote primary to backup priority (use high priority number to put it at the end)
			// This ensures it will be healthchecked periodically along with other backups
			maxPriority := len(lb.nodes)
			node.SetPriority(maxPriority)
			logger.Info("primary node demoted to backup",
				"node_name", node.Name,
				"new_priority", maxPriority,
			)

			// Remove from healthy nodes immediately
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
				}, 1)
			}
		}
	} else if statusCode >= HTTPStatusClientErrorMin {
		// For client errors (4xx), don't increment error count as it's likely a request issue
		lb.logClientError(node, r, statusCode, attemptNum)
	}

	if lb.metrics != nil {
		lb.metrics.Incr("request.failover", []string{
			fmt.Sprintf("from_node:%s", node.Name),
			fmt.Sprintf("status_code:%d", statusCode),
		}, 1)
	}
}

// logClientError logs detailed information about client errors for debugging
func (lb *LoadBalancer) logClientError(node *beaconnode.BeaconNode, r *http.Request, statusCode int, attemptNum int) {
	// Build full URL with parameters for debugging
	fullURL := node.URL + r.URL.Path
	if r.URL.RawQuery != "" {
		fullURL += "?" + r.URL.RawQuery
	}

	// Collect key headers for debugging API provider issues
	headers := make(map[string]string)
	for _, key := range []string{"User-Agent", "Accept", "Content-Type", "Host", "Authorization", "X-Forwarded-Proto"} {
		if value := r.Header.Get(key); value != "" {
			headers[key] = value
		}
	}

	logger.Warn("client error from beacon node",
		"node_name", node.Name,
		"node_url", node.URL,
		"full_request_url", fullURL,
		"request_method", r.Method,
		"request_path", r.URL.Path,
		"request_params", r.URL.RawQuery,
		"request_headers", headers,
		"status_code", statusCode,
		"attempt", attemptNum+1,
	)
}

// handleAllNodesFailed logs and records metrics when all nodes have failed
func (lb *LoadBalancer) handleAllNodesFailed(r *http.Request, start time.Time, lastStatusCode int, attempts int) {
	totalDuration := time.Since(start)
	logger.Error("all beacon nodes failed",
		"method", r.Method,
		"path", r.URL.Path,
		"total_duration", totalDuration.String(),
		"last_status_code", lastStatusCode,
		"attempts", attempts,
		"max_retries", lb.config.Server.MaxRetries,
	)

	if lb.metrics != nil {
		lb.metrics.Timing("request.duration", totalDuration, []string{
			"node:all",
			fmt.Sprintf("status_code:%d", lastStatusCode),
			"result:failure",
		}, 1)
		lb.metrics.Incr("request.failure", nil, 1)
	}
}
