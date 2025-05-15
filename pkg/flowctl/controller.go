package flowctl

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	
	// We'll mock these for now - they should be imported from the flow-proto module
	// controlplanepb "github.com/withObsrvr/flow-proto/proto/controlplane"
)

// DefaultHeartbeatInterval is the default interval for sending heartbeats
const DefaultHeartbeatInterval = 30 * time.Second

// errors
var (
	ErrFlowctlNotEnabled = errors.New("flowctl integration is not enabled")
	ErrNotRegistered     = errors.New("service not registered with flowctl")
)

// StandardController implements the Controller interface
type StandardController struct {
	config           Config
	serviceID        string
	conn             *grpc.ClientConn
	// client           controlplanepb.ControlPlaneClient
	stopHeartbeat    chan struct{}
	heartbeatRunning bool
	metrics          Metrics
	healthServer     HealthServer

	mu             sync.RWMutex
	customMetrics  map[string]interface{}
}

// NewController creates a new flowctl controller
func NewController(config Config, metrics Metrics, healthServer HealthServer) *StandardController {
	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = DefaultHeartbeatInterval
	}

	if config.ServiceID == "" {
		config.ServiceID = generateServiceID(string(config.ServiceType))
	}

	return &StandardController{
		config:          config,
		serviceID:       config.ServiceID,
		stopHeartbeat:   make(chan struct{}),
		metrics:         metrics,
		healthServer:    healthServer,
		customMetrics:   make(map[string]interface{}),
	}
}

// Register registers the service with the flowctl control plane
func (c *StandardController) Register(ctx context.Context) error {
	if !c.config.Enabled {
		return ErrFlowctlNotEnabled
	}

	// Connect to the flowctl control plane
	conn, err := grpc.DialContext(
		ctx,
		c.config.Endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to flowctl at %s: %w", c.config.Endpoint, err)
	}
	c.conn = conn

	// Create the client
	// c.client = controlplanepb.NewControlPlaneClient(conn)

	// Register based on service type
	switch c.config.ServiceType {
	case ServiceTypeProcessor:
		return c.registerProcessor(ctx)
	case ServiceTypeSource:
		return c.registerSource(ctx)
	case ServiceTypeConsumer:
		return c.registerConsumer(ctx)
	default:
		return fmt.Errorf("unknown service type: %s", c.config.ServiceType)
	}
}

// registerProcessor registers a processor with the flowctl control plane
func (c *StandardController) registerProcessor(ctx context.Context) error {
	/*
	// Mocked for now - should use actual proto implementation
	request := &controlplanepb.RegisterProcessorRequest{
		ProcessorId: c.serviceID,
		Endpoint:    c.config.Metadata["endpoint"],
		Metadata:    c.config.Metadata,
	}

	resp, err := c.client.RegisterProcessor(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to register processor: %w", err)
	}

	// Update service ID if provided by control plane
	if resp.ProcessorId != "" {
		c.serviceID = resp.ProcessorId
	}
	*/

	// Mock implementation
	fmt.Printf("Registered processor with ID: %s to flowctl at %s\n", c.serviceID, c.config.Endpoint)
	return nil
}

// registerSource registers a source with the flowctl control plane
func (c *StandardController) registerSource(ctx context.Context) error {
	/*
	// Mocked for now - should use actual proto implementation
	request := &controlplanepb.RegisterSourceRequest{
		SourceId:  c.serviceID,
		Endpoint:  c.config.Metadata["endpoint"],
		Metadata:  c.config.Metadata,
	}

	resp, err := c.client.RegisterSource(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to register source: %w", err)
	}

	// Update service ID if provided by control plane
	if resp.SourceId != "" {
		c.serviceID = resp.SourceId
	}
	*/

	// Mock implementation
	fmt.Printf("Registered source with ID: %s to flowctl at %s\n", c.serviceID, c.config.Endpoint)
	return nil
}

// registerConsumer registers a consumer with the flowctl control plane
func (c *StandardController) registerConsumer(ctx context.Context) error {
	/*
	// Mocked for now - should use actual proto implementation
	request := &controlplanepb.RegisterConsumerRequest{
		ConsumerId: c.serviceID,
		Endpoint:   c.config.Metadata["endpoint"],
		Metadata:   c.config.Metadata,
	}

	resp, err := c.client.RegisterConsumer(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	// Update service ID if provided by control plane
	if resp.ConsumerId != "" {
		c.serviceID = resp.ConsumerId
	}
	*/

	// Mock implementation
	fmt.Printf("Registered consumer with ID: %s to flowctl at %s\n", c.serviceID, c.config.Endpoint)
	return nil
}

// Start starts the controller, including heartbeat mechanism
func (c *StandardController) Start(ctx context.Context) error {
	if !c.config.Enabled {
		return nil
	}

	if c.conn == nil {
		return ErrNotRegistered
	}

	// Start health server
	if c.healthServer != nil {
		if err := c.healthServer.Start(); err != nil {
			return fmt.Errorf("failed to start health server: %w", err)
		}
	}

	// Start heartbeat goroutine
	c.mu.Lock()
	if !c.heartbeatRunning {
		c.heartbeatRunning = true
		go c.startHeartbeat(ctx)
	}
	c.mu.Unlock()

	if c.healthServer != nil {
		c.healthServer.SetHealth(HealthStatusHealthy)
	}

	return nil
}

