# Proto Strategy for Stellar Processors

## Question: Should We Have Our Own Protos?

**Answer: Yes**, you should define protobuf schemas for your processors, just like Stellar does.

## Rationale

### 1. Stellar Has Protos for Their Processors

Stellar's repository structure:

```
github.com/stellar/go-stellar-sdk/
├── processors/
│   └── token_transfer/
│       ├── token_transfer_processor.go
│       └── token_transfer_event.go
└── protos/
    └── token_transfer_event.proto
```

This separation makes sense:
- **Processor package**: Reusable Go logic
- **Proto schema**: Wire format definition

### 2. Benefits of Proto Over JSON

| Aspect | Proto | JSON (structpb.Struct) |
|--------|-------|------------------------|
| **Type Safety** | ✅ Compile-time validation | ❌ Runtime validation only |
| **Performance** | ✅ Binary encoding | ❌ Text-based |
| **Schema** | ✅ Self-documenting | ❌ Requires separate docs |
| **Versioning** | ✅ Built-in evolution | ❌ Manual management |
| **Code Generation** | ✅ Automatic | ❌ Manual structs |
| **Interoperability** | ✅ Cross-language | ⚠️ Language-specific |

### 3. Your Proto Strategy

```
github.com/withObsrvr/flow-proto/
└── proto/stellar/v1/
    ├── contract_invocation.proto     ✅ Created
    ├── token_transfer.proto          (Use Stellar's)
    └── your_custom_processor.proto   (Your additions)
```

## Implementation Pattern

### 1. Define Proto Schema

```protobuf
// proto/stellar/v1/contract_invocation.proto
syntax = "proto3";

package stellar.v1;

option go_package = "github.com/withObsrvr/flow-proto/gen/stellar/v1;stellarv1";

message ContractInvocationEvent {
  google.protobuf.Timestamp timestamp = 1;
  uint32 ledger_sequence = 2;
  string contract_id = 6;
  string function_name = 8;
  repeated string arguments = 9;
  bool successful = 10;
  // ... rich event data
}

message ContractInvocationBatch {
  repeated ContractInvocationEvent invocations = 1;
  uint32 ledger_sequence = 2;
  int32 invocation_count = 3;
}
```

### 2. Generate Go Code

```bash
cd /home/tillman/Documents/flow-proto
protoc --go_out=. --go_opt=paths=source_relative \
  proto/stellar/v1/contract_invocation.proto
```

This generates:
```
proto/stellar/v1/contract_invocation.pb.go
```

### 3. Create Converter in Processor Package

```go
// pkg/stellar/processors/contract_invocation/convert.go
package contract_invocation

import stellarv1 "github.com/withObsrvr/flow-proto/proto/stellar/v1"

func ConvertToProto(events []*ContractInvocationEvent) *stellarv1.ContractInvocationBatch {
	protoEvents := make([]*stellarv1.ContractInvocationEvent, 0, len(events))
	for _, event := range events {
		protoEvents = append(protoEvents, convertEventToProto(event))
	}

	return &stellarv1.ContractInvocationBatch{
		Invocations:     protoEvents,
		LedgerSequence:  events[0].LedgerSequence,
		InvocationCount: int32(len(protoEvents)),
	}
}
```

### 4. Use in Processor

```go
// examples/contract-invocation-processor/main.go
func main() {
	stellar.Run(stellar.ProcessorConfig{
		ProcessorName: "Contract Invocation Processor",
		OutputType:    "stellar.contract.invocation.v1",
		ProcessLedger: func(passphrase string, ledger xdr.LedgerCloseMeta) (proto.Message, error) {
			processor := contract_invocation.NewEventsProcessor(passphrase)
			events, err := processor.EventsFromLedger(ledger)
			if err != nil || len(events) == 0 {
				return nil, err
			}

			// Convert to proto - type-safe!
			return contract_invocation.ConvertToProto(events), nil
		},
	})
}
```

## Comparison: With vs Without Proto

### Without Proto (using structpb.Struct)

```go
// ❌ Hacky approach
func convertToProto(events []*Event) (proto.Message, error) {
	jsonData, err := json.Marshal(events)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	json.Unmarshal(jsonData, &data)

	return structpb.NewStruct(data)  // Loses type safety!
}
```

**Issues**:
- No compile-time validation
- JSON serialization overhead
- No schema documentation
- Hard to evolve format

### With Proto (proper approach)

```go
// ✅ Type-safe approach
func ConvertToProto(events []*Event) *stellarv1.ContractInvocationBatch {
	return &stellarv1.ContractInvocationBatch{
		Invocations: convertEvents(events),
		// Compiler enforces correct types!
	}
}
```

