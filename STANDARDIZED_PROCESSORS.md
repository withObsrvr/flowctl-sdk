# Standardized Stellar Processors

This document explains how to build standardized Stellar processors following the official Stellar SDK pattern.

## Overview

The flowctl-sdk now supports building processors following Stellar's standard pattern, as used in their official `token_transfer` processor. This approach:

✅ **Standardizes** processor interfaces across the ecosystem
✅ **Enables reuse** - processors can be imported by any project
✅ **Simplifies testing** - processors are pure functions
✅ **Allows contribution** - processors can be contributed back to Stellar SDK
✅ **Reduces boilerplate** - flowctl integration is automatic

## The Stellar Pattern

### Structure

Stellar processors follow this standard structure:

```
processors/
└── your_processor/
    ├── README.md                     # Documentation
    ├── processor.go                  # Main processor logic
    ├── event.go                      # Event structure definitions
    ├── helpers.go                    # Helper functions
    └── processor_test.go             # Tests
```

### Core Interface

```go
// EventsProcessor is the main processor struct
type EventsProcessor struct {
	networkPassphrase string
	// Optional configuration fields
}

// Constructor with functional options
func NewEventsProcessor(networkPassphrase string, options ...EventsProcessorOption) *EventsProcessor {
	proc := &EventsProcessor{
		networkPassphrase: networkPassphrase,
	}
	for _, opt := range options {
		opt(proc)
	}
	return proc
}

// Main processing method
func (p *EventsProcessor) EventsFromLedger(ledger xdr.LedgerCloseMeta) ([]*YourEvent, error) {
	// Extract events from ledger
	// Return empty slice if no events
	// Return error only for processing failures
}
```

## Example: Contract Invocation Processor

We've created a complete example following this pattern:

### Package Structure

```
pkg/stellar/processors/contract_invocation/
├── README.md                              # Full documentation
├── contract_invocation_processor.go       # Main processor
├── contract_invocation_event.go           # Event structures
└── helpers.go                             # ScVal conversion utilities
```

### Usage

```go
package main

import (
	"github.com/withObsrvr/flowctl-sdk/pkg/stellar"
	"github.com/withObsrvr/flowctl-sdk/pkg/stellar/processors/contract_invocation"
	"github.com/stellar/go-stellar-sdk/xdr"
	"google.golang.org/protobuf/proto"
)

func main() {
	stellar.Run(stellar.ProcessorConfig{
		ProcessorName: "Contract Invocation Processor",
		OutputType:    "stellar.contract.invocation.v1",
		ProcessLedger: func(passphrase string, ledger xdr.LedgerCloseMeta) (proto.Message, error) {
			// 1. Create processor
			processor := contract_invocation.NewEventsProcessor(passphrase)

			// 2. Extract events
			events, err := processor.EventsFromLedger(ledger)
			if err != nil {
				return nil, err
			}

			// 3. Skip if no events
			if len(events) == 0 {
				return nil, nil
			}

			// 4. Convert to proto and return
			return convertToProto(events), nil
		},
	})
}
```

## Key Design Principles

### 1. Stateless Processing

Each ledger is processed independently:

```go
// ✅ Good - Stateless
func (p *EventsProcessor) EventsFromLedger(ledger xdr.LedgerCloseMeta) ([]*Event, error) {
	// Process ledger and return events
}

// ❌ Bad - Stateful
type Processor struct {
	cache map[string]interface{} // Don't maintain state between ledgers
}
```

### 2. Use Stellar's Ingest Primitives

```go
// Use Stellar's standard ingest tools
txReader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(
	p.networkPassphrase,
	ledger,
)

// For state changes
changeReader, err := ingest.NewLedgerChangeReaderFromLedgerCloseMeta(
	p.networkPassphrase,
	ledger,
)
```

### 3. Multiple Data Representations

Provide both raw XDR and decoded values:

```go
type Event struct {
	// Raw XDR for programmatic access
	ArgumentsRaw []xdr.ScVal `json:"arguments_raw,omitempty"`

	// JSON-friendly representation
	Arguments []json.RawMessage `json:"arguments,omitempty"`

	// Decoded Go types for convenience
	ArgumentsDecoded map[string]interface{} `json:"arguments_decoded,omitempty"`
}
```

### 4. Helper Methods

Add convenience methods to event types:

```go
func (e *ContractInvocationEvent) GetEventType() string {
	return "contract_invocation"
}

func (e *ContractInvocationEvent) HasCrossContractCalls() bool {
	return len(e.ContractCalls) > 0
}

func (e *ContractInvocationEvent) GetMaxCallDepth() int {
	maxDepth := 0
	for _, call := range e.ContractCalls {
		if call.CallDepth > maxDepth {
			maxDepth = call.CallDepth
		}
	}
	return maxDepth
}
```

## Benefits Over cdp-pipeline-workflow Pattern

### Before (cdp-pipeline-workflow)

```go
type Processor struct {
	processors        []Processor        // Manual subscription
	networkPassphrase string
	mu                sync.RWMutex
	stats             struct { ... }     // Manual stats tracking
}

func (p *Processor) Subscribe(processor Processor)
func (p *Processor) Process(ctx context.Context, msg Message) error
func (p *Processor) forwardToProcessors(ctx context.Context, data interface{}) error
func (p *Processor) GetStats() struct { ... }
```

