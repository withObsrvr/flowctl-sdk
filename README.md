# Flowctl SDK

A comprehensive Go SDK for building [flowctl](https://github.com/withObsrvr/flowctl) data pipeline components with minimal boilerplate.

## Features

- **Three Complete SDKs**: Source, Processor, and Consumer
- **Event-First API**: Work with strongly-typed `*flowctlv1.Event` objects
- **Automatic Control Plane Integration**: Registration, heartbeats, and discovery
- **Historical Chunk Reporting**: Report bounded ledger work-unit progress, completion, verification, and failures
- **Built-in Observability**: Health checks, metrics, and logging
- **Production-Ready**: Graceful shutdown, error handling, and backpressure
- **Developer-Friendly**: 70-85% less code than manual gRPC implementation

## Installation

```bash
go get github.com/withObsrvr/flowctl-sdk
```

## Quick Start

### Processor Example

Build a processor that transforms events:

```go
package main

import (
    "context"
    "fmt"

    "github.com/withObsrvr/flowctl-sdk/pkg/processor"
    flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
)

func main() {
    proc, _ := processor.New(processor.DefaultConfig())

    proc.OnProcess(
        func(ctx context.Context, event *flowctlv1.Event) (*flowctlv1.Event, error) {
            // Transform the event
            return &flowctlv1.Event{
                Id:      fmt.Sprintf("%s-processed", event.Id),
                Type:    "example.processed.event",
                Payload: append(event.Payload, []byte(" - processed")...),
            }, nil
        },
        []string{"example.event"},         // Input types
        []string{"example.processed.event"}, // Output types
    )

    proc.Start(context.Background())
    waitForSignal()
    proc.Stop()
}
```

### Source Example

Build a source that produces events:

```go
import (
    "github.com/withObsrvr/flowctl-sdk/pkg/source"
    flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
)

func main() {
    src, _ := source.New(source.DefaultConfig())

    src.OnProduceStream(func(ctx context.Context, req *flowctlv1.StreamRequest) (*source.ProducerStream, error) {
        events := make(chan *flowctlv1.Event, 100)
        result := make(chan error, 1)
        go func() {
            defer close(events)
            for i := 0; i < 10; i++ { // bounded example
                event := &flowctlv1.Event{Id: fmt.Sprintf("event-%d", i), Type: "example.event"}
                select {
                case <-ctx.Done():
                    result <- ctx.Err()
                    return
                case events <- event:
                }
            }
            result <- nil // publish exactly once, before closing events
        }()
        return &source.ProducerStream{Events: events, Result: result, Bounded: true}, nil
    })

    src.Start(context.Background())
    waitForSignal()
    src.Stop()
}
```

`OnProduceStream` drains buffered events before reading the terminal result. Send
exactly one result into a capacity-one channel before closing `Events`. Always
select on `ctx.Done()` while producing, including when the buffer is full; a
client disconnect or SDK shutdown cancels that context. A bounded producer can
return `nil`; an unbounded producer must run until cancellation or report an
error. Legacy `OnProduce` retains its EOF-is-success behavior, is deprecated,
and is planned for removal in v2.0.0. Mixing the two handlers is rejected.
The existing `source.Source` interface remains unchanged for custom implementations;
`source.StreamSource` extends it with `OnProduceStream`.

Classify errors with `&source.StreamError{Kind: source.UpstreamUnavailable, Cause: err}`.
Kinds map to gRPC codes: `InvalidRequest` → `InvalidArgument`, `Canceled` →
`Canceled`, `UpstreamUnavailable` and `DownstreamUnavailable` → `Unavailable`,
`DataIntegrity` → `DataLoss`, `Internal` and unclassified errors → `Internal`.
Context deadlines map to `DeadlineExceeded`. Causes are logged locally; only
fixed, redacted messages reach clients. Non-cancellation terminal failures mark
source health `UNHEALTHY` (including `/ready` and `/live`).

### Consumer Example

Build a consumer that stores events:

```go
import (
    "github.com/withObsrvr/flowctl-sdk/pkg/consumer"
    flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
)

func main() {
    cons, _ := consumer.New(consumer.DefaultConfig())

    cons.OnConsume(func(ctx context.Context, event *flowctlv1.Event) error {
        // Store event in database
        return db.Insert(event)
    })

    cons.Start(context.Background())
    waitForSignal()
    cons.Stop()
}
```

## Integrating with Flowctl

Enable flowctl integration in your processor configuration:

```go
config := processor.DefaultConfig()
config.ID = "my-processor"
config.Name = "My Custom Processor"
config.Description = "Processes events in a custom way"
config.Endpoint = ":50052"

// Enable flowctl integration
config.FlowctlConfig.Enabled = true
config.FlowctlConfig.Endpoint = "flowctl-control-plane:8080"
config.FlowctlConfig.ServiceID = "my-processor-1" // Optional, will generate if not provided

proc, err := processor.New(config)
if err != nil {
    log.Fatalf("Failed to create processor: %v", err)
}
```

## Historical Chunk Reporting

For bounded historical workers, use the component reporter to emit flowctl chunk state:

```go
import (
    "github.com/withObsrvr/flowctl-sdk/pkg/component"
    flowctlpb "github.com/withobsrvr/flowctl/proto"
)

cfg := component.ConfigFromEnv()
reporter, err := component.NewReporter(ctx, cfg)
if err != nil {
    return err
}
defer reporter.Close()

_ = reporter.Register(ctx, flowctlpb.ServiceType_SERVICE_TYPE_SOURCE, map[string]string{
    "network": "pubnet",
})

_ = reporter.ReportChunkProgress(ctx, chunkStart, chunkEnd, "extract", nil, nil)
_ = reporter.ReportChunkCompleted(ctx, chunkStart, chunkEnd, true, map[string]int64{
    "ledgers": chunkEnd - chunkStart + 1,
}, map[string]string{"passed": "true"})
```

The reporter reads the standard `flowctl run` component environment contract:
`ENABLE_FLOWCTL`, `FLOWCTL_ENDPOINT`, `FLOWCTL_COMPONENT_ID`, `FLOWCTL_RUN_ID`,
`FLOWCTL_ATTEMPT`, and `FLOWCTL_HEARTBEAT_INTERVAL_MS`.

## Custom Metrics

Track custom metrics easily:

```go
// Add custom metrics
proc.Metrics().AddCounter("events_processed_by_type_x", 1)
proc.Metrics().AddGauge("queue_depth", 42.5)

// Get all metrics
metrics := proc.GetMetrics()
```

## Health Checks

The SDK automatically provides HTTP health check endpoints:

- `/health` - Simple health check
- `/ready` - Readiness probe (returns 200 when the processor is ready)
- `/live` - Liveness probe (returns 200 when the processor is running)
- `/metrics` - Returns current metrics as JSON

## Advanced Configuration

```go
config := processor.DefaultConfig()

// Basic configuration
config.ID = "custom-processor"
config.Name = "Custom Event Processor"
config.Description = "A processor that handles custom events"
config.Version = "1.2.3"
config.Endpoint = ":8888"
config.MaxConcurrent = 200

// Flowctl configuration
config.FlowctlConfig.Enabled = true
config.FlowctlConfig.Endpoint = "flowctl.example.com:8080"
config.FlowctlConfig.HeartbeatInterval = 15 * time.Second
config.FlowctlConfig.Metadata = map[string]string{
    "deployment": "production",
    "region": "us-west-2",
}

// Health check configuration
config.HealthPort = 9090
```

## Examples

Check out the [examples directory](./examples) for complete examples:

- [Basic Processor](./examples/basic_processor) - A simple processor example
- [Advanced Processor](./examples/advanced_processor) - More complex processor with custom handling logic

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.