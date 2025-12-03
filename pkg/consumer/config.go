package consumer

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// YAMLConsumerConfig represents the consumer section of the YAML config
type YAMLConsumerConfig struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
	Input       string `yaml:"input"`
	Output      string `yaml:"output"` // Empty for terminal consumers
}

// DatabaseConfig represents database connection configuration
type DatabaseConfig struct {
	Host           string `yaml:"host"`
	Port           int    `yaml:"port"`
	Database       string `yaml:"database"`
	User           string `yaml:"user"`
	Password       string `yaml:"password"`
	SSLMode        string `yaml:"ssl_mode"`
	MaxOpenConns   int    `yaml:"max_open_conns"`
	MaxIdleConns   int    `yaml:"max_idle_conns"`
	ConnectTimeout int    `yaml:"connect_timeout"` // seconds
}

// BatchConfig represents batching configuration
type BatchConfig struct {
	Enabled bool          `yaml:"enabled"`
	Size    int           `yaml:"size"`
	Timeout time.Duration `yaml:"timeout"`
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

// Config represents the complete consumer configuration
type Config struct {
	Consumer YAMLConsumerConfig `yaml:"consumer"`
	Database DatabaseConfig     `yaml:"database"`
	Batch    BatchConfig        `yaml:"batch"`
	Metrics  MetricsConfig      `yaml:"metrics"`
	Flowctl  FlowctlConfig      `yaml:"flowctl"`
	Health   HealthConfig       `yaml:"health"`
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Consumer: YAMLConsumerConfig{
			Name:        "Consumer",
			Version:     "1.0.0",
			Description: "Consumes events from the pipeline",
			Input:       "stellar.contract.events.v1",
			Output:      "", // Terminal consumer by default
		},
		Database: DatabaseConfig{
			Host:           "localhost",
			Port:           5432,
			Database:       "stellar_events",
			User:           "postgres",
			Password:       "",
			SSLMode:        "disable",
			MaxOpenConns:   25,
			MaxIdleConns:   5,
			ConnectTimeout: 10,
		},
		Batch: BatchConfig{
			Enabled: false,
			Size:    10,
			Timeout: 5 * time.Second,
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
			Port: 8089,
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
	// Consumer overrides
	if v := os.Getenv("COMPONENT_ID"); v != "" {
		if c.Consumer.Name == "Consumer" {
			c.Consumer.Name = v
		}
	}

	// Database overrides
	if v := os.Getenv("POSTGRES_HOST"); v != "" {
		c.Database.Host = v
	}
	if v := os.Getenv("POSTGRES_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Database.Port = port
		}
	}
	if v := os.Getenv("POSTGRES_DB"); v != "" {
		c.Database.Database = v
	}
	if v := os.Getenv("POSTGRES_USER"); v != "" {
		c.Database.User = v
	}
	if v := os.Getenv("POSTGRES_PASSWORD"); v != "" {
		c.Database.Password = v
	}
	if v := os.Getenv("POSTGRES_SSLMODE"); v != "" {
		c.Database.SSLMode = v
	}
	if v := os.Getenv("POSTGRES_MAX_OPEN_CONNS"); v != "" {
		if conns, err := strconv.Atoi(v); err == nil {
			c.Database.MaxOpenConns = conns
		}
	}
	if v := os.Getenv("POSTGRES_MAX_IDLE_CONNS"); v != "" {
		if conns, err := strconv.Atoi(v); err == nil {
			c.Database.MaxIdleConns = conns
		}
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

	// Batch overrides
	if v := os.Getenv("ENABLE_BATCHING"); v == "true" {
		c.Batch.Enabled = true
	}

	if v := os.Getenv("BATCH_SIZE"); v != "" {
		if size, err := strconv.Atoi(v); err == nil {
			c.Batch.Size = size
		}
	}

	// Metrics overrides
	if v := os.Getenv("ENABLE_METRICS"); v == "false" {
		c.Metrics.Enabled = false
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate batch configuration
	if c.Batch.Enabled {
		if c.Batch.Size <= 0 {
			return fmt.Errorf("batch size must be positive, got: %d", c.Batch.Size)
		}
	}

	// Database validation (if using database sink)
	if c.Database.Port <= 0 || c.Database.Port > 65535 {
		return fmt.Errorf("invalid database port: %d", c.Database.Port)
	}

	if c.Database.MaxOpenConns < 0 {
		return fmt.Errorf("max_open_conns must be non-negative, got: %d", c.Database.MaxOpenConns)
	}

	if c.Database.MaxIdleConns < 0 {
		return fmt.Errorf("max_idle_conns must be non-negative, got: %d", c.Database.MaxIdleConns)
	}

	return nil
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseInt(s string) int {
	var v int
	fmt.Sscanf(s, "%d", &v)
	return v
}

func toKebabCase(s string) string {
	// Simple conversion: lowercase and replace spaces with hyphens
	result := ""
	for i, r := range s {
		if r == ' ' {
			result += "-"
		} else if r >= 'A' && r <= 'Z' {
			if i > 0 && s[i-1] != ' ' {
				result += "-"
			}
			result += string(r + 32) // Convert to lowercase
		} else {
			result += string(r)
		}
	}
	return result
}
