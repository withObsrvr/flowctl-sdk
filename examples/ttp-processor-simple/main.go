package main

import (
	"github.com/withObsrvr/flowctl-sdk/pkg/stellar"
	"github.com/withObsrvr/flowctl-sdk/pkg/stellar/helpers"
	"github.com/stellar/go-stellar-sdk/processors/token_transfer"
	"github.com/stellar/go-stellar-sdk/xdr"
	"google.golang.org/protobuf/proto"
)

func main() {
	stellar.Run(stellar.ProcessorConfig{
		ProcessorName: "Token Transfer Processor",
		OutputType:    "stellar.token.transfer.v1",
		ProcessLedger: func(passphrase string, ledger xdr.LedgerCloseMeta) (proto.Message, error) {
			// Use Stellar's token transfer processor
			processor := token_transfer.NewEventsProcessor(passphrase)
			events, err := processor.EventsFromLedger(ledger)
			if err != nil {
				return nil, err
			}

			// No events? Skip this ledger
			if len(events) == 0 {
				return nil, nil
			}

			// Convert using our helper (or developers can write their own converter)
			return helpers.ConvertTokenTransferEvents(events), nil
		},
	})
}
