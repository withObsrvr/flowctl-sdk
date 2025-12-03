#!/bin/bash
set -e

echo "🚀 OBSRVR Flow - Contract Events Pipeline Demo"
echo "=============================================="
echo ""

# Check for flowctl
if ! command -v flowctl &> /dev/null; then
    echo "❌ flowctl not found in PATH"
    echo ""
    echo "Please install flowctl first:"
    echo "  go install github.com/withObsrvr/flowctl/cmd/flowctl@latest"
    echo ""
    echo "Or download from: https://github.com/withObsrvr/flowctl/releases"
    exit 1
fi

echo "✅ flowctl found: $(which flowctl)"
echo ""

# Build binaries if they don't exist
echo "📦 Building binaries..."
if [ ! -f "../contract-events-processor/bin/contract-events-processor" ]; then
    echo "Building contract-events-processor..."
    (cd ../contract-events-processor && go build -o bin/contract-events-processor main.go)
fi

if [ ! -f "../postgresql-consumer/bin/postgresql-consumer" ]; then
    echo "Building postgresql-consumer..."
    (cd ../postgresql-consumer && go build -o bin/postgresql-consumer main.go)
fi

if [ ! -f "../stellar-live-source/bin/stellar-live-source" ]; then
    echo "Building stellar-live-source..."
    (cd ../stellar-live-source && go build -o bin/stellar-live-source main.go)
fi
echo "✅ Binaries ready"
echo ""

# Check if PostgreSQL is running
echo "📦 Checking PostgreSQL..."
if ! docker-compose ps postgres 2>/dev/null | grep -q "Up"; then
    echo "Starting PostgreSQL..."
    docker-compose up -d postgres
    echo "Waiting for PostgreSQL to be ready..."
    sleep 5
fi
echo "✅ PostgreSQL is running"
echo ""

# Run the complete pipeline with flowctl
echo "🔧 Starting contract events pipeline..."
echo "   1. Stellar Live Source - Streams ledger data from Stellar RPC"
echo "   2. Contract Events Processor - Extracts ALL contract events"
echo "   3. PostgreSQL Consumer - Stores events in database"
echo ""
echo "📊 Processing ledgers 1910000-1910100 (configurable in YAML)"
echo ""
echo "Press Ctrl+C to stop the demo"
echo ""

# Run the pipeline
flowctl run contract-events-pipeline.yaml