// Stop stops the controller gracefully
func (c *StandardController) Stop() error {
	if !c.config.Enabled {
		return nil
	}

	// Stop health server
	if c.healthServer != nil {
		c.healthServer.SetHealth(HealthStatusStopping)
		if err := c.healthServer.Stop(); err != nil {
			fmt.Printf("Error stopping health server: %v\n", err)
		}
	}

	// Stop heartbeat
	c.mu.Lock()
	if c.heartbeatRunning {
		c.heartbeatRunning = false
		close(c.stopHeartbeat)
	}
	c.mu.Unlock()

	// Close gRPC connection
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			return fmt.Errorf("error closing gRPC connection: %w", err)
		}
		c.conn = nil
	}

	return nil
}

// UpdateMetrics updates the internal metrics to be reported in heartbeats
func (c *StandardController) UpdateMetrics(metrics map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.customMetrics = metrics
}

// ServiceID returns the assigned service ID
func (c *StandardController) ServiceID() string {
	return c.serviceID
}

// startHeartbeat runs a goroutine to send heartbeats periodically
func (c *StandardController) startHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopHeartbeat:
			return
		case <-ticker.C:
			c.sendHeartbeat(ctx)
		}
	}
}

// sendHeartbeat sends a heartbeat to the flowctl control plane
func (c *StandardController) sendHeartbeat(ctx context.Context) {
	if c.conn == nil {
		return
	}

	// Collect all metrics
	allMetrics := make(map[string]interface{})
	
	// Add standard metrics if available
	if c.metrics != nil {
		for k, v := range c.metrics.GetMetrics() {
			allMetrics[k] = v
		}
	}
	
	// Add custom metrics
	c.mu.RLock()
	for k, v := range c.customMetrics {
		allMetrics[k] = v
	}
	c.mu.RUnlock()

	/*
	// Convert metrics to protobuf format
	protoMetrics := make(map[string]float64)
	for k, v := range allMetrics {
		switch val := v.(type) {
		case int:
			protoMetrics[k] = float64(val)
		case int64:
			protoMetrics[k] = float64(val)
		case float32:
			protoMetrics[k] = float64(val)
		case float64:
			protoMetrics[k] = val
		default:
			// Skip non-numeric metrics
		}
	}
	*/

	// Send heartbeat based on service type
	switch c.config.ServiceType {
	case ServiceTypeProcessor:
		c.sendProcessorHeartbeat(ctx, allMetrics)
	case ServiceTypeSource:
		c.sendSourceHeartbeat(ctx, allMetrics)
	case ServiceTypeConsumer:
		c.sendConsumerHeartbeat(ctx, allMetrics)
	}
}

// sendProcessorHeartbeat sends a processor heartbeat
func (c *StandardController) sendProcessorHeartbeat(ctx context.Context, metrics map[string]interface{}) {
	/*
	// Mocked for now - should use actual proto implementation
	request := &controlplanepb.ProcessorHeartbeatRequest{
		ProcessorId: c.serviceID,
		Timestamp:   timestamppb.Now(),
		Metrics:     protoMetrics,
		Status:      string(c.healthServer.GetHealth()),
	}

	_, err := c.client.ProcessorHeartbeat(ctx, request)
	if err != nil {
		fmt.Printf("Error sending processor heartbeat: %v\n", err)
		return
	}
	*/

	// Mock implementation
	fmt.Printf("Sent processor heartbeat for ID %s with %d metrics\n", c.serviceID, len(metrics))
}

// sendSourceHeartbeat sends a source heartbeat
func (c *StandardController) sendSourceHeartbeat(ctx context.Context, metrics map[string]interface{}) {
	/*
	// Mocked for now - should use actual proto implementation
	request := &controlplanepb.SourceHeartbeatRequest{
		SourceId:  c.serviceID,
		Timestamp: timestamppb.Now(),
		Metrics:   protoMetrics,
		Status:    string(c.healthServer.GetHealth()),
	}

	_, err := c.client.SourceHeartbeat(ctx, request)
	if err != nil {
		fmt.Printf("Error sending source heartbeat: %v\n", err)
		return
	}
	*/

	// Mock implementation
	fmt.Printf("Sent source heartbeat for ID %s with %d metrics\n", c.serviceID, len(metrics))
}

// sendConsumerHeartbeat sends a consumer heartbeat
func (c *StandardController) sendConsumerHeartbeat(ctx context.Context, metrics map[string]interface{}) {
	/*
	// Mocked for now - should use actual proto implementation
	request := &controlplanepb.ConsumerHeartbeatRequest{
		ConsumerId: c.serviceID,
		Timestamp:  timestamppb.Now(),
		Metrics:    protoMetrics,
		Status:     string(c.healthServer.GetHealth()),
	}

	_, err := c.client.ConsumerHeartbeat(ctx, request)
	if err != nil {
		fmt.Printf("Error sending consumer heartbeat: %v\n", err)
		return
	}
	*/

	// Mock implementation
	fmt.Printf("Sent consumer heartbeat for ID %s with %d metrics\n", c.serviceID, len(metrics))
}

// Helper function to generate a random service ID
func generateServiceID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}