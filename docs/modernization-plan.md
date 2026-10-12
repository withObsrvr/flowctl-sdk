# flowctl-sdk modernization plan

**Role:** Generic runtime contract for flowctl sources, processors, and consumers  
**Local checkout:** `/home/tillman/Documents/flowctl-sdk`  
**First dependency:** None; this is the first behavioral change in the modernization sequence.

## Objective

Make asynchronous component failures observable to callers and health systems.
The SDK must stop translating producer failure into successful end-of-stream.
This is a generic runtime correction, not a Stellar-specific feature.

## Current failure

`pkg/source/source.go` defines:

```go
type ProducerFunc func(
    context.Context,
    *flowctlv1.StreamRequest,
) (<-chan *flowctlv1.Event, error)
```

The returned error covers only synchronous startup. After startup, the producer
can communicate events or close its channel, but it cannot communicate why it
closed. `StandardSource.StreamEvents` treats every channel close as successful
completion and returns `nil`.

The source package also has no focused tests for terminal producer behavior.

## Required outcome

Add a backward-compatible error-aware producer contract with these semantics:

- bounded completion can return success;
- cancellation is distinct from failure;
- unbounded EOF without cancellation is failure;
- terminal failure is delivered exactly once;
- buffered events and the terminal result have a documented ordering rule;
- client disconnect cannot leak a producer goroutine;
- terminal errors become safe, classified gRPC status errors;
- health no longer remains healthy after an unrecoverable stream failure.

Do not change the legacy `ProducerFunc` behavior silently. Prefer a parallel API
such as `OnProduceStream`, migrate components, deprecate the old API, and define
its removal release.

## Error classification

Provide stable categories usable by components without importing Stellar code:

- invalid request/configuration;
- canceled;
- upstream unavailable/timeout;
- data integrity;
- downstream unavailable;
- internal.

Map these to documented gRPC codes. Preserve full wrapped causes in local logs,
but return redacted messages to clients.

## Pull-request sequence

1. Add characterization tests for current `StreamEvents` behavior.
2. Add the error-aware producer result/API without changing existing callers.
3. Teach `StreamEvents` to drain events and observe the terminal result.
4. Map classified errors to gRPC status and SDK health.
5. Add cancellation, disconnect, full-buffer, and goroutine-leak tests.
6. Update README examples and publish a tagged SDK release.

## Required tests

- startup error;
- successful bounded completion;
- producer failure before first event;
- producer failure after events;
- cancellation by client;
- SDK shutdown;
- client disconnect while producer is blocked;
- terminal result delivered once;
- old `ProducerFunc` compatibility;
- configuring both old and new handlers is rejected;
- correct gRPC status and health transition.

## Verification

```bash
cd /home/tillman/Documents/flowctl-sdk
go test ./...
go vet ./...
go build ./...
```

Also run one synthetic gRPC integration test that receives an event followed by
a terminal `Unavailable` error.

## Non-goals

- Stellar ledger logic;
- source retries or checkpoints;
- pipeline supervision;
- Prometheus redesign;
- protobuf changes unless existing gRPC status cannot represent the contract;
- simultaneous dependency upgrades unrelated to this API.

## Downstream handoff

After releasing the SDK:

1. adopt it in `/home/tillman/Documents/stellar-raw-ledger-origin`;
2. update `/home/tillman/Documents/flowctl` to fail pipeline runs on the
   resulting gRPC errors;
3. upgrade `/home/tillman/Documents/obsrvr-stellar-components` only after its
   compatibility suite passes.

## Definition of done

A producer that emits one event and then fails causes `StreamEvents` to deliver
the event and terminate with a classified non-OK gRPC status; cancellation and
bounded completion remain non-failures, and old producers continue to compile.
