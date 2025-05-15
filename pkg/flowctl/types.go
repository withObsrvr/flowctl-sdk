package flowctl

import (
	"context"
	"time"
)

// ServiceType defines the type of service being registered with flowctl
type ServiceType string

const (
	// ServiceTypeProcessor identifies the service as a processor
	ServiceTypeProcessor ServiceType = "processor"
	// ServiceTypeSource identifies the service as a source
	ServiceTypeSource ServiceType = "source"
	// ServiceTypeConsumer identifies the service as a consumer
	ServiceTypeConsumer ServiceType = "consumer"
)

// Config holds the configuration for flowctl integration
type Config struct {
	// Enabled determines whether flowctl integration is enabled
	Enabled bool

	// Endpoint is the endpoint for the flowctl control plane
	Endpoint string

	// ServiceID is a unique identifier for this service instance
	ServiceID string

	// ServiceType identifies the type of service (processor, source, or consumer)
	ServiceType ServiceType

	// HeartbeatInterval is the interval at which to send heartbeats
	HeartbeatInterval time.Duration

	// Metadata contains additional properties to send with registration
	Metadata map[string]string
}

// Controller manages interactions with the flowctl control plane
type Controller interface {
	// Register registers the service with the flowctl control plane
	Register(ctx context.Context) error

	// Start starts the controller, including heartbeat mechanism
	Start(ctx context.Context) error

	// Stop stops the controller gracefully
	Stop() error

	// UpdateMetrics updates the internal metrics to be reported in heartbeats
	UpdateMetrics(metrics map[string]interface{})

	// ServiceID returns the assigned service ID
	ServiceID() string
}

// Metrics interface defines the standard metrics methods
type Metrics interface {
	// IncrementProcessedCount increments the count of processed events
	IncrementProcessedCount()

	// IncrementErrorCount increments the count of errors encountered
	IncrementErrorCount()

	// IncrementSuccessCount increments the count of successfully processed events
	IncrementSuccessCount()

	// RecordProcessingLatency records the latency of processing an event
	RecordProcessingLatency(durationMs float64)

	// AddCounter adds a custom counter metric
	AddCounter(name string, value int64)

	// AddGauge adds a custom gauge metric
	AddGauge(name string, value float64)

	// GetMetrics returns the current metrics as a map
	GetMetrics() map[string]interface{}

	// Reset resets all the metrics to their initial values
	Reset()
}

// HealthStatus represents the health state of a service
type HealthStatus string

const (
	// HealthStatusUnknown indicates an unknown health status
	HealthStatusUnknown HealthStatus = "UNKNOWN"
	// HealthStatusStarting indicates the service is starting
	HealthStatusStarting HealthStatus = "STARTING"
	// HealthStatusHealthy indicates the service is healthy
	HealthStatusHealthy HealthStatus = "HEALTHY"
	// HealthStatusUnhealthy indicates the service is unhealthy
	HealthStatusUnhealthy HealthStatus = "UNHEALTHY"
	// HealthStatusStopping indicates the service is stopping
	HealthStatusStopping HealthStatus = "STOPPING"
)

// HealthProvider defines a service that reports health status
type HealthProvider interface {
	// GetHealth returns the current health status
	GetHealth() HealthStatus

	// SetHealth sets the current health status
	SetHealth(status HealthStatus)
}

// HealthServer provides HTTP health check endpoints
type HealthServer interface {
	// Start starts the health server
	Start() error

	// Stop stops the health server
	Stop() error

	// SetHealth sets the current health status
	SetHealth(status HealthStatus)

	// GetHealth returns the current health status
	GetHealth() HealthStatus
}