package tests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zircuit-labs/consensus-proxy/cmd/logger"

	"github.com/zircuit-labs/consensus-proxy/cmd/loadbalancer"

	"github.com/zircuit-labs/consensus-proxy/cmd/config"
)

type StressTestResults struct {
	TotalRequests     int64
	SuccessfulReqs    int64
	FailedReqs        int64
	TotalDuration     time.Duration
	MinDuration       time.Duration
	MaxDuration       time.Duration
	AvgDuration       time.Duration
	P50Duration       time.Duration
	P95Duration       time.Duration
	P99Duration       time.Duration
	RequestsPerSecond float64
	ErrorBreakdown    map[string]int64
	TestMode          string
}

func TestStressSuite(t *testing.T) {
	testMode := getTestMode()

	fmt.Println("\n🚀 Beacon Proxy Stress Test Suite")
	fmt.Println("==================================")
	if testMode == "real" {
		fmt.Println("Mode: REAL - Testing against actual beacon nodes")
	} else {
		fmt.Println("Mode: MOCK - Testing with local mock servers")
	}
	fmt.Println("Tip: Set CONSENSUS_PROXY_TEST_MODE=real to test with real nodes")
	fmt.Println()

	// Run different stress test scenarios
	scenarios := []struct {
		name        string
		concurrency int
		duration    time.Duration
		reqTimeout  time.Duration
	}{
		{"Low Load", 5, 5 * time.Second, 1500 * time.Millisecond},
		{"Medium Load", 10, 5 * time.Second, 1500 * time.Millisecond},
		{"High Load", 15, 5 * time.Second, 1500 * time.Millisecond},
	}

	allPassed := true
	for _, scenario := range scenarios {
		fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("📊 %s Test\n", scenario.name)
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("   Concurrency: %d workers | Duration: %v | Request Timeout: %v\n",
			scenario.concurrency, scenario.duration, scenario.reqTimeout)

		results := runStressTest(scenario.concurrency, scenario.duration, scenario.reqTimeout)
		printResults(results)

		// Validate performance requirements
		passed := validatePerformance(scenario.name, results)
		if !passed {
			allPassed = false
		}

		if scenario.name != "High Load" {
			fmt.Println("\n   ⏸  Pausing 2s before next scenario...")
			time.Sleep(2 * time.Second)
		}
	}

	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	if allPassed {
		fmt.Println("✅ All stress tests completed successfully!")
	} else {
		fmt.Println("⚠️  Some tests had performance warnings (see above)")
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

func runStressTest(concurrency int, duration time.Duration, reqTimeout time.Duration) *StressTestResults {
	var servers []*httptest.Server
	var cleanup func()

	testMode := getTestMode()

	// Only create mock servers if in mock mode
	if testMode == "mock" {
		// Create mock beacon servers with different response characteristics
		servers = createMockServers()
		defer func() {
			for _, server := range servers {
				server.Close()
			}
		}()
	}

	// Create load balancer (with mock servers in mock mode, real nodes in real mode)
	lb, cleanup := createLoadBalancer(servers, reqTimeout, testMode)
	defer cleanup()

	// Track results
	var totalReqs, successReqs, failedReqs int64
	durations := make([]time.Duration, 0, 100000)
	errorBreakdown := make(map[string]int64)
	var durationMu sync.Mutex
	var errorMu sync.Mutex

	// Determine client timeout based on test mode
	clientTimeout := reqTimeout * 2
	if testMode == "real" {
		// Real nodes need much longer timeout due to network latency
		clientTimeout = 35 * time.Second
	}

	// Shared HTTP client to avoid port exhaustion
	sharedClient := &http.Client{
		Timeout: clientTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        500,              // Increased pool
			MaxIdleConnsPerHost: 250,              // Increased per-host pool
			IdleConnTimeout:     90 * time.Second, // Keep connections longer
			DisableKeepAlives:   false,            // Enable connection reuse
			MaxConnsPerHost:     250,              // Limit concurrent connections
			DisableCompression:  true,             // Reduce CPU overhead
		},
	}

	// Start time
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	// Progress ticker
	progressTicker := time.NewTicker(1 * time.Second)
	defer progressTicker.Stop()
	go func() {
		for range progressTicker.C {
			current := atomic.LoadInt64(&totalReqs)
			success := atomic.LoadInt64(&successReqs)
			elapsed := time.Since(startTime)
			rps := float64(current) / elapsed.Seconds()
			successRate := float64(success) / float64(current) * 100
			if current > 0 {
				fmt.Printf("\r   ⏱️  Progress: %d reqs | %.1f req/s | %.1f%% success | Elapsed: %.1fs   ",
					current, rps, successRate, elapsed.Seconds())
			}
		}
	}()

	// Worker pool
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			client := sharedClient

			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Make request
					reqStart := time.Now()
					resp, err := client.Get("http://" + lb.addr + "/eth/v1/beacon/headers/head")
					reqDuration := time.Since(reqStart)

					atomic.AddInt64(&totalReqs, 1)

					if err != nil || resp == nil || resp.StatusCode != 200 {
						atomic.AddInt64(&failedReqs, 1)

						// Track error type
						errorMu.Lock()
						if err != nil {
							errorBreakdown["error: "+err.Error()]++
						} else if resp != nil {
							errorBreakdown[fmt.Sprintf("status_%d", resp.StatusCode)]++
						} else {
							errorBreakdown["nil_response"]++
						}
						errorMu.Unlock()

						if resp != nil {
							resp.Body.Close()
						}
					} else {
						atomic.AddInt64(&successReqs, 1)
						resp.Body.Close()

						// Record all successful request timings
						durationMu.Lock()
						durations = append(durations, reqDuration)
						durationMu.Unlock()
					}

					// Small delay to prevent port exhaustion
					time.Sleep(time.Millisecond * 1)
				}
			}
		}(i)
	}

	// Wait for completion
	wg.Wait()
	totalDuration := time.Since(startTime)

	// Stop progress ticker and print newline
	progressTicker.Stop()
	fmt.Println() // Move to next line after progress output

	// Calculate statistics
	return calculateResults(totalReqs, successReqs, failedReqs, totalDuration, durations, errorBreakdown, testMode)
}

