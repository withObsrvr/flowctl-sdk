# Flowctl SDK - Complete Implementation

## Problem

Building flowctl components (sources, processors, consumers) currently requires:
- Writing 200+ lines of gRPC boilerplate
- Manually implementing control plane registration
- Custom health check and metrics setup
- Understanding complex proto streaming patterns
- Duplicating connection/lifecycle code across components

**Current Reality (Building a Processor):**
```go
// ~250 lines of boilerplate before writing actual business logic
- Set up gRPC server
- Implement ProcessorService interface
- Handle bidirectional streaming
- Parse events, marshal responses
- Connect to control plane manually
- Send heartbeats
- Implement health endpoints
- Track metrics
```

**Example from contract-events-processor:**
```go
// go/main.go - 300+ lines just for infrastructure
// Business logic (processing contract events) is only ~50 lines
// Ratio: 85% infrastructure, 15% domain logic
```

**Impact:**
- **High Friction**: Takes 2-3 days to build a new processor from scratch
- **Copy-Paste Errors**: Developers copy existing components, introduce bugs
- **Inconsistent Patterns**: Each component does registration/health differently
- **Hard to Onboard**: New contributors intimidated by complexity
- **Slow Iteration**: Want to test new processor idea? Spend a day on boilerplate first

**What We Want:**
```go
// 20 lines to build a processor
func main() {
    proc := processor.New(processor.DefaultConfig())

    proc.OnProcess(func(ctx context.Context, event *Event) (*Event, error) {
        // Just write business logic
        return processContractEvent(event), nil
    })

    proc.Start(context.Background())
}
```

**This reduces friction from days to minutes.**

## Appetite

**2 weeks** - Complete Source, Processor, and Consumer SDKs with working examples. This is focused implementation building on existing foundations.

## Solution

Build Terraform-quality SDKs that make writing flowctl components feel like writing simple functions. Hide all gRPC/infrastructure complexity, expose clean functional interfaces.

### Fat-Marker Sketch

```
SDK Architecture:

flowctl-sdk/
├── pkg/
│   ├── common/              # Shared utilities
│   │   ├── config.go        # Base configuration
│   │   ├── metrics.go       # Metrics tracking
│   │   └── health.go        # Health checks
│   │
│   ├── source/              # Source SDK
│   │   ├── source.go        # Source interface + impl
│   │   ├── config.go        # Source configuration
│   │   └── server.go        # gRPC server wrapper
│   │
│   ├── processor/           # Processor SDK (ENHANCE EXISTING)
│   │   ├── processor.go     # ✅ Already good structure
│   │   ├── config.go        # ✅ Already exists
│   │   └── handler.go       # ✅ Already exists
│   │
│   └── consumer/            # Consumer SDK
│       ├── consumer.go      # Consumer interface + impl
│       ├── config.go        # Consumer configuration
│       └── client.go        # gRPC client wrapper
│
└── examples/
    ├── stellar-ledger-source/     # Full Stellar source example
    ├── contract-processor/        # Full Stellar processor example
    └── postgres-consumer/         # Full consumer example
```

**Developer Experience:**

```go
// BEFORE: 300 lines of infrastructure + 50 lines of logic

// AFTER: Just the logic
func main() {
    src := source.New(source.Config{
        ID: "stellar-ledger-source",
        OutputEventTypes: []string{"stellar.ledger.v1"},
    })

    src.OnProduce(func(ctx context.Context, params map[string]string) (<-chan *Event, error) {
        ch := make(chan *Event, 100)
        go streamStellarLedgers(ctx, ch, params)
        return ch, nil
    })

    src.Start(context.Background())
}
```

### Key Design Decisions

**1. Function-Based API (Not Interfaces)**

Users provide functions, SDK handles infrastructure:
```go
// User writes simple function
type ProcessorFunc func(ctx context.Context, event *Event) (*Event, error)

// SDK wraps it in full gRPC server
processor.OnProcess(myProcessorFunc)
```

**Why?** Easier to understand, less boilerplate, feels natural to Go developers.

**2. Automatic Control Plane Integration**

