package consumer

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"sync/atomic"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ConsumerService implements the flowctl ConsumerService gRPC interface
type ConsumerService struct {
	flowctlv1.UnimplementedConsumerServiceServer

	config         *Config
	consumerName   string
	inputType      string
	onEvent        EventHandlerFunc
	processingWg   sync.WaitGroup
	stopCh         chan struct{}
	eventsConsumed atomic.Int64
	eventsFailed   atomic.Int64
	errorCount     atomic.Int64
	successCount   atomic.Int64
}

// NewConsumerService creates a new consumer service
func NewConsumerService(config *Config, consumerName, inputType string, onEvent EventHandlerFunc) *ConsumerService {
	return &ConsumerService{
		config:       config,
		consumerName: consumerName,
		inputType:    inputType,
		onEvent:      onEvent,
		stopCh:       make(chan struct{}),
	}
}

// GetInfo implements the gRPC GetInfo method
func (c *ConsumerService) GetInfo(ctx context.Context, _ *emptypb.Empty) (*flowctlv1.ComponentInfo, error) {
	return &flowctlv1.ComponentInfo{
		Id:               getEnv("COMPONENT_ID", toKebabCase(c.consumerName)),
		Name:             c.config.Consumer.Name,
		Description:      c.config.Consumer.Description,
		Version:          c.config.Consumer.Version,
		Type:             flowctlv1.ComponentType_COMPONENT_TYPE_CONSUMER,
		InputEventTypes:  []string{c.inputType},
		OutputEventTypes: []string{}, // Terminal consumer
		Endpoint:         getEnv("PORT", ":50052"),
		Metadata:         map[string]string{},
	}, nil
}

// HealthCheck implements the gRPC HealthCheck method
func (c *ConsumerService) HealthCheck(ctx context.Context, req *flowctlv1.HealthCheckRequest) (*flowctlv1.HealthCheckResponse, error) {
	return &flowctlv1.HealthCheckResponse{
		Status:  flowctlv1.HealthStatus_HEALTH_STATUS_HEALTHY,
		Metrics: c.getMetrics(),
	}, nil
}

// getMetrics returns current metrics
func (c *ConsumerService) getMetrics() map[string]string {
	return map[string]string{
		"events_consumed": fmt.Sprintf("%d", c.eventsConsumed.Load()),
		"events_failed":   fmt.Sprintf("%d", c.eventsFailed.Load()),
		"error_count":     fmt.Sprintf("%d", c.errorCount.Load()),
		"success_count":   fmt.Sprintf("%d", c.successCount.Load()),
	}
}

// Consume implements the gRPC Consume method (client streaming)
func (c *ConsumerService) Consume(stream flowctlv1.ConsumerService_ConsumeServer) error {
	ctx := stream.Context()

	var consumedCount int64
	var failedCount int64
	var errors []string

	for {
		select {
		case <-ctx.Done():
			// Return summary response
			return stream.SendAndClose(&flowctlv1.ConsumeResponse{
				EventsConsumed: consumedCount,
				EventsFailed:   failedCount,
				Errors:         errors,
			})
		case <-c.stopCh:
			// Return summary response
			return stream.SendAndClose(&flowctlv1.ConsumeResponse{
				EventsConsumed: consumedCount,
				EventsFailed:   failedCount,
				Errors:         errors,
			})
		default:
			// Receive an event
			event, err := stream.Recv()
			if err == io.EOF {
				// Stream ended, send summary
				return stream.SendAndClose(&flowctlv1.ConsumeResponse{
					EventsConsumed: consumedCount,
					EventsFailed:   failedCount,
					Errors:         errors,
				})
			}
			if err != nil {
				log.Printf("Error receiving event: %v", err)
				failedCount++
				errors = append(errors, fmt.Sprintf("receive error: %v", err))
				continue
			}

			// Only process events matching our input type
			if event.Type != c.inputType {
				log.Printf("Skipping event with non-matching type: %s (expected %s)", event.Type, c.inputType)
				continue
			}

			// Process the event
			c.processingWg.Add(1)
			go func(evt *flowctlv1.Event) {
				defer c.processingWg.Done()

				if err := c.onEvent(ctx, evt); err != nil {
					log.Printf("Error processing event: %v", err)
					c.eventsFailed.Add(1)
					c.errorCount.Add(1)
					// Note: can't add to errors slice here due to concurrency
				} else {
					c.eventsConsumed.Add(1)
					c.successCount.Add(1)
				}
			}(event)

			consumedCount++
		}
	}
}

// Stop stops the consumer service
func (c *ConsumerService) Stop() error {
	close(c.stopCh)
	c.processingWg.Wait()
	return nil
}
