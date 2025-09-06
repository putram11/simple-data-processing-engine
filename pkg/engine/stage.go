package engine

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Event represents a data item flowing through the pipeline
type Event[T any] struct {
	ID        string    `json:"id"`
	Data      T         `json:"data"`
	Timestamp time.Time `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// NewEvent creates a new event with auto-generated ID
func NewEvent[T any](data T) *Event[T] {
	return &Event[T]{
		ID:        generateID(),
		Data:      data,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}
}

// Processor defines the interface for processing stages
type Processor[T, R any] interface {
	Process(ctx context.Context, event *Event[T]) (*Event[R], error)
	Name() string
}

// Stage represents a processing stage in the pipeline
type Stage[T, R any] struct {
	name      string
	processor Processor[T, R]
	workers   int
	bufferSize int
	metrics   *StageMetrics
	
	input  <-chan *Event[T]
	output chan *Event[R]
	
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

// StageConfig holds configuration for a stage
type StageConfig struct {
	Workers     int `yaml:"workers" json:"workers"`
	BufferSize  int `yaml:"buffer_size" json:"buffer_size"`
	Timeout     time.Duration `yaml:"timeout" json:"timeout"`
	RetryPolicy *RetryPolicy `yaml:"retry_policy" json:"retry_policy"`
}

// RetryPolicy defines retry behavior for failed operations
type RetryPolicy struct {
	MaxRetries int           `yaml:"max_retries" json:"max_retries"`
	Backoff    time.Duration `yaml:"backoff" json:"backoff"`
}

// NewStage creates a new processing stage
func NewStage[T, R any](name string, processor Processor[T, R], config StageConfig) *Stage[T, R] {
	if config.Workers <= 0 {
		config.Workers = 1
	}
	if config.BufferSize <= 0 {
		config.BufferSize = 100
	}

	ctx, cancel := context.WithCancel(context.Background())
	
	return &Stage[T, R]{
		name:       name,
		processor:  processor,
		workers:    config.Workers,
		bufferSize: config.BufferSize,
		metrics:    NewStageMetrics(name),
		output:     make(chan *Event[R], config.BufferSize),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start begins processing events in the stage
func (s *Stage[T, R]) Start(input <-chan *Event[T]) <-chan *Event[R] {
	s.input = input
	
	// Start worker goroutines
	for i := 0; i < s.workers; i++ {
		s.wg.Add(1)
		go s.worker(i)
	}
	
	// Start output closer goroutine
	go func() {
		s.wg.Wait()
		close(s.output)
	}()
	
	return s.output
}

// worker processes events from input channel
func (s *Stage[T, R]) worker(workerID int) {
	defer s.wg.Done()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case event, ok := <-s.input:
			if !ok {
				return
			}
			
			s.processEvent(event, workerID)
		}
	}
}

// processEvent handles a single event with metrics and error handling
func (s *Stage[T, R]) processEvent(event *Event[T], workerID int) {
	start := time.Now()
	
	// Update input metrics
	s.metrics.RecordInput()
	
	defer func() {
		// Record processing duration
		duration := time.Since(start)
		s.metrics.RecordDuration(duration)
	}()
	
	// Process the event with retry logic
	result, err := s.processWithRetry(event)
	if err != nil {
		s.metrics.RecordError()
		// Log error (in production, use structured logging)
		fmt.Printf("Stage %s worker %d error: %v\n", s.name, workerID, err)
		return
	}
	
	// Send result to output channel
	select {
	case s.output <- result:
		s.metrics.RecordOutput()
	case <-s.ctx.Done():
		return
	}
}

// processWithRetry implements retry logic for processing
func (s *Stage[T, R]) processWithRetry(event *Event[T]) (*Event[R], error) {
	var lastErr error
	
	for attempt := 0; attempt <= 3; attempt++ { // Default max 3 retries
		result, err := s.processor.Process(s.ctx, event)
		if err == nil {
			return result, nil
		}
		
		lastErr = err
		if attempt < 3 {
			// Exponential backoff
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
		}
	}
	
	return nil, fmt.Errorf("failed after retries: %w", lastErr)
}

// Stop gracefully shuts down the stage
func (s *Stage[T, R]) Stop(timeout time.Duration) error {
	s.cancel()
	
	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("stage %s shutdown timeout", s.name)
	}
}

// GetMetrics returns current metrics for the stage
func (s *Stage[T, R]) GetMetrics() *StageMetrics {
	return s.metrics
}

// generateID creates a unique identifier for events
func generateID() string {
	return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}
