# Flowctl-SDK Quickstart

**Build a production-ready Stellar processor in 5 minutes.**

This guide walks you through creating your first flowctl processor using the proto-first approach.

## Table of Contents

- [Prerequisites](#prerequisites)
- [The Stellar Pattern](#the-stellar-pattern)
- [5-Minute Hello World](#5-minute-hello-world)
- [Understanding the Code](#understanding-the-code)
- [Configuration Options](#configuration-options)
- [Control Plane Integration](#control-plane-integration)
- [Building and Running](#building-and-running)
- [Next Steps](#next-steps)

---

## Prerequisites

### Required Tools

```bash
# Go 1.21+
go version  # Should show go1.21 or higher

# Protocol Buffers (for custom protos)
protoc --version  # Optional, only if creating new proto types

# flowctl CLI (for pipeline orchestration)
flowctl version
```

### Get the SDK

```bash
go get github.com/withObsrvr/flowctl-sdk
go get github.com/withObsrvr/flow-proto
```

---

## The Stellar Pattern

All Stellar processors in the flowctl ecosystem follow the same pattern, inspired by Stellar's official `token_transfer` processor:

```go
// The Stellar Pattern: stateless event extraction
func EventsFromLedger(networkPassphrase string, ledger xdr.LedgerCloseMeta) (proto.Message, error) {
    // 1. Iterate through transactions
    // 2. Extract relevant events
    // 3. Return as proto message
}
```

This pattern provides:

- **Stateless processing**: Each ledger is processed independently
- **Type safety**: Proto messages provide compile-time validation
- **Composability**: Processors can be chained in pipelines
- **Testability**: Pure functions are easy to unit test

---

## 5-Minute Hello World

Let's create a processor that extracts payment operations from Stellar ledgers.

### Step 1: Create Project Structure

```bash
mkdir payment-processor && cd payment-processor
go mod init github.com/yourorg/payment-processor
```

### Step 2: Define Your Proto (Optional)

If using existing flow-proto types, skip this step. For custom types:

```protobuf
// proto/payment.proto
syntax = "proto3";
package payment.v1;
option go_package = "github.com/yourorg/payment-processor/proto";

message PaymentEvent {
  string from = 1;
  string to = 2;
  string amount = 3;
  string asset_code = 4;
  string asset_issuer = 5;
  EventMeta meta = 6;
}

message EventMeta {
  uint32 ledger_sequence = 1;
  string tx_hash = 2;
  int64 ledger_close_time = 3;
}

message PaymentBatch {
  repeated PaymentEvent events = 1;
}
```

Generate Go code:

```bash
mkdir -p proto
protoc --go_out=. --go_opt=paths=source_relative proto/payment.proto
```

### Step 3: Write the Processor

```go
// main.go
package main

import (
    "github.com/stellar/go/xdr"
    "github.com/withObsrvr/flowctl-sdk/pkg/stellar"
    "google.golang.org/protobuf/proto"

    paymentpb "github.com/yourorg/payment-processor/proto"
)

func main() {
    stellar.Run(stellar.ProcessorConfig{
        ProcessorName: "Payment Processor",
        OutputType:    "stellar.payment.v1",
        ProcessLedger: EventsFromLedger,
    })
}

// EventsFromLedger extracts payment events from a ledger
func EventsFromLedger(networkPassphrase string, ledger xdr.LedgerCloseMeta) (proto.Message, error) {
    var events []*paymentpb.PaymentEvent

    ledgerSeq := ledger.LedgerSequence()
    closeTime := int64(ledger.LedgerHeaderHistoryEntry().Header.ScpValue.CloseTime)

    // Iterate through all transactions
    for _, tx := range ledger.TransactionsWithMeta() {
        txHash := tx.TransactionHash()

        // Check each operation
        for _, op := range tx.Operations() {
            // Only process Payment operations
            if op.Body.Type != xdr.OperationTypePayment {
                continue
            }

            payment := op.Body.MustPaymentOp()

            // Extract event
            event := &paymentpb.PaymentEvent{
                From:        op.SourceAccount.Address(),
                To:          payment.Destination.Address(),
                Amount:      formatAmount(payment.Amount),
                AssetCode:   getAssetCode(payment.Asset),
                AssetIssuer: getAssetIssuer(payment.Asset),
                Meta: &paymentpb.EventMeta{
                    LedgerSequence:  ledgerSeq,
                    TxHash:          txHash.HexString(),
                    LedgerCloseTime: closeTime,
                },
            }

            events = append(events, event)
        }
    }

    // Return nil if no events (no output)
    if len(events) == 0 {
        return nil, nil
    }

    return &paymentpb.PaymentBatch{Events: events}, nil
}

// Helper functions
func formatAmount(amount xdr.Int64) string {
    return fmt.Sprintf("%d", amount)
}

func getAssetCode(asset xdr.Asset) string {
    switch asset.Type {
    case xdr.AssetTypeAssetTypeNative:
        return "XLM"
    case xdr.AssetTypeAssetTypeCreditAlphanum4:
        return string(asset.AlphaNum4.AssetCode[:])
    case xdr.AssetTypeAssetTypeCreditAlphanum12:
        return string(asset.AlphaNum12.AssetCode[:])
    }
    return ""
}

func getAssetIssuer(asset xdr.Asset) string {
    switch asset.Type {
    case xdr.AssetTypeAssetTypeNative:
        return ""
    case xdr.AssetTypeAssetTypeCreditAlphanum4:
        return asset.AlphaNum4.Issuer.Address()
    case xdr.AssetTypeAssetTypeCreditAlphanum12:
        return asset.AlphaNum12.Issuer.Address()
    }
    return ""
}
```

### Step 4: Build and Run

```bash
# Build
go build -o bin/payment-processor

# Run standalone (for testing)
NETWORK_PASSPHRASE="Test SDF Network ; September 2015" \
PORT=":50051" \
./bin/payment-processor
```

That's it! You have a working flowctl processor.

---

## Understanding the Code

### The stellar.Run() Function

The `stellar.Run()` function handles all the complexity:

```go
stellar.Run(stellar.ProcessorConfig{
    ProcessorName: "Payment Processor",     // Human-readable name
    OutputType:    "stellar.payment.v1",    // Event type identifier
    ProcessLedger: EventsFromLedger,        // Your extraction function
})
```

**What it handles for you:**

- gRPC server setup and lifecycle
- Control plane registration and heartbeats
- Health checks (HTTP endpoints)
- Configuration loading (YAML + env vars)
- Graceful shutdown
- Input event parsing (stellar.ledger.v1)
- Output event wrapping

### The ProcessLedger Function

Your extraction logic lives in a single function:

```go
func EventsFromLedger(networkPassphrase string, ledger xdr.LedgerCloseMeta) (proto.Message, error)
```

**Inputs:**
- `networkPassphrase`: The Stellar network (mainnet, testnet)
- `ledger`: The decoded ledger close meta (XDR)

**Outputs:**
- `proto.Message`: Your event batch (or `nil` to skip)
- `error`: Any error (will be logged, processing continues)

### Event Flow

```
Source                       Your Processor               Downstream
───────                     ──────────────               ──────────

RawLedger                   ┌─────────────────┐
(XDR bytes)   ─────────────▶│ EventsFromLedger │         PaymentBatch
              stellar.ledger│  - Decode XDR    │────────▶ (proto bytes)
                 .v1        │  - Extract events│          stellar.payment.v1
                            │  - Return batch  │
                            └─────────────────┘
```

---

## Configuration Options

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NETWORK_PASSPHRASE` | Required | Stellar network passphrase |
| `COMPONENT_ID` | Derived from name | Unique component identifier |
| `PORT` | `:50051` | gRPC server port |
| `HEALTH_PORT` | `8088` | Health check HTTP port |
| `ENABLE_FLOWCTL` | `false` | Enable control plane integration |
| `FLOWCTL_ENDPOINT` | `localhost:8080` | Control plane address |

### Configuration File (Optional)

Create `processor.yaml` for persistent configuration:

```yaml
processor:
  name: "Payment Processor"
  description: "Extracts payment operations from Stellar ledgers"
  version: "1.0.0"
  input: "stellar.ledger.v1"
  output: "stellar.payment.v1"

network:
  passphrase: "Public Global Stellar Network ; September 2015"

flowctl:
  enabled: true
  endpoint: "localhost:8080"
  heartbeat_interval: 10000
```

---

## Control Plane Integration

When running as part of a flowctl pipeline, your processor automatically:

### 1. Registers on Startup

```
[INFO] Registered with flowctl control plane
[INFO] Component ID: payment-processor
[INFO] Endpoint: :50051
```

### 2. Sends Heartbeats

```
[DEBUG] Heartbeat sent (status: healthy)
```

### 3. Provides Health Endpoints

```bash
# Health check
curl http://localhost:8088/health
# {"status": "healthy"}

# Readiness probe
curl http://localhost:8088/ready
# {"status": "ready"}

# Liveness probe
curl http://localhost:8088/live
# {"status": "alive"}
```

### 4. Handles Graceful Shutdown

```
[INFO] Received SIGTERM, shutting down...
[INFO] Deregistered from control plane
[INFO] Processor stopped successfully
```

---

## Building and Running

### Local Development

```bash
# Build
go build -o bin/payment-processor

# Run standalone
NETWORK_PASSPHRASE="Test SDF Network ; September 2015" \
./bin/payment-processor
```

### With Flowctl Pipeline

Create `pipeline.yaml`:

```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: payment-pipeline

spec:
  driver: process

  sources:
    - id: stellar-source
      command: ["stellar-ledger-source"]
      env:
        NETWORK_PASSPHRASE: "Test SDF Network ; September 2015"
        START_LEDGER: "1000"

  processors:
    - id: payment-processor
      command: ["./bin/payment-processor"]
      inputs: ["stellar-source"]
      env:
        NETWORK_PASSPHRASE: "Test SDF Network ; September 2015"

  sinks:
    - id: stdout-sink
      command: ["event-logger"]
      inputs: ["payment-processor"]
```

Run:

```bash
flowctl run pipeline.yaml
```

### Docker Deployment

```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o payment-processor

FROM alpine:3.18
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/payment-processor .
ENTRYPOINT ["./payment-processor"]
```

```bash
docker build -t payment-processor:latest .
docker run -e NETWORK_PASSPHRASE="Test SDF Network ; September 2015" payment-processor:latest
```

---

## Next Steps

### Learn More

1. **Proto-First Development**: [nebu/BUILDING_PROTO_PROCESSORS.md](https://github.com/withObsrvr/nebu-processor-registry/blob/main/BUILDING_PROTO_PROCESSORS.md) - Deep dive into protobuf patterns
2. **Building Components**: [flowctl/docs/building-components.md](https://github.com/withObsrvr/flowctl/blob/main/docs/building-components.md) - Complete component guide
3. **Migrating from Nebu**: [GRADUATING_TO_FLOWCTL.md](https://github.com/withObsrvr/nebu-processor-registry/blob/main/docs/GRADUATING_TO_FLOWCTL.md) - Migration guide

### Study Examples

- **Contract Events Processor**: `flowctl-sdk/examples/contract-events-processor/`
- **Token Transfer Helper**: `flowctl-sdk/pkg/stellar/helpers/token_transfer.go`
- **PostgreSQL Consumer**: `flowctl-sdk/examples/postgresql-consumer/`
- **Dual-Mode Template**: `flowctl-sdk/examples/dual-mode-template/`

### Reference Implementation

The official Stellar `token_transfer` processor is the gold standard:

```go
// From github.com/stellar/go-stellar-sdk/processors/token_transfer
processor := token_transfer.NewEventsProcessor(networkPassphrase)
events, err := processor.EventsFromLedger(ledger)
```

This pattern (`EventsFromLedger`) is what flowctl-sdk builds upon.

---

## Common Patterns

### Filtering Events

```go
func EventsFromLedger(networkPassphrase string, ledger xdr.LedgerCloseMeta) (proto.Message, error) {
    var events []*mypb.Event

    for _, tx := range ledger.TransactionsWithMeta() {
        // Skip failed transactions
        if !tx.Result.Successful() {
            continue
        }

        // Process operations...
    }

    return &mypb.EventBatch{Events: events}, nil
}
```

### Error Handling

```go
func EventsFromLedger(networkPassphrase string, ledger xdr.LedgerCloseMeta) (proto.Message, error) {
    events, err := extractEvents(ledger)
    if err != nil {
        // Log error but continue processing
        log.Printf("Warning: failed to extract from ledger %d: %v", ledger.LedgerSequence(), err)
        return nil, nil  // Return nil to skip this ledger
    }

    return &mypb.EventBatch{Events: events}, nil
}
```

### Batch Processing

```go
func EventsFromLedger(networkPassphrase string, ledger xdr.LedgerCloseMeta) (proto.Message, error) {
    batch := &mypb.EventBatch{
        LedgerSequence: ledger.LedgerSequence(),
        EventCount:     0,
    }

    for _, tx := range ledger.TransactionsWithMeta() {
        events := extractFromTx(tx)
        batch.Events = append(batch.Events, events...)
        batch.EventCount += uint32(len(events))
    }

    return batch, nil
}
```

---

## Troubleshooting

### Processor Won't Start

```bash
# Check port availability
lsof -i :50051

# Check environment
echo $NETWORK_PASSPHRASE

# Enable debug logging
LOG_LEVEL=debug ./bin/payment-processor
```

### Not Receiving Events

```bash
# Verify source is producing ledgers
flowctl status

# Check event type matches
# Source must output: stellar.ledger.v1
# Processor must accept: stellar.ledger.v1
```

### Control Plane Connection Failed

```bash
# Check control plane is running
curl http://localhost:8080/health

# Verify endpoint configuration
echo $FLOWCTL_ENDPOINT
```

---

## Resources

- **flowctl-sdk Repository**: https://github.com/withObsrvr/flowctl-sdk
- **flowctl Documentation**: https://github.com/withObsrvr/flowctl
- **flow-proto Types**: https://github.com/withObsrvr/flow-proto
- **Stellar Go SDK**: https://github.com/stellar/go

---

## Getting Help

- **SDK Questions**: https://github.com/withObsrvr/flowctl-sdk/discussions
- **Bug Reports**: https://github.com/withObsrvr/flowctl-sdk/issues
- **Flowctl Issues**: https://github.com/withObsrvr/flowctl/issues