SDK handles registration automatically:
```go
config := processor.DefaultConfig()
config.FlowctlConfig.Enabled = true  // That's it!

// SDK handles:
// - Component registration
// - Heartbeats
// - Service discovery
// - Graceful shutdown
```

**3. Channel-Based Event Streaming**

Sources produce events via channels (familiar Go pattern):
```go
src.OnProduce(func(ctx context.Context, params map[string]string) (<-chan *Event, error) {
    eventCh := make(chan *Event, 100)

    go func() {
        for ledger := startLedger; ; ledger++ {
            event := fetchAndConvert(ledger)
            eventCh <- event
        }
    }()

    return eventCh, nil  // SDK streams from channel to gRPC
})
```

**4. Built-in Observability**

Every SDK includes metrics, health, and logging:
```go
// Automatic metrics
proc.Metrics().IncrementProcessedCount()
proc.Metrics().RecordLatency(duration)

// Automatic health endpoints
// GET /health - 200 if healthy
// GET /metrics - JSON metrics

// Structured logging
proc.Logger().Info("Processing event", zap.String("type", event.Type))
```

## Rabbit Holes

**Don't** build a plugin system - we already decided gRPC services are the right architecture. SDK makes writing gRPC services easy.

**Don't** abstract away proto definitions - components should still import and use protos directly for strong typing.

**Don't** try to support every possible pattern - focus on the 80% case (simple function handlers). Advanced users can still use gRPC directly.

**Don't** build configuration UI/DSL - YAML config + environment variables is enough.

## No-Gos

- No breaking changes to existing proto interfaces
- No magic - users should understand what SDK does (just less typing)
- No hidden state - configuration should be explicit
- No vendor lock-in - components can drop SDK and use gRPC directly if needed
- No framework takeover - SDK is optional wrapper, not required runtime

## Done Looks Like

### Success Demo 1: Stellar Ledger Source (5 Minutes)

```bash
$ mkdir my-stellar-source && cd my-stellar-source
$ go mod init github.com/myorg/my-stellar-source
$ go get github.com/withObsrvr/flowctl-sdk
```

```go
// main.go - Complete working source in 30 lines
package main

import (
    "context"
    "github.com/withObsrvr/flowctl-sdk/pkg/source"
    flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
    stellarv1 "github.com/withObsrvr/flow-proto/go/gen/stellar/v1"
)

func main() {
    config := source.DefaultConfig()
    config.ID = "my-stellar-source"
    config.Name = "My Stellar Source"
    config.Endpoint = ":50051"
    config.FlowctlConfig.Enabled = true
    config.FlowctlConfig.Endpoint = "localhost:8080"

    src, _ := source.New(config)

    src.OnProduce(func(ctx context.Context, params map[string]string) (<-chan *flowctlv1.Event, error) {
        eventCh := make(chan *flowctlv1.Event, 100)
        startLedger := parseUint32(params["start_ledger"])

        go func() {
            defer close(eventCh)
            for ledger := startLedger; ; ledger++ {
                rawLedger := fetchLedgerFromHorizon(ledger)
                event := &flowctlv1.Event{
                    Type: "stellar.ledger.v1",
                    Payload: rawLedger.LedgerCloseMetaXdr,
                    Metadata: map[string]string{
                        "ledger_sequence": fmt.Sprintf("%d", ledger),
                    },
                    StellarCursor: &flowctlv1.StellarCursor{
                        LedgerSequence: uint64(ledger),
                    },
                }
                select {
                case <-ctx.Done():
                    return
                case eventCh <- event:
                }
            }
        }()

        return eventCh, nil
    })

    src.Start(context.Background())
    waitForSignal()
}
```

```bash
$ go run main.go
✓ Stellar source started on :50051
✓ Registered with control plane
✓ Health endpoint: http://localhost:8088/health
✓ Streaming ledgers starting from 1000...
```

### Success Demo 2: Contract Events Processor (10 Minutes)

