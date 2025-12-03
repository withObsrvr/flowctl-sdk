package consumer

import (
	"github.com/withObsrvr/flowctl-sdk/pkg/flowctl"
)

// Config holds consumer configuration
type Config struct {
	// Consumer configuration
	ID          string
	Name        string
	Description string
	Version     string
	Endpoint    string

	// Input event types this consumer accepts
	InputEventTypes []string

	// Concurrency configuration
	MaxConcurrent int // Maximum concurrent event handlers

	// Flowctl configuration
	FlowctlConfig *flowctl.Config

	// Health check configuration
	HealthPort int
}

// DefaultConfig returns a default consumer configuration
func DefaultConfig() *Config {
	return &Config{
		ID:              "",
		Name:            "consumer",
		Description:     "A flowctl consumer",
		Version:         "1.0.0",
		Endpoint:        ":50051",
		InputEventTypes: []string{},
		MaxConcurrent:   10,
		FlowctlConfig: &flowctl.Config{
			Enabled:           false,
			Endpoint:          "localhost:8080",
			ServiceID:         "",
			ServiceType:       flowctl.ServiceTypeConsumer,
			HeartbeatInterval: flowctl.DefaultHeartbeatInterval,
			Metadata:          make(map[string]string),
		},
		HealthPort: 8088,
	}
}

// Validate validates the consumer configuration
func (c *Config) Validate() error {
	if c.ID == "" {
		c.ID = "consumer-" + generateID()
	}

	if c.FlowctlConfig != nil && c.FlowctlConfig.Enabled {
		// Ensure service ID is set
		if c.FlowctlConfig.ServiceID == "" {
			c.FlowctlConfig.ServiceID = c.ID
		}

		// Ensure service type is set
		if c.FlowctlConfig.ServiceType == "" {
			c.FlowctlConfig.ServiceType = flowctl.ServiceTypeConsumer
		}

		// Ensure metadata includes consumer info
		if c.FlowctlConfig.Metadata == nil {
			c.FlowctlConfig.Metadata = make(map[string]string)
		}

		c.FlowctlConfig.Metadata["consumer_id"] = c.ID
		c.FlowctlConfig.Metadata["consumer_name"] = c.Name
		c.FlowctlConfig.Metadata["consumer_version"] = c.Version
		c.FlowctlConfig.Metadata["consumer_description"] = c.Description
		c.FlowctlConfig.Metadata["endpoint"] = c.Endpoint

		// Pass event types to flowctl config
		c.FlowctlConfig.InputEventTypes = c.InputEventTypes
	}

	return nil
}
