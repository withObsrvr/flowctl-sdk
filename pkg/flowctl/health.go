package flowctl

import (
	"context"
	"encoding/json"
	"fmt"
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

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Health server error: %v\n", err)
		}
	}()

	fmt.Printf("Health server started on port %d\n", s.port)
	return nil
}

// Stop stops the health server
func (s *StandardHealthServer) Stop() error {
	if s.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.SetHealth(HealthStatusStopping)
	return s.server.Shutdown(ctx)
}