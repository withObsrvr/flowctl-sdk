package stellar

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// YAMLProcessorConfig represents the processor section of the YAML config
type YAMLProcessorConfig struct {
	Name        string   `yaml:"name"`
	Version     string   `yaml:"version"`
	Description string   `yaml:"description"`
	Input       string   `yaml:"input"`
	Output      string   `yaml:"output"`
}

// FilterConfig represents filtering configuration
type FilterConfig struct {
	Enabled      bool     `yaml:"enabled"`
	EventTypes   []string `yaml:"event_types"`
	MinAmount    *int64   `yaml:"min_amount"`
	ContractIDs  []string `yaml:"contract_ids"`
}

// BatchConfig represents batching configuration
type BatchConfig struct {
	Enabled bool          `yaml:"enabled"`
	Size    int           `yaml:"size"`
	Timeout time.Duration `yaml:"timeout"`
	Mode    string        `yaml:"mode"` // "synchronous" or "parallel"
}

// MetricsConfig represents metrics configuration
type MetricsConfig struct {
	Enabled     bool `yaml:"enabled"`
	Dimensional bool `yaml:"dimensional"`
}

// FlowctlConfig represents flowctl integration configuration
type FlowctlConfig struct {
	Enabled           bool          `yaml:"enabled"`
	Endpoint          string        `yaml:"endpoint"`
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`
}

// HealthConfig represents health check configuration
type HealthConfig struct {
	Port int `yaml:"port"`
}

// NetworkConfig represents network configuration
type NetworkConfig struct {
	Passphrase string `yaml:"passphrase"`
}

// Config represents the complete processor configuration
type Config struct {
	Processor YAMLProcessorConfig `yaml:"processor"`
	Filter    FilterConfig        `yaml:"filter"`
	Batch     BatchConfig         `yaml:"batch"`
	Metrics   MetricsConfig       `yaml:"metrics"`
	Flowctl   FlowctlConfig       `yaml:"flowctl"`
	Health    HealthConfig        `yaml:"health"`
	Network   NetworkConfig       `yaml:"network"`
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Processor: YAMLProcessorConfig{
			Name:        "Stellar Processor",
			Version:     "1.0.0",
			Description: "Processes Stellar ledger data",
			Input:       "stellar.ledger.v1",
			Output:      "stellar.token.transfer.v1",
		},
		Filter: FilterConfig{
			Enabled:     false,
			EventTypes:  []string{},
			MinAmount:   nil,
			ContractIDs: []string{},
		},
		Batch: BatchConfig{
			Enabled: false,
			Size:    10,
			Timeout: 5 * time.Second,
			Mode:    "synchronous",
		},
		Metrics: MetricsConfig{
			Enabled:     true,
			Dimensional: true,
		},
		Flowctl: FlowctlConfig{
			Enabled:           false,
			Endpoint:          "localhost:8080",
			HeartbeatInterval: 10 * time.Second,
		},
		Health: HealthConfig{
			Port: 8088,
		},
		Network: NetworkConfig{
			Passphrase: "",
		},
	}
}

// LoadConfig loads configuration from file and applies environment variable overrides
func LoadConfig(configPath string) (*Config, error) {
	config := DefaultConfig()

	// Try to load from file if it exists
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			data, err := os.ReadFile(configPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}

			if err := yaml.Unmarshal(data, config); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}
		}
	}

	// Apply environment variable overrides
	config.applyEnvOverrides()

	return config, nil
}

// applyEnvOverrides applies environment variable overrides to the config
func (c *Config) applyEnvOverrides() {
	// Processor overrides
	if v := os.Getenv("COMPONENT_ID"); v != "" {
		// Use component ID as name if not set
		if c.Processor.Name == "Stellar Processor" {
			c.Processor.Name = v
		}
	}

	// Network overrides (required)
	if v := os.Getenv("NETWORK_PASSPHRASE"); v != "" {
		c.Network.Passphrase = v
	}

	// Port overrides
	if v := os.Getenv("PORT"); v != "" {
		// Will be used by Run() function
	}

	if v := os.Getenv("HEALTH_PORT"); v != "" {
		// Will be parsed by Run() function
	}

	// Flowctl overrides
	if v := os.Getenv("ENABLE_FLOWCTL"); v == "true" {
		c.Flowctl.Enabled = true
	}

	if v := os.Getenv("FLOWCTL_ENDPOINT"); v != "" {
		c.Flowctl.Endpoint = v
	}

	if v := os.Getenv("FLOWCTL_HEARTBEAT_INTERVAL"); v != "" {
		if duration, err := time.ParseDuration(v); err == nil {
			c.Flowctl.HeartbeatInterval = duration
		}
	}

	// Filter overrides
	if v := os.Getenv("FILTER_EVENT_TYPES"); v != "" {
		// Will be handled by Run() function
		c.Filter.Enabled = true
	}

	if v := os.Getenv("MIN_AMOUNT"); v != "" {
		// Will be parsed by Run() function
		c.Filter.Enabled = true
	}

	if v := os.Getenv("FILTER_CONTRACT_IDS"); v != "" {
		// Will be handled by Run() function
		c.Filter.Enabled = true
	}

	// Batch overrides
	if v := os.Getenv("ENABLE_BATCHING"); v == "true" {
		c.Batch.Enabled = true
	}

	if v := os.Getenv("BATCH_SIZE"); v != "" {
		// Will be parsed by Run() function
	}

	// Metrics overrides
	if v := os.Getenv("ENABLE_METRICS"); v == "false" {
		c.Metrics.Enabled = false
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Network passphrase is required
	if c.Network.Passphrase == "" {
		return fmt.Errorf("NETWORK_PASSPHRASE environment variable is required")
	}

	// Validate batch mode
	if c.Batch.Enabled {
		if c.Batch.Mode != "synchronous" && c.Batch.Mode != "parallel" {
			return fmt.Errorf("batch mode must be 'synchronous' or 'parallel', got: %s", c.Batch.Mode)
		}

		if c.Batch.Size <= 0 {
			return fmt.Errorf("batch size must be positive, got: %d", c.Batch.Size)
		}
	}

	return nil
}
