# Flowctl Processor SDK

A Go-based SDK for simplifying the development of [flowctl](https://github.com/withObsrvr/flowctl) processors.

## Features

- Streamlined processor creation with minimal boilerplate
- Automatic flowctl control plane integration
- Built-in health checks and metrics
- Standardized event handling patterns
- Graceful startup and shutdown

## Installation

```bash
go get github.com/withObsrvr/flowctl-sdk
```

## Quick Start

Create a simple processor:

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/withObsrvr/flowctl-sdk/pkg/processor"
)

func main() {
    // Create a processor with default configuration
    proc, err := processor.New(processor.DefaultConfig())
    if err != nil {
        log.Fatalf("Failed to create processor: %v", err)
    }

    // Register a handler for processing events
    proc.OnProcess(
        // Handler function
        func(ctx context.Context, input []byte, metadata map[string]string) ([]byte, map[string]string, error) {
            // Process the event
            output := append(input, []byte(" - processed")...)
            
            // Update metrics
            proc.Metrics().IncrementProcessedCount()
            proc.Metrics().IncrementSuccessCount()
            
            return output, metadata, nil
        },
        // Input types
        []string{"example.event"},
        // Output types
        []string{"example.processed.event"},
    )

    // Start the processor
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    if err := proc.Start(ctx); err != nil {
        log.Fatalf("Failed to start processor: %v", err)
    }

    // Wait for shutdown signal
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh

    // Graceful shutdown
    if err := proc.Stop(); err != nil {
        log.Printf("Error stopping processor: %v", err)
    }
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