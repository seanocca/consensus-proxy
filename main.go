package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/zircuit-labs/consensus-proxy/cmd/config"
	"github.com/zircuit-labs/consensus-proxy/cmd/handlers"
	"github.com/zircuit-labs/consensus-proxy/cmd/loadbalancer"
	"github.com/zircuit-labs/consensus-proxy/cmd/logger"
	"github.com/zircuit-labs/consensus-proxy/cmd/ratelimit"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	var cfg *config.Config
	var err error
	var log *logger.Logger

	// Initialize a basic logger for early startup logging
	log = logger.NewFromConfigStruct("info", "json", "stdout")

	// Default configuration path
	configPath := "config.toml"

	// Check for config file location from environment variable
	if envConfigPath := os.Getenv("CONSENSUS_PROXY_CONFIG"); envConfigPath != "" {
		configPath = envConfigPath
	}

	// Parse command line arguments
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--config", "-c":
			if len(os.Args) > 2 {
				configPath = os.Args[2]
			}
		case "--help", "-h":
			fmt.Println("Usage:")
			fmt.Println("  consensus-proxy [options]")
			fmt.Println()
			fmt.Println("Options:")
			fmt.Println("  --config, -c <toml_file>  Load TOML config file (default: config.toml)")
			fmt.Println("  --help, -h                Show this help")
			fmt.Println()
			fmt.Println("Environment Variables:")
			fmt.Println("  CONSENSUS_PROXY_CONFIG       Path to config.toml file (default: config.toml)")
			fmt.Println()
			fmt.Println("Default behavior: Load config.toml from current directory")
			return
		default:
			// Assume it's a config file path for backward compatibility
			configPath = os.Args[1]
		}
	}

	// Load TOML configuration
	if cfg == nil {
		cfg = config.LoadOrDefault(configPath)
		if err != nil {
			log.LogError("config loading", err, "source", "toml", "path", configPath)
			log.Info("using default configuration")
			cfg = config.LoadOrDefault("")
		}
		// Log configuration source
		configSource := configPath
		if os.Getenv("CONSENSUS_PROXY_CONFIG") != "" {
			configSource += " (via CONSENSUS_PROXY_CONFIG)"
		}
		log.LogConfig(configSource, false, len(cfg.GetAllNodes()))
	}

	// Reinitialize logger with configuration settings
	log = logger.NewFromConfigStruct(cfg.Logger.Level, cfg.Logger.Format, cfg.Logger.Output)
	loggerConfig := &logger.Config{
		Level:  logger.LogLevel(cfg.Logger.Level),
		Format: cfg.Logger.Format,
		Output: cfg.Logger.Output,
	}
	logger.Init(loggerConfig)

	// Create load balancer
	lb, err := loadbalancer.New(cfg)
	if err != nil {
		log.LogError("loadbalancer creation", err)
		os.Exit(1)
	}

	// Perform startup health check on all nodes
	if err := lb.StartupHealthCheck(); err != nil {
		log.LogError("startup health check", err)
		os.Exit(1)
	}

	// Start periodic health check routine
	lb.StartPeriodicHealthCheck()

	// Create rate limiter if enabled
	var rateLimiter *ratelimit.RateLimiter
	if cfg.RateLimit.Enabled {
		rateLimiter = ratelimit.New(cfg.RateLimit.RequestsPerSecond, cfg.RateLimit.Window)
		defer rateLimiter.Close()
		log.Info("rate limiting enabled",
			"requests_per_second", cfg.RateLimit.RequestsPerSecond,
			"window", cfg.RateLimit.Window.String())
	}

	// Setup routes
	setupRoutes(lb, rateLimiter)

	// Get all configured nodes (beacons)
	allNodes := cfg.GetAllNodes()

	// Log startup information using structured logging
	log.LogStartup(cfg.Server.Port, configPath, len(allNodes))

	// Log node configuration with priority information
	for i, node := range allNodes {
		priority := "primary"
		if i > 0 {
			priority = fmt.Sprintf("backup-%d", i)
		}

		logFields := []any{
			"name", node.Name,
			"priority", priority,
		}

		// Add beacon type if specified
		if node.Type != "" {
			logFields = append(logFields, "type", node.Type)
		}

		log.Info("beacon node configured", logFields...)
	}

	server := &http.Server{
		Addr:              cfg.GetListenAddr(),
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
	}

	// Start HTTP server
	log.Info("starting HTTP server", "port", cfg.Server.Port)

	if err := server.ListenAndServe(); err != nil {
		log.LogError("HTTP server startup", err, "port", cfg.Server.Port)
		os.Exit(1)
	}
}

func setupRoutes(lb *loadbalancer.LoadBalancer, rateLimiter *ratelimit.RateLimiter) {

	// Health endpoint for the proxy itself
	http.HandleFunc("/healthz", handlers.HealthzHandler)

	// Prometheus metrics endpoint
	http.Handle("/metrics", promhttp.Handler())

	// Create middleware chain with rate limiting and CORS
	var handler http.Handler = lb
	handler = handlers.NewCORSHandler(handler)

	// Add rate limiting if enabled
	if rateLimiter != nil {
		handler = rateLimiter.Middleware(handler)
	}

	// Route all other requests through the middleware chain
	http.Handle("/", handler)
}
