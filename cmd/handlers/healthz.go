package handlers

import "net/http"

// HealthzHandler responds with a simple JSON indicating the service is healthy.
// This endpoint is for Kubernetes liveness/readiness probes and always returns 200 OK.
// It bypasses the validator and all middleware.
func HealthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