**Issues**:
- ❌ Subscription pattern adds complexity
- ❌ Manual message passing
- ❌ Hard to test (needs mock subscribers)
- ❌ Hard to reuse (tightly coupled to pipeline)
- ❌ Mixes concerns (processing + orchestration)

### After (Stellar Pattern)

```go
type EventsProcessor struct {
	networkPassphrase string
}

func NewEventsProcessor(networkPassphrase string) *EventsProcessor
func (p *EventsProcessor) EventsFromLedger(ledger xdr.LedgerCloseMeta) ([]*Event, error)
```

**Benefits**:
- ✅ Simple interface (ledger in, events out)
- ✅ Easy to test (pure function)
- ✅ Reusable (no dependencies on pipeline)
- ✅ Standard (matches Stellar's processors)
- ✅ Separation of concerns (processing only)

## Code Reduction

### Token Transfer Processor

- **Before**: 425 lines (ttp-processor-sdk)
- **After**: 32 lines (using stellar.Run() + Stellar's processor)
- **Reduction**: 92%

### Contract Invocation Processor

- **Before**: 1,123 lines (cdp-pipeline-workflow)
- **After**: 60 lines (using stellar.Run() + standardized processor)
- **Reduction**: 95%

## Creating Your Own Processor

### Step 1: Define Event Structure

```go
// your_processor/event.go
package your_processor

type YourEvent struct {
	Timestamp       time.Time
	LedgerSequence  uint32
	TransactionHash string
	// ... your specific fields
}

func (e *YourEvent) GetEventType() string {
	return "your_event_type"
}
```

### Step 2: Implement Processor

```go
// your_processor/processor.go
package your_processor

type EventsProcessor struct {
	networkPassphrase string
}

func NewEventsProcessor(networkPassphrase string) *EventsProcessor {
	return &EventsProcessor{
		networkPassphrase: networkPassphrase,
	}
}

func (p *EventsProcessor) EventsFromLedger(ledger xdr.LedgerCloseMeta) ([]*YourEvent, error) {
	var events []*YourEvent

	// Create transaction reader
	txReader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(
		p.networkPassphrase,
		ledger,
	)
	if err != nil {
		return nil, err
	}
	defer txReader.Close()

	// Process transactions
	for {
		tx, err := txReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Extract your events from transaction
		event := p.extractEvent(tx, ledger)
		if event != nil {
			events = append(events, event)
		}
	}

	return events, nil
}
```

### Step 3: Use with stellar.Run()

```go
// examples/your-processor/main.go
func main() {
	stellar.Run(stellar.ProcessorConfig{
		ProcessorName: "Your Processor",
		OutputType:    "your.event.v1",
		ProcessLedger: func(passphrase string, ledger xdr.LedgerCloseMeta) (proto.Message, error) {
			processor := your_processor.NewEventsProcessor(passphrase)
			events, err := processor.EventsFromLedger(ledger)
			if err != nil {
				return nil, err
			}
			if len(events) == 0 {
				return nil, nil
			}
			return convertToProto(events), nil
		},
	})
}
```

## Testing

Processors are easy to test because they're pure functions:

```go
func TestEventsFromLedger(t *testing.T) {
	// Load test ledger
	ledger := loadTestLedger(t, "testdata/ledger_12345.xdr")

	// Create processor
	processor := NewEventsProcessor("Test SDF Network ; September 2015")

	// Process ledger
	events, err := processor.EventsFromLedger(ledger)

	// Assert results
	require.NoError(t, err)
	require.Len(t, events, 3)
	assert.Equal(t, "expected_contract_id", events[0].ContractID)
}
```

## Contributing to Stellar SDK

Processors built with this pattern can be contributed to the official Stellar SDK:

```
github.com/stellar/go-stellar-sdk/processors/your_processor/
```

**Requirements**:
1. Follow the standard structure (processor.go, event.go, helpers.go)
2. Include comprehensive tests
3. Add README with examples
4. Document all public methods
5. No external dependencies beyond Stellar SDK

## References

- [Stellar Token Transfer Processor](https://github.com/stellar/go-stellar-sdk/tree/master/processors/token_transfer)
- [Building Stellar Processors](https://developers.stellar.org/docs/data/indexers/build-your-own/processors/token-transfer-processor)
- [Contract Invocation Processor Example](/home/tillman/Documents/flowctl-sdk/pkg/stellar/processors/contract_invocation/)
- [Migration Guide](/home/tillman/Documents/flowctl-sdk/examples/contract-invocation-processor/MIGRATION_GUIDE.md)

## Summary

The standardized processor pattern provides:

1. **95%+ code reduction** compared to manual pipeline implementation
2. **Standard interface** matching Stellar's official processors
3. **Reusability** across projects and potential contribution to Stellar SDK
4. **Testability** through pure functions
5. **Automatic integration** with flowctl, health checks, and metrics
6. **Separation of concerns** between processing logic and orchestration

Start with one of the examples and adapt it to your use case!
