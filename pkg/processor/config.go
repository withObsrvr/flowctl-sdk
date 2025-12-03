package processor

import (
	"github.com/withObsrvr/flowctl-sdk/pkg/flowctl"
)

// Config holds processor configuration
type Config struct {
	// Processor configuration
	ID          string
	Name        string
	Description string
	Version     string
	Endpoint    string

	// Event type configuration
	InputEventTypes  []string
	OutputEventTypes []string

	// Concurrency configuration
	MaxConcurrent int

	// Flowctl configuration
	FlowctlConfig *flowctl.Config

	// Health check configuration
	HealthPort int
}

// DefaultConfig returns a default processor configuration
func DefaultConfig() *Config {
	return &Config{
		ID:            "",
		Name:          "processor",
		Description:   "A flowctl processor",
		Version:       "1.0.0",
		Endpoint:      ":50051",
		MaxConcurrent: 100,
		FlowctlConfig: &flowctl.Config{
			Enabled:           false,
			Endpoint:          "localhost:8080",
			ServiceID:         "",
			ServiceType:       flowctl.ServiceTypeProcessor,
			HeartbeatInterval: flowctl.DefaultHeartbeatInterval,
			Metadata:          make(map[string]string),
		},
		HealthPort: 8088,
	}
}

// Validate validates the processor configuration
func (c *Config) Validate() error {
	if c.ID == "" {
		c.ID = "processor-" + generateID()
	}
	
	if c.FlowctlConfig != nil && c.FlowctlConfig.Enabled {
		// Ensure service ID is set
		if c.FlowctlConfig.ServiceID == "" {
			c.FlowctlConfig.ServiceID = c.ID
		}
		
		// Ensure service type is set
		if c.FlowctlConfig.ServiceType == "" {
			c.FlowctlConfig.ServiceType = flowctl.ServiceTypeProcessor
		}
		
		// Ensure metadata includes processor info
		if c.FlowctlConfig.Metadata == nil {
			c.FlowctlConfig.Metadata = make(map[string]string)
		}
		
		c.FlowctlConfig.Metadata["processor_id"] = c.ID
		c.FlowctlConfig.Metadata["processor_name"] = c.Name
		c.FlowctlConfig.Metadata["processor_version"] = c.Version
		c.FlowctlConfig.Metadata["processor_description"] = c.Description
		c.FlowctlConfig.Metadata["endpoint"] = c.Endpoint

		// Pass event types to flowctl config
		c.FlowctlConfig.InputEventTypes = c.InputEventTypes
		c.FlowctlConfig.OutputEventTypes = c.OutputEventTypes
	}

	return nil
}