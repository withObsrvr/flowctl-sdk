package main

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

func main() {
	// Create processor with configuration
	config := processor.DefaultConfig()
	config.ID = "example-processor"
	config.Name = "Example Processor"
	config.Description = "A simple example processor"
	config.Version = "1.0.0"
	config.Endpoint = ":50052"
	
	// Enable flowctl integration if endpoint is provided
	flowctlEndpoint := getEnv("FLOWCTL_ENDPOINT", "")
	if flowctlEndpoint != "" {
		config.FlowctlConfig.Enabled = true
		config.FlowctlConfig.Endpoint = flowctlEndpoint
		config.FlowctlConfig.ServiceID = getEnv("SERVICE_ID", "example-processor-1")
	}
	
	proc, err := processor.New(config)
	if err != nil {
		log.Fatalf("Failed to create processor: %v", err)
	}

	// Register processing handler
	err = proc.OnProcess(
		// Handler function
		func(ctx context.Context, event *flowctlv1.Event) (*flowctlv1.Event, error) {
			// Process the event (in this case, just append to the payload)
			processedPayload := append(event.Payload, []byte(" - processed")...)

			// Create output event
			outputEvent := &flowctlv1.Event{
				Id:                fmt.Sprintf("%s-processed", event.Id),
				Type:              "example.processed.event",
				Payload:           processedPayload,
				Metadata:          make(map[string]string),
				SourceComponentId: config.ID,
				ContentType:       event.ContentType,
			}

			// Copy original metadata and add processing metadata
			for k, v := range event.Metadata {
				outputEvent.Metadata[k] = v
			}
			outputEvent.Metadata["processor_id"] = config.ID
			outputEvent.Metadata["original_type"] = event.Type

			return outputEvent, nil
		},
		// Input types
		[]string{"example.event"},
		// Output types
		[]string{"example.processed.event"},
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

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Stop the processor gracefully
	fmt.Println("Shutting down processor...")
	if err := proc.Stop(); err != nil {
		log.Fatalf("Failed to stop processor: %v", err)
	}
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}