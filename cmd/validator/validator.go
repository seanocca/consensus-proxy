package validator

import (
	"regexp"
	"strings"
)

// BeaconEndpointValidator validates that requests are for legitimate Beacon Chain API endpoints
type BeaconEndpointValidator struct {
	// Compiled regex patterns for efficient validation
	validPaths []*regexp.Regexp
}

// NewBeaconEndpointValidator creates a new validator with all valid Beacon Chain API patterns
func NewBeaconEndpointValidator() *BeaconEndpointValidator {
	// Based on official Ethereum Beacon Chain API specification
	// https://ethereum.github.io/beacon-APIs/
	patterns := []string{
		// Beacon endpoints
		`^/eth/v1/beacon/genesis$`,
		`^/eth/v1/beacon/states/[^/]+/root$`,
		`^/eth/v1/beacon/states/[^/]+/fork$`,
		`^/eth/v1/beacon/states/[^/]+/finality_checkpoints$`,
		`^/eth/v1/beacon/states/[^/]+/validators$`,
		`^/eth/v1/beacon/states/[^/]+/validators/[^/]+$`,
		`^/eth/v1/beacon/states/[^/]+/validator_balances$`,
		`^/eth/v1/beacon/states/[^/]+/committees$`,
		`^/eth/v1/beacon/states/[^/]+/sync_committees$`,
		`^/eth/v1/beacon/states/[^/]+/randao$`,
		`^/eth/v1/beacon/headers$`,
		`^/eth/v1/beacon/headers/[^/]+$`,
		`^/eth/v1/beacon/blocks/[^/]+$`,
		`^/eth/v1/beacon/blocks/[^/]+/root$`,
		`^/eth/v1/beacon/blocks/[^/]+/attestations$`,
		`^/eth/v1/beacon/blob_sidecars/[^/]+$`,
		`^/eth/v1/beacon/blobs/[^/]+$`,
		`^/eth/v1/beacon/pool/attestations$`,
		`^/eth/v1/beacon/pool/attester_slashings$`,
		`^/eth/v1/beacon/pool/proposer_slashings$`,
		`^/eth/v1/beacon/pool/voluntary_exits$`,
		`^/eth/v1/beacon/pool/bls_to_execution_changes$`,
		`^/eth/v1/beacon/light_client/bootstrap/[^/]+$`,
		`^/eth/v1/beacon/light_client/updates$`,
		`^/eth/v1/beacon/light_client/finality_update$`,
		`^/eth/v1/beacon/light_client/optimistic_update$`,
		`^/eth/v1/beacon/deposit_snapshot$`,
		`^/eth/v1/beacon/rewards/blocks/[^/]+$`,
		`^/eth/v1/beacon/rewards/attestations/[^/]+$`,
		`^/eth/v1/beacon/rewards/sync_committee/[^/]+$`,

		// V2 Beacon endpoints
		`^/eth/v2/beacon/blocks/[^/]+$`,
		`^/eth/v2/beacon/pool/attestations$`,

		// V3 Beacon endpoints
		`^/eth/v3/beacon/blocks/[^/]+$`,

		// Config endpoints
		`^/eth/v1/config/fork_schedule$`,
		`^/eth/v1/config/spec$`,
		`^/eth/v1/config/deposit_contract$`,

		// Debug endpoints (may want to restrict these in production)
		`^/eth/v1/debug/beacon/states/[^/]+$`,
		`^/eth/v1/debug/beacon/heads$`,
		`^/eth/v1/debug/fork_choice$`,
		`^/eth/v2/debug/beacon/states/[^/]+$`,
		`^/eth/v2/debug/beacon/heads$`,

		// Events (WebSocket)
		`^/eth/v1/events$`,

		// Node endpoints
		`^/eth/v1/node/identity$`,
		`^/eth/v1/node/peers$`,
		`^/eth/v1/node/peers/[^/]+$`,
		`^/eth/v1/node/peer_count$`,
		`^/eth/v1/node/version$`,
		`^/eth/v1/node/syncing$`,
		`^/eth/v1/node/health$`,

		// Validator endpoints
		`^/eth/v1/validator/duties/attester/[^/]+$`,
		`^/eth/v1/validator/duties/proposer/[^/]+$`,
		`^/eth/v1/validator/duties/sync/[^/]+$`,
		`^/eth/v1/validator/blocks/[^/]+$`,
		`^/eth/v1/validator/attestation_data$`,
		`^/eth/v1/validator/aggregate_attestation$`,
		`^/eth/v1/validator/aggregate_and_proofs$`,
		`^/eth/v1/validator/beacon_committee_subscriptions$`,
		`^/eth/v1/validator/sync_committee_subscriptions$`,
		`^/eth/v1/validator/sync_committee_contribution$`,
		`^/eth/v1/validator/contribution_and_proofs$`,
		`^/eth/v1/validator/prepare_beacon_proposer$`,
		`^/eth/v1/validator/register_validator$`,
		`^/eth/v1/validator/liveness/[^/]+$`,

		// V2 Validator endpoints
		`^/eth/v2/validator/blocks/[^/]+$`,
		`^/eth/v2/validator/aggregate_attestation$`,

		// V3 Validator endpoints
		`^/eth/v3/validator/blocks/[^/]+$`,

		// Builder endpoints (MEV-Boost)
		`^/eth/v1/builder/states/[^/]+/expected_withdrawals$`,

		// Rewards endpoints
		`^/eth/v1/beacon/rewards/blocks/[^/]+$`,
		`^/eth/v1/beacon/rewards/attestations/[^/]+$`,
		`^/eth/v1/beacon/rewards/sync_committee/[^/]+$`,
	}

	// Compile all patterns
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		compiled = append(compiled, re)
	}

	return &BeaconEndpointValidator{
		validPaths: compiled,
	}
}

// IsValidBeaconEndpoint checks if the given path is a valid Beacon Chain API endpoint
func (v *BeaconEndpointValidator) IsValidBeaconEndpoint(path string) bool {
	// Normalize the path
	path = strings.TrimSpace(path)

	// Remove trailing slashes
	path = strings.TrimRight(path, "/")

	// Empty paths are not valid
	if path == "" {
		return false
	}

	// Check against all patterns
	for _, pattern := range v.validPaths {
		if pattern.MatchString(path) {
			return true
		}
	}

	return false
}

// GetValidationError returns a descriptive error for an invalid endpoint
func (v *BeaconEndpointValidator) GetValidationError(path string) string {
	if !v.IsValidBeaconEndpoint(path) {
		return "Invalid Beacon Chain API endpoint: " + path
	}
	return ""
}
