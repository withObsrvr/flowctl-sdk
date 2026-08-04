module github.com/withObsrvr/flowctl-sdk/examples/postgresql-consumer

go 1.26.1

replace github.com/withObsrvr/flowctl-sdk => ../..

require (
	github.com/withObsrvr/flow-proto v0.1.3
	github.com/withObsrvr/flowctl-sdk v0.0.0-00010101000000-000000000000
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/lib/pq v1.10.9 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260209200024-4cfbd4190f57 // indirect
	google.golang.org/grpc v1.79.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
