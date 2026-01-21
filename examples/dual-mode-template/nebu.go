//go:build !flowctl_only
// +build !flowctl_only

package main

import (
	"context"

	"github.com/stellar/go/xdr"
	proto "github.com/withObsrvr/flowctl-sdk/examples/dual-mode-template/proto"
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

// ProcessLedger implements the nebu processor interface
func (o *NebuOrigin) ProcessLedger(ctx context.Context, ledger xdr.LedgerCloseMeta) error {
	// Call the shared extraction logic
	events, err := EventsFromLedger(o.networkPass, ledger)
	if err != nil {
		return err
	}

	// Emit events to the output channel
	for _, event := range events {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case o.out <- event:
			// Event emitted successfully
		}
	}

	return nil
}

// Out returns the output channel (required by nebu interface)
func (o *NebuOrigin) Out() <-chan *proto.ExampleEvent {
	return o.out
}

// Close closes the output channel (required by nebu interface)
func (o *NebuOrigin) Close() {
	close(o.out)
}

// Name returns the processor name (required by nebu interface)
func (o *NebuOrigin) Name() string {
	return "dual-mode-processor"
}

// runNebuMode starts the processor in nebu mode
// This function is called when FLOWCTL_ENDPOINT is not set
func runNebuMode() {
	// Note: This requires the nebu CLI package
	// Import: "github.com/withObsrvr/nebu/pkg/processor/cli"
	//
	// cli.RunProtoOriginCLI(cli.OriginConfig{
	//     Name:        "dual-mode-processor",
	//     Description: "Dual-mode processor for nebu and flowctl",
	//     Version:     version,
	// }, func(networkPass string) cli.ProtoOriginProcessor[*proto.ExampleEvent] {
	//     return NewNebuOrigin(networkPass)
	// })

	// For this template, we'll print a message indicating nebu mode
	// In a real implementation, uncomment the above and import the nebu CLI package
	println("Running in nebu mode (stub)")
	println("To enable full nebu mode:")
	println("1. Add nebu CLI dependency: go get github.com/withObsrvr/nebu/pkg/processor/cli")
	println("2. Uncomment the RunProtoOriginCLI call in nebu.go")
}
