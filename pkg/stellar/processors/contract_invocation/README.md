# Contract Invocation Processor

A Stellar processor that extracts contract invocation events from Soroban smart contract executions. This processor follows the same pattern as Stellar's official `token_transfer` processor.

## Features

- **Contract Invocations**: Extract all `InvokeHostFunction` operations
- **Function Details**: Capture function names and arguments
- **Diagnostic Events**: Include all contract events emitted during execution
- **Cross-Contract Calls**: Track contract-to-contract invocations with call depth
- **State Changes**: Monitor contract storage modifications
- **TTL Extensions**: Track Time-To-Live extensions for contract storage

## Usage

### Basic Usage

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
			// Create processor
			processor := contract_invocation.NewEventsProcessor(passphrase)

			// Extract events
			events, err := processor.EventsFromLedger(ledger)
			if err != nil {
				return nil, err
			}

			// No events? Skip this ledger
			if len(events) == 0 {
				return nil, nil
			}

			// TODO: Convert to proto message
			// For now, you can use JSON serialization or create your own proto schema
			return nil, nil
		},
	})
}
```

### With Filtering

```go
ProcessLedger: func(passphrase string, ledger xdr.LedgerCloseMeta) (proto.Message, error) {
	processor := contract_invocation.NewEventsProcessor(passphrase)
	events, err := processor.EventsFromLedger(ledger)
	if err != nil {
		return nil, err
	}

	// Filter for only successful invocations with cross-contract calls
	var filtered []*contract_invocation.ContractInvocationEvent
	for _, event := range events {
		if event.Successful && event.HasCrossContractCalls() {
			filtered = append(filtered, event)
		}
	}

	if len(filtered) == 0 {
		return nil, nil
	}

	// Convert to your proto format
	return convertToProto(filtered), nil
}
```

## Event Structure

### ContractInvocationEvent

```go
type ContractInvocationEvent struct {
	// Core metadata
	Timestamp        time.Time
	LedgerSequence   uint32
	TransactionIndex uint32
	OperationIndex   uint32
	TransactionHash  string

	// Contract invocation details
	ContractID      string
	InvokingAccount string
	FunctionName    string

	// Function arguments (multiple representations)
	ArgumentsRaw     []xdr.ScVal           // Raw XDR values
	Arguments        []json.RawMessage     // JSON-encoded values
	ArgumentsDecoded map[string]interface{} // Decoded Go types

	// Execution results
	Successful bool

	// Rich execution context
	DiagnosticEvents []DiagnosticEvent
	ContractCalls    []ContractCall
	StateChanges     []StateChange
	TtlExtensions    []TtlExtension
}
```

### Helper Methods

- `GetEventType()` - Returns "contract_invocation"
- `HasCrossContractCalls()` - True if the invocation triggered cross-contract calls
- `HasStateChanges()` - True if the invocation modified contract state
- `GetMaxCallDepth()` - Returns the maximum depth of the call stack

## Cross-Contract Calls

The processor extracts cross-contract calls from two sources:

1. **Authorization Data**: From the `SorobanAuthorizedInvocation` tree
2. **Diagnostic Events**: Inferred from contract event patterns

Each `ContractCall` includes:
- Source and destination contract IDs
- Function name and arguments
- Call depth (for nested invocations)
- Authorization type ("source_account", "contract", or "inferred")
- Success status
- Execution order

## Architecture

This processor follows Stellar's standard processor pattern:

```
EventsProcessor
├── NewEventsProcessor(networkPassphrase string) *EventsProcessor
└── EventsFromLedger(ledger xdr.LedgerCloseMeta) ([]*ContractInvocationEvent, error)
```

**Key Design Principles:**
- **Stateless**: Each ledger is processed independently
- **Stellar Primitives**: Uses `ingest.NewLedgerTransactionReaderFromLedgerCloseMeta()`
- **Multiple Representations**: Provides both raw XDR and decoded values
- **Progressive Disclosure**: Rich data available but optional to use

## Comparison to cdp-pipeline-workflow

If you're migrating from the older `cdp-pipeline-workflow` pattern:

**Before (cdp-pipeline-workflow)**:
```go
type ContractInvocationProcessor struct {
	processors        []Processor
	networkPassphrase string
	stats             struct { ... }
}

func (p *ContractInvocationProcessor) Subscribe(processor Processor)
func (p *ContractInvocationProcessor) Process(ctx context.Context, msg Message) error
```

**After (Stellar pattern)**:
```go
type EventsProcessor struct {
	networkPassphrase string
}

func NewEventsProcessor(networkPassphrase string) *EventsProcessor
func (p *EventsProcessor) EventsFromLedger(ledger xdr.LedgerCloseMeta) ([]*ContractInvocationEvent, error)
```

**Benefits**:
- ✅ Simpler API - No subscription pattern needed
- ✅ Standard - Matches Stellar's token_transfer processor
- ✅ Reusable - Can be imported by any project
- ✅ Testable - Easy to unit test with ledger fixtures
- ✅ Composable - Works with flowctl-sdk wrapper

## Contributing to Stellar

This processor could be contributed to the official Stellar SDK:
```
github.com/stellar/go-stellar-sdk/processors/contract_invocation
```

It follows all the conventions:
- Same package structure as `token_transfer`
- Standard constructor pattern with functional options
- `EventsFromLedger()` method signature
- Comprehensive event structures
- Helper functions for common operations

## Examples

See the [examples directory](/home/tillman/Documents/flowctl-sdk/examples/contract-invocation-processor/) for complete working examples.

## References

- [Stellar Token Transfer Processor](https://github.com/stellar/go-stellar-sdk/tree/master/processors/token_transfer)
- [Building Stellar Processors](https://developers.stellar.org/docs/data/indexers/build-your-own/processors/token-transfer-processor)
- [Soroban Smart Contracts](https://developers.stellar.org/docs/smart-contracts)
