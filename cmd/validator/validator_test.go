package validator

import (
	"testing"
)

func TestBeaconEndpointValidator_ValidEndpoints(t *testing.T) {
	validator := NewBeaconEndpointValidator()

	validEndpoints := []string{
		// Beacon endpoints
		"/eth/v1/beacon/genesis",
		"/eth/v1/beacon/states/head/root",
		"/eth/v1/beacon/states/finalized/fork",
		"/eth/v1/beacon/states/0x1234/validators",
		"/eth/v1/beacon/states/12345/validator_balances",
		"/eth/v1/beacon/states/head/committees",
		"/eth/v1/beacon/headers",
		"/eth/v1/beacon/headers/head",
		"/eth/v1/beacon/blocks/12345",
		"/eth/v1/beacon/blocks/head/attestations",
		"/eth/v1/beacon/pool/attestations",
		"/eth/v1/beacon/pool/attester_slashings",
		"/eth/v1/beacon/pool/proposer_slashings",
		"/eth/v1/beacon/pool/voluntary_exits",

		// V2 endpoints
		"/eth/v2/beacon/blocks/head",
		"/eth/v2/beacon/pool/attestations",

		// V3 endpoints
		"/eth/v3/beacon/blocks/head",

		// Config endpoints
		"/eth/v1/config/fork_schedule",
		"/eth/v1/config/spec",
		"/eth/v1/config/deposit_contract",

		// Debug endpoints
		"/eth/v1/debug/beacon/states/head",
		"/eth/v1/debug/beacon/heads",
		"/eth/v1/debug/fork_choice",

		// Events
		"/eth/v1/events",

		// Node endpoints
		"/eth/v1/node/identity",
		"/eth/v1/node/peers",
		"/eth/v1/node/peers/16Uiu2HAm1",
		"/eth/v1/node/peer_count",
		"/eth/v1/node/version",
		"/eth/v1/node/syncing",
		"/eth/v1/node/health",

		// Validator endpoints
		"/eth/v1/validator/duties/attester/12345",
		"/eth/v1/validator/duties/proposer/12345",
		"/eth/v1/validator/attestation_data",
		"/eth/v1/validator/aggregate_attestation",
		"/eth/v1/validator/blocks/12345",

		// Blob sidecars (post-Dencun)
		"/eth/v1/beacon/blob_sidecars/12345",

		// Rewards
		"/eth/v1/beacon/rewards/blocks/12345",
		"/eth/v1/beacon/rewards/attestations/12345",
	}

	for _, endpoint := range validEndpoints {
		if !validator.IsValidBeaconEndpoint(endpoint) {
			t.Errorf("Expected endpoint to be valid: %s", endpoint)
		}
	}
}

func TestBeaconEndpointValidator_InvalidEndpoints(t *testing.T) {
	validator := NewBeaconEndpointValidator()

	invalidEndpoints := []string{
		// Wrong version
		"/eth/v99/beacon/genesis",

		// Wrong namespace
		"/api/v1/beacon/genesis",
		"/v1/beacon/genesis",

		// Execution layer endpoints (not beacon)
		"/eth/v1/execution/blocks",
		"/web3/clientVersion",

		// Random paths
		"/admin/config",
		"/metrics",
		"/status",
		"/../etc/passwd",

		// Empty paths
		"",
		"/",

		// Malicious paths
		"/eth/v1/beacon/states/../../etc/passwd",
		"/eth/v1/beacon/blocks/<script>alert(1)</script>",

		// Non-existent beacon endpoints
		"/eth/v1/beacon/fake_endpoint",
		"/eth/v1/beacon/blocks/12345/fake_subpath",
	}

	for _, endpoint := range invalidEndpoints {
		if validator.IsValidBeaconEndpoint(endpoint) {
			t.Errorf("Expected endpoint to be invalid: %s", endpoint)
		}
	}
}

func TestBeaconEndpointValidator_EdgeCases(t *testing.T) {
	validator := NewBeaconEndpointValidator()

	testCases := []struct {
		endpoint string
		valid    bool
		name     string
	}{
		{"/eth/v1/beacon/genesis/", true, "trailing slash should be normalized and valid"},
		{" /eth/v1/beacon/genesis ", true, "whitespace should be trimmed and valid"},
		{"/eth/v1/beacon/genesis ", true, "trailing whitespace should be trimmed and valid"},
		{" /eth/v1/beacon/genesis", true, "leading whitespace should be trimmed and valid"},
		{"", false, "empty string should be invalid"},
		{"/", false, "root path should be invalid"},
		{"/invalid/path", false, "invalid path should be rejected"},
	}

	for _, tc := range testCases {
		result := validator.IsValidBeaconEndpoint(tc.endpoint)
		if result != tc.valid {
			t.Errorf("%s: endpoint %q - expected valid=%v, got %v", tc.name, tc.endpoint, tc.valid, result)
		}
	}
}

func TestBeaconEndpointValidator_NormalizationHandling(t *testing.T) {
	validator := NewBeaconEndpointValidator()

	// Test that trailing slashes are removed
	endpoint := "/eth/v1/beacon/genesis"
	endpointWithSlash := endpoint + "/"

	// The validator should normalize by removing trailing slashes
	resultWithSlash := validator.IsValidBeaconEndpoint(endpointWithSlash)
	resultWithout := validator.IsValidBeaconEndpoint(endpoint)

	if resultWithSlash != resultWithout {
		t.Errorf("Normalization failed: %s and %s should have same validity", endpoint, endpointWithSlash)
	}
}

func TestBeaconEndpointValidator_GetValidationError(t *testing.T) {
	validator := NewBeaconEndpointValidator()

	// Test valid endpoint
	validEndpoint := "/eth/v1/beacon/genesis"
	err := validator.GetValidationError(validEndpoint)
	if err != "" {
		t.Errorf("Expected no error for valid endpoint, got: %s", err)
	}

	// Test invalid endpoint
	invalidEndpoint := "/invalid/endpoint"
	err = validator.GetValidationError(invalidEndpoint)
	if err == "" {
		t.Error("Expected error for invalid endpoint, got none")
	}
	if err != "Invalid Beacon Chain API endpoint: "+invalidEndpoint {
		t.Errorf("Unexpected error message: %s", err)
	}
}

func BenchmarkIsValidBeaconEndpoint_Valid(b *testing.B) {
	validator := NewBeaconEndpointValidator()
	endpoint := "/eth/v1/beacon/states/head/validators"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.IsValidBeaconEndpoint(endpoint)
	}
}

func BenchmarkIsValidBeaconEndpoint_Invalid(b *testing.B) {
	validator := NewBeaconEndpointValidator()
	endpoint := "/invalid/endpoint/path"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.IsValidBeaconEndpoint(endpoint)
	}
}
