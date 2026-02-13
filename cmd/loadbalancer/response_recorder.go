package loadbalancer

import "net/http"

// responseRecorder captures the status code and response body for our failover logic
type responseRecorder struct {
	statusCode int
	header     http.Header
	body       []byte
}

// Header implements http.ResponseWriter
func (rr *responseRecorder) Header() http.Header {
	if rr.header == nil {
		rr.header = make(http.Header)
	}
	return rr.header
}

// WriteHeader implements http.ResponseWriter
func (rr *responseRecorder) WriteHeader(code int) {
	if rr.statusCode == 0 {
		rr.statusCode = code
	}
}

// Write implements http.ResponseWriter
func (rr *responseRecorder) Write(data []byte) (int, error) {
	if rr.statusCode == 0 {
		rr.statusCode = 200
	}
	rr.body = append(rr.body, data...)
	return len(data), nil
}

// copyToResponseWriter copies the recorded response to the actual ResponseWriter
func (rr *responseRecorder) copyToResponseWriter(w http.ResponseWriter) {
	// Copy headers
	for k, v := range rr.header {
		w.Header()[k] = v
	}
	// Write status code
	if rr.statusCode != 0 {
		w.WriteHeader(rr.statusCode)
	}
	// Write body
	w.Write(rr.body)
}
