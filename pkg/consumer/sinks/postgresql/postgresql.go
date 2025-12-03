package postgresql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
	stellarv1 "github.com/withObsrvr/flow-proto/go/gen/stellar/v1"
)

// PostgreSQLSink handles storing contract events in PostgreSQL
type PostgreSQLSink struct {
	db *sql.DB
}

// Config represents PostgreSQL connection configuration
type Config struct {
	Host           string
	Port           int
	Database       string
	User           string
	Password       string
	SSLMode        string
	MaxOpenConns   int
	MaxIdleConns   int
	ConnectTimeout int
}

// New creates a new PostgreSQL sink
func New(config Config) (*PostgreSQLSink, error) {
	// Build connection string
	connStr := fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s connect_timeout=%d",
		config.Host, config.Port, config.Database, config.User, config.Password, config.SSLMode, config.ConnectTimeout,
	)

	log.Printf("Connecting to PostgreSQL at %s:%d...", config.Host, config.Port)

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.ConnectTimeout)*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	log.Printf("Successfully connected to PostgreSQL at %s:%d", config.Host, config.Port)

	// Initialize schema
	if err := initializeSchema(db); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &PostgreSQLSink{db: db}, nil
}

// initializeSchema creates the necessary tables and indexes
func initializeSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS contract_events (
			id BIGSERIAL PRIMARY KEY,
			timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
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
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);
		
		-- Create indexes for efficient querying
		CREATE INDEX IF NOT EXISTS idx_contract_events_contract_id ON contract_events(contract_id);
		CREATE INDEX IF NOT EXISTS idx_contract_events_timestamp ON contract_events(timestamp);
		CREATE INDEX IF NOT EXISTS idx_contract_events_ledger_sequence ON contract_events(ledger_sequence);
		CREATE INDEX IF NOT EXISTS idx_contract_events_transaction_hash ON contract_events(transaction_hash);
		CREATE INDEX IF NOT EXISTS idx_contract_events_event_type ON contract_events(event_type);
		CREATE INDEX IF NOT EXISTS idx_contract_events_tx_index ON contract_events(transaction_index);
		
		-- GIN index for JSONB columns for efficient querying
		CREATE INDEX IF NOT EXISTS idx_contract_events_topics_gin ON contract_events USING GIN (topics);
		CREATE INDEX IF NOT EXISTS idx_contract_events_data_gin ON contract_events USING GIN (data);
	`)

	if err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	log.Println("Database schema initialized successfully")
	return nil
}

// InsertEvent inserts a single contract event into the database
func (s *PostgreSQLSink) InsertEvent(ctx context.Context, event *stellarv1.ContractEvent) error {
	// Convert topics to JSON
	topicsJSON, err := json.Marshal(event.Topics)
	if err != nil {
		return fmt.Errorf("failed to marshal topics: %w", err)
	}

	// Convert data to JSON
	dataJSON, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	// Insert the event
	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO contract_events (
			timestamp, ledger_sequence, transaction_hash, transaction_index,
			contract_id, event_type, topics, data, in_successful_tx,
			event_index, operation_index
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		time.Unix(event.Meta.LedgerClosedAt, 0),
		event.Meta.LedgerSequence,
		event.Meta.TxHash,
		event.Meta.TxIndex,
		event.ContractId,
		event.EventType,
		topicsJSON,
		dataJSON,
		event.InSuccessfulTx,
		event.EventIndex,
		event.OperationIndex,
	)

	if err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	return nil
}

// InsertBatch inserts multiple contract events in a single transaction
func (s *PostgreSQLSink) InsertBatch(ctx context.Context, batch *stellarv1.ContractEventBatch) error {
	if len(batch.Events) == 0 {
		return nil // Nothing to insert
	}

	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Prepare statement
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO contract_events (
			timestamp, ledger_sequence, transaction_hash, transaction_index,
			contract_id, event_type, topics, data, in_successful_tx,
			event_index, operation_index
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Insert all events
	for _, event := range batch.Events {
		// Convert topics and data to JSON
		topicsJSON, err := json.Marshal(event.Topics)
		if err != nil {
			return fmt.Errorf("failed to marshal topics: %w", err)
		}

		dataJSON, err := json.Marshal(event.Data)
		if err != nil {
			return fmt.Errorf("failed to marshal data: %w", err)
		}

		_, err = stmt.ExecContext(
			ctx,
			time.Unix(event.Meta.LedgerClosedAt, 0),
			event.Meta.LedgerSequence,
			event.Meta.TxHash,
			event.Meta.TxIndex,
			event.ContractId,
			event.EventType,
			topicsJSON,
			dataJSON,
			event.InSuccessfulTx,
			event.EventIndex,
			event.OperationIndex,
		)

		if err != nil {
			return fmt.Errorf("failed to insert event: %w", err)
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Inserted %d contract events", len(batch.Events))
	return nil
}

// Close closes the database connection
func (s *PostgreSQLSink) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// GetStats returns database statistics
func (s *PostgreSQLSink) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get total event count
	var totalEvents int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM contract_events").Scan(&totalEvents)
	if err != nil {
		return nil, err
	}
	stats["total_events"] = totalEvents

	// Get events by contract
	rows, err := s.db.QueryContext(ctx, `
		SELECT contract_id, COUNT(*) as count
		FROM contract_events
		GROUP BY contract_id
		ORDER BY count DESC
		LIMIT 10
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	topContracts := make(map[string]int64)
	for rows.Next() {
		var contractID string
		var count int64
		if err := rows.Scan(&contractID, &count); err != nil {
			return nil, err
		}
		topContracts[contractID] = count
	}
	stats["top_contracts"] = topContracts

	return stats, nil
}
