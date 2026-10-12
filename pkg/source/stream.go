package source

import (
	"context"
	"errors"
	"fmt"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrorKind classifies a terminal stream error without exposing its cause to clients.
type ErrorKind string

const (
	InvalidRequest        ErrorKind = "invalid_request"
	Canceled              ErrorKind = "canceled"
	UpstreamUnavailable   ErrorKind = "upstream_unavailable"
	DataIntegrity         ErrorKind = "data_integrity"
	DownstreamUnavailable ErrorKind = "downstream_unavailable"
	Internal              ErrorKind = "internal"
)

// StreamError retains Cause for local logging only. Never send Cause to a client.
type StreamError struct {
	Kind  ErrorKind
	Cause error
}

func (e *StreamError) Error() string { return fmt.Sprintf("%s: %v", e.Kind, e.Cause) }
func (e *StreamError) Unwrap() error { return e.Cause }

func streamStatus(err error) error {
	if err == nil {
		return nil
	}
	var classified *StreamError
	if errors.As(err, &classified) {
		switch classified.Kind {
		case InvalidRequest:
			return status.Error(codes.InvalidArgument, "invalid source request")
		case Canceled:
			return status.Error(codes.Canceled, "source stream canceled")
		case UpstreamUnavailable:
			return status.Error(codes.Unavailable, "source upstream unavailable")
		case DataIntegrity:
			return status.Error(codes.DataLoss, "source data integrity failure")
		case DownstreamUnavailable:
			return status.Error(codes.Unavailable, "source downstream unavailable")
		}
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "source stream canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "source stream timed out")
	}
	return status.Error(codes.Internal, "source internal failure")
}

// ProducerStream contains events and exactly one terminal result. The producer must
// send one result (nil on success) into a channel of capacity one BEFORE closing
// Events, and never send another. Events may be buffered; StreamEvents drains all
// events before reading Result. A nil result is valid only for Bounded streams.
// The producer owns both channels and must stop on ctx cancellation, including
// when blocked on a full event buffer (use select with ctx.Done()).
type ProducerStream struct {
	Events  <-chan *flowctlv1.Event
	Result  <-chan error
	Bounded bool
}

// ProducerStreamFunc starts a cooperative producer. Startup errors are returned
// immediately; asynchronous errors are sent via ProducerStream.Result.
type ProducerStreamFunc func(context.Context, *flowctlv1.StreamRequest) (*ProducerStream, error)