func createMockServers() []*httptest.Server {
	// Primary server - fast responses
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Syncing status endpoint (for startup health check)
		if r.URL.Path == "/eth/v1/node/syncing" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": {"is_syncing": false, "sync_distance": "0"}}`))
			return
		}

		// Health check endpoint
		if r.URL.Path == "/eth/v1/node/health" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": {"is_healthy": true, "is_optimistic": false, "is_el_offline": false}}`))
			return
		}

		// Simulate very fast beacon API response (0.1-1ms) for stress testing
		delay := time.Microsecond * time.Duration(100+(time.Now().UnixNano()%900))
		time.Sleep(delay)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Simulate different beacon API endpoints
		switch r.URL.Path {
		case "/eth/v1/beacon/headers/head":
			w.Write([]byte(`{
				"execution_optimistic": false,
				"finalized": false,
				"data": {
					"root": "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
					"canonical": true,
					"header": {
						"message": {
							"slot": "12345",
							"proposer_index": "123",
							"parent_root": "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
							"state_root": "0x9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba",
							"body_root": "0xdeadbeefcafebabedeadbeefcafebabedeadbeefcafebabedeadbeefcafebabe"
						},
						"signature": "0x123456789abcdef"
					}
				}
			}`))
		case "/eth/v1/beacon/genesis":
			w.Write([]byte(`{
				"data": {
					"genesis_time": "1606824023",
					"genesis_validators_root": "0x4b363db94e286120d76eb905340fdd4e54bfe9f06bf33ff6cf5ad27f511bfe95",
					"genesis_fork_version": "0x00000000"
				}
			}`))
		default:
			w.Write([]byte(`{"data": {"message": "mock beacon node response"}}`))
		}
	}))

	// Backup server - medium responses
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Syncing status endpoint (for startup health check)
		if r.URL.Path == "/eth/v1/node/syncing" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": {"is_syncing": false, "sync_distance": "0"}}`))
			return
		}

		// Health check endpoint
		if r.URL.Path == "/eth/v1/node/health" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": {"is_healthy": true, "is_optimistic": false, "is_el_offline": false}}`))
			return
		}

		// Simulate fast beacon API response (0.5-2ms) for stress testing
		delay := time.Microsecond * time.Duration(500+(time.Now().UnixNano()%1500))
		time.Sleep(delay)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"execution_optimistic": false,
			"finalized": false,
			"data": {
				"root": "0xbackup123456789abcdefbackup123456789abcdefbackup123456789abcdef",
				"canonical": true,
				"header": {
					"message": {
						"slot": "12346",
						"proposer_index": "124"
					}
				}
			}
		}`))
	}))

	return []*httptest.Server{primary, backup}
}

type testServer struct {
	lb   http.Handler
	addr string
}

func (ts *testServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ts.lb.ServeHTTP(w, r)
}

func createLoadBalancer(servers []*httptest.Server, reqTimeout time.Duration, testMode string) (*testServer, func()) {
	// Load the standard config.toml and modify for testing
	cfg := config.LoadOrDefault("../config.toml")

	// Override settings for stress testing
	cfg.Server.MaxRetries = 2                  // Reduce retries to speed up failures
	cfg.Server.RequestTimeout = reqTimeout * 3 // Give more time for load balancer processing
	cfg.Metrics.Enabled = false                // Disable metrics for stress test
	cfg.Logger.Level = "error"                 // Reduce logging noise during stress test

	// Reinitialize logger with error level to reduce noise
	logger.Init(&logger.Config{
		Level:  logger.LogLevel(cfg.Logger.Level),
		Format: cfg.Logger.Format,
		Output: cfg.Logger.Output,
	})

	if testMode == "real" {
		// Use real beacon nodes from config.toml - adjust timeout for real nodes
		cfg.Server.RequestTimeout = 30 * time.Second // More generous timeout for real nodes
		fmt.Printf("   🌐 Using real beacon nodes from config.toml (%d nodes [%v])\n", len(cfg.GetAllNodes()), cfg.GetAllNodes())
		// Keep the nodes from config.toml - don't modify cfg.Beacons
	} else {
		// Mock mode - replace with test servers
		var beaconNames []string
		var beaconConfigs []config.NodeConfig
		for i, server := range servers {
			name := fmt.Sprintf("server-%d", i+1)
			beaconNames = append(beaconNames, name)
			beaconConfigs = append(beaconConfigs, config.NodeConfig{
				Name: name,
				URL:  server.URL,
				Type: "lighthouse",
			})
		}
		cfg.Beacons.Nodes = beaconNames
		cfg.Beacons.SetParsedNodes(beaconConfigs)
		fmt.Printf("   🧪 Using mock servers (%d servers)\n", len(servers))
	}

	lb, err := loadbalancer.New(cfg)
	if err != nil {
		panic(fmt.Sprintf("Failed to create load balancer: %v", err))
	}

	// Perform startup health check to populate healthy nodes
	if err := lb.StartupHealthCheck(); err != nil {
		// In mock mode, this should never fail
		// In real mode, we want to know if nodes are unhealthy
		if testMode == "real" {
			fmt.Printf("   ⚠️  Warning: Startup health check failed: %v\n", err)
			fmt.Printf("   ⚠️  No healthy nodes available - test may fail\n")
			os.Exit(1)
		} else {
			panic(fmt.Sprintf("Startup health check failed: %v", err))
		}
	}

	// Start test server
	httpServer := httptest.NewServer(lb)

	cleanup := func() {
		httpServer.Close()
	}

	return &testServer{
		lb:   lb,
		addr: httpServer.Listener.Addr().String(),
	}, cleanup
}

func calculateResults(totalReqs, successReqs, failedReqs int64, totalDuration time.Duration, durations []time.Duration, errorBreakdown map[string]int64, testMode string) *StressTestResults {
	if len(durations) == 0 {
		return &StressTestResults{
			TotalRequests:     totalReqs,
			SuccessfulReqs:    successReqs,
			FailedReqs:        failedReqs,
			TotalDuration:     totalDuration,
			RequestsPerSecond: float64(totalReqs) / totalDuration.Seconds(),
			ErrorBreakdown:    errorBreakdown,
			TestMode:          testMode,
		}
	}

	// Sort durations for percentile calculation
	for i := 0; i < len(durations); i++ {
		for j := i + 1; j < len(durations); j++ {
			if durations[i] > durations[j] {
				durations[i], durations[j] = durations[j], durations[i]
			}
		}
	}

	// Calculate statistics
	var totalDur time.Duration
	minDur := durations[0]
	maxDur := durations[len(durations)-1]

	for _, d := range durations {
		totalDur += d
		if d < minDur {
			minDur = d
		}
		if d > maxDur {
			maxDur = d
		}
	}

	avgDur := totalDur / time.Duration(len(durations))
	p50Dur := durations[int(float64(len(durations))*0.50)]
	p95Dur := durations[int(float64(len(durations))*0.95)]
	p99Dur := durations[int(float64(len(durations))*0.99)]

	return &StressTestResults{
		TotalRequests:     totalReqs,
		SuccessfulReqs:    successReqs,
		FailedReqs:        failedReqs,
		TotalDuration:     totalDuration,
		MinDuration:       minDur,
		MaxDuration:       maxDur,
		AvgDuration:       avgDur,
		P50Duration:       p50Dur,
		P95Duration:       p95Dur,
		P99Duration:       p99Dur,
		RequestsPerSecond: float64(totalReqs) / totalDuration.Seconds(),
		ErrorBreakdown:    errorBreakdown,
		TestMode:          testMode,
	}
}

func printResults(results *StressTestResults) {
	fmt.Printf("\n   📈 Results:\n")
	fmt.Printf("     Mode: %s\n", results.TestMode)
	fmt.Printf("     Total Duration: %v\n", results.TotalDuration)
	fmt.Printf("     Total Requests: %d\n", results.TotalRequests)
	fmt.Printf("     Successful: %d (%.1f%%)\n", results.SuccessfulReqs,
		float64(results.SuccessfulReqs)/float64(results.TotalRequests)*100)
	fmt.Printf("     Failed: %d (%.1f%%)\n", results.FailedReqs,
		float64(results.FailedReqs)/float64(results.TotalRequests)*100)
	fmt.Printf("     Throughput: %.1f req/s\n", results.RequestsPerSecond)

	if len(results.ErrorBreakdown) > 0 {
		fmt.Printf("\n     ⚠️  Error Breakdown:\n")
		for errType, count := range results.ErrorBreakdown {
			fmt.Printf("       • %s: %d (%.1f%%)\n", errType, count,
				float64(count)/float64(results.TotalRequests)*100)
		}
	}

	if results.MinDuration > 0 {
		fmt.Printf("\n     ⏱️  Response Times (successful requests only):\n")
		fmt.Printf("       Min: %v\n", results.MinDuration)
		fmt.Printf("       P50 (median): %v\n", results.P50Duration)
		fmt.Printf("       Avg: %v\n", results.AvgDuration)
		fmt.Printf("       P95: %v\n", results.P95Duration)
		fmt.Printf("       P99: %v\n", results.P99Duration)
		fmt.Printf("       Max: %v\n", results.MaxDuration)
	}
}

func validatePerformance(scenarioName string, results *StressTestResults) bool {
	fmt.Printf("\n   🔍 Performance Validation:\n")

	allPassed := true

	// Success rate validation
	successRate := float64(results.SuccessfulReqs) / float64(results.TotalRequests) * 100
	targetSuccessRate := 95.0
	if successRate < targetSuccessRate {
		fmt.Printf("     ❌ Success rate: %.1f%% (target: >%.0f%%)\n", successRate, targetSuccessRate)
		allPassed = false
	} else {
		fmt.Printf("     ✅ Success rate: %.1f%%\n", successRate)
	}

	// Response time validation - different targets for mock vs real
	var avgTarget, p95Target, p99Target time.Duration
	if results.TestMode == "mock" {
		avgTarget = 100 * time.Millisecond
		p95Target = 200 * time.Millisecond
		p99Target = 500 * time.Millisecond
	} else {
		// Real nodes - more lenient
		avgTarget = 500 * time.Millisecond
		p95Target = 1000 * time.Millisecond
		p99Target = 2000 * time.Millisecond
	}

	if results.AvgDuration > 0 {
		if results.AvgDuration > avgTarget {
			fmt.Printf("     ⚠️  Avg response: %v (target: <%v for %s mode)\n", results.AvgDuration, avgTarget, results.TestMode)
			allPassed = false
		} else {
			fmt.Printf("     ✅ Avg response: %v\n", results.AvgDuration)
		}

		if results.P95Duration > p95Target {
			fmt.Printf("     ⚠️  P95 response: %v (target: <%v for %s mode)\n", results.P95Duration, p95Target, results.TestMode)
			allPassed = false
		} else {
			fmt.Printf("     ✅ P95 response: %v\n", results.P95Duration)
		}

		if results.P99Duration > p99Target {
			fmt.Printf("     ⚠️  P99 response: %v (target: <%v for %s mode)\n", results.P99Duration, p99Target, results.TestMode)
			allPassed = false
		} else {
			fmt.Printf("     ✅ P99 response: %v\n", results.P99Duration)
		}
	}

	// Throughput validation
	minRPS := getMinRPSForScenario(scenarioName, results.TestMode)
	if results.RequestsPerSecond < minRPS {
		fmt.Printf("     ⚠️  Throughput: %.1f req/s (target: >%.0f for %s mode)\n", results.RequestsPerSecond, minRPS, results.TestMode)
		allPassed = false
	} else {
		fmt.Printf("     ✅ Throughput: %.1f req/s\n", results.RequestsPerSecond)
	}

	return allPassed
}

func getMinRPSForScenario(scenarioName string, testMode string) float64 {
	if testMode == "mock" {
		switch scenarioName {
		case "Low Load":
			return 200.0 // 5 concurrent should achieve ~200 RPS with mock
		case "Medium Load":
			return 400.0 // 10 concurrent should achieve ~400 RPS with mock
		case "High Load":
			return 600.0 // 15 concurrent should achieve ~600 RPS with mock
		default:
			return 100.0
		}
	} else {
		// Real nodes - lower expectations due to network latency
		switch scenarioName {
		case "Low Load":
			return 50.0 // Real nodes are slower
		case "Medium Load":
			return 100.0
		case "High Load":
			return 150.0
		default:
			return 25.0
		}
	}
}