```go
// Migrate existing contract-events-processor to SDK
// BEFORE: 300 lines
// AFTER: 50 lines

func main() {
    proc := processor.New(processor.DefaultConfig())

    proc.OnProcess(func(ctx context.Context, event *flowctlv1.Event) (*flowctlv1.Event, error) {
        // Existing business logic unchanged
        if event.Type != "stellar.ledger.v1" {
            return nil, nil  // Skip
        }

        contractEvents := extractContractEvents(event.Payload)

        return &flowctlv1.Event{
            Type: "contract.event.v1",
            Payload: marshalContractEvents(contractEvents),
        }, nil
    })

    proc.Start(context.Background())
}
```

### Success Demo 3: PostgreSQL Consumer (15 Minutes)

```go
package main

import (
    "database/sql"
    "github.com/withObsrvr/flowctl-sdk/pkg/consumer"
)

func main() {
    db, _ := sql.Open("postgres", os.Getenv("DATABASE_URL"))

    config := consumer.DefaultConfig()
    config.ID = "postgres-consumer"
    config.InputEventTypes = []string{"contract.event.v1"}

    cons, _ := consumer.New(config)

    cons.OnConsume(func(ctx context.Context, event *flowctlv1.Event) error {
        switch event.Type {
        case "contract.event.v1":
            return insertContractEvent(db, event)
        default:
            return fmt.Errorf("unknown event type: %s", event.Type)
        }
    })

    cons.Start(context.Background())
}

func insertContractEvent(db *sql.DB, event *flowctlv1.Event) error {
    _, err := db.Exec(
        "INSERT INTO contract_events (event_id, ledger, contract_id, data) VALUES ($1, $2, $3, $4)",
        event.Id,
        event.Metadata["ledger_sequence"],
        event.Metadata["contract_id"],
        event.Payload,
    )
    return err
}
```

### Concrete Acceptance Criteria

**Processor SDK:**
1. ✅ Uncomment all gRPC server registration code (currently commented out)
2. ✅ Implement bidirectional streaming using flow-proto definitions
3. ✅ Automatic control plane registration when enabled
4. ✅ Example: Migrate contract-events-processor to SDK (<100 LOC)

**Source SDK:**
1. ✅ Create source.go with Source interface and StandardSource implementation
2. ✅ Channel-based event production (user returns <-chan *Event)
3. ✅ Implement SourceService gRPC server with StreamEvents RPC
4. ✅ Example: Working stellar-ledger-source (<100 LOC)

**Consumer SDK:**
1. ✅ Create consumer.go with Consumer interface
2. ✅ Function-based event handling (OnConsume callback)
3. ✅ Implement ConsumerService gRPC client + server
4. ✅ Example: Working postgres-consumer (<100 LOC)

**Documentation:**
1. ✅ README with quickstart for each SDK (source, processor, consumer)
2. ✅ Migration guide from manual gRPC → SDK
3. ✅ API reference for all public interfaces
4. ✅ Best practices guide

**Testing:**
1. ✅ Integration test: source → processor → consumer pipeline working end-to-end
2. ✅ Control plane integration test
3. ✅ Verify health endpoints work
4. ✅ Verify metrics collection works

## Scope Line

```
══════════════════ MUST HAVE ══════════════════
Week 1:
- Complete Processor SDK (uncomment gRPC, use flow-proto)
- Create Source SDK (channel-based streaming)
- Create Consumer SDK (callback-based consumption)
- All SDKs have control plane integration
- All SDKs have health + metrics built-in

Week 2:
- Example: stellar-ledger-source
- Example: contract-events-processor
- Example: postgres-consumer
- End-to-end integration test
- Documentation: Quickstart + API reference

─────────────────── NICE TO HAVE ──────────────────
- Migration examples for existing ttp-processor-demo components
- Performance benchmarks (SDK vs manual gRPC)
- Dockerfile templates for each SDK type
- GitHub Action templates for building components
- Structured logging with configurable levels

─────────────────── COULD HAVE ────────────────────
- Python SDK (same patterns, different language)
- Rust SDK (for performance-critical components)
- Component testing framework (mock control plane)
- Auto-generated OpenAPI specs from protos
- Distributed tracing integration (OpenTelemetry)
```

## Implementation Plan

### Week 1: Core SDK Implementation

**Day 1-2: Complete Processor SDK**

Update `flowctl-sdk/pkg/processor/processor.go`:

