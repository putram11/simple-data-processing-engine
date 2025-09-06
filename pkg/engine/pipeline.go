package engine

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Pipeline orchestrates multiple processing stages
type Pipeline struct {
	name    string
	stages  []StageRunner
	metrics *MetricsCollector

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Configuration
	config PipelineConfig

	// State management
	state   PipelineState
	stateMu sync.RWMutex

	// Error handling
	errorCh chan error

	// Lifecycle hooks
	onStart []func() error
	onStop  []func() error
}

// PipelineState represents the current state of the pipeline
type PipelineState int

const (
	StateStopped PipelineState = iota
	StateStarting
	StateRunning
	StateStopping
	StateError
)

// String returns string representation of pipeline state
func (s PipelineState) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// PipelineConfig holds pipeline-level configuration
type PipelineConfig struct {
	Name                string        `yaml:"name" json:"name"`
	ShutdownTimeout     time.Duration `yaml:"shutdown_timeout" json:"shutdown_timeout"`
	ErrorBufferSize     int           `yaml:"error_buffer_size" json:"error_buffer_size"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval" json:"health_check_interval"`
}

// StageRunner interface abstracts different types of stages
type StageRunner interface {
	Start(input <-chan interface{}) <-chan interface{}
	Stop(timeout time.Duration) error
	GetMetrics() *StageMetrics
	Name() string
}

// GenericStage wraps a typed stage to implement StageRunner
type GenericStage[T, R any] struct {
	stage *Stage[T, R]
}

// NewGenericStage creates a generic stage wrapper
func NewGenericStage[T, R any](stage *Stage[T, R]) *GenericStage[T, R] {
	return &GenericStage[T, R]{stage: stage}
}

// Start converts between interface{} and typed channels
func (g *GenericStage[T, R]) Start(input <-chan interface{}) <-chan interface{} {
	typedInput := make(chan *Event[T], 100)
	genericOutput := make(chan interface{}, 100)
	
	// Convert input
	go func() {
		defer close(typedInput)
		for item := range input {
			if event, ok := item.(*Event[T]); ok {
				typedInput <- event
			}
		}
	}()
	
	// Start typed stage
	typedOutput := g.stage.Start(typedInput)
	
	// Convert output
	go func() {
		defer close(genericOutput)
		for item := range typedOutput {
			genericOutput <- item
		}
	}()
	
	return genericOutput
}

// Stop delegates to the wrapped stage
func (g *GenericStage[T, R]) Stop(timeout time.Duration) error {
	return g.stage.Stop(timeout)
}

// GetMetrics delegates to the wrapped stage
func (g *GenericStage[T, R]) GetMetrics() *StageMetrics {
	return g.stage.GetMetrics()
}

// Name delegates to the wrapped stage
func (g *GenericStage[T, R]) Name() string {
	return g.stage.Name()
}

// NewPipeline creates a new data processing pipeline
func NewPipeline(name string, config PipelineConfig) *Pipeline {
	if config.ShutdownTimeout == 0 {
		config.ShutdownTimeout = 30 * time.Second
	}
	if config.ErrorBufferSize == 0 {
		config.ErrorBufferSize = 100
	}
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 5 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Pipeline{
		name:    name,
		config:  config,
		metrics: NewMetricsCollector(),
		ctx:     ctx,
		cancel:  cancel,
		errorCh: make(chan error, config.ErrorBufferSize),
		state:   StateStopped,
	}
}

// AddStage appends a processing stage to the pipeline
func (p *Pipeline) AddStage(stage StageRunner) {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()

	if p.state != StateStopped {
		panic("cannot add stages to running pipeline")
	}

	p.stages = append(p.stages, stage)
	p.metrics.RegisterStage(stage.Name(), stage.GetMetrics())
}

// AddStartHook adds a function to be called when pipeline starts
func (p *Pipeline) AddStartHook(hook func() error) {
	p.onStart = append(p.onStart, hook)
}

// AddStopHook adds a function to be called when pipeline stops
func (p *Pipeline) AddStopHook(hook func() error) {
	p.onStop = append(p.onStop, hook)
}

// Start begins processing data through the pipeline
func (p *Pipeline) Start(source <-chan interface{}) error {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()

	if p.state != StateStopped {
		return fmt.Errorf("pipeline is already running or starting")
	}

	p.setState(StateStarting)

	// Execute start hooks
	for _, hook := range p.onStart {
		if err := hook(); err != nil {
			p.setState(StateError)
			return fmt.Errorf("start hook failed: %w", err)
		}
	}

	// Validate pipeline has stages
	if len(p.stages) == 0 {
		p.setState(StateError)
		return fmt.Errorf("pipeline has no stages")
	}

	// Start error handler
	p.wg.Add(1)
	go p.errorHandler()

	// Start health checker
	p.wg.Add(1)
	go p.healthChecker()

	// Chain stages together
	currentInput := source
	for i, stage := range p.stages {
		output := stage.Start(currentInput)
		currentInput = output

		// Start stage monitor
		p.wg.Add(1)
		go p.stageMonitor(stage, i)
	}

	// Start final output consumer
	p.wg.Add(1)
	go p.outputConsumer(currentInput)

	p.setState(StateRunning)
	return nil
}

