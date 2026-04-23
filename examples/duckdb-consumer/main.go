package main

import (
	"context"
	"fmt"

	"github.com/withObsrvr/flowctl-sdk/pkg/consumer"
	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
)

func main() {
	// Initialize database (in real implementation, this would be DuckDB)
	// For this example, we'll just log events
	db := &MockDatabase{
		events: make([]EventRecord, 0),
	}

	consumer.Run(consumer.ConsumerConfig{
		ConsumerName: "DuckDB Consumer",
		InputTypes:   []string{"stellar.ledger.v1", "contract.event.v1"},
		OnEvent: func(ctx context.Context, event *flowctlv1.Event) error {
			switch event.Type {
			case "stellar.ledger.v1":
				return handleLedgerEvent(db, event)
			case "contract.event.v1":
				return handleContractEvent(db, event)
			default:
				fmt.Printf("Unknown event type: %s\n", event.Type)
				return nil
			}
		},
	})

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