```go
import (
    flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
)

// Implement gRPC server methods
func (p *StandardProcessor) Process(stream flowctlv1.ProcessorService_ProcessServer) error {
    // UNCOMMENT AND IMPLEMENT
    for {
        event, err := stream.Recv()
        if err == io.EOF {
            return nil
        }

        // Find handler
        handlers := p.registry.GetHandlersForType(event.Type)

        // Process
        output, err := handlers[0].Handle(ctx, event)

        // Send
        stream.Send(output)
    }
}

func (p *StandardProcessor) GetInfo(ctx context.Context, _ *emptypb.Empty) (*flowctlv1.ComponentInfo, error) {
    // UNCOMMENT AND IMPLEMENT
    return &flowctlv1.ComponentInfo{
        Id: p.config.ID,
        Type: flowctlv1.ComponentType_COMPONENT_TYPE_PROCESSOR,
        // ...
    }, nil
}
```

**Day 3-4: Build Source SDK**

Create `flowctl-sdk/pkg/source/source.go`:

```go
package source

type EventProducer func(ctx context.Context, params map[string]string) (<-chan *flowctlv1.Event, error)

type Source interface {
    Start(ctx context.Context) error
    Stop() error
    OnProduce(producer EventProducer) error
}

type StandardSource struct {
    config *Config
    producer EventProducer
    server *grpc.Server
    // ... (mirrors StandardProcessor structure)
}

// StreamEvents implements gRPC service
func (s *StandardSource) StreamEvents(req *flowctlv1.StreamRequest, stream flowctlv1.SourceService_StreamEventsServer) error {
    eventCh, err := s.producer(stream.Context(), req.Params)

    for {
        select {
        case <-stream.Context().Done():
            return stream.Context().Err()
        case event, ok := <-eventCh:
            if !ok {
                return nil
            }
            if err := stream.Send(event); err != nil {
                return err
            }
            s.metrics.IncrementProcessedCount()
        }
    }
}
```

**Day 5: Build Consumer SDK**

Create `flowctl-sdk/pkg/consumer/consumer.go`:

```go
package consumer

type EventHandler func(ctx context.Context, event *flowctlv1.Event) error

type Consumer interface {
    Start(ctx context.Context) error
    Stop() error
    OnConsume(handler EventHandler) error
}

type StandardConsumer struct {
    config *Config
    handler EventHandler
    server *grpc.Server
    // ... (mirrors StandardProcessor structure)
}

// Consume implements gRPC service
func (c *StandardConsumer) Consume(stream flowctlv1.ConsumerService_ConsumeServer) error {
    for {
        event, err := stream.Recv()
        if err == io.EOF {
            return stream.SendAndClose(&flowctlv1.ConsumeResponse{
                EventsConsumed: c.metrics.ProcessedCount(),
            })
        }

        if err := c.handler(stream.Context(), event); err != nil {
            c.metrics.IncrementErrorCount()
        } else {
            c.metrics.IncrementSuccessCount()
        }
    }
}
```

### Week 2: Examples & Documentation

**Day 1-2: Build Examples**

```bash
examples/
├── stellar-ledger-source/
│   ├── main.go              # 50 lines
│   ├── Dockerfile
│   └── README.md
├── contract-processor/
│   ├── main.go              # 50 lines
│   ├── Dockerfile
│   └── README.md
└── postgres-consumer/
    ├── main.go              # 60 lines
    ├── Dockerfile
    └── README.md
```

**Day 3: Integration Testing**

```go
// examples/integration_test/pipeline_test.go
func TestFullPipeline(t *testing.T) {
    // Start control plane
    cp := startMockControlPlane(t)

    // Start source
    src := startStellarSource(t, cp.Endpoint)

    // Start processor
    proc := startContractProcessor(t, cp.Endpoint)

    // Start consumer
    cons := startPostgresConsumer(t, cp.Endpoint)

    // Verify data flows through pipeline
    waitForEvents(t, cons, 100)
}
```

**Day 4-5: Documentation**

- README.md: Quickstart for each SDK
- GUIDE.md: Detailed SDK usage guide
- MIGRATION.md: Migrating from manual gRPC
- API.md: Full API reference

