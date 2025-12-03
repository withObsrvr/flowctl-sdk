#!/bin/bash

# Query contract events from PostgreSQL

echo "📊 Contract Events Database - Query Results"
echo "============================================"
echo ""

# Use docker exec to connect to PostgreSQL
DB_CONTAINER="stellar-postgres"
DB_NAME="stellar_events"
DB_USER="postgres"

# Check if container is running
if ! docker ps | grep -q "$DB_CONTAINER"; then
    echo "❌ PostgreSQL container is not running"
    echo "Run: docker-compose up -d postgres"
    exit 1
fi

echo "✅ Connected to PostgreSQL"
echo ""

echo "1️⃣  Total Events Stored:"
docker exec -it "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -c "
SELECT COUNT(*) as total_events FROM contract_events;
"
echo ""

echo "2️⃣  Events by Contract (Top 10):"
docker exec -it "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -c "
SELECT
    LEFT(contract_id, 20) || '...' as contract_id,
    COUNT(*) as event_count,
    MAX(timestamp) as latest_event
FROM contract_events
GROUP BY contract_id
ORDER BY event_count DESC
LIMIT 10;
"
echo ""

echo "3️⃣  Events by Type:"
docker exec -it "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -c "
SELECT
    COALESCE(event_type, 'unknown') as event_type,
    COUNT(*) as count
FROM contract_events
GROUP BY event_type
ORDER BY count DESC
LIMIT 10;
"
echo ""

echo "4️⃣  Recent Events (Last 5):"
docker exec -it "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -c "
SELECT
    ledger_sequence,
    LEFT(transaction_hash, 12) || '...' as tx_hash,
    event_type,
    LEFT(contract_id, 12) || '...' as contract_id
FROM contract_events
ORDER BY ledger_sequence DESC, event_index DESC
LIMIT 5;
"
echo ""

echo "5️⃣  Ledger Range:"
docker exec -it "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -c "
SELECT
    MIN(ledger_sequence) as first_ledger,
    MAX(ledger_sequence) as last_ledger,
    COUNT(DISTINCT ledger_sequence) as ledgers_processed
FROM contract_events;
"
echo ""

echo "💡 To explore interactively:"
echo "   docker exec -it stellar-postgres psql -U postgres -d stellar_events"
