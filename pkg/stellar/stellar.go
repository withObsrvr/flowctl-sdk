// Package stellar provides a zero-config wrapper for building Stellar ledger processors.
//
// Example usage:
//
//	import (
//	    "github.com/withobsrvr/flowctl-sdk/pkg/stellar"
//	    "github.com/stellar/go-stellar-sdk/processors/token_transfer"
//	)
//
//	func main() {
//	    stellar.Run(stellar.Config{
//	        ProcessorName: "Token Transfer Processor",
//	        OutputType:    "stellar.token.transfer.v1",
//	        ProcessLedger: func(passphrase string, ledger xdr.LedgerCloseMeta) (proto.Message, error) {
//	            processor := token_transfer.NewEventsProcessor(passphrase)
//	            events, err := processor.EventsFromLedger(ledger)
//	            if err != nil {
//	                return nil, err
//	            }
//	            // Convert to proto using Stellar's own proto definitions
//	            return &stellarprotos.TokenTransferBatch{Events: events}, nil
//	        },
//	    })
//	}
//
// That's it! All configuration is handled through environment variables or an optional processor.yaml file.
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
	stellarv1 "github.com/withObsrvr/flow-proto/go/gen/stellar/v1"
	"github.com/stellar/go-stellar-sdk/xdr"
	"google.golang.org/protobuf/proto"
)

// ProcessorFunc is a function that processes a Stellar ledger and returns a proto message.
// The function receives the network passphrase and the ledger close meta.
// It should return a proto.Message containing the processed data.
type ProcessorFunc func(networkPassphrase string, ledger xdr.LedgerCloseMeta) (proto.Message, error)

// ProcessorConfig defines the configuration for a generic Stellar processor.
type ProcessorConfig struct {
	// ProcessorName is the human-readable name of the processor (e.g., "Token Transfer Processor")
	ProcessorName string

	// OutputType is the event type this processor produces (e.g., "stellar.token.transfer.v1")
	OutputType string

	// ProcessLedger is the function that processes each ledger
	ProcessLedger ProcessorFunc

	// Optional: Custom configuration file path (defaults to "processor.yaml")
	ConfigPath string

	// Optional: Component ID (defaults to processor name in kebab-case)
	ComponentID string
}

// Run starts a generic Stellar processor with zero configuration required.
//
// Configuration is loaded from:
// 1. Default values
// 2. processor.yaml file (if present)
// 3. Environment variables (override file)
//
// Required environment variables:
//   - NETWORK_PASSPHRASE: Stellar network passphrase
//
// Optional environment variables:
//   - COMPONENT_ID: Component identifier (default: derived from ProcessorName)
//   - PORT: gRPC server port (default: ":50051")
//   - HEALTH_PORT: Health check port (default: "8088")
//   - ENABLE_FLOWCTL: Enable flowctl integration (default: "false")
//   - FLOWCTL_ENDPOINT: Flowctl endpoint (default: "localhost:8080")
//   - ENABLE_METRICS: Enable metrics (default: "true")
func Run(cfg ProcessorConfig) {
	// Load configuration
	configPath := cfg.ConfigPath
	if configPath == "" {
		configPath = "processor.yaml"
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Set processor-specific config
	if cfg.ProcessorName != "" {
		config.Processor.Name = cfg.ProcessorName
	}
	if cfg.OutputType != "" {
		config.Processor.Output = cfg.OutputType
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
	procConfig.InputEventTypes = []string{"stellar.ledger.v1"}
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

	log.Printf("Starting %s", config.Processor.Name)
	log.Printf("Network: %s", config.Network.Passphrase)
	log.Printf("Output type: %s", cfg.OutputType)

	// Register processor handler
	err = proc.OnProcess(
		func(ctx context.Context, event *flowctlv1.Event) (*flowctlv1.Event, error) {
			// Only process stellar ledger events
			if event.Type != "stellar.ledger.v1" {
				return nil, nil // Skip non-ledger events
			}

			// Parse the RawLedger from payload
			var rawLedger stellarv1.RawLedger
			if err := proto.Unmarshal(event.Payload, &rawLedger); err != nil {
				return nil, fmt.Errorf("failed to unmarshal ledger: %w", err)
			}

			// Decode XDR
			var ledgerCloseMeta xdr.LedgerCloseMeta
			if err := xdr.SafeUnmarshal(rawLedger.LedgerCloseMetaXdr, &ledgerCloseMeta); err != nil {
				return nil, fmt.Errorf("failed to unmarshal XDR: %w", err)
			}

			// Call the user's processor function
			processedData, err := cfg.ProcessLedger(config.Network.Passphrase, ledgerCloseMeta)
			if err != nil {
				return nil, fmt.Errorf("failed to process ledger: %w", err)
			}

			// If no data produced, skip
			if processedData == nil {
				return nil, nil
			}

			// Marshal the processed data
			payload, err := proto.Marshal(processedData)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal processed data: %w", err)
			}

			// Create output event
			outputEvent := &flowctlv1.Event{
				Id:                fmt.Sprintf("%s-%d", componentID, rawLedger.Sequence),
				Type:              cfg.OutputType,
				Payload:           payload,
				Metadata:          make(map[string]string),
				SourceComponentId: componentID,
				ContentType:       "application/protobuf",
				StellarCursor: &flowctlv1.StellarCursor{
					LedgerSequence: uint64(rawLedger.Sequence),
				},
			}

			// Copy metadata
			for k, v := range event.Metadata {
				outputEvent.Metadata[k] = v
			}
			outputEvent.Metadata["ledger_sequence"] = fmt.Sprintf("%d", rawLedger.Sequence)
			outputEvent.Metadata["processor_version"] = config.Processor.Version

			return outputEvent, nil
		},
		[]string{"stellar.ledger.v1"},
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
	log.Println("Shutting down processor...")
	if err := proc.Stop(); err != nil {
		log.Fatalf("Failed to stop processor: %v", err)
	}

	log.Println("Processor stopped successfully")
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseInt(s string) int {
	var v int
	fmt.Sscanf(s, "%d", &v)
	return v
}

func toKebabCase(s string) string {
	// Simple conversion: lowercase and replace spaces with hyphens
	result := ""
	for i, r := range s {
		if r == ' ' {
			result += "-"
		} else if r >= 'A' && r <= 'Z' {
			if i > 0 && s[i-1] != ' ' {
				result += "-"
			}
			result += string(r + 32) // Convert to lowercase
		} else {
			result += string(r)
		}
	}
	return result
}
