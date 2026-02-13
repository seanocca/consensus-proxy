package loadbalancer

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/zircuit-labs/consensus-proxy/cmd/beaconnode"
	"github.com/zircuit-labs/consensus-proxy/cmd/logger"
)

// healthCheckResult holds the result of a health check for a single node
type healthCheckResult struct {
	node      *beaconnode.BeaconNode
	isHealthy bool
	err       error
}

// StartupHealthCheck performs health checks on all nodes concurrently before the application starts serving requests.
// It returns an error if no nodes are healthy, or logs warnings for unhealthy nodes.
func (lb *LoadBalancer) StartupHealthCheck() error {
	logger.Info("performing startup health checks on all beacon nodes")

	// Create a channel to receive results
	resultsChan := make(chan healthCheckResult, len(lb.nodes))
	var wg sync.WaitGroup

	// Launch concurrent health checks for all nodes
	for _, node := range lb.nodes {
		wg.Add(1)
		go func(n *beaconnode.BeaconNode) {
			defer wg.Done()

			logger.Info("checking node health",
				"node_name", n.Name,
			)

			// Use lightweight CheckSyncStatus method to avoid double logging
			isHealthy, err := n.CheckSyncStatus(lb.config.HealthCheck)

			resultsChan <- healthCheckResult{
				node:      n,
				isHealthy: isHealthy,
				err:       err,
			}
		}(node)
	}

	// Wait for all checks to complete and close the channel
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	unhealthyNodes := []string{}
	lb.healthyNodes = make([]*beaconnode.BeaconNode, 0)

	for result := range resultsChan {
		if result.isHealthy {
			lb.healthyNodes = append(lb.healthyNodes, result.node)
			logger.Info("node is healthy and synced",
				"node_name", result.node.Name,
			)
		} else {
			unhealthyNodes = append(unhealthyNodes, result.node.Name)
			logger.Warn("node is not healthy or not synced",
				"node_name", result.node.Name,
				"error", result.err,
			)
		}
	}

	// Sort healthy nodes by priority (lower priority number = higher priority)
	sort.Slice(lb.healthyNodes, func(i, j int) bool {
		return lb.healthyNodes[i].GetPriority() < lb.healthyNodes[j].GetPriority()
	})

	// Log summary
	logger.Info("startup health check completed",
		"total_nodes", len(lb.nodes),
		"healthy_nodes", len(lb.healthyNodes),
		"unhealthy_nodes", len(unhealthyNodes),
	)

	// Fail if no nodes are healthy
	if len(lb.healthyNodes) == 0 {
		return fmt.Errorf("startup health check failed: no healthy nodes available (total nodes: %d)", len(lb.nodes))
	}

	// Warn if some nodes are unhealthy
	if len(unhealthyNodes) > 0 {
		logger.Warn("some nodes are unhealthy at startup",
			"unhealthy_node_names", unhealthyNodes,
			"healthy_count", len(lb.healthyNodes),
		)
	}

	return nil
}

// StartPeriodicHealthCheck starts a background goroutine that periodically checks all nodes
// and updates the healthyNodes slice. This ensures backup servers are monitored continuously.
func (lb *LoadBalancer) StartPeriodicHealthCheck() {
	ticker := time.NewTicker(lb.config.HealthCheck.Interval)

	go func() {
		for range ticker.C {
			lb.performHealthCheck()
		}
	}()

	logger.Info("started periodic health check routine",
		"interval", lb.config.HealthCheck.Interval,
	)
}

