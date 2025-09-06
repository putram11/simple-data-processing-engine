package engine

import (
	"sync"
	"sync/atomic"
	"time"
)

// StageMetrics holds performance metrics for a processing stage
type StageMetrics struct {
	name string
	
	// Counters (atomic for thread safety)
	inputCount   int64
	outputCount  int64
	errorCount   int64
	
	// Timing metrics
	totalDuration int64 // nanoseconds
	minDuration   int64 // nanoseconds
	maxDuration   int64 // nanoseconds
	
	// Rate tracking
	lastResetTime time.Time
	mu            sync.RWMutex
	
	// Worker pool metrics
	activeWorkers int32
	maxWorkers    int32
}

// NewStageMetrics creates a new metrics instance for a stage
func NewStageMetrics(name string) *StageMetrics {
	return &StageMetrics{
		name:          name,
		lastResetTime: time.Now(),
		minDuration:   int64(^uint64(0) >> 1), // max int64
	}
}

// RecordInput increments the input counter
func (m *StageMetrics) RecordInput() {
	atomic.AddInt64(&m.inputCount, 1)
}

// RecordOutput increments the output counter
func (m *StageMetrics) RecordOutput() {
	atomic.AddInt64(&m.outputCount, 1)
}

// RecordError increments the error counter
func (m *StageMetrics) RecordError() {
	atomic.AddInt64(&m.errorCount, 1)
}

// RecordDuration records processing duration and updates timing stats
func (m *StageMetrics) RecordDuration(duration time.Duration) {
	nanos := duration.Nanoseconds()
	
	atomic.AddInt64(&m.totalDuration, nanos)
	
	// Update min duration
	for {
		current := atomic.LoadInt64(&m.minDuration)
		if nanos >= current || atomic.CompareAndSwapInt64(&m.minDuration, current, nanos) {
			break
		}
	}
	
	// Update max duration
	for {
		current := atomic.LoadInt64(&m.maxDuration)
		if nanos <= current || atomic.CompareAndSwapInt64(&m.maxDuration, current, nanos) {
			break
		}
	}
}

// RecordWorkerStart increments active worker count
func (m *StageMetrics) RecordWorkerStart() {
	current := atomic.AddInt32(&m.activeWorkers, 1)
	
	// Update max workers if needed
	for {
		max := atomic.LoadInt32(&m.maxWorkers)
		if current <= max || atomic.CompareAndSwapInt32(&m.maxWorkers, max, current) {
			break
		}
	}
}

// RecordWorkerStop decrements active worker count
func (m *StageMetrics) RecordWorkerStop() {
	atomic.AddInt32(&m.activeWorkers, -1)
}

// Snapshot returns a point-in-time view of metrics
type MetricsSnapshot struct {
	Name string `json:"name"`
	
	// Counters
	InputCount  int64 `json:"input_count"`
	OutputCount int64 `json:"output_count"`
	ErrorCount  int64 `json:"error_count"`
	
	// Rates (per second)
	InputRate  float64 `json:"input_rate"`
	OutputRate float64 `json:"output_rate"`
	ErrorRate  float64 `json:"error_rate"`
	
	// Timing (milliseconds)
	AvgDuration time.Duration `json:"avg_duration"`
	MinDuration time.Duration `json:"min_duration"`
	MaxDuration time.Duration `json:"max_duration"`
	
	// Workers
	ActiveWorkers int32 `json:"active_workers"`
	MaxWorkers    int32 `json:"max_workers"`
	
	// Health
	SuccessRate  float64 `json:"success_rate"`
	Throughput   float64 `json:"throughput"`
	
	Timestamp time.Time `json:"timestamp"`
}