// Stop gracefully shuts down the pipeline
func (p *Pipeline) Stop() error {
	p.stateMu.Lock()
	if p.state != StateRunning {
		p.stateMu.Unlock()
		return fmt.Errorf("pipeline is not running")
	}
	p.setState(StateStopping)
	p.stateMu.Unlock()

	// Cancel context to signal shutdown
	p.cancel()

	// Stop all stages in reverse order
	var stopErrors []error
	for i := len(p.stages) - 1; i >= 0; i-- {
		if err := p.stages[i].Stop(p.config.ShutdownTimeout); err != nil {
			stopErrors = append(stopErrors, fmt.Errorf("stage %s: %w", p.stages[i].Name(), err))
		}
	}

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Execute stop hooks
		for _, hook := range p.onStop {
			if err := hook(); err != nil {
				stopErrors = append(stopErrors, fmt.Errorf("stop hook failed: %w", err))
			}
		}

		p.stateMu.Lock()
		p.setState(StateStopped)
		p.stateMu.Unlock()

		if len(stopErrors) > 0 {
			return fmt.Errorf("shutdown errors: %v", stopErrors)
		}
		return nil

	case <-time.After(p.config.ShutdownTimeout):
		p.stateMu.Lock()
		p.setState(StateError)
		p.stateMu.Unlock()
		return fmt.Errorf("shutdown timeout exceeded")
	}
}

// GetState returns the current pipeline state
func (p *Pipeline) GetState() PipelineState {
	p.stateMu.RLock()
	defer p.stateMu.RUnlock()
	return p.state
}

// setState updates the pipeline state (must be called with lock held)
func (p *Pipeline) setState(state PipelineState) {
	p.state = state
}

// GetMetrics returns aggregated metrics for the pipeline
func (p *Pipeline) GetMetrics() AggregatedMetrics {
	return p.metrics.GetAggregatedMetrics(p.name)
}

// errorHandler processes errors from pipeline components
func (p *Pipeline) errorHandler() {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case err := <-p.errorCh:
			// Log error (in production, use structured logging)
			fmt.Printf("Pipeline %s error: %v\n", p.name, err)

			// Could implement error escalation logic here
			// For now, just log and continue
		}
	}
}

// healthChecker periodically checks pipeline health
func (p *Pipeline) healthChecker() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.performHealthCheck()
		}
	}
}

// performHealthCheck evaluates pipeline health
func (p *Pipeline) performHealthCheck() {
	metrics := p.GetMetrics()

	// Simple health check: if overall health < 50%, log warning
	if metrics.OverallHealth < 50 {
		fmt.Printf("Pipeline %s health warning: %.2f%%\n", p.name, metrics.OverallHealth)
	}

	// Check for stalled stages (no throughput)
	for _, stage := range metrics.StageMetrics {
		if stage.Throughput == 0 && stage.InputCount > 0 {
			fmt.Printf("Stage %s appears stalled (no output)\n", stage.Name)
		}
	}
}

// stageMonitor monitors individual stage health
func (p *Pipeline) stageMonitor(stage StageRunner, index int) {
	defer p.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			metrics := stage.GetMetrics().GetSnapshot()

			// Check for high error rate
			if metrics.ErrorRate > 10 { // More than 10 errors per second
				select {
				case p.errorCh <- fmt.Errorf("stage %s high error rate: %.2f/sec", stage.Name(), metrics.ErrorRate):
				default:
					// Error channel full, skip
				}
			}
		}
	}
}

// outputConsumer handles final pipeline output
func (p *Pipeline) outputConsumer(output <-chan interface{}) {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case event, ok := <-output:
			if !ok {
				return
			}

			// In a real implementation, this would send to final destination
			// For demo, we can just count or log
			_ = event
		}
	}
}

// PipelineBuilder provides a fluent interface for building pipelines
type PipelineBuilder struct {
	pipeline *Pipeline
}

// NewPipelineBuilder creates a new pipeline builder
func NewPipelineBuilder(name string) *PipelineBuilder {
	config := PipelineConfig{
		Name:            name,
		ShutdownTimeout: 30 * time.Second,
		ErrorBufferSize: 100,
	}

	return &PipelineBuilder{
		pipeline: NewPipeline(name, config),
	}
}

// WithConfig sets pipeline configuration
func (b *PipelineBuilder) WithConfig(config PipelineConfig) *PipelineBuilder {
	b.pipeline.config = config
	return b
}

// AddStage adds a processing stage
func (b *PipelineBuilder) AddStage(stage StageRunner) *PipelineBuilder {
	b.pipeline.AddStage(stage)
	return b
}

// AddStartHook adds a start hook
func (b *PipelineBuilder) AddStartHook(hook func() error) *PipelineBuilder {
	b.pipeline.AddStartHook(hook)
	return b
}

// AddStopHook adds a stop hook
func (b *PipelineBuilder) AddStopHook(hook func() error) *PipelineBuilder {
	b.pipeline.AddStopHook(hook)
	return b
}

// Build returns the constructed pipeline
func (b *PipelineBuilder) Build() *Pipeline {
	return b.pipeline
}