// performHealthCheck checks backup nodes and updates the healthyNodes list
// Only backup nodes are checked periodically; primary is checked on-demand when failing
func (lb *LoadBalancer) performHealthCheck() {
	// Get backup nodes to check
	backupNodes := make([]*beaconnode.BeaconNode, 0)
	for _, node := range lb.nodes {
		if node.IsBackup() {
			backupNodes = append(backupNodes, node)
		}
	}

	if len(backupNodes) == 0 {
		logger.Debug("no backup nodes to health check")
		return
	}

	logger.Debug("performing periodic health check on backup nodes", "count", len(backupNodes))

	// Create a channel to receive results
	resultsChan := make(chan healthCheckResult, len(backupNodes))
	var wg sync.WaitGroup

	// Launch concurrent health checks for backup nodes only
	for _, node := range backupNodes {
		wg.Add(1)
		go func(n *beaconnode.BeaconNode) {
			defer wg.Done()

			// Use the HealthCheck method
			isHealthy, healthErr := n.HealthCheck(lb.config.HealthCheck)

			resultsChan <- healthCheckResult{
				node:      n,
				isHealthy: isHealthy,
				err:       healthErr,
			}
		}(node)
	}

	// Wait for all checks to complete and close the channel
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results and update healthyNodes for backups
	newHealthyBackups := make([]*beaconnode.BeaconNode, 0)
	unhealthyBackupCount := 0

	for result := range resultsChan {
		if result.isHealthy {
			// Increment consecutive successes for healthy nodes
			result.node.IncrementSuccess()
			result.node.ResetErrors()
			newHealthyBackups = append(newHealthyBackups, result.node)

			// Emit success metric
			if lb.metrics != nil {
				lb.metrics.Incr("healthcheck.success", []string{
					fmt.Sprintf("node:%s", result.node.Name),
				}, 1)
			}
		} else {
			// Reset consecutive successes for unhealthy nodes
			result.node.ResetSuccesses()
			unhealthyBackupCount++

			// Emit failure metrics with detailed reason
			if lb.metrics != nil {
				if healthErr, ok := result.err.(*beaconnode.HealthCheckError); ok {
					// Handle specific health check errors
					switch healthErr.Reason {
					case "request_failed":
						lb.metrics.Incr("healthcheck.failed", []string{
							fmt.Sprintf("node:%s", result.node.Name),
							"reason:request_failed",
						}, 1)
					case "non_200_status":
						lb.metrics.Incr("healthcheck.failed", []string{
							fmt.Sprintf("node:%s", result.node.Name),
							"reason:non_200_status",
							fmt.Sprintf("status_code:%d", healthErr.StatusCode),
						}, 1)
					case "read_body_failed":
						lb.metrics.Incr("healthcheck.failed", []string{
							fmt.Sprintf("node:%s", result.node.Name),
							"reason:read_body_failed",
						}, 1)
					case "json_parse_failed":
						lb.metrics.Incr("healthcheck.failed", []string{
							fmt.Sprintf("node:%s", result.node.Name),
							"reason:json_parse_failed",
						}, 1)
					case "is_syncing", "sync_distance_not_zero", "not_synced":
						// Node is not synced
						lb.metrics.Incr("healthcheck.not_synced", []string{
							fmt.Sprintf("node:%s", result.node.Name),
							fmt.Sprintf("reason:%s", healthErr.Reason),
							fmt.Sprintf("is_syncing:%t", healthErr.IsSyncing),
							fmt.Sprintf("sync_distance:%s", healthErr.SyncDistance),
						}, 1)
					default:
						// Generic failure
						lb.metrics.Incr("healthcheck.failed", []string{
							fmt.Sprintf("node:%s", result.node.Name),
							"reason:unknown",
						}, 1)
					}
				} else if result.err != nil {
					// Generic error
					lb.metrics.Incr("healthcheck.failed", []string{
						fmt.Sprintf("node:%s", result.node.Name),
						"reason:unknown",
					}, 1)
				}
			}
		}
	}

	// Update the healthyNodes slice with thread safety
	lb.mu.Lock()

	// Check if original primary is ready for failback
	var originalPrimary *beaconnode.BeaconNode
	for _, node := range newHealthyBackups {
		if node.OriginalPriority == 0 {
			originalPrimary = node
			break
		}
	}

	// Determine if we should perform failback
	shouldFailback := false
	if originalPrimary != nil {
		consecutiveSuccesses := originalPrimary.GetConsecutiveSuccesses()
		if consecutiveSuccesses >= int64(lb.config.HealthCheck.SuccessfulChecksForFailback) {
			shouldFailback = true
			logger.Info("original primary ready for failback",
				"node_name", originalPrimary.Name,
				"consecutive_successes", consecutiveSuccesses,
				"required", lb.config.HealthCheck.SuccessfulChecksForFailback,
			)
		}
	}

	// Find current primary
	var currentPrimary *beaconnode.BeaconNode
	for _, node := range lb.nodes {
		if node.IsPrimary() {
			currentPrimary = node
			break
		}
	}

	// Perform failback if conditions are met
	if shouldFailback && currentPrimary != nil && currentPrimary.Name != originalPrimary.Name {
		// Restore original primary to priority 0
		originalPrimary.SetPriority(0)
		originalPrimary.ResetErrors()
		logger.Info("failback: restoring original primary node",
			"node_name", originalPrimary.Name,
			"original_priority", originalPrimary.OriginalPriority,
		)

		// Demote current primary back to its original priority
		currentPrimary.SetPriority(currentPrimary.OriginalPriority)
		logger.Info("failback: demoting temporary primary to original priority",
			"node_name", currentPrimary.Name,
			"restored_priority", currentPrimary.OriginalPriority,
		)

		if lb.metrics != nil {
			lb.metrics.Incr("node.failback_to_original_primary", []string{
				fmt.Sprintf("node:%s", originalPrimary.Name),
			}, 1)
		}
	}

	// Restore all healthy nodes to their original priorities
	for _, node := range newHealthyBackups {
		if node.GetPriority() != node.OriginalPriority {
			logger.Debug("restoring node to original priority",
				"node_name", node.Name,
				"current_priority", node.GetPriority(),
				"original_priority", node.OriginalPriority,
			)
			node.SetPriority(node.OriginalPriority)
		}
	}

	// Build updated healthy nodes list including current primary if healthy
	updatedHealthyNodes := make([]*beaconnode.BeaconNode, 0)

	// Add current primary if it's in the healthy list (from previous checks or just promoted)
	currentPrimary = nil // Re-check after potential failback
	for _, node := range lb.nodes {
		if node.IsPrimary() {
			currentPrimary = node
			// Check if it's in the healthy list
			for _, healthyNode := range lb.healthyNodes {
				if healthyNode.Name == node.Name {
					updatedHealthyNodes = append(updatedHealthyNodes, node)
					break
				}
			}
			break
		}
	}

	// If no primary exists, promote the first healthy backup
	if currentPrimary == nil && len(newHealthyBackups) > 0 {
		// Find the backup with OriginalPriority == 0 first (original primary)
		var nodeToPromote *beaconnode.BeaconNode
		for _, node := range newHealthyBackups {
			if node.OriginalPriority == 0 {
				nodeToPromote = node
				break
			}
		}

		// If original primary not available, promote lowest priority backup
		if nodeToPromote == nil {
			lowestPriority := int(^uint(0) >> 1) // max int
			for _, node := range newHealthyBackups {
				if node.GetPriority() < lowestPriority && node.GetPriority() > 0 {
					lowestPriority = node.GetPriority()
					nodeToPromote = node
				}
			}
		}

		// Promote the selected node to primary
		if nodeToPromote != nil {
			previousPriority := nodeToPromote.GetPriority()
			nodeToPromote.SetPriority(0)
			nodeToPromote.ResetErrors()
			logger.Info("backup node promoted to primary",
				"node_name", nodeToPromote.Name,
				"previous_priority", previousPriority,
				"original_priority", nodeToPromote.OriginalPriority,
			)

			if lb.metrics != nil {
				lb.metrics.Incr("node.backup_promoted", []string{
					fmt.Sprintf("node:%s", nodeToPromote.Name),
				}, 1)
			}
		}
	}

	// Add all healthy backup nodes
	updatedHealthyNodes = append(updatedHealthyNodes, newHealthyBackups...)
	lb.healthyNodes = updatedHealthyNodes
	lb.mu.Unlock()

	// Log summary
	logger.Debug("periodic health check completed",
		"backup_nodes_checked", len(backupNodes),
		"healthy_backups", len(newHealthyBackups),
		"unhealthy_backups", unhealthyBackupCount,
	)

	// Emit metrics
	if lb.metrics != nil {
		lb.metrics.Gauge("loadbalancer.healthy_backup_nodes", float64(len(newHealthyBackups)), nil, 1)
		lb.metrics.Gauge("loadbalancer.unhealthy_backup_nodes", float64(unhealthyBackupCount), nil, 1)
	}
}
