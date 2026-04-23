// Package stellar provides helpers for building Stellar event processors, including
// zero-config wrappers for event-to-event transform processors.
//
// Example usage:
//
//	import (
//	    "github.com/withObsrvr/flowctl-sdk/pkg/stellar"
//	    flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
//	)
//
//	func main() {
//	    stellar.RunTransform(stellar.TransformConfig{
//	        ProcessorName: "Soroswap Detector",
//	        InputType:     "stellar.swap_candidate.v1",
//	        OutputType:    "stellar.swap_candidate.v1",
//	        Transform: func(ctx context.Context, event *flowctlv1.Event) (*flowctlv1.Event, error) {
//	            // Enrich or filter the event
//	            return enrichedEvent, nil
//	        },
//	    })
//	}
package stellar

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/withObsrvr/flowctl-sdk/pkg/processor"
	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
)

// TransformFunc is a function that transforms one event into another.
// Return nil to filter out (skip) the event.
type TransformFunc func(ctx context.Context, event *flowctlv1.Event) (*flowctlv1.Event, error)

// TransformConfig defines the configuration for an event-to-event transform processor.
type TransformConfig struct {
	// ProcessorName is the human-readable name of the processor
	ProcessorName string

	// InputType is the event type this processor consumes (e.g., "stellar.swap_candidate.v1")
	InputType string

	// OutputType is the event type this processor produces (e.g., "stellar.dex_swap.v1")
	OutputType string

	// Transform is the function that transforms each event
	Transform TransformFunc

	// Optional: Custom configuration file path (defaults to "processor.yaml")
	ConfigPath string

	// Optional: Component ID (defaults to processor name in kebab-case)
	ComponentID string
}

// RunTransform starts an event-to-event transform processor with zero configuration required.
//
// Unlike Run() which processes raw Stellar ledgers, RunTransform() receives already-processed
// events and transforms them into new events. This is used for enrichment, filtering,
// normalization, and aggregation stages in a pipeline.
//
// Configuration is loaded from:
// 1. Default values
// 2. processor.yaml file (if present)
// 3. Environment variables (override file)
//
// Optional environment variables:
//   - COMPONENT_ID: Component identifier (default: derived from ProcessorName)
//   - PORT: gRPC server port (default: ":50051")
//   - HEALTH_PORT: Health check port (default: "8088")
//   - ENABLE_FLOWCTL: Enable flowctl integration (default: "false")
//   - FLOWCTL_ENDPOINT: Flowctl endpoint (default: "localhost:8080")
func RunTransform(cfg TransformConfig) {
	if cfg.InputType == "" {
		log.Fatal("RunTransform requires a non-empty InputType")
	}
	if cfg.OutputType == "" {
		log.Fatal("RunTransform requires a non-empty OutputType")
	}
	if cfg.Transform == nil {
		log.Fatal("RunTransform requires a non-nil Transform function")
	}

	// Load configuration
	configPath := cfg.ConfigPath
	if configPath == "" {
		configPath = "processor.yaml"
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Set processor-specific config
	if cfg.ProcessorName != "" {
		config.Processor.Name = cfg.ProcessorName
	}

	// Get configuration from environment with defaults
	componentID := cfg.ComponentID
	if componentID == "" {
		componentID = getEnv("COMPONENT_ID", toKebabCase(config.Processor.Name))
	}
	port := getEnv("PORT", ":50051")
	healthPort := parseInt(getEnv("HEALTH_PORT", "8088"))

	// Create processor SDK config
	procConfig := processor.DefaultConfig()
	procConfig.ID = componentID
	procConfig.Name = config.Processor.Name
	procConfig.Description = config.Processor.Description
	procConfig.Version = config.Processor.Version
	procConfig.Endpoint = port
	procConfig.HealthPort = healthPort
	procConfig.InputEventTypes = []string{cfg.InputType}
	procConfig.OutputEventTypes = []string{cfg.OutputType}

	// Configure flowctl integration
	if config.Flowctl.Enabled {
		procConfig.FlowctlConfig.Enabled = true
		procConfig.FlowctlConfig.Endpoint = config.Flowctl.Endpoint
		procConfig.FlowctlConfig.HeartbeatInterval = config.Flowctl.HeartbeatInterval
	}

	// Create processor
	proc, err := processor.New(procConfig)
	if err != nil {
		log.Fatalf("Failed to create processor: %v", err)
	}

	log.Printf("Starting transform processor: %s", config.Processor.Name)
	log.Printf("Input type: %s", cfg.InputType)
	log.Printf("Output type: %s", cfg.OutputType)

	// Register transform handler
	err = proc.OnProcess(
		func(ctx context.Context, event *flowctlv1.Event) (*flowctlv1.Event, error) {
			// Only process events matching our input type
			if event.Type != cfg.InputType {
				return nil, nil
			}

			// Call the user's transform function
			result, err := cfg.Transform(ctx, event)
			if err != nil {
				return nil, fmt.Errorf("transform failed: %w", err)
			}

			// If transform returns nil, skip this event
			if result == nil {
				return nil, nil
			}

			// Ensure output event has correct type and source
			result.Type = cfg.OutputType
			result.SourceComponentId = componentID

			return result, nil
		},
		[]string{cfg.InputType},
		[]string{cfg.OutputType},
	)
	if err != nil {
		log.Fatalf("Failed to register handler: %v", err)
	}

	// Start the processor
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := proc.Start(ctx); err != nil {
		log.Fatalf("Failed to start processor: %v", err)
	}

	log.Printf("%s is running on %s (health port: %d)", config.Processor.Name, port, healthPort)
	if config.Flowctl.Enabled {
		log.Printf("Flowctl integration enabled (endpoint: %s)", config.Flowctl.Endpoint)
	}

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Stop the processor gracefully
	log.Println("Shutting down transform processor...")
	if err := proc.Stop(); err != nil {
		log.Fatalf("Failed to stop processor: %v", err)
	}

	log.Println("Transform processor stopped successfully")
}
