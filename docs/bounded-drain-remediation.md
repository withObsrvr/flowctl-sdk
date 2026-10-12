# Bounded drain: SDK consumer and processor remediation

**Priority:** P0 correctness (a bounded pipeline can report success before accepted sink work finishes)

**Affected runtime:** `flowctl-sdk` v0.1.4, as vendored by `obsrvr-stellar-components`

**Companion:** [`flowctl` runner plan](https://github.com/withObsrvr/flowctl/blob/main/docs/bounded-drain-remediation.md)

**Evidence:** `obsrvr-stellar-components/docs/flowctl-bounded-drain-handoff.md` and `docs/archive-catchup-measurement-2026-10-12.md` in that repository. Local `/tmp/obsrvr-archive-catchup-120/` evidence is not durable.

## Failure and contract

In a 120-ledger bounded archive run, flowctl marked the run `COMPLETED` while
only 32 ledger watermarks had committed. The other 88 accepted events were not
proved durable; shutdown canceled queued callbacks. Do **not** interpret an
upstream EOF, a gRPC `Send`, or the current consumer `EventsConsumed` count as a
successful database write. The recent archive throughput probes measured only
committed prefixes of unbounded runs, not bounded completion.

In `pkg/consumer/service.go`, `Consume` starts an unrestricted goroutine per
received event, increments `consumedCount` on enqueue, and responds to `io.EOF`
without waiting for handlers. Handler failures only update global counters;
per-stream `failedCount` and `Errors` do not include them. A non-EOF `Recv`
error is currently logged and retried in the loop rather than terminating the
stream. Context/stop paths also attempt to send a success-looking summary
without draining. A slow
one-slot DuckLake write gate serializes writes but **not** the goroutines
waiting to acquire it. `Stop` waits on a package-wide WaitGroup, so stream
lifecycle and stop/cancellation ordering need an explicit contract.

For **each Consume RPC** define:

- `EventsConsumed`: accepted events whose `OnEvent` returned nil **after** the
  intended work completed. For durable sinks, the handler must return only
  after commit; this SDK cannot make an asynchronous handler durable by itself.
- `EventsFailed`: accepted events whose handler returned a non-cancellation
  failure. Preserve a bounded, sanitized error summary, without racing over a
  shared slice. Never report a successful acknowledgement while work remains.
- Events rejected by documented type filters are not accepted or counted; a
  sink whose declared input types disagree with flowctl's routing must not
  silently satisfy a positive acknowledgement.
- `io.EOF` (client half-close): stop receiving new events, finish all accepted
  work, then return the final response. A handler failure must produce either
  a failed summary or a non-OK gRPC status; never an all-success response.
- A non-EOF `Recv` error must terminate the stream rather than busy-loop or
  be counted as a failed handler invocation.
- Stream cancellation/client disconnect and SDK shutdown: cancel outstanding
  work, wait/reap it without deadlock, return a non-OK cancellation outcome;
  neither is a completed drain. Keep cancellation distinct from a genuine
  handler error already captured. Decide how in-flight work is counted when
  cancellation wins; never claim it succeeded without confirmation.

## SDK changes, in order

1. **P0 — Bound consumer work.** Default to serial processing for ordered
   sinks, or use an explicitly configured finite worker limit with clear
   ordering semantics. Apply capacity limits *before the next Recv*, so there
   is no unbounded queue of waiting goroutines. Use per-stream state, counts,
   cancellation, and finalization (multiple source streams may call `Consume`
   concurrently); do not let one stream's EOF or counters acknowledge another.
   Make `Stop` idempotent and ensure it can unblock active streams without
   waiting forever for `Recv` or an abandoned handler.
2. **P0 — Return processor errors.** In `pkg/processor/processor.go`,
   `processAndSend` currently logs and returns nil for both a missing handler
   and a handler error. Return an error through `Process` so flowctl fails the
   run. An intentional `nil` output **without** an error is an explicit filter;
   an error plus `nil` output is still an error. Preserve the existing
   send-before-EOF behavior so the final output is not lost. Keep cancellation
   distinct from handler failure and avoid returning sensitive payloads or
   credentials in gRPC status messages.
3. **P1 — Fix duplicate health listener startup.** `source.Start` and
   `processor.Start` call `health.Start`, then `controller.Start` calls the same
   server's `Start` again when flowctl integration is enabled. Choose one
   owner (or a truly idempotent start/stop), bind the HTTP listener
   synchronously, and return bind errors from startup. Do not mark `/ready`
   healthy when the listener could not bind. Test unique and occupied ports
   for both source and processor. The observed double "started" messages plus
   `bind: address already in use` are consistent with this code path; they do
   not explain the 32/120 drain mismatch.

## Deterministic tests

- Feed a fast bounded stream of **120 numbered events** to a 10 ms/event
  serial handler. Assert EOF cannot produce `SendAndClose` until every handler
  returns; the response is 120 consumed, zero failed, and the final event was
  processed. Assert bounded active handler count and no leaked goroutines.
- Fail handler event N. Assert no success claim for N or unprocessed later
  events, and a failed response or non-OK status with correct per-stream counts.
  Exercise multiple simultaneous streams and type filtering independently.
- Cancel during an in-flight callback and while waiting for capacity. Neither
  case may return success or hang; distinguish cancellation from a real error
  that completed first. Test `Stop` during an active stream and a terminal
  non-EOF `Recv` failure.
- Processor tests: handler error, missing handler, deliberate nil-output
  filter, last output before EOF, and cancellation/error status behavior.
- Health tests: with integration enabled, exactly one listener binds;
  occupied port fails startup immediately; start/stop cannot create a second
  listener or leave a stale readiness signal.

## Release handoff

Run `go test ./...`, `go test -race ./pkg/consumer ./pkg/processor ./pkg/source ./pkg/flowctl`,
`go vet ./...`, and `go build ./...`; then tag a new SDK release (not v0.1.4).
`obsrvr-stellar-components` must upgrade its `go.mod`, refresh `vendor/`, and
rebuild source/processor/sink binaries or images; fixing this checkout alone
does not change its vendored v0.1.4 behavior. Coordinate with flowctl's
positive-acknowledgement change before claiming bounded pipeline completion.
The raw-ledger source's previously released error-aware behavior does not
solve consumer drain or processor error propagation.
