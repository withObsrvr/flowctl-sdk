# DuckDB Consumer Example

A simple example demonstrating how to build a consumer that stores events in a database using the flowctl SDK.

## What This Does

This consumer:
- Accepts events via gRPC streaming
- Handles multiple event types (stellar.ledger.v1, contract.event.v1)
- Processes events concurrently with configurable limit
- Stores events in a database (mock implementation, easily replaced with real DuckDB)
- Automatic control plane integration
- Built-in health checks and metrics

## Running the Example

```bash
# Build
go build -o duckdb-consumer .

# Run locally (no control plane)
./duckdb-consumer

# Run with control plane integration
FLOWCTL_ENDPOINT=localhost:8080 ./duckdb-consumer
```

## How It Works

### 1. Create Consumer with Config

```go
config := consumer.DefaultConfig()
config.ID = "duckdb-consumer"
config.InputEventTypes = []string{"stellar.ledger.v1", "contract.event.v1"}
config.MaxConcurrent = 10 // Process up to 10 events concurrently

cons, _ := consumer.New(config)
```

### 2. Register Handler Function

The handler function:
- Receives one event at a time
- Returns an error if processing fails
- SDK automatically tracks metrics

```go
cons.OnConsume(func(ctx context.Context, event *flowctlv1.Event) error {
    switch event.Type {
    case "stellar.ledger.v1":
        return storeLedgerInDB(db, event)
    case "contract.event.v1":
        return storeContractEventInDB(db, event)
    default:
        return nil // Skip unknown events
    }
})
```

### 3. Start the Consumer

```go
cons.Start(context.Background())
// SDK handles:
// - gRPC server startup
// - Control plane registration
// - Automatic heartbeats
// - Health endpoints
// - Metrics collection
// - Concurrent event processing
```

## Testing with grpcurl

The consumer implements the ConsumerService interface, so you can test it directly:

```bash
# Get consumer info
grpcurl -plaintext localhost:50053 flowctl.v1.ConsumerService/GetInfo

# Health check
curl http://localhost:8088/health

# Metrics
curl http://localhost:8088/metrics
```

To test end-to-end, you need a source or processor to send events. See the `stellar-ledger-source` example.

## Real-World Implementation with DuckDB

For a production consumer with DuckDB:

```go
import "database/sql"
import _ "github.com/marcboeker/go-duckdb"

// Initialize DuckDB
db, _ := sql.Open("duckdb", "./events.duckdb")

// Create table
db.Exec(`
    CREATE TABLE IF NOT EXISTS events (
        id VARCHAR PRIMARY KEY,
        type VARCHAR NOT NULL,
        payload BLOB,
        metadata JSON,
        timestamp TIMESTAMP,
        ledger_sequence BIGINT,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )
`)

// In handler
cons.OnConsume(func(ctx context.Context, event *flowctlv1.Event) error {
    _, err := db.ExecContext(ctx, `
        INSERT INTO events (id, type, payload, metadata, ledger_sequence)
        VALUES (?, ?, ?, ?, ?)
    `,
        event.Id,
        event.Type,
        event.Payload,
        marshalMetadata(event.Metadata),
        event.StellarCursor.GetLedgerSequence(),
    )
    return err
})
```

## Concurrency Control

The consumer SDK automatically limits concurrency:

```go
config.MaxConcurrent = 10 // Process up to 10 events at once
```

Events are handled in separate goroutines, with automatic backpressure:
- If 10 events are being processed, new events wait
- Once a handler completes, the next event is processed
- Prevents overwhelming the database or downstream systems

## Error Handling

The SDK tracks errors automatically:

```go
cons.OnConsume(func(ctx context.Context, event *flowctlv1.Event) error {
    // Any error returned is tracked as failed
    if err := db.Insert(event); err != nil {
        return fmt.Errorf("database error: %w", err)
    }
    // Nil return is tracked as success
    return nil
})

// Check metrics
metrics := cons.GetMetrics()
fmt.Printf("Success: %d, Errors: %d\n",
    metrics["success_count"],
    metrics["error_count"])
```

## Lines of Code

- **Total**: ~160 lines
- **Business logic**: ~80 lines (event handling and storage)
- **Infrastructure**: 0 lines (handled by SDK)

Compare to manual gRPC implementation: ~350 lines with infrastructure.

## Performance

With `MaxConcurrent = 10`:
- Can process up to 10 events simultaneously
- Automatic backpressure management
- No goroutine leaks
- Graceful shutdown waits for in-flight events

Typical throughput (with fast storage):
- **Single-threaded**: 500-1000 events/sec
- **MaxConcurrent = 10**: 5000-10000 events/sec
- **MaxConcurrent = 100**: 20000+ events/sec
