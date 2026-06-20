package component

import (
	"context"
	"testing"
	"time"

	flowctlpb "github.com/withobsrvr/flowctl/proto"
)

func TestConfigFromEnvDefaultsWhenDisabled(t *testing.T) {
	t.Setenv("ENABLE_FLOWCTL", "")
	t.Setenv("FLOWCTL_ENDPOINT", "")
	t.Setenv("FLOWCTL_COMPONENT_ID", "")
	t.Setenv("FLOWCTL_RUN_ID", "")
	t.Setenv("FLOWCTL_ATTEMPT", "")
	t.Setenv("FLOWCTL_HEARTBEAT_INTERVAL_MS", "")

	cfg := ConfigFromEnv()
	if cfg.Enabled {
		t.Fatal("expected flowctl to be disabled")
	}
	if cfg.Attempt != 1 {
		t.Fatalf("expected default attempt 1, got %d", cfg.Attempt)
	}
	if cfg.HeartbeatInterval != defaultHeartbeatInterval {
		t.Fatalf("expected default heartbeat interval %s, got %s", defaultHeartbeatInterval, cfg.HeartbeatInterval)
	}
}

func TestConfigFromEnvParsesFlowctlContract(t *testing.T) {
	t.Setenv("ENABLE_FLOWCTL", "true")
	t.Setenv("FLOWCTL_ENDPOINT", "flowctl.service.consul:8080")
	t.Setenv("FLOWCTL_COMPONENT_ID", "bronze-history-loader")
	t.Setenv("FLOWCTL_RUN_ID", "mainnet-bronze-repair-20260612")
	t.Setenv("FLOWCTL_ATTEMPT", "3")
	t.Setenv("FLOWCTL_HEARTBEAT_INTERVAL_MS", "2500")

	cfg := ConfigFromEnv()
	if !cfg.Enabled {
		t.Fatal("expected flowctl to be enabled")
	}
	if cfg.Endpoint != "flowctl.service.consul:8080" {
		t.Fatalf("unexpected endpoint: %s", cfg.Endpoint)
	}
	if cfg.ComponentID != "bronze-history-loader" {
		t.Fatalf("unexpected component id: %s", cfg.ComponentID)
	}
	if cfg.RunID != "mainnet-bronze-repair-20260612" {
		t.Fatalf("unexpected run id: %s", cfg.RunID)
	}
	if cfg.Attempt != 3 {
		t.Fatalf("expected attempt 3, got %d", cfg.Attempt)
	}
	if cfg.HeartbeatInterval != 2500*time.Millisecond {
		t.Fatalf("expected heartbeat interval 2500ms, got %s", cfg.HeartbeatInterval)
	}
}

func TestDisabledReporterIsNoop(t *testing.T) {
	reporter, err := NewReporter(context.Background(), Config{Enabled: false})
	if err != nil {
		t.Fatalf("disabled reporter should not error: %v", err)
	}
	if reporter.Enabled() {
		t.Fatal("expected reporter to be disabled")
	}
	if err := reporter.Register(context.Background(), flowctlpb.ServiceType_SERVICE_TYPE_SOURCE, nil); err != nil {
		t.Fatalf("disabled Register should be noop: %v", err)
	}
	if err := reporter.Heartbeat(context.Background(), nil); err != nil {
		t.Fatalf("disabled Heartbeat should be noop: %v", err)
	}
	if err := reporter.ReportChunkProgress(context.Background(), 1, 2, "extract", nil, nil); err != nil {
		t.Fatalf("disabled ReportChunkProgress should be noop: %v", err)
	}
}

func TestReporterRequiresEndpointAndComponentWhenEnabled(t *testing.T) {
	_, err := NewReporter(context.Background(), Config{Enabled: true})
	if err == nil {
		t.Fatal("expected missing endpoint error")
	}

	_, err = NewReporter(context.Background(), Config{Enabled: true, Endpoint: "127.0.0.1:1"})
	if err == nil {
		t.Fatal("expected missing component id error")
	}
}
