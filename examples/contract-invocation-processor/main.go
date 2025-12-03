package main

import (
	"log"

	"github.com/withObsrvr/flowctl-sdk/pkg/stellar"
	"github.com/withObsrvr/flowctl-sdk/pkg/stellar/processors/contract_invocation"
	"github.com/stellar/go-stellar-sdk/xdr"
	"google.golang.org/protobuf/proto"
)

func main() {
	stellar.Run(stellar.ProcessorConfig{
		ProcessorName: "Contract Invocation Processor",
		OutputType:    "stellar.contract.invocation.v1",
		ProcessLedger: func(passphrase string, ledger xdr.LedgerCloseMeta) (proto.Message, error) {
			// Create the processor using Stellar's standard pattern
			processor := contract_invocation.NewEventsProcessor(passphrase)

			// Extract contract invocation events from the ledger
			events, err := processor.EventsFromLedger(ledger)
			if err != nil {
				return nil, err
			}

			// No events? Skip this ledger
			if len(events) == 0 {
				return nil, nil
			}

			// Log what we found
			log.Printf("Found %d contract invocations in ledger %d",
				len(events), ledger.LedgerSequence())

			// Log details about cross-contract calls
			for _, event := range events {
				if event.HasCrossContractCalls() {
					log.Printf("  Contract %s invoked %s with %d cross-contract calls (max depth: %d)",
						event.ContractID,
						event.FunctionName,
						len(event.ContractCalls),
						event.GetMaxCallDepth())
				}
			}

			// Convert to proper protobuf message
			return contract_invocation.ConvertToProto(events), nil
		},
	})
}
