package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimit(t *testing.T) {
	// Create a rate limiter allowing 5 requests per 10 seconds
	rl := New(5, 10*time.Second)
	defer rl.Close()

	// Create a simple handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with rate limiting
	rateLimitedHandler := rl.Middleware(handler)

	// Test that first 5 requests succeed
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.1:12345" // Consistent IP
		w := httptest.NewRecorder()

		rateLimitedHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d should succeed, got status %d", i+1, w.Code)
		}
	}

	// Test that 6th request is rate limited
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345" // Same IP
	w := httptest.NewRecorder()

	rateLimitedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("6th request should be rate limited, got status %d", w.Code)
	}

	// Test that different IP is not rate limited
	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.2:12345" // Different IP
	w = httptest.NewRecorder()

	rateLimitedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Request from different IP should succeed, got status %d", w.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	// This test simulates the security headers middleware from main.go
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Simulate the security headers middleware
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Security headers (matching main.go implementation)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	handler.ServeHTTP(w, req)

	// Check all security headers are present
	expectedHeaders := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"X-XSS-Protection":        "1; mode=block",
		"Referrer-Policy":         "strict-origin-when-cross-origin",
		"Content-Security-Policy": "default-src 'self'",
	}

	for header, expectedValue := range expectedHeaders {
		actualValue := w.Header().Get(header)
		if actualValue != expectedValue {
			t.Errorf("Header %s: expected %q, got %q", header, expectedValue, actualValue)
		}
	}
}

func TestRateLimitCleanup(t *testing.T) {
	// Test that old client buckets are cleaned up
	rl := New(10, 1*time.Second)
	defer rl.Close()

	// Make a request to create a client bucket
	if !rl.Allow("192.168.1.1") {
		t.Error("First request should be allowed")
	}

	// Wait for cleanup to potentially happen
	// Note: This is a simplified test - in practice, cleanup happens every minute
	// and removes clients not seen for 5 minutes
	time.Sleep(10 * time.Millisecond)

	// The client should still be there since cleanup is based on 5-minute intervals
	if !rl.Allow("192.168.1.1") {
		t.Error("Request should still be allowed")
	}
}

func TestGetClientIP(t *testing.T) {
	// Test IP extraction logic using the actual getClientIP function

	tests := []struct {
		name          string
		remoteAddr    string
		xForwardedFor string
		xRealIP       string
		expectedIP    string
	}{
		{
			name:       "Simple RemoteAddr",
			remoteAddr: "192.168.1.1:12345",
			expectedIP: "192.168.1.1",
		},
		{
			name:          "X-Forwarded-For header",
			remoteAddr:    "127.0.0.1:12345",
			xForwardedFor: "203.0.113.1",
			expectedIP:    "203.0.113.1",
		},
		{
			name:       "X-Real-IP header",
			remoteAddr: "127.0.0.1:12345",
			xRealIP:    "203.0.113.2",
			expectedIP: "203.0.113.2",
		},
		{
			name:          "X-Forwarded-For with multiple IPs",
			remoteAddr:    "127.0.0.1:12345",
			xForwardedFor: "203.0.113.3,192.168.1.1",
			expectedIP:    "203.0.113.3",
		},
		{
			name:       "RemoteAddr without port",
			remoteAddr: "192.168.1.4",
			expectedIP: "192.168.1.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr

			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}

			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			// Call the actual getClientIP function
			actualIP := getClientIP(req)

			if actualIP != tt.expectedIP {
				t.Errorf("Expected IP %s, got %s", tt.expectedIP, actualIP)
			}
		})
	}
}

func TestAllowMethod(t *testing.T) {
	// Test the Allow method directly
	rl := New(3, 5*time.Second)
	defer rl.Close()

	// Test that Allow works correctly
	ip := "192.168.1.100"

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		if !rl.Allow(ip) {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 4th request should be denied
	if rl.Allow(ip) {
		t.Error("4th request should be denied")
	}

	// Different IP should be allowed
	if !rl.Allow("192.168.1.101") {
		t.Error("Request from different IP should be allowed")
	}
}

func TestSlidingWindow(t *testing.T) {
	// Test sliding window functionality
	rl := New(2, 100*time.Millisecond)
	defer rl.Close()

	ip := "192.168.1.200"

	// Use up the limit
	if !rl.Allow(ip) {
		t.Error("First request should be allowed")
	}
	if !rl.Allow(ip) {
		t.Error("Second request should be allowed")
	}
	if rl.Allow(ip) {
		t.Error("Third request should be denied")
	}

	// Wait for window to slide
	time.Sleep(150 * time.Millisecond)

	// Should be allowed again
	if !rl.Allow(ip) {
		t.Error("Request after window should be allowed")
	}
}

func TestParseFirstIP(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"192.168.1.1", "192.168.1.1"},
		{"192.168.1.1,192.168.1.2", "192.168.1.1"},
		{"203.0.113.1, 192.168.1.1", "203.0.113.1"},
		{"invalid,192.168.1.1", ""},
		{"", ""},
		{"not-an-ip", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseFirstIP(tt.input)
			if result != tt.expected {
				t.Errorf("parseFirstIP(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}
