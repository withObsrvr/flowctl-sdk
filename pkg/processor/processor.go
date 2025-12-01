package processor

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
	ErrProcessorStopped = errors.New("processor has been stopped")
	ErrHandlerNotFound  = errors.New("no handler found for event type")
)

// Processor is the main interface for a flowctl processor
type Processor interface {
	// Start starts the processor
	Start(ctx context.Context) error
	
	// Stop stops the processor
	Stop() error
	
	// RegisterHandler registers a handler for processing events
	RegisterHandler(handler Handler) error
	
	// GetMetrics returns the current metrics
	GetMetrics() map[string]interface{}
	
	// Metrics returns the metrics interface
	Metrics() flowctl.Metrics
}

// StandardProcessor implements the Processor interface
type StandardProcessor struct {
	flowctlv1.UnimplementedProcessorServiceServer

	config     *Config
	registry   *HandlerRegistry
	server     *grpc.Server
	controller flowctl.Controller
	metrics    flowctl.Metrics
	health     flowctl.HealthServer

	mu          sync.RWMutex
	started     bool
	stopCh      chan struct{}
	processingWg sync.WaitGroup
}

// New creates a new processor
func New(config *Config) (*StandardProcessor, error) {
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

	return &StandardProcessor{
		config:     config,
		registry:   NewHandlerRegistry(),
		metrics:    metrics,
		controller: controller,
		health:     health,
		stopCh:     make(chan struct{}),
	}, nil
}

// RegisterHandler registers a handler for processing events
func (p *StandardProcessor) RegisterHandler(handler Handler) error {
	p.registry.Register(handler)
	return nil
}

// OnProcess registers a handler function
func (p *StandardProcessor) OnProcess(
	handleFunc HandlerFunc,
	inputTypes []string,
	outputTypes []string,
) error {
	handler := NewHandler(handleFunc, inputTypes, outputTypes)
	return p.RegisterHandler(handler)
}

// Start starts the processor
func (p *StandardProcessor) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return nil
	}
	p.started = true
	p.mu.Unlock()

	// Start health server
	p.health.SetHealth(flowctl.HealthStatusStarting)
	if err := p.health.Start(); err != nil {
		return fmt.Errorf("failed to start health server: %w", err)
	}

	// Register with flowctl if enabled
	if p.controller != nil {
		if err := p.controller.Register(ctx); err != nil {
			return fmt.Errorf("failed to register with flowctl: %w", err)
		}

		if err := p.controller.Start(ctx); err != nil {
			return fmt.Errorf("failed to start flowctl controller: %w", err)
		}
	}

	// Start gRPC server
	lis, err := net.Listen("tcp", p.config.Endpoint)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", p.config.Endpoint, err)
	}

	p.server = grpc.NewServer()

	// Register the processor service
	flowctlv1.RegisterProcessorServiceServer(p.server, p)

	// Enable reflection for development
	reflection.Register(p.server)

	// Start the server
	go func() {
		if err := p.server.Serve(lis); err != nil {
			fmt.Printf("Failed to serve: %v\n", err)
		}
	}()

	fmt.Printf("Processor %s started on %s\n", p.config.ID, p.config.Endpoint)
	p.health.SetHealth(flowctl.HealthStatusHealthy)

	return nil
}

// Stop stops the processor
func (p *StandardProcessor) Stop() error {
	p.mu.Lock()
	if !p.started {
		p.mu.Unlock()
		return nil
	}
	p.started = false
	close(p.stopCh)
	p.mu.Unlock()

	p.health.SetHealth(flowctl.HealthStatusStopping)

	// Stop the controller if enabled
	if p.controller != nil {
		if err := p.controller.Stop(); err != nil {
			fmt.Printf("Error stopping controller: %v\n", err)
		}
	}

	// Stop the gRPC server
	if p.server != nil {
		p.server.GracefulStop()
	}

	// Wait for all processing to complete
	p.processingWg.Wait()

	// Stop the health server
	if err := p.health.Stop(); err != nil {
		fmt.Printf("Error stopping health server: %v\n", err)
	}

	fmt.Printf("Processor %s stopped\n", p.config.ID)
	return nil
}

// GetMetrics returns the current metrics
func (p *StandardProcessor) GetMetrics() map[string]interface{} {
	return p.metrics.GetMetrics()
}

// Metrics returns the metrics interface
func (p *StandardProcessor) Metrics() flowctl.Metrics {
	return p.metrics
}

// Process implements the gRPC Process method
func (p *StandardProcessor) Process(stream flowctlv1.ProcessorService_ProcessServer) error {
	ctx := stream.Context()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.stopCh:
			return ErrProcessorStopped
		default:
			// Receive an event
			event, err := stream.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}

			// Process the event asynchronously
			p.processingWg.Add(1)
			go func(inputEvent *flowctlv1.Event) {
				defer p.processingWg.Done()

				startTime := time.Now()

				// Find handler for the event type
				handlers := p.registry.GetHandlersForType(inputEvent.Type)

				if len(handlers) == 0 {
					p.metrics.IncrementErrorCount()
					fmt.Printf("No handler for event type: %s\n", inputEvent.Type)
					return
				}

				// Use first handler for now (could implement more complex routing)
				handler := handlers[0]

				// Process the event
				outputEvent, err := handler.Handle(ctx, inputEvent)

				// Record metrics
				p.metrics.RecordProcessingLatency(float64(time.Since(startTime).Milliseconds()))
				p.metrics.IncrementProcessedCount()

				if err != nil {
					p.metrics.IncrementErrorCount()
					fmt.Printf("Error processing event: %v\n", err)
					return
				}

				// Skip nil events (processor chose to filter this event)
				if outputEvent == nil {
					return
				}

				p.metrics.IncrementSuccessCount()

				// Send the response
				if err := stream.Send(outputEvent); err != nil {
					fmt.Printf("Error sending response: %v\n", err)
				}
			}(event)
		}
	}
}

// GetInfo implements the gRPC GetInfo method
func (p *StandardProcessor) GetInfo(ctx context.Context, _ *emptypb.Empty) (*flowctlv1.ComponentInfo, error) {
	inputTypes := p.registry.GetAllInputTypes()
	outputTypes := p.registry.GetAllOutputTypes()

	return &flowctlv1.ComponentInfo{
		Id:               p.config.ID,
		Name:             p.config.Name,
		Description:      p.config.Description,
		Version:          p.config.Version,
		Type:             flowctlv1.ComponentType_COMPONENT_TYPE_PROCESSOR,
		InputEventTypes:  inputTypes,
		OutputEventTypes: outputTypes,
		Endpoint:         p.config.Endpoint,
		Metadata: map[string]string{
			"max_concurrent": fmt.Sprintf("%d", p.config.MaxConcurrent),
		},
	}, nil
}

// HealthCheck implements the gRPC HealthCheck method
func (p *StandardProcessor) HealthCheck(ctx context.Context, req *flowctlv1.HealthCheckRequest) (*flowctlv1.HealthCheckResponse, error) {
	status := p.health.GetHealth()

	return &flowctlv1.HealthCheckResponse{
		Status:  flowctlv1.HealthStatus(flowctlv1.HealthStatus_value[string(status)]),
		Message: fmt.Sprintf("Processor %s is %s", p.config.ID, status),
	}, nil
}