package main

import (
	"log"

	"github.com/withObsrvr/flowctl-sdk/pkg/stellar"
	"github.com/withObsrvr/flowctl-sdk/pkg/stellar/processors/contract_events"
	"github.com/stellar/go-stellar-sdk/xdr"
	"google.golang.org/protobuf/proto"
)

// Contract Events Processor - Extracts ALL contract invocation events from Stellar ledgers
//
// This is the flagship demo for OBSRVR Flow/flowctl. It shows how to:
// 1. Extract ALL contract events from ledgers (not just transfers)
// 2. Use zero-config wrappers (40 lines vs 700+ lines of boilerplate)
// 3. Feed data into consumers (PostgreSQL, webhooks, files, etc.)
//
// Configuration:
//   - NETWORK_PASSPHRASE: Stellar network (required)
//   - COMPONENT_ID: Component identifier (optional, default: "contract-events-processor")
//   - PORT: gRPC server port (optional, default: ":50051")
//   - HEALTH_PORT: Health check port (optional, default: "8088")
//   - ENABLE_FLOWCTL: Enable flowctl integration (optional, default: "false")
//
// Usage:
//   NETWORK_PASSPHRASE="Test SDF Network ; September 2015" go run main.go
func main() {
	stellar.Run(stellar.ProcessorConfig{
		ProcessorName: "Contract Events Processor",
		OutputType:    "stellar.contract.event.v1",
		ProcessLedger: func(passphrase string, ledger xdr.LedgerCloseMeta) (proto.Message, error) {
			// Use the contract events processor from flowctl-sdk
			processor := contract_events.NewEventsProcessor(passphrase)
			events, err := processor.EventsFromLedger(ledger)
			if err != nil {
				return nil, err
			}

			// If no events found, return nil (skip this ledger)
			if len(events) == 0 {
				return nil, nil
			}

			// Convert to proto format
			batch := contract_events.ConvertToProto(events)
			log.Printf("Sending batch with %d events to downstream", len(batch.Events))
			return batch, nil
		},
	})
}