// GetSnapshot returns current metrics snapshot
func (m *StageMetrics) GetSnapshot() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	now := time.Now()
	elapsed := now.Sub(m.lastResetTime).Seconds()
	
	inputCount := atomic.LoadInt64(&m.inputCount)
	outputCount := atomic.LoadInt64(&m.outputCount)
	errorCount := atomic.LoadInt64(&m.errorCount)
	totalDuration := atomic.LoadInt64(&m.totalDuration)
	minDuration := atomic.LoadInt64(&m.minDuration)
	maxDuration := atomic.LoadInt64(&m.maxDuration)
	activeWorkers := atomic.LoadInt32(&m.activeWorkers)
	maxWorkers := atomic.LoadInt32(&m.maxWorkers)
	
	var avgDuration time.Duration
	if inputCount > 0 {
		avgDuration = time.Duration(totalDuration / inputCount)
	}
	
	var successRate float64
	if inputCount > 0 {
		successRate = float64(outputCount) / float64(inputCount) * 100
	}
	
	// Reset min duration if it's still at max value
	if minDuration == int64(^uint64(0)>>1) {
		minDuration = 0
	}
	
	return MetricsSnapshot{
		Name:          m.name,
		InputCount:    inputCount,
		OutputCount:   outputCount,
		ErrorCount:    errorCount,
		InputRate:     float64(inputCount) / elapsed,
		OutputRate:    float64(outputCount) / elapsed,
		ErrorRate:     float64(errorCount) / elapsed,
		AvgDuration:   avgDuration,
		MinDuration:   time.Duration(minDuration),
		MaxDuration:   time.Duration(maxDuration),
		ActiveWorkers: activeWorkers,
		MaxWorkers:    maxWorkers,
		SuccessRate:   successRate,
		Throughput:    float64(outputCount) / elapsed,
		Timestamp:     now,
	}
}

// Reset clears all metrics and resets timing
func (m *StageMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	atomic.StoreInt64(&m.inputCount, 0)
	atomic.StoreInt64(&m.outputCount, 0)
	atomic.StoreInt64(&m.errorCount, 0)
	atomic.StoreInt64(&m.totalDuration, 0)
	atomic.StoreInt64(&m.minDuration, int64(^uint64(0)>>1))
	atomic.StoreInt64(&m.maxDuration, 0)
	atomic.StoreInt32(&m.activeWorkers, 0)
	atomic.StoreInt32(&m.maxWorkers, 0)
	
	m.lastResetTime = time.Now()
}

// AggregatedMetrics holds metrics for the entire pipeline
type AggregatedMetrics struct {
	PipelineName   string            `json:"pipeline_name"`
	StageMetrics   []MetricsSnapshot `json:"stage_metrics"`
	TotalThroughput float64          `json:"total_throughput"`
	TotalLatency   time.Duration     `json:"total_latency"`
	OverallHealth  float64          `json:"overall_health"`
	Timestamp      time.Time        `json:"timestamp"`
}

// MetricsCollector aggregates metrics from multiple stages
type MetricsCollector struct {
	stages map[string]*StageMetrics
	mu     sync.RWMutex
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		stages: make(map[string]*StageMetrics),
	}
}

// RegisterStage adds a stage to metrics collection
func (c *MetricsCollector) RegisterStage(name string, metrics *StageMetrics) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stages[name] = metrics
}

// GetAggregatedMetrics returns combined metrics from all stages
func (c *MetricsCollector) GetAggregatedMetrics(pipelineName string) AggregatedMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	var snapshots []MetricsSnapshot
	var totalThroughput float64
	var totalLatency time.Duration
	var healthSum float64
	
	for _, metrics := range c.stages {
		snapshot := metrics.GetSnapshot()
		snapshots = append(snapshots, snapshot)
		totalThroughput += snapshot.Throughput
		totalLatency += snapshot.AvgDuration
		healthSum += snapshot.SuccessRate
	}
	
	var overallHealth float64
	if len(snapshots) > 0 {
		overallHealth = healthSum / float64(len(snapshots))
	}
	
	return AggregatedMetrics{
		PipelineName:    pipelineName,
		StageMetrics:    snapshots,
		TotalThroughput: totalThroughput,
		TotalLatency:    totalLatency,
		OverallHealth:   overallHealth,
		Timestamp:       time.Now(),
	}
}
