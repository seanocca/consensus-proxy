package metrics

import (
	"time"

	"github.com/zircuit-labs/consensus-proxy/cmd/config"
	"github.com/zircuit-labs/consensus-proxy/cmd/logger"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Client interface for metrics collection
type Client interface {
	Incr(name string, tags []string, rate float64) error
	Timing(name string, value time.Duration, tags []string, rate float64) error
	Gauge(name string, value float64, tags []string, rate float64) error
	Close() error
}

// PrometheusClient wraps Prometheus metrics
type PrometheusClient struct {
	namespace string
	counters  map[string]*prometheus.CounterVec
	gauges    map[string]*prometheus.GaugeVec
	summaries map[string]*prometheus.SummaryVec
}

// NoOpClient is a no-op implementation of the Client interface
type NoOpClient struct{}

func (c *NoOpClient) Incr(name string, tags []string, rate float64) error { return nil }
func (c *NoOpClient) Timing(name string, value time.Duration, tags []string, rate float64) error {
	return nil
}
func (c *NoOpClient) Gauge(name string, value float64, tags []string, rate float64) error {
	return nil
}
func (c *NoOpClient) Close() error { return nil }

// NewClient creates a new metrics client based on configuration
func NewClient(cfg *config.MetricsConfig) (Client, error) {
	if !cfg.Enabled {
		logger.Info("metrics collection disabled")
		return &NoOpClient{}, nil
	}

	logger.Info("metrics collection enabled (Prometheus)", "namespace", cfg.Namespace)

	return &PrometheusClient{
		namespace: cfg.Namespace,
		counters:  make(map[string]*prometheus.CounterVec),
		gauges:    make(map[string]*prometheus.GaugeVec),
		summaries: make(map[string]*prometheus.SummaryVec),
	}, nil
}

// parseTags converts tag array ["key:value", "key2:value2"] to label names and values
func parseTags(tags []string) ([]string, []string) {
	if len(tags) == 0 {
		return []string{}, []string{}
	}

	labelNames := make([]string, 0, len(tags))
	labelValues := make([]string, 0, len(tags))

	for _, tag := range tags {
		// Parse "key:value" format
		for i := 0; i < len(tag); i++ {
			if tag[i] == ':' {
				labelNames = append(labelNames, tag[:i])
				labelValues = append(labelValues, tag[i+1:])
				break
			}
		}
	}

	return labelNames, labelValues
}

func (c *PrometheusClient) Incr(name string, tags []string, rate float64) error {
	counter, ok := c.counters[name]
	if !ok {
		labelNames, _ := parseTags(tags)
		counter = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: c.namespace,
				Name:      name,
				Help:      name,
			},
			labelNames,
		)
		c.counters[name] = counter
	}

	_, labelValues := parseTags(tags)
	counter.WithLabelValues(labelValues...).Add(rate)
	return nil
}

func (c *PrometheusClient) Timing(name string, value time.Duration, tags []string, rate float64) error {
	summary, ok := c.summaries[name]
	if !ok {
		labelNames, _ := parseTags(tags)
		summary = promauto.NewSummaryVec(
			prometheus.SummaryOpts{
				Namespace:  c.namespace,
				Name:       name,
				Help:       name,
				Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
			},
			labelNames,
		)
		c.summaries[name] = summary
	}

	_, labelValues := parseTags(tags)
	summary.WithLabelValues(labelValues...).Observe(value.Seconds())
	return nil
}

func (c *PrometheusClient) Gauge(name string, value float64, tags []string, rate float64) error {
	gauge, ok := c.gauges[name]
	if !ok {
		labelNames, _ := parseTags(tags)
		gauge = promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: c.namespace,
				Name:      name,
				Help:      name,
			},
			labelNames,
		)
		c.gauges[name] = gauge
	}

	_, labelValues := parseTags(tags)
	gauge.WithLabelValues(labelValues...).Set(value)
	return nil
}

func (c *PrometheusClient) Close() error {
	return nil
}
