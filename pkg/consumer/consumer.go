package consumer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/withObsrvr/flowctl-sdk/pkg/flowctl"
	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Error types
var (
	ErrConsumerStopped = errors.New("consumer has been stopped")
	ErrHandlerNotSet   = errors.New("handler function not set")
)

// HandlerFunc is a function that handles incoming events
// Return an error if the event cannot be processed
// The consumer will track success/error metrics automatically
type HandlerFunc func(ctx context.Context, event *flowctlv1.Event) error

// Consumer is the main interface for a flowctl consumer
type Consumer interface {
	// Start starts the consumer
	Start(ctx context.Context) error

	// Stop stops the consumer
	Stop() error

	// OnConsume registers the event handler function
	OnConsume(handler HandlerFunc) error

	// GetMetrics returns the current metrics
	GetMetrics() map[string]interface{}

	// Metrics returns the metrics interface
	Metrics() flowctl.Metrics
}

// StandardConsumer implements the Consumer interface
type StandardConsumer struct {
	flowctlv1.UnimplementedConsumerServiceServer

	config     *Config
	handler    HandlerFunc
	server     *grpc.Server
	controller flowctl.Controller
	metrics    flowctl.Metrics
	health     flowctl.HealthServer

	mu           sync.RWMutex
	started      bool
	stopCh       chan struct{}
	consumingWg  sync.WaitGroup
	semaphore    chan struct{} // For limiting concurrency
}

// New creates a new consumer
func New(config *Config) (*StandardConsumer, error) {
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

	// Create semaphore for concurrency control
	semaphore := make(chan struct{}, config.MaxConcurrent)

	return &StandardConsumer{
		config:     config,
		metrics:    metrics,
		controller: controller,
		health:     health,
		stopCh:     make(chan struct{}),
		semaphore:  semaphore,
	}, nil
}

// OnConsume registers the event handler function
func (c *StandardConsumer) OnConsume(handler HandlerFunc) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.handler = handler
	return nil
}

// Start starts the consumer
func (c *StandardConsumer) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return nil
	}
	c.started = true
	c.mu.Unlock()

	if c.handler == nil {
		return ErrHandlerNotSet
	}

	// Start health server
	c.health.SetHealth(flowctl.HealthStatusStarting)
	if err := c.health.Start(); err != nil {
		return fmt.Errorf("failed to start health server: %w", err)
	}

	// Register with flowctl if enabled
	if c.controller != nil {
		if err := c.controller.Register(ctx); err != nil {
			return fmt.Errorf("failed to register with flowctl: %w", err)
		}

		if err := c.controller.Start(ctx); err != nil {
			return fmt.Errorf("failed to start flowctl controller: %w", err)
		}
	}

	// Start gRPC server
	lis, err := net.Listen("tcp", c.config.Endpoint)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", c.config.Endpoint, err)
	}

	c.server = grpc.NewServer()

	// Register the consumer service
	flowctlv1.RegisterConsumerServiceServer(c.server, c)

	// Enable reflection for development
	reflection.Register(c.server)

	// Start the server
	go func() {
		if err := c.server.Serve(lis); err != nil {
			fmt.Printf("Failed to serve: %v\n", err)
		}
	}()

	fmt.Printf("Consumer %s started on %s\n", c.config.ID, c.config.Endpoint)
	c.health.SetHealth(flowctl.HealthStatusHealthy)

	return nil
}

// Stop stops the consumer
func (c *StandardConsumer) Stop() error {
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return nil
	}
	c.started = false
	close(c.stopCh)
	c.mu.Unlock()

	c.health.SetHealth(flowctl.HealthStatusStopping)

	// Stop the controller if enabled
	if c.controller != nil {
		if err := c.controller.Stop(); err != nil {
			fmt.Printf("Error stopping controller: %v\n", err)
		}
	}

	// Stop the gRPC server
	if c.server != nil {
		c.server.GracefulStop()
	}

	// Wait for all consuming to complete
	c.consumingWg.Wait()

	// Stop the health server
	if err := c.health.Stop(); err != nil {
		fmt.Printf("Error stopping health server: %v\n", err)
	}

	fmt.Printf("Consumer %s stopped\n", c.config.ID)
	return nil
}

