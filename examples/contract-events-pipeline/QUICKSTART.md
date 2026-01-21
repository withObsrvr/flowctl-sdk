# Contract Events Pipeline - Quick Start Guide

**Get a complete Stellar contract events pipeline running in < 5 minutes!**

## What You'll Get

- ✅ Extracts ALL contract events from Stellar ledgers
- ✅ Stores them in queryable PostgreSQL database
- ✅ ~265 sample events from testnet ledgers 1910000-1910100
- ✅ Full schema with indexes and JSONB support

## Prerequisites (One-Time Setup)

1. **Install Go 1.21+**
   ```bash
   # Check if you have Go
   go version
   ```

2. **Install Docker & Docker Compose**
   ```bash
   # Check if you have Docker
   docker --version
   docker-compose --version
   ```

3. **Install flowctl**
   ```bash
   go install github.com/withObsrvr/flowctl/cmd/flowctl@latest

   # Verify installation
   flowctl version
   ```

## Run the Demo (3 Commands)

```bash
# 1. Clone the repo (if you haven't already)
git clone https://github.com/withObsrvr/flowctl-sdk.git
cd flowctl-sdk/examples/contract-events-pipeline

# 2. Run the demo
./demo.sh

# 3. In a separate terminal, query the results
./query-events.sh
```

That's it! You'll see:
- Real-time logs of events being processed
- 56 batches containing 265 total contract events
- Events stored in PostgreSQL with full metadata

## What Gets Started

The demo automatically:
1. Checks for flowctl
2. Builds three binaries:
   - `stellar-live-source` - Streams ledger data from Stellar RPC
   - `contract-events-processor` - Extracts contract events
   - `postgresql-consumer` - Stores events in database
3. Starts PostgreSQL (via Docker)
4. Runs the complete pipeline with `flowctl run`

## Expected Output

```
🚀 OBSRVR Flow - Contract Events Pipeline Demo
==============================================

✅ flowctl found: /path/to/flowctl
📦 Building binaries...
✅ Binaries ready
📦 Checking PostgreSQL...
✅ PostgreSQL is running

🔧 Starting contract events pipeline...
   1. Stellar Live Source - Streams ledger data from Stellar RPC
   2. Contract Events Processor - Extracts ALL contract events
   3. PostgreSQL Consumer - Stores events in database

📊 Processing ledgers 1910000-1910100 (configurable in YAML)

Press Ctrl+C to stop the demo
```

You'll then see logs showing:
- Ledgers being streamed from Stellar
- Contract events being extracted
- Events being inserted into PostgreSQL

## Query the Results

While the pipeline is running (or after), run:

```bash
./query-events.sh
```

This shows:
- Total events stored
- Events by contract
- Events by type (transfer, mint, burn, swap, etc.)
- Recent events
- Ledger range processed

## What to Try Next

### Modify the Ledger Range

Edit `contract-events-pipeline.yaml`:

```yaml
sources:
  - id: stellar-live-source
    env:
      START_LEDGER: "1910000"  # Change these
      END_LEDGER: "1910200"    # to process more ledgers
```

### Query the Database Interactively

```bash
docker exec -it stellar-postgres psql -U postgres -d stellar_events
```

Try queries like:
```sql
-- Find all transfer events
SELECT * FROM contract_events WHERE event_type = 'transfer';

-- Search in JSONB topics
SELECT * FROM contract_events
WHERE topics @> '[{"type": "symbol", "value": "mint"}]';

-- Events by ledger
SELECT ledger_sequence, COUNT(*)
FROM contract_events
GROUP BY ledger_sequence
ORDER BY ledger_sequence;
```

### Add Your Own Consumer

The SDK makes it easy to add new consumers:

```go
// Your custom consumer
consumer.Run(consumer.ConsumerConfig{
    ConsumerName: "My Consumer",
    InputType:    "stellar.contract.event.v1",
    OnEvent: func(ctx context.Context, event *flowctlv1.Event) error {
        // Process the event however you want!
        // - Send to webhook
        // - Write to file
        // - Send to Kafka
        // - Trigger Lambda
        return nil
    },
})
```

Add it to the pipeline YAML:
```yaml
sinks:
  - id: postgresql-consumer
    command: [...]

  - id: my-consumer
    command: ["go", "run", "../my-consumer/main.go"]
```

## Stop the Demo

Press `Ctrl+C` in the terminal running the demo.

To completely clean up:
```bash
docker-compose down -v  # Remove database and volumes
rm -rf logs/            # Remove log files
```

## Troubleshooting

### flowctl not found
```bash
# Make sure $GOPATH/bin is in your PATH
export PATH="$PATH:$(go env GOPATH)/bin"

# Or install to a location already in PATH
sudo env "PATH=$PATH" go install github.com/withObsrvr/flowctl/cmd/flowctl@latest
```

### PostgreSQL port already in use
```bash
# Stop any existing PostgreSQL
docker-compose down

# Or change the port in docker-compose.yml
ports:
  - "5433:5432"  # Use 5433 instead of 5432
```

### Build errors
```bash
# Make sure you're in the right directory
cd flowctl-sdk/examples/contract-events-pipeline

# Update dependencies
cd ../contract-events-processor && go mod tidy
cd ../postgresql-consumer && go mod tidy
cd ../stellar-live-source && go mod tidy
```

## Learn More

- **[Full README](README.md)** - Complete documentation
- **[Pipeline Configuration](contract-events-pipeline.yaml)** - YAML config
- **[Consumer SDK](../../pkg/consumer/)** - Build your own consumers
- **[flowctl Documentation](https://github.com/withObsrvr/flowctl)** - Pipeline orchestrator

## Next Steps

1. Try modifying the ledger range
2. Add your own custom consumer
3. Query the database with complex JSONB searches
4. Explore the SDK source code
5. Build your own processor for different event types

**Questions?** Open an issue at https://github.com/withObsrvr/flowctl-sdk/issues
