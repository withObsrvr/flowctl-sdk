package main

import (
	"context"
	"log"

	"github.com/withObsrvr/flowctl-sdk/pkg/consumer"
	"github.com/withObsrvr/flowctl-sdk/pkg/consumer/sinks/postgresql"
	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	stellarv1 "github.com/withObsrvr/flow-proto/go/gen/stellar/v1"
	"google.golang.org/protobuf/proto"
)

// PostgreSQL Consumer - Stores contract invocation events in PostgreSQL
//
// This consumer receives contract invocation events from the contract-events-processor
// and stores them in a queryable PostgreSQL database.
//
// Configuration (via environment variables or consumer.yaml):
//   - POSTGRES_HOST: Database host (default: "localhost")
//   - POSTGRES_PORT: Database port (default: 5432)
//   - POSTGRES_DB: Database name (default: "stellar_events")
//   - POSTGRES_USER: Database user (default: "postgres")
//   - POSTGRES_PASSWORD: Database password (required)
//   - POSTGRES_SSLMODE: SSL mode (default: "disable")
//   - COMPONENT_ID: Component identifier (optional)
//   - PORT: gRPC server port (optional, default: ":50052")
//   - HEALTH_PORT: Health check port (optional, default: "8089")
//
// Usage:
//   POSTGRES_PASSWORD=mysecret go run main.go
func main() {
	// Initialize PostgreSQL sink
	sink, err := initializePostgreSQL()
	if err != nil {
		log.Fatalf("Failed to initialize PostgreSQL: %v", err)
	}
	defer sink.Close()

	// Run the consumer
	consumer.Run(consumer.ConsumerConfig{
		ConsumerName: "PostgreSQL Consumer",
		InputType:    "stellar.contract.event.v1",
		OnEvent: func(ctx context.Context, event *flowctlv1.Event) error {
			log.Printf("Consumer received event with payload size: %d bytes", len(event.Payload))

			// Parse the contract event batch
			var batch stellarv1.ContractEventBatch
			if err := proto.Unmarshal(event.Payload, &batch); err != nil {
				log.Printf("Error unmarshaling batch: %v", err)
				return err
			}

			log.Printf("Batch contains %d events", len(batch.Events))

			// Insert the batch into PostgreSQL
			return sink.InsertBatch(ctx, &batch)
		},
	})
}

// initializePostgreSQL creates and configures the PostgreSQL sink
func initializePostgreSQL() (*postgresql.PostgreSQLSink, error) {
	// Load configuration
	config, err := consumer.LoadConfig("consumer.yaml")
	if err != nil {
		return nil, err
	}

	// Create PostgreSQL sink with config
	return postgresql.New(postgresql.Config{
		Host:           config.Database.Host,
		Port:           config.Database.Port,
		Database:       config.Database.Database,
		User:           config.Database.User,
		Password:       config.Database.Password,
		SSLMode:        config.Database.SSLMode,
		MaxOpenConns:   config.Database.MaxOpenConns,
		MaxIdleConns:   config.Database.MaxIdleConns,
		ConnectTimeout: config.Database.ConnectTimeout,
	})
}
