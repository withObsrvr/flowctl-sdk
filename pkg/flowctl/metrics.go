package flowctl

import (
	"sync"
	"time"
)

// StandardMetrics provides a standard implementation of the Metrics interface
type StandardMetrics struct {
	mu sync.RWMutex

	// Standard metrics
	processedCount    int64
	errorCount        int64
	successCount      int64
	processingLatency float64
	lastUpdateTime    time.Time

	// Custom metrics
	counters map[string]int64
	gauges   map[string]float64
}

// NewStandardMetrics creates a new instance of StandardMetrics
func NewStandardMetrics() *StandardMetrics {
	return &StandardMetrics{
		lastUpdateTime: time.Now(),
		counters:       make(map[string]int64),
		gauges:         make(map[string]float64),
	}
}

// IncrementProcessedCount increments the count of processed events
func (m *StandardMetrics) IncrementProcessedCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.processedCount++
	m.lastUpdateTime = time.Now()
}

// IncrementErrorCount increments the count of errors encountered
func (m *StandardMetrics) IncrementErrorCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorCount++
	m.lastUpdateTime = time.Now()
}

// IncrementSuccessCount increments the count of successfully processed events
func (m *StandardMetrics) IncrementSuccessCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.successCount++
	m.lastUpdateTime = time.Now()
}

// RecordProcessingLatency records the latency of processing an event
func (m *StandardMetrics) RecordProcessingLatency(durationMs float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Using a simple moving average for now
	if m.processedCount > 0 {
		m.processingLatency = (m.processingLatency + durationMs) / 2
	} else {
		m.processingLatency = durationMs
	}
	m.lastUpdateTime = time.Now()
}

// AddCounter adds a custom counter metric
func (m *StandardMetrics) AddCounter(name string, value int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name] = value
	m.lastUpdateTime = time.Now()
}

// AddGauge adds a custom gauge metric
func (m *StandardMetrics) AddGauge(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gauges[name] = value
	m.lastUpdateTime = time.Now()
}

// GetMetrics returns the current metrics as a map
func (m *StandardMetrics) GetMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := map[string]interface{}{
		"processed_count":     m.processedCount,
		"error_count":         m.errorCount,
		"success_count":       m.successCount,
		"processing_latency":  m.processingLatency,
		"last_update_time":    m.lastUpdateTime.Unix(),
		"uptime_seconds":      time.Since(m.lastUpdateTime).Seconds(),
	}

	// Add custom counters
	for name, value := range m.counters {
		metrics["counter_"+name] = value
	}

	// Add custom gauges
	for name, value := range m.gauges {
		metrics["gauge_"+name] = value
	}

	return metrics
}

// Reset resets all the metrics to their initial values
func (m *StandardMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.processedCount = 0
	m.errorCount = 0
	m.successCount = 0
	m.processingLatency = 0
	m.lastUpdateTime = time.Now()
	m.counters = make(map[string]int64)
	m.gauges = make(map[string]float64)
}