package beaconnode

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/zircuit-labs/consensus-proxy/cmd/config"
	"github.com/zircuit-labs/consensus-proxy/cmd/logger"
)

// SyncingResponse represents the response from /eth/v1/node/syncing endpoint
type SyncingResponse struct {
	Data struct {
		IsSyncing    bool   `json:"is_syncing"`
		SyncDistance string `json:"sync_distance"`
	} `json:"data"`
}

// CheckSyncStatus performs a lightweight health check on the beacon node using `/eth/v1/node/syncing` endpoint.
// Returns true if the node is not syncing (is_syncing=false) and sync_distance=0.
// This is a simpler version without extensive logging, suitable for startup checks.
func (bn *BeaconNode) CheckSyncStatus(timeout config.HealthCheckConfig) (bool, error) {
	endpoint := bn.URL + "/eth/v1/node/syncing"

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: timeout.Timeout,
	}

	// Make GET request to syncing endpoint
	resp, err := client.Get(endpoint)
	if err != nil {
		return false, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("non-200 status code: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse JSON response
	var syncResp SyncingResponse
	if err := json.Unmarshal(body, &syncResp); err != nil {
		return false, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Check if node is synced: is_syncing must be false and sync_distance must be "0"
	isHealthy := !syncResp.Data.IsSyncing && syncResp.Data.SyncDistance == "0"

	if !isHealthy {
		return false, fmt.Errorf("not synced (is_syncing=%t, sync_distance=%s)", syncResp.Data.IsSyncing, syncResp.Data.SyncDistance)
	}

	return true, nil
}

// HealthCheckError represents detailed information about a health check failure
type HealthCheckError struct {
	Reason       string
	StatusCode   int
	IsSyncing    bool
	SyncDistance string
	Err          error
}

func (e *HealthCheckError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Reason, e.Err)
	}
	return e.Reason
}

// HealthCheck performs a health check on the beacon node using `/eth/v1/node/syncing` endpoint.
// Returns true if the node is not syncing (is_syncing=false) and sync_distance=0.
// Returns an error with detailed information if the check fails.
func (bn *BeaconNode) HealthCheck(cfg config.HealthCheckConfig) (bool, *HealthCheckError) {
	endpoint := bn.URL + "/eth/v1/node/syncing"

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: cfg.Timeout,
	}

	// Make GET request to syncing endpoint
	resp, err := client.Get(endpoint)
	if err != nil {
		logger.Warn("health check request failed",
			"node_name", bn.Name,
			"error", err,
		)
		return false, &HealthCheckError{
			Reason: "request_failed",
			Err:    err,
		}
	}
	defer resp.Body.Close()

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		logger.Warn("health check returned non-200 status",
			"node_name", bn.Name,
			"status_code", resp.StatusCode,
		)
		return false, &HealthCheckError{
			Reason:     "non_200_status",
			StatusCode: resp.StatusCode,
		}
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Warn("health check failed to read response body",
			"node_name", bn.Name,
			"error", err,
		)
		return false, &HealthCheckError{
			Reason: "read_body_failed",
			Err:    err,
		}
	}

	// Parse JSON response
	var syncResp SyncingResponse
	if err := json.Unmarshal(body, &syncResp); err != nil {
		logger.Warn("health check failed to parse JSON response",
			"node_name", bn.Name,
			"error", err,
			"body", string(body),
		)
		return false, &HealthCheckError{
			Reason: "json_parse_failed",
			Err:    err,
		}
	}

	// Check if node is synced: is_syncing must be false and sync_distance must be "0"
	isHealthy := !syncResp.Data.IsSyncing && syncResp.Data.SyncDistance == "0"

	if !isHealthy {
		logger.Info("node not fully synced",
			"node_name", bn.Name,
			"is_syncing", syncResp.Data.IsSyncing,
			"sync_distance", syncResp.Data.SyncDistance,
		)

		// Determine the specific reason
		var reason string
		if syncResp.Data.IsSyncing {
			reason = "is_syncing"
		} else if syncResp.Data.SyncDistance != "0" {
			reason = "sync_distance_not_zero"
		} else {
			reason = "not_synced"
		}

		return false, &HealthCheckError{
			Reason:       reason,
			IsSyncing:    syncResp.Data.IsSyncing,
			SyncDistance: syncResp.Data.SyncDistance,
		}
	}

	logger.Debug("node health check passed",
		"node_name", bn.Name,
		"priority", bn.Priority,
	)

	return true, nil
}