// GetMetrics returns the current metrics
func (c *StandardConsumer) GetMetrics() map[string]interface{} {
	return c.metrics.GetMetrics()
}

// Metrics returns the metrics interface
func (c *StandardConsumer) Metrics() flowctl.Metrics {
	return c.metrics
}

// Consume implements the gRPC Consume method
func (c *StandardConsumer) Consume(stream flowctlv1.ConsumerService_ConsumeServer) error {
	ctx := stream.Context()

	eventsConsumed := int64(0)

	fmt.Printf("Consumer %s: Started consuming events\n", c.config.ID)

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("Consumer %s: Stream context cancelled\n", c.config.ID)
			return ctx.Err()
		case <-c.stopCh:
			fmt.Printf("Consumer %s: Consumer stopped\n", c.config.ID)
			return stream.SendAndClose(&flowctlv1.ConsumeResponse{
				EventsConsumed: eventsConsumed,
			})
		default:
			// Receive event
			event, err := stream.Recv()
			if err == io.EOF {
				// Stream closed
				fmt.Printf("Consumer %s: Stream closed, consumed %d events\n", c.config.ID, eventsConsumed)
				return stream.SendAndClose(&flowctlv1.ConsumeResponse{
					EventsConsumed: eventsConsumed,
				})
			}
			if err != nil {
				fmt.Printf("Consumer %s: Error receiving event: %v\n", c.config.ID, err)
				return err
			}

			// Acquire semaphore (limit concurrency)
			c.semaphore <- struct{}{}

			// Handle event asynchronously
			c.consumingWg.Add(1)
			go func(evt *flowctlv1.Event) {
				defer c.consumingWg.Done()
				defer func() { <-c.semaphore }() // Release semaphore

				startTime := time.Now()

				// Call handler
				err := c.handler(ctx, evt)

				// Record metrics
				c.metrics.RecordProcessingLatency(float64(time.Since(startTime).Milliseconds()))
				c.metrics.IncrementProcessedCount()

				if err != nil {
					c.metrics.IncrementErrorCount()
					fmt.Printf("Consumer %s: Error handling event %s: %v\n", c.config.ID, evt.Id, err)
				} else {
					c.metrics.IncrementSuccessCount()
				}
			}(event)

			eventsConsumed++
		}
	}
}

// GetInfo implements the gRPC GetInfo method
func (c *StandardConsumer) GetInfo(ctx context.Context, _ *emptypb.Empty) (*flowctlv1.ComponentInfo, error) {
	return &flowctlv1.ComponentInfo{
		Id:              c.config.ID,
		Name:            c.config.Name,
		Description:     c.config.Description,
		Version:         c.config.Version,
		Type:            flowctlv1.ComponentType_COMPONENT_TYPE_CONSUMER,
		InputEventTypes: c.config.InputEventTypes,
		Endpoint:        c.config.Endpoint,
		Metadata: map[string]string{
			"max_concurrent": fmt.Sprintf("%d", c.config.MaxConcurrent),
		},
	}, nil
}

// HealthCheck implements the gRPC HealthCheck method
func (c *StandardConsumer) HealthCheck(ctx context.Context, req *flowctlv1.HealthCheckRequest) (*flowctlv1.HealthCheckResponse, error) {
	status := c.health.GetHealth()

	return &flowctlv1.HealthCheckResponse{
		Status:  flowctlv1.HealthStatus(flowctlv1.HealthStatus_value[string(status)]),
		Message: fmt.Sprintf("Consumer %s is %s", c.config.ID, status),
	}, nil
}
