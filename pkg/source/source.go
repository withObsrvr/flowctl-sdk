package source

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"github.com/withObsrvr/flowctl-sdk/pkg/flowctl"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Error types
var (
	ErrSourceStopped    = errors.New("source has been stopped")
	ErrProducerNotSet   = errors.New("producer function not set")
	ErrAlreadyStreaming = errors.New("already streaming events")
	ErrProducerConflict = errors.New("old and stream producers cannot both be configured")
)

// ProducerFunc is a function that produces events on a channel
// Deprecated: use ProducerStreamFunc and OnProduceStream. Removal is planned for v2.0.0.
// The function should:
// - Create a channel with appropriate buffer size
// - Start a goroutine to produce events
// - Return the channel immediately
// - Close the channel when done
// - Respect context cancellation
type ProducerFunc func(ctx context.Context, request *flowctlv1.StreamRequest) (<-chan *flowctlv1.Event, error)

// Source is the main interface for a flowctl source
type Source interface {
	// Start starts the source
	Start(ctx context.Context) error

	// Stop stops the source
	Stop() error

	// OnProduce registers the event producer function
	OnProduce(producer ProducerFunc) error

	// OnProduceStream registers the error-aware producer.
	OnProduceStream(producer ProducerStreamFunc) error

	// GetMetrics returns the current metrics
	GetMetrics() map[string]interface{}

	// Metrics returns the metrics interface
	Metrics() flowctl.Metrics
}

// StandardSource implements the Source interface
type StandardSource struct {
	flowctlv1.UnimplementedSourceServiceServer

	config         *Config
	producer       ProducerFunc
	streamProducer ProducerStreamFunc
	server         *grpc.Server
	controller     flowctl.Controller
	metrics        flowctl.Metrics
	health         flowctl.HealthServer

	mu          sync.RWMutex
	started     bool
	streaming   bool
	stopCh      chan struct{}
	streamingWg sync.WaitGroup
}

// New creates a new source
func New(config *Config) (*StandardSource, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create metrics
	metrics := flowctl.NewStandardMetrics()

	// Create health server
	health := flowctl.NewHealthServer(config.HealthPort, metrics)

	// Create controller if flowctl is enabled
	var controller flowctl.Controller
	if config.FlowctlConfig != nil && config.FlowctlConfig.Enabled {
		controller = flowctl.NewController(*config.FlowctlConfig, metrics, health)
	}

	return &StandardSource{
		config:     config,
		metrics:    metrics,
		controller: controller,
		health:     health,
		stopCh:     make(chan struct{}),
	}, nil
}

// OnProduce registers the event producer function
func (s *StandardSource) OnProduce(producer ProducerFunc) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if producer == nil {
		return ErrProducerNotSet
	}
	if s.streamProducer != nil {
		return ErrProducerConflict
	}
	s.producer = producer
	return nil
}

// OnProduceStream registers an error-aware producer.
func (s *StandardSource) OnProduceStream(producer ProducerStreamFunc) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if producer == nil {
		return ErrProducerNotSet
	}
	if s.producer != nil {
		return ErrProducerConflict
	}
	s.streamProducer = producer
	return nil
}

// Start starts the source
func (s *StandardSource) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}
	if s.producer == nil && s.streamProducer == nil {
		s.mu.Unlock()
		return ErrProducerNotSet
	}
	s.started = true
	s.mu.Unlock()

	// Start health server
	s.health.SetHealth(flowctl.HealthStatusStarting)
	if err := s.health.Start(); err != nil {
		return fmt.Errorf("failed to start health server: %w", err)
	}

	// Register with flowctl if enabled
	if s.controller != nil {
		if err := s.controller.Register(ctx); err != nil {
			return fmt.Errorf("failed to register with flowctl: %w", err)
		}

		if err := s.controller.Start(ctx); err != nil {
			return fmt.Errorf("failed to start flowctl controller: %w", err)
		}
	}

	// Start gRPC server
	lis, err := net.Listen("tcp", s.config.Endpoint)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.config.Endpoint, err)
	}

	s.server = grpc.NewServer()

	// Register the source service
	flowctlv1.RegisterSourceServiceServer(s.server, s)

	// Enable reflection for development
	reflection.Register(s.server)

	// Start the server
	go func() {
		if err := s.server.Serve(lis); err != nil {
			fmt.Printf("Failed to serve: %v\n", err)
		}
	}()

	fmt.Printf("Source %s started on %s\n", s.config.ID, s.config.Endpoint)
	s.health.SetHealth(flowctl.HealthStatusHealthy)

	return nil
}

// Stop stops the source
func (s *StandardSource) Stop() error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = false
	close(s.stopCh)
	s.mu.Unlock()

	s.health.SetHealth(flowctl.HealthStatusStopping)

	// Stop the controller if enabled
	if s.controller != nil {
		if err := s.controller.Stop(); err != nil {
			fmt.Printf("Error stopping controller: %v\n", err)
		}
	}

	// Stop the gRPC server
	if s.server != nil {
		s.server.GracefulStop()
	}

	// Wait for all streaming to complete
	s.streamingWg.Wait()

	// Stop the health server
	if err := s.health.Stop(); err != nil {
		fmt.Printf("Error stopping health server: %v\n", err)
	}

	fmt.Printf("Source %s stopped\n", s.config.ID)
	return nil
}

// GetMetrics returns the current metrics
func (s *StandardSource) GetMetrics() map[string]interface{} {
	return s.metrics.GetMetrics()
}

// Metrics returns the metrics interface
func (s *StandardSource) Metrics() flowctl.Metrics {
	return s.metrics
}

