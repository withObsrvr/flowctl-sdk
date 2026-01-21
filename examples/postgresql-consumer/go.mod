module github.com/withObsrvr/flowctl-sdk/examples/postgresql-consumer

go 1.24

replace github.com/withObsrvr/flowctl-sdk => ../..

require (
	github.com/withObsrvr/flow-proto v0.0.0-20251209215201-bd54ee3e43e9
	github.com/withObsrvr/flowctl-sdk v0.0.0-00010101000000-000000000000
	google.golang.org/protobuf v1.36.6
)

require (
	github.com/lib/pq v1.10.9 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/grpc v1.71.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
