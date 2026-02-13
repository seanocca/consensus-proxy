package beaconnode

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/zircuit-labs/consensus-proxy/cmd/config"
)

// DNS cache for faster resolution
var (
	dnsCache = make(map[string][]net.IP)
	dnsMutex sync.RWMutex
)

// cachedResolver implements DNS caching for faster lookups
func cachedResolver(cfg *config.Config) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}

		// Check cache first
		dnsMutex.RLock()
		cachedIPs, exists := dnsCache[host]
		dnsMutex.RUnlock()

		var ips []net.IP
		if exists && len(cachedIPs) > 0 {
			ips = cachedIPs
		} else {
			// Resolve DNS and cache result
			resolvedIPs, err := net.LookupIP(host)
			if err != nil {
				return nil, err
			}

			dnsMutex.Lock()
			dnsCache[host] = resolvedIPs
			dnsMutex.Unlock()

			// Schedule cache expiry
			go func() {
				time.Sleep(cfg.DNS.CacheTTL)
				dnsMutex.Lock()
				delete(dnsCache, host)
				dnsMutex.Unlock()
			}()

			ips = resolvedIPs
		}

		// Try connecting to cached IPs
		for _, ip := range ips {
			addr := net.JoinHostPort(ip.String(), port)
			conn, err := net.DialTimeout(network, addr, cfg.DNS.ConnectionTimeout)
			if err == nil {
				return conn, nil
			}
		}

		return nil, fmt.Errorf("failed to connect to any cached IP for %s", host)
	}
}
