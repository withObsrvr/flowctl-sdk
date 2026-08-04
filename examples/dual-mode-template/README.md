# Dual-Mode Processor Template

This template demonstrates a processor that works in **both** nebu and flowctl modes:

- **Nebu mode**: Unix pipes, JSON output, rapid prototyping
- **Flowctl mode**: gRPC, binary protobuf, production deployment

## How It Works

The processor detects the runtime environment and switches modes:

```go
if os.Getenv("FLOWCTL_ENDPOINT") != "" || os.Getenv("ENABLE_FLOWCTL") == "true" {
    runFlowctlMode()  // Production: gRPC + control plane
} else {
    runNebuMode()     // Development: Unix pipes + JSON
}
```

## Project Structure

```
dual-mode-template/
├── main.go           # Entry point with mode detection
├── processor.go      # Core extraction logic (shared)
├── nebu.go           # Nebu-specific wrapper
├── flowctl.go        # Flowctl-specific wrapper
├── proto/
│   ├── events.proto  # Protobuf schema
│   └── events.pb.go  # Generated Go code
├── go.mod
└── README.md
```

## Building

```bash
# Generate proto (if modified)
protoc --go_out=. --go_opt=paths=source_relative proto/events.proto

# Build the processor
go build -o bin/dual-mode-processor
```

## Running in Nebu Mode

```bash
# With nebu fetch
nebu fetch 60000000 60000001 | ./bin/dual-mode-processor

# Standalone with XDR file
cat ledger.xdr | ./bin/dual-mode-processor
```

Output: JSON to stdout (one event per line)

## Running in Flowctl Mode

```bash
# Set environment variables
export ENABLE_FLOWCTL=true
export FLOWCTL_ENDPOINT=localhost:8080
export NETWORK_PASSPHRASE="Public Global Stellar Network ; September 2015"

# Run the processor
./bin/dual-mode-processor
```

Or use in a pipeline:

```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: dual-mode-pipeline

spec:
  driver: process

  sources:
    - id: stellar-source
      command: ["stellar-ledger-source"]
      env:
        START_LEDGER: "60000000"

  processors:
    - id: dual-mode-processor
      command: ["./bin/dual-mode-processor"]
      inputs: ["stellar-source"]
      env:
        ENABLE_FLOWCTL: "true"
        NETWORK_PASSPHRASE: "Public Global Stellar Network ; September 2015"
```

## Key Patterns

### Shared Core Logic

The `EventsFromLedger()` function contains all extraction logic and is used by both modes:

```go
// processor.go
func EventsFromLedger(networkPassphrase string, ledger xdr.LedgerCloseMeta) ([]*proto.MyEvent, error) {
    // Core extraction logic - identical for both modes
    // ...
}
```

### Mode-Specific Wrappers

**Nebu mode** (`nebu.go`):
```go
func runNebuMode() {
    cli.RunProtoOriginCLI(config, func(networkPass string) cli.ProtoOriginProcessor[*proto.MyEvent] {
        return NewNebuOrigin(networkPass)
    })
}
```

**Flowctl mode** (`flowctl.go`):
```go
func runFlowctlMode() {
    stellar.Run(stellar.ProcessorConfig{
        ProcessorName: "My Processor",
        OutputType:    "myorg.events.v1",
        ProcessLedger: func(passphrase string, ledger xdr.LedgerCloseMeta) (proto.Message, error) {
            events, err := EventsFromLedger(passphrase, ledger)
            if err != nil {
                return nil, err
            }
            return &proto.MyEventBatch{Events: events}, nil
        },
    })
}
```

## Testing

```bash
# Test nebu mode
nebu fetch 60000000 60000001 | ./bin/dual-mode-processor | jq .

# Test flowctl mode
ENABLE_FLOWCTL=true ./bin/dual-mode-processor &
curl http://localhost:8088/health
```

## Adapting This Template

1. Replace `proto/events.proto` with your event schema
2. Implement your extraction logic in `processor.go`
3. Update configuration in `main.go`
4. Build and test in both modes
