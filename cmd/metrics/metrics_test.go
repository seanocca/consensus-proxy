package metrics

import (
	"testing"
	"time"

	"github.com/zircuit-labs/consensus-proxy/cmd/config"
)

func ExampleNewClient() {
	// Example with metrics disabled
	cfg := &config.MetricsConfig{
		Enabled: false,
	}

	client, err := NewClient(cfg)
	if err != nil {
		panic(err)
	}

	// This will do nothing since metrics are disabled
	client.Incr("test.counter", []string{"tag:value"}, 1.0)
	client.Timing("test.timing", 100*time.Millisecond, []string{"tag:value"}, 1.0)
	client.Gauge("test.gauge", 42.0, []string{"tag:value"}, 1.0)

	client.Close()
}

func TestNoOpClient(t *testing.T) {
	cfg := &config.MetricsConfig{
		Enabled: false,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create NoOp client: %v", err)
	}

	// These should all succeed without error
	if err := client.Incr("test.counter", nil, 1.0); err != nil {
		t.Errorf("NoOp Incr failed: %v", err)
	}

	if err := client.Timing("test.timing", time.Second, nil, 1.0); err != nil {
		t.Errorf("NoOp Timing failed: %v", err)
	}

	if err := client.Gauge("test.gauge", 100.0, nil, 1.0); err != nil {
		t.Errorf("NoOp Gauge failed: %v", err)
	}

	if err := client.Close(); err != nil {
		t.Errorf("NoOp Close failed: %v", err)
	}
}
