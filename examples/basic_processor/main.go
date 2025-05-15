package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/withObsrvr/flowctl-sdk/pkg/flowctl"
	"github.com/withObsrvr/flowctl-sdk/pkg/processor"
)

// SimpleEvent represents a basic event structure
type SimpleEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Data      string    `json:"data"`
}

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
		func(ctx context.Context, input []byte, metadata map[string]string) ([]byte, map[string]string, error) {
			// Parse input event
			var event SimpleEvent
			if err := json.Unmarshal(input, &event); err != nil {
				return nil, nil, fmt.Errorf("failed to parse event: %w", err)
			}

			// Process the event (in this case, just append to the data)
			event.Data = event.Data + " - Processed"
			event.Timestamp = time.Now()

			// Update metrics
			proc.Metrics().IncrementProcessedCount()
			proc.Metrics().IncrementSuccessCount()
			proc.Metrics().RecordProcessingLatency(10.0) // Mock 10ms latency
			
			// Custom metrics
			proc.Metrics().AddCounter("events_processed_by_type_"+event.Type, 1)

			// Marshal the processed event
			output, err := json.Marshal(event)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal event: %w", err)
			}

			// Return the processed event with updated metadata
			metadata["processed_at"] = time.Now().Format(time.RFC3339)
			metadata["processor_id"] = config.ID
			
			return output, metadata, nil
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