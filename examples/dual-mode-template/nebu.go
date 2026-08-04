package main

import (
	"context"
	"fmt"

	"github.com/stellar/go-stellar-sdk/xdr"
	proto "github.com/withObsrvr/flowctl-sdk/examples/dual-mode-template/proto"
	"github.com/withObsrvr/nebu/pkg/processor"
	"github.com/withObsrvr/nebu/pkg/processor/cli"
)

// NebuOrigin implements the nebu ProtoOriginProcessor interface
type NebuOrigin struct {
	networkPass string
	out         chan *proto.ExampleEvent
}

// NewNebuOrigin creates a new nebu-compatible processor
func NewNebuOrigin(networkPass string) *NebuOrigin {
	return &NebuOrigin{
		networkPass: networkPass,
		out:         make(chan *proto.ExampleEvent, 128),
	}
}

// ProcessLedger implements the nebu processor interface.
func (o *NebuOrigin) ProcessLedger(ctx context.Context, ledger xdr.LedgerCloseMeta) {
	events, err := EventsFromLedger(o.networkPass, ledger)
	if err != nil {
		processor.ReportWarning(ctx, o.Name(), fmt.Errorf("ledger %d: %w", ledger.LedgerSequence(), err))
		return
	}

	for _, event := range events {
		select {
		case <-ctx.Done():
			return
		case o.out <- event:
		}
	}
}

// Out returns the output channel (required by nebu interface)
func (o *NebuOrigin) Out() <-chan *proto.ExampleEvent {
	return o.out
}

// Close closes the output channel (required by nebu interface)
func (o *NebuOrigin) Close() {
	close(o.out)
}

// Name returns the processor name (required by nebu interface).
func (o *NebuOrigin) Name() string {
	return "dual-mode-processor"
}

// Type identifies this processor as an origin.
func (o *NebuOrigin) Type() processor.Type {
	return processor.TypeOrigin
}

// runNebuMode starts the processor in nebu mode
// This function is called when FLOWCTL_ENDPOINT is not set
func runNebuMode() {
	cli.RunProtoOriginCLI(cli.OriginConfig{
		Name:        "dual-mode-processor",
		Description: "Extract example events from Stellar ledgers",
		Version:     version,
		SchemaID:    "dualmode.events.v1",
	}, func(networkPass string) cli.ProtoOriginProcessor[*proto.ExampleEvent] {
		return NewNebuOrigin(networkPass)
	})
}
