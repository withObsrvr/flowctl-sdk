// Package consumer provides a zero-config wrapper for building event consumers.
//
// Example usage:
//
//	import (
//	    "github.com/withObsrvr/flowctl-sdk/pkg/consumer"
//	    flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
//	)
//
//	func main() {
//	    consumer.Run(consumer.ConsumerConfig{
//	        ConsumerName: "PostgreSQL Consumer",
//	        InputType:    "stellar.contract.events.v1",
//	        OnEvent: func(ctx context.Context, event *flowctlv1.Event) error {
//	            // Process the event
//	            return processEvent(event)
//	        },
//	    })
//	}
//
// That's it! All configuration is handled through environment variables or an optional consumer.yaml file.
package consumer

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// EventHandlerFunc is a function that processes an incoming event.
// The function receives the event and should return an error if processing fails.
type EventHandlerFunc func(ctx context.Context, event *flowctlv1.Event) error

// ConsumerConfig defines the configuration for a generic consumer.
type ConsumerConfig struct {
	// ConsumerName is the human-readable name of the consumer (e.g., "PostgreSQL Consumer")
	ConsumerName string

	// InputType is the event type this consumer subscribes to (e.g., "stellar.contract.events.v1")
	InputType string

	// OnEvent is the function that processes each event
	OnEvent EventHandlerFunc

	// Optional: Custom configuration file path (defaults to "consumer.yaml")
	ConfigPath string

	// Optional: Component ID (defaults to consumer name in kebab-case)
	ComponentID string

	// Optional: Output event type if consumer forwards events (empty = terminal consumer)
	OutputType string
}

// Run starts a generic consumer with zero configuration required.
//
// Configuration is loaded from:
// 1. Default values
// 2. consumer.yaml file (if present)
// 3. Environment variables (override file)
//
// Required environment variables: None (all have defaults)
//
// Optional environment variables:
//   - COMPONENT_ID: Component identifier (default: derived from ConsumerName)
//   - PORT: gRPC server port (default: ":50052")
//   - HEALTH_PORT: Health check port (default: "8089")
//   - ENABLE_FLOWCTL: Enable flowctl integration (default: "false")
//   - FLOWCTL_ENDPOINT: Flowctl endpoint (default: "localhost:8080")
//   - ENABLE_METRICS: Enable metrics (default: "true")
func Run(cfg ConsumerConfig) {
	// Load configuration
	configPath := cfg.ConfigPath
	if configPath == "" {
		configPath = "consumer.yaml"
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Set consumer-specific config
	if cfg.ConsumerName != "" {
		config.Consumer.Name = cfg.ConsumerName
	}
	if cfg.InputType != "" {
		config.Consumer.Input = cfg.InputType
	}
	if cfg.OutputType != "" {
		config.Consumer.Output = cfg.OutputType
	}

	// Get configuration from environment with defaults
	componentID := cfg.ComponentID
	if componentID == "" {
		componentID = getEnv("COMPONENT_ID", toKebabCase(config.Consumer.Name))
	}
	port := getEnv("PORT", ":50052")
	healthPort := getEnv("HEALTH_PORT", "8089")

	log.Printf("Starting %s", config.Consumer.Name)
	log.Printf("Input type: %s", cfg.InputType)
	log.Printf("Terminal consumer (no output)")

	// Create consumer service
	consumerService := NewConsumerService(config, cfg.ConsumerName, cfg.InputType, cfg.OnEvent)

	// Start gRPC server
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", port, err)
	}

	// Create gRPC server with large message limits
	maxMsgSize := 50 * 1024 * 1024 // 50MB
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(maxMsgSize),
		grpc.MaxSendMsgSize(maxMsgSize),
	)

	// Register consumer service
	flowctlv1.RegisterConsumerServiceServer(grpcServer, consumerService)

	// Register health service
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// Start server in background
	go func() {
		log.Printf("%s is running", config.Consumer.Name)
		log.Printf("Endpoint: %s", port)
		log.Printf("Health port: %s", healthPort)
		if config.Flowctl.Enabled {
			log.Printf("Flowctl enabled: %s", config.Flowctl.Endpoint)
		}

		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Register with flowctl control plane if enabled
	if config.Flowctl.Enabled {
		go func() {
			// Connect to control plane
			conn, err := grpc.Dial(config.Flowctl.Endpoint,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				log.Fatalf("Failed to connect to flowctl: %v", err)
			}
			defer conn.Close()

			client := flowctlv1.NewControlPlaneServiceClient(conn)

			// Register consumer
			_, err = client.RegisterComponent(context.Background(), &flowctlv1.RegisterRequest{
				ComponentId: componentID,
				Component: &flowctlv1.ComponentInfo{
					Id:               componentID,
					Name:             config.Consumer.Name,
					Description:      config.Consumer.Description,
					Version:          config.Consumer.Version,
					Type:             flowctlv1.ComponentType_COMPONENT_TYPE_CONSUMER,
					InputEventTypes:  []string{cfg.InputType},
					OutputEventTypes: []string{},
					Endpoint:         port,
					Metadata:         map[string]string{"health_port": healthPort},
				},
			})
			if err != nil {
				log.Fatalf("Failed to register with flowctl: %v", err)
			}

			log.Printf("Registered with flowctl control plane")

			// Send heartbeats
			ticker := time.NewTicker(time.Duration(config.Flowctl.HeartbeatInterval) * time.Millisecond)
			defer ticker.Stop()

			for range ticker.C {
				_, err := client.Heartbeat(context.Background(), &flowctlv1.HeartbeatRequest{
					ServiceId: componentID,
				})
				if err != nil {
					log.Printf("Failed to send heartbeat: %v", err)
				}
			}
		}()
	}

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Stop the consumer gracefully
	log.Printf("Shutting down %s...", config.Consumer.Name)
	if err := consumerService.Stop(); err != nil {
		log.Printf("Error stopping consumer service: %v", err)
	}
	grpcServer.GracefulStop()
	log.Printf("%s stopped successfully", config.Consumer.Name)
}
