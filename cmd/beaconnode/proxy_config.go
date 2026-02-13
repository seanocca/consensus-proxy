package beaconnode

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/zircuit-labs/consensus-proxy/cmd/config"
)

// createProxyDirector creates a custom Director function for the reverse proxy
// that properly handles headers for API providers like Chainstack
func createProxyDirector(originalDirector func(*http.Request), targetURL *url.URL, userAgent, nodeURL string) func(*http.Request) {
	return func(req *http.Request) {
		originalDirector(req)

		// Preserve original User-Agent or set a configurable one if missing
		if req.Header.Get("User-Agent") == "" {
			req.Header.Set("User-Agent", userAgent)
		}

		// Ensure Host header is set correctly for API providers
		req.Host = targetURL.Host

		// Add headers that some API providers expect
		req.Header.Set("Accept", "application/json")
		if req.Header.Get("Content-Type") == "" && (req.Method == "POST" || req.Method == "PUT") {
			req.Header.Set("Content-Type", "application/json")
		}

		// For HTTPS endpoints, ensure proper protocol handling
		if targetURL.Scheme == "https" {
			req.Header.Set("X-Forwarded-Proto", "https")
		}

		// Remove problematic headers that some API providers reject
		req.Header.Del("X-Forwarded-For")
		req.Header.Del("X-Real-IP")

		// Ensure clean referer for API calls
		req.Header.Del("Referer")

		// Some API providers are strict about connection headers
		if req.Header.Get("Connection") == "close" {
			req.Header.Del("Connection")
		}
	}
}

// createProxyTransport creates an optimized HTTP transport for the reverse proxy
func createProxyTransport(cfg *config.Config, nodeURL string) *http.Transport {
	return &http.Transport{
		// Connection pooling optimized for speed
		MaxIdleConns:        cfg.Proxy.MaxIdleConns,
		IdleConnTimeout:     cfg.Proxy.IdleConnTimeout,
		MaxIdleConnsPerHost: cfg.Proxy.MaxIdleConnsPerHost,
		MaxConnsPerHost:     cfg.Proxy.MaxConnsPerHost,

		// Speed optimizations
		DisableCompression: true,                                         // Avoid compression overhead
		DisableKeepAlives:  false,                                        // Keep-alives essential for performance
		ForceAttemptHTTP2:  !strings.Contains(nodeURL, "chainstack.com"), // Some API providers have HTTP/2 issues

		// Configurable timeouts for performance tuning
		ResponseHeaderTimeout: cfg.Proxy.ResponseHeaderTimeout,
		TLSHandshakeTimeout:   cfg.Proxy.TLSHandshakeTimeout,
		ExpectContinueTimeout: cfg.Proxy.ExpectContinueTimeout,

		// DNS caching and fast connection establishment
		DialContext: cachedResolver(cfg),
	}
}
