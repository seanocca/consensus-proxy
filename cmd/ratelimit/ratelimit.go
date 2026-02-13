package ratelimit

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// RateLimiter implements a simple sliding window rate limiter
type RateLimiter struct {
	mu            sync.RWMutex
	clients       map[string]*clientBucket
	rps           int
	window        time.Duration
	cleanupTicker *time.Ticker
}

type clientBucket struct {
	requests []time.Time
	lastSeen time.Time
}

// New creates a new rate limiter
func New(requestsPerSecond int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		clients:       make(map[string]*clientBucket),
		rps:           requestsPerSecond,
		window:        window,
		cleanupTicker: time.NewTicker(time.Minute),
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a request from the given IP should be allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Get or create client bucket
	bucket, exists := rl.clients[ip]
	if !exists {
		bucket = &clientBucket{
			requests: make([]time.Time, 0),
			lastSeen: now,
		}
		rl.clients[ip] = bucket
	}

	bucket.lastSeen = now

	// Remove expired requests
	cutoff := now.Add(-rl.window)
	validRequests := bucket.requests[:0] // Reuse slice
	for _, reqTime := range bucket.requests {
		if reqTime.After(cutoff) {
			validRequests = append(validRequests, reqTime)
		}
	}
	bucket.requests = validRequests

	// Check if we're under the limit
	if len(bucket.requests) >= rl.rps {
		return false
	}

	// Add this request
	bucket.requests = append(bucket.requests, now)
	return true
}

// cleanup removes old client buckets to prevent memory leaks
func (rl *RateLimiter) cleanup() {
	for range rl.cleanupTicker.C {
		rl.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-5 * time.Minute) // Remove clients not seen for 5 minutes

		for ip, bucket := range rl.clients {
			if bucket.lastSeen.Before(cutoff) {
				delete(rl.clients, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Close stops the cleanup ticker
func (rl *RateLimiter) Close() {
	if rl.cleanupTicker != nil {
		rl.cleanupTicker.Stop()
	}
}

// Middleware returns an HTTP middleware that applies rate limiting
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract client IP
		ip := getClientIP(r)

		if !rl.Allow(ip) {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getClientIP extracts the real client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (most common)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the first IP if there are multiple
		if ip := parseFirstIP(xff); ip != "" {
			return ip
		}
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		if ip := net.ParseIP(xri); ip != nil {
			return xri
		}
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// parseFirstIP extracts the first valid IP from a comma-separated list
func parseFirstIP(ips string) string {
	for i, char := range ips {
		if char == ',' {
			ip := ips[:i]
			if net.ParseIP(ip) != nil {
				return ip
			}
			break
		}
	}

	// No comma found, check if the whole string is an IP
	if net.ParseIP(ips) != nil {
		return ips
	}

	return ""
}