## Technical Details

### Control Plane Integration Pattern

All SDKs follow same pattern:

```go
// In StandardProcessor/StandardSource/StandardConsumer
func (p *Standard*) Start(ctx context.Context) error {
    // 1. Start health server
    p.health.Start()

    // 2. Register with control plane (if enabled)
    if p.controller != nil {
        p.controller.Register(ctx)
        p.controller.Start(ctx)  // Starts heartbeat loop
    }

    // 3. Start gRPC server
    p.server = grpc.NewServer()
    *pb.Register*ServiceServer(p.server, p)
    go p.server.Serve(lis)

    return nil
}
```

### Metrics Interface

All SDKs provide consistent metrics:

```go
type Metrics interface {
    IncrementProcessedCount()
    IncrementSuccessCount()
    IncrementErrorCount()
    RecordProcessingLatency(ms float64)

    ProcessedCount() int64
    SuccessCount() int64
    ErrorCount() int64

    GetMetrics() map[string]interface{}
}
```

### Configuration Pattern

All SDKs use similar config structure:

```go
type Config struct {
    // Component identity
    ID          string
    Name        string
    Description string
    Version     string

    // Networking
    Endpoint   string  // gRPC endpoint
    HealthPort int     // Health check port

    // Control plane
    FlowctlConfig *flowctl.Config

    // Type-specific
    // For Source: OutputEventTypes []string
    // For Processor: InputEventTypes, OutputEventTypes []string
    // For Consumer: InputEventTypes []string
}
```

## Migration Path for Existing Components

### Step-by-Step Migration

**Before (contract-events-processor/go/main.go):**
```go
// 300+ lines of boilerplate
func main() {
    // Manual gRPC server setup
    lis, _ := net.Listen("tcp", ":50053")
    server := grpc.NewServer()

    // Manual service implementation
    svc := &contractEventService{...}
    pb.RegisterContractEventServiceServer(server, svc)

    // Manual control plane registration
    conn, _ := grpc.Dial(controlPlaneAddr)
    client := controlpb.NewControlPlaneClient(conn)
    client.RegisterComponent(...)

    // Manual heartbeat loop
    go func() {
        ticker := time.NewTicker(10 * time.Second)
        for range ticker.C {
            client.Heartbeat(...)
        }
    }()

    // Manual health server
    http.HandleFunc("/health", healthHandler)
    go http.ListenAndServe(":8088", nil)

    server.Serve(lis)
}
```

**After (using SDK):**
```go
// 50 lines total
func main() {
    proc := processor.New(processor.DefaultConfig())

    proc.OnProcess(func(ctx context.Context, event *flowctlv1.Event) (*flowctlv1.Event, error) {
        // Same business logic, just moved here
        return extractContractEvents(event)
    })

    proc.Start(context.Background())
}
```

**Lines of Code Reduction:**
- contract-events-processor: 300 → 50 (83% reduction)
- stellar-live-source-datalake: 400 → 80 (80% reduction)
- postgres-consumer: 250 → 60 (76% reduction)

## Success Metrics

1. **Developer Velocity**: New component from idea → working in <1 hour (vs 2-3 days)
2. **Code Reduction**: SDK-based components are 70-85% less code than manual
3. **Consistency**: All components use same patterns (registration, health, metrics)
4. **Onboarding**: New contributor can build first component in <30 minutes
5. **Adoption**: Migrate 3+ existing ttp-processor-demo components to SDK

## Dependencies

**Critical Path:**
- Depends on flow-proto modernization (need clean proto definitions)
- Processor SDK depends on flow-proto v1 being published

**Can Work in Parallel:**
- Source and Consumer SDKs are independent
- Examples can use local proto generation during development

## References

- Current processor SDK: `/home/tillman/Documents/flowctl-sdk/pkg/processor/processor.go`
- Example component to migrate: `/home/tillman/Documents/ttp-processor-demo/contract-events-processor/`
- Proto definitions: `/home/tillman/Documents/flow-proto/proto/`
- Terraform Plugin SDK (inspiration): https://developer.hashicorp.com/terraform/plugin/framework
