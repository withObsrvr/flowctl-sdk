package source

import (
	"github.com/withObsrvr/flowctl-sdk/pkg/flowctl"
)

// Config holds source configuration
type Config struct {
	// Source configuration
	ID          string
	Name        string
	Description string
	Version     string
	Endpoint    string

	// Output event types this source produces
	OutputEventTypes []string

	// Concurrency configuration
	BufferSize int // Channel buffer size

	// Flowctl configuration
	FlowctlConfig *flowctl.Config

	// Health check configuration
	HealthPort int
}

// DefaultConfig returns a default source configuration
func DefaultConfig() *Config {
	return &Config{
		ID:               "",
		Name:             "source",
		Description:      "A flowctl source",
		Version:          "1.0.0",
		Endpoint:         ":50051",
		OutputEventTypes: []string{},
		BufferSize:       100,
		FlowctlConfig: &flowctl.Config{
			Enabled:           false,
			Endpoint:          "localhost:8080",
			ServiceID:         "",
			ServiceType:       flowctl.ServiceTypeSource,
			HeartbeatInterval: flowctl.DefaultHeartbeatInterval,
			Metadata:          make(map[string]string),
		},
		HealthPort: 8088,
	}
}

// Validate validates the source configuration
func (c *Config) Validate() error {
	if c.ID == "" {
		c.ID = "source-" + generateID()
	}

	if c.FlowctlConfig != nil && c.FlowctlConfig.Enabled {
		// Ensure service ID is set
		if c.FlowctlConfig.ServiceID == "" {
			c.FlowctlConfig.ServiceID = c.ID
		}

		// Ensure service type is set
		if c.FlowctlConfig.ServiceType == "" {
			c.FlowctlConfig.ServiceType = flowctl.ServiceTypeSource
		}

		// Ensure metadata includes source info
		if c.FlowctlConfig.Metadata == nil {
			c.FlowctlConfig.Metadata = make(map[string]string)
		}

		c.FlowctlConfig.Metadata["source_id"] = c.ID
		c.FlowctlConfig.Metadata["source_name"] = c.Name
		c.FlowctlConfig.Metadata["source_version"] = c.Version
		c.FlowctlConfig.Metadata["source_description"] = c.Description
		c.FlowctlConfig.Metadata["endpoint"] = c.Endpoint
	}

	return nil
}
