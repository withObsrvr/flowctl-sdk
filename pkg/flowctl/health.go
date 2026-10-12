package flowctl

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// StandardHealthServer implements the HealthServer interface
type StandardHealthServer struct {
	mu             sync.RWMutex
	status         HealthStatus
	port           int
	server         *http.Server
	listener       net.Listener
	metrics        Metrics
	startTime      time.Time
	readinessCheck func() bool
	livenessCheck  func() bool
}

// HealthResponse represents the response from health endpoints
type HealthResponse struct {
	Status    string                 `json:"status"`
	Uptime    string                 `json:"uptime"`
	Timestamp string                 `json:"timestamp"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
}

// NewHealthServer creates a new health server
func NewHealthServer(port int, metrics Metrics) *StandardHealthServer {
	return &StandardHealthServer{
		status:    HealthStatusStarting,
		port:      port,
		metrics:   metrics,
		startTime: time.Now(),
		readinessCheck: func() bool {
			return true
		},
		livenessCheck: func() bool {
			return true
		},
	}
}

// SetHealth sets the current health status
func (s *StandardHealthServer) SetHealth(status HealthStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

// GetHealth returns the current health status
func (s *StandardHealthServer) GetHealth() HealthStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// SetReadinessCheck sets a custom function for readiness checks
func (s *StandardHealthServer) SetReadinessCheck(check func() bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.readinessCheck = check
}

// SetLivenessCheck sets a custom function for liveness checks
func (s *StandardHealthServer) SetLivenessCheck(check func() bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.livenessCheck = check
}

// Start starts the health server
func (s *StandardHealthServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server != nil {
		return nil
	}
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return err
	}
	mux := http.NewServeMux()

	// Simple health endpoint - always returns 200 if the server is running
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := HealthResponse{
			Status:    string(s.GetHealth()),
			Uptime:    fmt.Sprintf("%s", time.Since(s.startTime).Round(time.Second)),
			Timestamp: time.Now().Format(time.RFC3339),
		}

		json.NewEncoder(w).Encode(response)
	})

	// Readiness probe - returns 200 only if the service is ready to accept traffic
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		s.mu.RLock()
		status := s.status
		readinessCheck := s.readinessCheck
		s.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")

		isReady := (status == HealthStatusHealthy) && readinessCheck()
		response := HealthResponse{
			Status:    string(status),
			Uptime:    fmt.Sprintf("%s", time.Since(s.startTime).Round(time.Second)),
			Timestamp: time.Now().Format(time.RFC3339),
		}

		if isReady {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		json.NewEncoder(w).Encode(response)
	})

	// Liveness probe - returns 200 if the service is alive, 500 if it's failing
	mux.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		s.mu.RLock()
		status := s.status
		livenessCheck := s.livenessCheck
		s.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")

		isLive := status != HealthStatusUnhealthy && livenessCheck()
		response := HealthResponse{
			Status:    string(status),
			Uptime:    fmt.Sprintf("%s", time.Since(s.startTime).Round(time.Second)),
			Timestamp: time.Now().Format(time.RFC3339),
		}

		if isLive {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}

		json.NewEncoder(w).Encode(response)
	})

	// Metrics endpoint - returns service metrics
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		var metrics map[string]interface{}
		if s.metrics != nil {
			metrics = s.metrics.GetMetrics()
		} else {
			metrics = make(map[string]interface{})
		}

		response := HealthResponse{
			Status:    string(s.GetHealth()),
			Uptime:    fmt.Sprintf("%s", time.Since(s.startTime).Round(time.Second)),
			Timestamp: time.Now().Format(time.RFC3339),
			Metrics:   metrics,
		}

		json.NewEncoder(w).Encode(response)
	})

	s.listener = listener
	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	server := s.server
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Health server error: %v\n", err)
		}
	}()

	fmt.Printf("Health server started on port %d\n", s.port)
	return nil
}

// Stop stops the health server
func (s *StandardHealthServer) Stop() error {
	s.mu.Lock()
	server := s.server
	listener := s.listener
	s.listener = nil
	s.server = nil
	s.status = HealthStatusStopping
	s.mu.Unlock()
	if server == nil {
		return nil
	}
	// Close the bound socket even if Serve has not yet started.
	_ = listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}