// StreamEvents implements the gRPC StreamEvents method
func (s *StandardSource) StreamEvents(req *flowctlv1.StreamRequest, stream flowctlv1.SourceService_StreamEventsServer) error {
	ctx := stream.Context()

	s.mu.Lock()
	if s.streaming {
		s.mu.Unlock()
		return ErrAlreadyStreaming
	}
	s.streaming = true
	producer := s.producer
	streamProducer := s.streamProducer
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.streaming = false
		s.mu.Unlock()
	}()

	if streamProducer != nil {
		return s.streamWithResult(ctx, req, stream, streamProducer)
	}
	if producer == nil {
		return ErrProducerNotSet
	}
	// Legacy behavior: EOF is successful, even for an unbounded producer.
	eventCh, err := producer(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to start producer: %w", err)
	}

	fmt.Printf("Source %s: Started streaming events\n", s.config.ID)

	// Stream events from channel
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("Source %s: Stream context cancelled\n", s.config.ID)
			return ctx.Err()
		case <-s.stopCh:
			fmt.Printf("Source %s: Source stopped\n", s.config.ID)
			return ErrSourceStopped
		case event, ok := <-eventCh:
			if !ok {
				// Channel closed, producer is done
				fmt.Printf("Source %s: Producer completed\n", s.config.ID)
				return nil
			}

			// Send event
			if err := stream.Send(event); err != nil {
				fmt.Printf("Source %s: Error sending event: %v\n", s.config.ID, err)
				return err
			}

			// Update metrics
			s.metrics.IncrementProcessedCount()
			s.metrics.IncrementSuccessCount()
		}
	}
}

// streamWithResult drains buffered events before observing the terminal result.
func (s *StandardSource) streamWithResult(ctx context.Context, req *flowctlv1.StreamRequest, stream flowctlv1.SourceService_StreamEventsServer, producer ProducerStreamFunc) error {
	ctx, cancel := context.WithCancel(ctx)
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-s.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()
	defer func() { cancel(); <-watcherDone }()

	result, err := producer(ctx, req)
	if ctx.Err() != nil {
		return streamStatus(ctx.Err())
	}
	if err != nil {
		return s.failStream(err)
	}
	if result == nil || result.Events == nil || result.Result == nil {
		return s.failStream(errors.New("producer returned incomplete stream"))
	}
	for {
		// Cancellation takes precedence over any buffered events.
		if err := ctx.Err(); err != nil {
			return streamStatus(err)
		}
		select {
		case <-ctx.Done():
			return streamStatus(ctx.Err())
		case event, ok := <-result.Events:
			if !ok {
				if err := ctx.Err(); err != nil {
					return streamStatus(err)
				}
				// Result must have been sent before Events was closed: never wait for a
				// broken producer after EOF, and detect missing/duplicate terminal values.
				select {
				case terminal, ok := <-result.Result:
					if !ok {
						return s.failStream(errors.New("missing terminal result"))
					}
					if len(result.Result) != 0 {
						return s.failStream(errors.New("multiple terminal results"))
					}
					if terminal != nil {
						if errors.Is(terminal, context.Canceled) && ctx.Err() != nil {
							return streamStatus(ctx.Err())
						}
						return s.failStream(terminal)
					}
					if !result.Bounded {
						return s.failStream(errors.New("unbounded producer ended without cancellation"))
					}
					return nil
				default:
					return s.failStream(errors.New("missing terminal result"))
				}
			}
			if err := ctx.Err(); err != nil {
				return streamStatus(err)
			}
			if event == nil {
				return s.failStream(errors.New("nil producer event"))
			}
			if err := stream.Send(event); err != nil {
				// Disconnect cancels the producer via the deferred cancel.
				fmt.Printf("Source %s: send failed: %v\n", s.config.ID, err)
				return streamStatus(&StreamError{Kind: DownstreamUnavailable, Cause: err})
			}
			s.metrics.IncrementProcessedCount()
			s.metrics.IncrementSuccessCount()
		}
	}
}

func (s *StandardSource) failStream(err error) error {
	fmt.Printf("Source %s: producer failed: %v\n", s.config.ID, err)
	s.metrics.IncrementErrorCount()
	var classified *StreamError
	if !errors.Is(err, context.Canceled) && (!errors.As(err, &classified) || classified.Kind != Canceled) {
		s.health.SetHealth(flowctl.HealthStatusUnhealthy)
	}
	return streamStatus(err)
}

// GetInfo implements the gRPC GetInfo method
func (s *StandardSource) GetInfo(ctx context.Context, _ *emptypb.Empty) (*flowctlv1.ComponentInfo, error) {
	return &flowctlv1.ComponentInfo{
		Id:               s.config.ID,
		Name:             s.config.Name,
		Description:      s.config.Description,
		Version:          s.config.Version,
		Type:             flowctlv1.ComponentType_COMPONENT_TYPE_SOURCE,
		OutputEventTypes: s.config.OutputEventTypes,
		Endpoint:         s.config.Endpoint,
		Metadata: map[string]string{
			"buffer_size": fmt.Sprintf("%d", s.config.BufferSize),
		},
	}, nil
}

// HealthCheck implements the gRPC HealthCheck method
func (s *StandardSource) HealthCheck(ctx context.Context, req *flowctlv1.HealthCheckRequest) (*flowctlv1.HealthCheckResponse, error) {
	status := s.health.GetHealth()

	return &flowctlv1.HealthCheckResponse{
		Status:  flowctlv1.HealthStatus(flowctlv1.HealthStatus_value[string(status)]),
		Message: fmt.Sprintf("Source %s is %s", s.config.ID, status),
	}, nil
}