**Benefits**:
- ✅ Compile-time type checking
- ✅ Efficient binary encoding
- ✅ Self-documenting schema
- ✅ Easy to version and evolve

## When to Define Your Own Proto

Define your own proto schema when:

1. **Custom Processors**: You're building a processor not in Stellar SDK
   - ✅ Contract invocations
   - ✅ Liquidity pool analysis
   - ✅ Governance events
   - ✅ Custom business logic

2. **Wire Format Matters**: Events are transmitted or stored
   - ✅ gRPC streaming
   - ✅ Message queues
   - ✅ Long-term storage
   - ✅ Cross-service communication

3. **Interoperability**: Multiple consumers need the data
   - ✅ Different languages
   - ✅ External teams
   - ✅ Public API

## When You Can Skip Proto

You can skip proto if:

1. **Using Stellar's Existing Protos**: Token transfer already has proto
   ```go
   // Just use Stellar's proto
   import "github.com/stellar/go-stellar-sdk/protos"
   ```

2. **Internal-Only Processing**: Events never leave your service
   ```go
   // Process events in-memory only
   events := processor.EventsFromLedger(ledger)
   // Use events directly, no serialization needed
   ```

3. **Prototyping**: Early development phase
   ```go
   // Use structpb.Struct temporarily
   // But plan to migrate to proper proto
   ```

## Best Practices

### 1. Proto Versioning

Use version numbers in package name:
```protobuf
package stellar.v1;  // ✅ Version in package

// When breaking changes needed:
package stellar.v2;  // New version
```

### 2. Go Package Option

Always specify go_package:
```protobuf
option go_package = "github.com/withObsrvr/flow-proto/gen/stellar/v1;stellarv1";
```

### 3. Message Batching

Create batch messages for efficiency:
```protobuf
message ContractInvocationBatch {
  repeated ContractInvocationEvent invocations = 1;
  uint32 ledger_sequence = 2;
  int32 invocation_count = 3;
}
```

### 4. Metadata Fields

Include metadata in batch:
```protobuf
message Batch {
  repeated Event events = 1;
  uint32 ledger_sequence = 2;  // Where events came from
  int32 event_count = 3;         // Quick count
}
```

## Repository Structure

### Recommended Layout

```
flow-proto/                     (Separate proto repo)
└── proto/
    ├── flowctl/v1/            (Control plane protos)
    │   ├── consumer.proto
    │   ├── processor.proto
    │   └── source.proto
    └── stellar/v1/            (Stellar event protos)
        ├── raw_ledger.proto
        ├── token_transfer.proto
        └── contract_invocation.proto  ✅ Your addition

flowctl-sdk/                   (SDK repo)
└── pkg/stellar/processors/
    └── contract_invocation/
        ├── processor.go       (Go processor logic)
        ├── event.go          (Go event structures)
        ├── convert.go        (Proto conversion) ✅
        └── helpers.go        (Utilities)
```

### Why Separate Proto Repo?

1. **Shared Definition**: Multiple services can import
2. **Version Control**: Independent versioning
3. **Code Generation**: Generate for multiple languages
4. **Documentation**: Central source of truth

## Migration Path

If you started with `structpb.Struct`:

### Step 1: Define Proto
```protobuf
// proto/stellar/v1/your_event.proto
message YourEvent { ... }
```

### Step 2: Generate Code
```bash
protoc --go_out=. proto/stellar/v1/your_event.proto
```

### Step 3: Create Converter
```go
// pkg/stellar/processors/your_processor/convert.go
func ConvertToProto(events []*Event) *stellarv1.YourEventBatch
```

### Step 4: Update Example
```go
// Change from:
return structpb.NewStruct(jsonData)

// To:
return your_processor.ConvertToProto(events), nil
```

### Step 5: Test
```bash
go build -o processor
./processor
```

## Summary

**Yes, define your own protos** for custom Stellar processors:

1. ✅ **Follows Stellar's pattern** - Same as token_transfer
2. ✅ **Type safety** - Compile-time validation
3. ✅ **Performance** - Binary encoding
4. ✅ **Documentation** - Self-documenting schema
5. ✅ **Versioning** - Built-in evolution support
6. ✅ **Interoperability** - Cross-language support

The contract invocation processor now has:
- ✅ Proto schema: `/home/tillman/Documents/flow-proto/proto/stellar/v1/contract_invocation.proto`
- ✅ Generated code: `contract_invocation.pb.go`
- ✅ Converter: `/home/tillman/Documents/flowctl-sdk/pkg/stellar/processors/contract_invocation/convert.go`
- ✅ Working example: Successfully compiles and uses proper proto!

This is the correct, production-ready approach! 🎉
