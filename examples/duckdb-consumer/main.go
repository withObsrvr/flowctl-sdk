package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/withObsrvr/flowctl-sdk/pkg/consumer"
	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
)

func main() {
	// Create consumer with configuration
	config := consumer.DefaultConfig()
	config.ID = "duckdb-consumer"
	config.Name = "DuckDB Consumer"
	config.Description = "Consumes events and stores them in DuckDB"
	config.Version = "1.0.0"
	config.Endpoint = ":50053"
	config.InputEventTypes = []string{"stellar.ledger.v1", "contract.event.v1"}
	config.MaxConcurrent = 10

	// Enable flowctl integration if endpoint is provided
	flowctlEndpoint := getEnv("FLOWCTL_ENDPOINT", "")
	if flowctlEndpoint != "" {
		config.FlowctlConfig.Enabled = true
		config.FlowctlConfig.Endpoint = flowctlEndpoint
		config.FlowctlConfig.ServiceID = getEnv("SERVICE_ID", "duckdb-consumer-1")
	}

	cons, err := consumer.New(config)
	if err != nil {
		log.Fatalf("Failed to create consumer: %v", err)
	}

	// Initialize database (in real implementation, this would be DuckDB)
	// For this example, we'll just log events
	db := &MockDatabase{
		events: make([]EventRecord, 0),
	}

	// Register event handler
	err = cons.OnConsume(func(ctx context.Context, event *flowctlv1.Event) error {
		// Handle different event types
		switch event.Type {
		case "stellar.ledger.v1":
			return handleLedgerEvent(db, event)
		case "contract.event.v1":
			return handleContractEvent(db, event)
		default:
			fmt.Printf("Unknown event type: %s\n", event.Type)
			return nil // Skip unknown events
		}
	})
	if err != nil {
		log.Fatalf("Failed to register handler: %v", err)
	}

	// Start the consumer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := cons.Start(ctx); err != nil {
		log.Fatalf("Failed to start consumer: %v", err)
	}

	fmt.Println("DuckDB consumer is running...")
	fmt.Printf("Health endpoint: http://localhost:%d/health\n", config.HealthPort)
	fmt.Printf("gRPC endpoint: %s\n", config.Endpoint)
	fmt.Printf("Consuming event types: %v\n", config.InputEventTypes)

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Stop the consumer gracefully
	fmt.Println("Shutting down consumer...")
	if err := cons.Stop(); err != nil {
		log.Fatalf("Failed to stop consumer: %v", err)
	}

	// Print summary
	fmt.Printf("\nProcessed %d events:\n", len(db.events))
	for eventType, count := range db.GetEventTypeCounts() {
		fmt.Printf("  %s: %d\n", eventType, count)
	}
}

// EventRecord represents a stored event
type EventRecord struct {
	EventID    string
	EventType  string
	PayloadLen int
	Metadata   map[string]string
}

// MockDatabase simulates a database
type MockDatabase struct {
	events []EventRecord
}

// Insert adds an event to the database
func (db *MockDatabase) Insert(record EventRecord) error {
	db.events = append(db.events, record)
	fmt.Printf("Stored event: %s (type: %s, payload: %d bytes)\n",
		record.EventID, record.EventType, record.PayloadLen)
	return nil
}

// GetEventTypeCounts returns counts by event type
func (db *MockDatabase) GetEventTypeCounts() map[string]int {
	counts := make(map[string]int)
	for _, event := range db.events {
		counts[event.EventType]++
	}
	return counts
}

// handleLedgerEvent handles stellar ledger events
func handleLedgerEvent(db *MockDatabase, event *flowctlv1.Event) error {
	fmt.Printf("Processing ledger event: %s (ledger: %s)\n",
		event.Id, event.Metadata["ledger_sequence"])

	record := EventRecord{
		EventID:    event.Id,
		EventType:  event.Type,
		PayloadLen: len(event.Payload),
		Metadata:   event.Metadata,
	}

	return db.Insert(record)
}

// handleContractEvent handles contract events
func handleContractEvent(db *MockDatabase, event *flowctlv1.Event) error {
	fmt.Printf("Processing contract event: %s (contract: %s)\n",
		event.Id, event.Metadata["contract_id"])

	record := EventRecord{
		EventID:    event.Id,
		EventType:  event.Type,
		PayloadLen: len(event.Payload),
		Metadata:   event.Metadata,
	}

	return db.Insert(record)
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
