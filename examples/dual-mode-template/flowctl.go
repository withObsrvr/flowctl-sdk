package main

import (
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/withObsrvr/flowctl-sdk/pkg/stellar"
	"google.golang.org/protobuf/proto"

	eventpb "github.com/withObsrvr/flowctl-sdk/examples/dual-mode-template/proto"
)

// runFlowctlMode starts the processor in flowctl mode
// This function is called when FLOWCTL_ENDPOINT is set or ENABLE_FLOWCTL=true
func runFlowctlMode() {
	stellar.Run(stellar.ProcessorConfig{
		ProcessorName: "Dual-Mode Processor",
		OutputType:    "dualmode.events.v1",
		ProcessLedger: func(networkPassphrase string, ledger xdr.LedgerCloseMeta) (proto.Message, error) {
			// Call the shared extraction logic
			events, err := EventsFromLedger(networkPassphrase, ledger)
			if err != nil {
				return nil, err
			}

			// Return nil if no events (flowctl convention)
			if len(events) == 0 {
				return nil, nil
			}

			// Wrap events in a batch for efficient transmission
			return &eventpb.ExampleEventBatch{
				Events:         events,
				LedgerSequence: ledger.LedgerSequence(),
				EventCount:     uint32(len(events)),
			}, nil
		},
	})
}
