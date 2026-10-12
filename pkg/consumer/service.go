package consumer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ConsumerService implements the flowctl ConsumerService gRPC interface.
type ConsumerService struct {
	flowctlv1.UnimplementedConsumerServiceServer
	config         *Config
	consumerName   string
	inputTypes     []string
	inputTypeSet   map[string]struct{}
	onEvent        EventHandlerFunc
	stopCh         chan struct{}
	stopOnce       sync.Once
	eventsConsumed atomic.Int64
	eventsFailed   atomic.Int64
	errorCount     atomic.Int64
	successCount   atomic.Int64
}

func NewConsumerService(config *Config, consumerName string, inputTypes []string, onEvent EventHandlerFunc) *ConsumerService {
	types := make(map[string]struct{}, len(inputTypes))
	for _, t := range inputTypes {
		types[t] = struct{}{}
	}
	return &ConsumerService{config: config, consumerName: consumerName, inputTypes: inputTypes, inputTypeSet: types, onEvent: onEvent, stopCh: make(chan struct{})}
}

func (c *ConsumerService) GetInfo(ctx context.Context, _ *emptypb.Empty) (*flowctlv1.ComponentInfo, error) {
	return &flowctlv1.ComponentInfo{Id: getEnv("COMPONENT_ID", toKebabCase(c.consumerName)), Name: c.config.Consumer.Name, Description: c.config.Consumer.Description, Version: c.config.Consumer.Version, Type: flowctlv1.ComponentType_COMPONENT_TYPE_CONSUMER, InputEventTypes: c.inputTypes, OutputEventTypes: []string{}, Endpoint: getEnv("PORT", ":50052"), Metadata: map[string]string{}}, nil
}

func (c *ConsumerService) HealthCheck(ctx context.Context, req *flowctlv1.HealthCheckRequest) (*flowctlv1.HealthCheckResponse, error) {
	return &flowctlv1.HealthCheckResponse{Status: flowctlv1.HealthStatus_HEALTH_STATUS_HEALTHY, Metrics: c.getMetrics()}, nil
}

func (c *ConsumerService) getMetrics() map[string]string {
	return map[string]string{"events_consumed": fmt.Sprintf("%d", c.eventsConsumed.Load()), "events_failed": fmt.Sprintf("%d", c.eventsFailed.Load()), "error_count": fmt.Sprintf("%d", c.errorCount.Load()), "success_count": fmt.Sprintf("%d", c.successCount.Load())}
}

// Consume processes events in receive order, with at most one callback in flight.
// OnEvent must return only after its intended work has completed (and committed
// for durable sinks). A cancelled callback must honor its context; the SDK
// cannot forcibly terminate user code that ignores cancellation.
func (c *ConsumerService) Consume(stream flowctlv1.ConsumerService_ConsumeServer) error {
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()
	var consumed, failed int64
	var filtered int64
	var summaries []string
	cancelled := func() error {
		if err := stream.Context().Err(); err != nil {
			return status.FromContextError(err).Err()
		}
		select {
		case <-c.stopCh:
			return status.Error(codes.Canceled, "consumer stopped")
		default:
			return nil
		}
	}
	for {
		if err := cancelled(); err != nil {
			return err
		}
		// Recv can block after Stop; the RPC transport is closed by Run on shutdown.
		// Only one receive is outstanding per stream, and no receive queue is built.
		type result struct {
			event *flowctlv1.Event
			err   error
		}
		received := make(chan result, 1)
		go func() { event, err := stream.Recv(); received <- result{event, err} }()
		var r result
		select {
		case <-ctx.Done():
			return status.FromContextError(ctx.Err()).Err()
		case <-c.stopCh:
			return status.Error(codes.Canceled, "consumer stopped")
		case r = <-received:
		}
		if err := cancelled(); err != nil {
			return err
		}
		if r.err == io.EOF {
			if filtered > 0 && consumed+failed == 0 {
				return status.Error(codes.FailedPrecondition, "no events matched consumer input types")
			}
			return stream.SendAndClose(&flowctlv1.ConsumeResponse{EventsConsumed: consumed, EventsFailed: failed, Errors: summaries})
		}
		if r.err != nil {
			return status.Error(codes.Unavailable, "consumer receive failed")
		}
		if r.event == nil {
			return status.Error(codes.InvalidArgument, "nil event")
		}
		if len(c.inputTypeSet) > 0 {
			if _, ok := c.inputTypeSet[r.event.Type]; !ok {
				filtered++
				continue
			}
		}
		if c.onEvent == nil {
			return status.Error(codes.FailedPrecondition, "consumer handler not configured")
		}
		done := make(chan error, 1)
		go func() { done <- c.onEvent(ctx, r.event) }()
		var handlerErr error
		select {
		case <-ctx.Done():
			return status.FromContextError(ctx.Err()).Err()
		case <-c.stopCh:
			return status.Error(codes.Canceled, "consumer stopped")
		case handlerErr = <-done:
		}
		if err := cancelled(); err != nil {
			return err
		}
		if handlerErr != nil {
			if errors.Is(handlerErr, context.Canceled) || errors.Is(handlerErr, context.DeadlineExceeded) {
				return status.FromContextError(handlerErr).Err()
			}
			failed++
			c.eventsFailed.Add(1)
			c.errorCount.Add(1)
			// Do not expose handler errors: they may contain event data or credentials.
			if len(summaries) < 8 {
				summaries = append(summaries, "consumer handler failed")
			}
		} else {
			consumed++
			c.eventsConsumed.Add(1)
			c.successCount.Add(1)
		}
	}
}

// Stop is idempotent. It cancels active RPCs; it is not a successful drain.
func (c *ConsumerService) Stop() error { c.stopOnce.Do(func() { close(c.stopCh) }); return nil }
