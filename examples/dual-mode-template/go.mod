module github.com/withObsrvr/flowctl-sdk/examples/dual-mode-template

go 1.21

require (
	github.com/stellar/go v0.0.0-20241101000000-000000000000
	github.com/withObsrvr/flow-proto v0.0.0
	github.com/withObsrvr/flowctl-sdk v0.0.0
	google.golang.org/protobuf v1.36.0
)

// For local development, use replace directives:
// replace github.com/withObsrvr/flowctl-sdk => ../..
// replace github.com/withObsrvr/flow-proto => /path/to/flow-proto
