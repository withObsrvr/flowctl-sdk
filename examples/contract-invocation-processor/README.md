# Contract Invocation Processor Example

This example demonstrates how to use the contract invocation processor with the flowctl-sdk wrapper.

## What This Example Shows

1. **Stellar Pattern**: Using `contract_invocation.NewEventsProcessor()` - same pattern as Stellar's `token_transfer` processor
2. **Zero Boilerplate**: ~60 lines of code vs 1,123 lines in the original cdp-pipeline-workflow version
3. **Rich Event Data**: Access to contract invocations, cross-contract calls, state changes, and diagnostic events
4. **Automatic Integration**: Flowctl integration, health checks, metrics, and signal handling are all handled automatically

## Code Breakdown

```go
// 1. Create processor using Stellar's standard pattern
processor := contract_invocation.NewEventsProcessor(passphrase)

// 2. Extract events from ledger
events, err := processor.EventsFromLedger(ledger)

// 3. Process or filter events
for _, event := range events {
	if event.HasCrossContractCalls() {
		// Do something with cross-contract calls
	}
}

// 4. Convert to proto and return
return convertToProto(events)
```

## Running the Example

```bash
cd /home/tillman/Documents/flowctl-sdk/examples/contract-invocation-processor

# Set network passphrase
export NETWORK_PASSPHRASE="Test SDF Network ; September 2015"

# Optional: Enable flowctl integration
export ENABLE_FLOWCTL=true
export FLOWCTL_ENDPOINT=localhost:8080

# Build and run
go build -o contract-invocation-processor
./contract-invocation-processor
```

## Configuration

The example supports all standard flowctl-sdk configuration options:

### Environment Variables

```bash
# Required
NETWORK_PASSPHRASE="Test SDF Network ; September 2015"

# Optional flowctl integration
ENABLE_FLOWCTL=true
FLOWCTL_ENDPOINT=localhost:8080
COMPONENT_ID=contract-invocation-processor

# Optional health check
HEALTH_PORT=8088

# Optional metrics
METRICS_ENABLED=true
```

### YAML Configuration

Create `processor.yaml`:

```yaml
processor:
  name: "Contract Invocation Processor"
  version: "1.0.0"
  description: "Extracts contract invocations with cross-contract calls"
  output: "stellar.contract.invocation.v1"

network:
  passphrase: "${NETWORK_PASSPHRASE}"

flowctl:
  enabled: true
  endpoint: localhost:8080

metrics:
  enabled: true

health:
  port: 8088
```

## Event Structure

Each `ContractInvocationEvent` includes:

- **Core Metadata**: Timestamp, ledger sequence, transaction hash
- **Contract Details**: Contract ID, invoking account, function name
- **Arguments**: Raw XDR, JSON, and decoded representations
- **Execution Context**:
  - `DiagnosticEvents` - Events emitted by the contract
  - `ContractCalls` - Cross-contract invocations with call depth
  - `StateChanges` - Contract storage modifications
  - `TtlExtensions` - Time-to-live extensions

## Filtering Events

You can filter events based on various criteria:

```go
// Only successful invocations
if event.Successful {
	// ...
}

// Only invocations with cross-contract calls
if event.HasCrossContractCalls() {
	// ...
}

// Only invocations that modify state
if event.HasStateChanges() {
	// ...
}

// Only invocations with deep call stacks
if event.GetMaxCallDepth() > 3 {
	// ...
}

// Only specific contracts
if event.ContractID == "CA..." {
	// ...
}

// Only specific functions
if event.FunctionName == "swap" {
	// ...
}
```

## Comparison to cdp-pipeline-workflow

### Before (1,123 lines)

```go
type ContractInvocationProcessor struct {
	processors        []Processor
	networkPassphrase string
	mu                sync.RWMutex
	stats             struct { ... }
}

// Manual subscription management
func (p *ContractInvocationProcessor) Subscribe(processor Processor)

// Manual message processing
func (p *ContractInvocationProcessor) Process(ctx context.Context, msg Message) error

// Manual JSON marshaling
func (p *ContractInvocationProcessor) forwardToProcessors(ctx context.Context, invocation *ContractInvocation) error

// Manual statistics tracking
func (p *ContractInvocationProcessor) GetStats() struct { ... }

// ~1,000 lines of extraction logic
```

### After (60 lines + reusable processor package)

```go
stellar.Run(stellar.ProcessorConfig{
	ProcessorName: "Contract Invocation Processor",
	OutputType:    "stellar.contract.invocation.v1",
	ProcessLedger: func(passphrase string, ledger xdr.LedgerCloseMeta) (proto.Message, error) {
		// Use standardized processor
		processor := contract_invocation.NewEventsProcessor(passphrase)
		events, err := processor.EventsFromLedger(ledger)

		// Optional filtering
		filtered := filterEvents(events)

		// Convert and return
		return convertToProto(filtered)
	},
})
```

**Benefits**:
- ✅ 95% code reduction (1,123 → 60 lines)
- ✅ Reusable processor package (could be contributed to Stellar SDK)
- ✅ Standard Stellar pattern (same as token_transfer)
- ✅ No boilerplate (flowctl, health, metrics all automatic)
- ✅ Easy to test (processor is pure function)
- ✅ Easy to customize (simple filtering and conversion)

## Next Steps

1. **Define Proto Schema**: Create a proper `.proto` file for contract invocations (see MIGRATION_GUIDE.md)
2. **Generate Proto Code**: Use `protoc` to generate Go types
3. **Replace structpb**: Use your generated proto types instead of `structpb.Struct`
4. **Add Filtering**: Implement domain-specific event filtering
5. **Deploy**: Run with flowctl control plane for orchestration

## References

- [Contract Invocation Processor Package](../../pkg/stellar/processors/contract_invocation/)
- [Migration Guide](./MIGRATION_GUIDE.md)
- [Stellar Token Transfer Processor](https://github.com/stellar/go-stellar-sdk/tree/master/processors/token_transfer)
- [Flowctl SDK Documentation](../../README.md)
