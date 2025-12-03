# Contract Events Pipeline - OBSRVR Flow Flagship Demo

This is the **flagship demo** for OBSRVR Flow/flowctl. It demonstrates a complete end-to-end pipeline that:
1. Extracts **ALL contract events** from Stellar ledgers (not just transfers!)
2. Stores them in a queryable PostgreSQL database
3. Takes **< 5 minutes** to get running from cold start

## 🎯 What This Demo Shows

- **Zero Boilerplate**: ~110 lines of application code (vs 700+ in old approach)
- **One-Command Deployment**: `./demo.sh` starts the entire pipeline
- **Production Ready**: Proper schema, indexes, transactions, connection pooling
- **Extensible**: Easy to add more consumers (webhooks, Kafka, files, etc.)
- **Observable**: Built-in metrics, health checks, structured logging via flowctl
- **Comprehensive**: Captures operation-level AND transaction-level events (Protocol V4+)

## 📊 Architecture

```
Stellar Ledgers → Contract Events Processor → PostgreSQL Consumer → Queryable Database
```

## 🚀 Quick Start

> **New to this demo?** Check out [QUICKSTART.md](QUICKSTART.md) for a step-by-step guide!

### Prerequisites
- **Go 1.21+** (for building binaries)
- **Docker & Docker Compose** (for PostgreSQL only - no containerized services needed!)
- **PostgreSQL client** (psql) - for querying results
- **[flowctl](https://github.com/withObsrvr/flowctl)** - Install with:
  ```bash
  go install github.com/withObsrvr/flowctl/cmd/flowctl@latest
  ```

### Run the Demo (< 5 minutes)

```bash
./demo.sh
```

That's it! This single script will:
1. ✅ Check that flowctl is installed
2. 🔨 Build all binaries (contract-events-processor, postgresql-consumer, stellar-live-source)
3. 🐘 Start PostgreSQL (if not already running)
4. 🚀 Launch the complete pipeline with flowctl
5. 📊 Process ledgers 1910000-1910100 and store **~265 contract events** in PostgreSQL

The pipeline processes in **real-time** - you'll see events flowing from Stellar → Processor → Database!

### What Gets Started

The `flowctl run` command orchestrates these three components:
1. **Stellar Live Source** - Streams ledger data from Stellar RPC (https://soroban-testnet.stellar.org)
2. **Contract Events Processor** - Extracts ALL contract events using proper SDK methods
3. **PostgreSQL Consumer** - Stores events with full schema, indexes, and JSONB support

### Query the Data

**While the pipeline is running** (or after it finishes), open a new terminal:

```bash
./query-events.sh
```

This will show you:
- Total events stored
- Events by contract
- Events by type
- Sample event data

Or connect directly with psql:

```bash
docker exec -it stellar-postgres psql -U postgres -d stellar_events
```

## 📈 Sample Queries

### Total Events
```sql
SELECT COUNT(*) FROM contract_events;
```

### Events by Contract
```sql
SELECT
    contract_id,
    COUNT(*) as event_count,
    MAX(timestamp) as latest_event
FROM contract_events
GROUP BY contract_id
ORDER BY event_count DESC
LIMIT 10;
```

### Events by Type
```sql
SELECT
    event_type,
    COUNT(*) as count
FROM contract_events
GROUP BY event_type
ORDER BY count DESC;
```

### Search Event Topics (JSONB)
```sql
SELECT
    contract_id,
    topics,
    data,
    timestamp
FROM contract_events
WHERE topics @> '[{"type": "symbol", "value": "transfer"}]'
LIMIT 10;
```

## 🗄️ Database Schema

```sql
CREATE TABLE contract_events (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL,
    ledger_sequence INTEGER NOT NULL,
    transaction_hash TEXT NOT NULL,
    transaction_index INTEGER NOT NULL,
    contract_id TEXT NOT NULL,
    event_type TEXT,
    topics JSONB NOT NULL,
    data JSONB,
    in_successful_tx BOOLEAN NOT NULL,
    event_index INTEGER NOT NULL,
    operation_index INTEGER,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for efficient querying
CREATE INDEX idx_contract_events_contract_id ON contract_events(contract_id);
CREATE INDEX idx_contract_events_timestamp ON contract_events(timestamp);
CREATE INDEX idx_contract_events_ledger_sequence ON contract_events(ledger_sequence);
CREATE INDEX idx_contract_events_transaction_hash ON contract_events(transaction_hash);
CREATE INDEX idx_contract_events_event_type ON contract_events(event_type);

-- JSONB indexes for flexible querying
CREATE INDEX idx_contract_events_topics_gin ON contract_events USING GIN (topics);
CREATE INDEX idx_contract_events_data_gin ON contract_events USING GIN (data);
```

## ⚙️ Configuration

### Pipeline Configuration (contract-events-pipeline.yaml)

The main configuration is in `contract-events-pipeline.yaml`:

```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: contract-events-pipeline
  description: Contract Events Pipeline - Stellar ledgers to PostgreSQL

spec:
  driver: process

  sources:
    - id: stellar-live-source
      command: ["go", "run", "../stellar-live-source/main.go"]
      env:
        RPC_ENDPOINT: "https://soroban-testnet.stellar.org:443"
        NETWORK_PASSPHRASE: "Test SDF Network ; September 2015"
        START_LEDGER: "1873306"  # Configure ledger range here
        END_LEDGER: "1873320"

  processors:
    - id: contract-events-processor
      command: ["go", "run", "main.go"]
      env:
        NETWORK_PASSPHRASE: "Test SDF Network ; September 2015"

  sinks:
    - id: postgresql-consumer
      command: ["go", "run", "main.go"]
      env:
        POSTGRES_HOST: "localhost"
        POSTGRES_DB: "stellar_events"
        POSTGRES_PASSWORD: "stellarpassword"
```

### Component-Level Configuration

Each component can be configured via environment variables:

**Contract Events Processor** (`examples/contract-events-processor/`):
- `NETWORK_PASSPHRASE` (required): Stellar network passphrase
- `PORT` (optional, default: `:50051`): gRPC server port
- `HEALTH_PORT` (optional, default: `8088`): Health check port
- `ENABLE_FLOWCTL` (optional, default: `false`): Enable flowctl integration

**PostgreSQL Consumer** (`examples/postgresql-consumer/`):
- `POSTGRES_HOST` (default: `localhost`)
- `POSTGRES_PORT` (default: `5432`)
- `POSTGRES_DB` (default: `stellar_events`)
- `POSTGRES_USER` (default: `postgres`)
- `POSTGRES_PASSWORD` (required)
- `POSTGRES_SSLMODE` (default: `disable`)

The consumer also supports a `consumer.yaml` file for more complex configuration:

```yaml
consumer:
  name: "PostgreSQL Consumer"
  description: "Stores Stellar contract events in PostgreSQL"
  version: "1.0.0"
  input: "stellar.contract.event.v1"
  output: ""  # Terminal consumer

database:
  host: localhost
  port: 5432
  database: stellar_events
  user: postgres
  password: stellarpassword
  sslmode: disable
  max_open_conns: 10
  max_idle_conns: 5
  connect_timeout: 10

flowctl:
  enabled: true
  endpoint: "127.0.0.1:8080"
  heartbeat_interval: 10000
```

## 📦 Code Comparison

### Old Way (cdp-pipeline-workflow)
- **processor_contract_events.go**: 457 lines (manual event extraction, V3/V4 handling)
- **consumer_save_contract_events_to_postgresql.go**: 260 lines (manual DB setup, connection management)
- **pipeline wiring**: ~100 lines (manual gRPC streams, error handling)
- **Total**: **~820 lines** of complex, hard-to-maintain code

### New Way (This Demo)
- **contract-events-processor/main.go**: **40 lines** (SDK handles everything)
- **postgresql-consumer/main.go**: **70 lines** (SDK handles gRPC, DB, health checks)
- **Pipeline wiring**: **0 lines** (flowctl YAML config)
- **Total**: **110 lines** of simple, declarative code

**Result**: **86% less code**, infinitely easier to understand, test, and maintain

## 🔧 Development

### Run Tests
```bash
cd ../contract-events-processor
go test ./...

cd ../postgresql-consumer
go test ./...
```

### Build Binaries
```bash
cd ../contract-events-processor
go build -o bin/contract-events-processor

cd ../postgresql-consumer
go build -o bin/postgresql-consumer
```

### Clean Up
```bash
docker-compose down -v  # Remove database and volumes
```

## 🛠️ Troubleshooting

### PostgreSQL Connection Issues
```bash
# Check if PostgreSQL is running
docker-compose ps

# Restart PostgreSQL
docker-compose down
docker-compose up -d postgres

# View PostgreSQL logs
docker-compose logs postgres
```

### Pipeline Not Starting
```bash
# Check flowctl is in PATH
which flowctl

# Check binaries exist
ls -la ../contract-events-processor/bin/
ls -la ../postgresql-consumer/bin/
ls -la ../stellar-live-source/bin/

# Rebuild binaries
cd ../contract-events-processor && go build -o bin/contract-events-processor main.go
cd ../postgresql-consumer && go build -o bin/postgresql-consumer main.go
cd ../stellar-live-source && go build -o bin/stellar-live-source main.go
```

### No Events Being Stored
```bash
# Check if events are being extracted
# Look for "Found contract event" in logs

# Verify PostgreSQL connection
docker exec -it stellar-postgres psql -U postgres -d stellar_events -c "SELECT COUNT(*) FROM contract_events;"

# Check consumer logs for errors
# Consumer should log "Consumer received event" and "Inserted N contract events"
```

### Clean Start
```bash
# Stop everything and clean up
docker-compose down -v
rm -rf logs/

# Start fresh
./demo.sh
```

## 🎓 Learn More

### SDK Documentation
- **[Contract Events Processor](../contract-events-processor/)** - Source code and docs
- **[Consumer SDK](../../pkg/consumer/)** - Build your own consumers
- **[PostgreSQL Sink](../../pkg/consumer/sinks/postgresql/)** - Database integration
- **[Processor SDK](../../pkg/processor/)** - Build custom processors
- **[Stellar SDK Wrappers](../../pkg/stellar/)** - Simplified Stellar integration

### flowctl
- **[flowctl Repository](https://github.com/withObsrvr/flowctl)** - Pipeline orchestrator
- **[flowctl Documentation](https://github.com/withObsrvr/flowctl/blob/main/README.md)** - Full guide

### Stellar
- **[Stellar Smart Contracts](https://developers.stellar.org/docs/smart-contracts)** - Soroban documentation
- **[Contract Events](https://developers.stellar.org/docs/smart-contracts/guides/events)** - Event emission guide

## 📝 License

MIT License - See [LICENSE](../../../LICENSE) for details
