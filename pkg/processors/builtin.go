package processors

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"simple-data-processing-engine/pkg/engine"
	"strconv"
	"strings"
	"time"
)

// NumberData represents numeric data flowing through the pipeline
type NumberData struct {
	Value  float64 `json:"value"`
	Source string  `json:"source"`
}

// StringData represents string data
type StringData struct {
	Text   string `json:"text"`
	Length int    `json:"length"`
}

// AggregateData represents aggregated numeric data
type AggregateData struct {
	Count   int64   `json:"count"`
	Sum     float64 `json:"sum"`
	Average float64 `json:"average"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	StdDev  float64 `json:"std_dev"`
}

// FilterProcessor filters events based on a predicate function
type FilterProcessor[T any] struct {
	name      string
	predicate func(*engine.Event[T]) bool
}

// NewFilterProcessor creates a new filter processor
func NewFilterProcessor[T any](name string, predicate func(*engine.Event[T]) bool) *FilterProcessor[T] {
	return &FilterProcessor[T]{
		name:      name,
		predicate: predicate,
	}
}

// Process filters events based on the predicate
func (p *FilterProcessor[T]) Process(ctx context.Context, event *engine.Event[T]) (*engine.Event[T], error) {
	if event == nil {
		return nil, fmt.Errorf("received nil event")
	}

	if p.predicate(event) {
		return event, nil
	}
	return nil, nil // Filtered out
}

// Name returns the processor name
func (p *FilterProcessor[T]) Name() string {
	return p.name
}

// TransformProcessor transforms data from one type to another
type TransformProcessor[T, R any] struct {
	name      string
	transform func(*engine.Event[T]) (*engine.Event[R], error)
}

// NewTransformProcessor creates a new transform processor
func NewTransformProcessor[T, R any](name string, transform func(*engine.Event[T]) (*engine.Event[R], error)) *TransformProcessor[T, R] {
	return &TransformProcessor[T, R]{
		name:      name,
		transform: transform,
	}
}

// Process transforms the event data
func (p *TransformProcessor[T, R]) Process(ctx context.Context, event *engine.Event[T]) (*engine.Event[R], error) {
	return p.transform(event)
}

// Name returns the processor name
func (p *TransformProcessor[T, R]) Name() string {
	return p.name
}

// BatchProcessor processes events in batches
type BatchProcessor[T, R any] struct {
	name      string
	batchSize int
	timeout   time.Duration
	process   func([]*engine.Event[T]) ([]*engine.Event[R], error)

	buffer []*engine.Event[T]
	timer  *time.Timer
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor[T, R any](name string, batchSize int, timeout time.Duration,
	process func([]*engine.Event[T]) ([]*engine.Event[R], error)) *BatchProcessor[T, R] {
	return &BatchProcessor[T, R]{
		name:      name,
		batchSize: batchSize,
		timeout:   timeout,
		process:   process,
		buffer:    make([]*engine.Event[T], 0, batchSize),
	}
}

// Process accumulates events into batches
func (p *BatchProcessor[T, R]) Process(ctx context.Context, event *engine.Event[T]) (*engine.Event[R], error) {
	// This is a simplified batch processor
	// In a real implementation, you'd need more sophisticated batch management
	p.buffer = append(p.buffer, event)

	if len(p.buffer) >= p.batchSize {
		return p.processBatch()
	}

	return nil, nil // Wait for more events
}

// processBatch processes the current batch
func (p *BatchProcessor[T, R]) processBatch() (*engine.Event[R], error) {
	if len(p.buffer) == 0 {
		return nil, nil
	}

	results, err := p.process(p.buffer)
	if err != nil {
		return nil, err
	}

	p.buffer = p.buffer[:0] // Clear buffer

	if len(results) > 0 {
		return results[0], nil // Return first result
	}

	return nil, nil
}

// Name returns the processor name
func (p *BatchProcessor[T, R]) Name() string {
	return p.name
}

// --- Built-in Processors ---

// EvenNumberFilter creates a filter for even numbers
func EvenNumberFilter() *FilterProcessor[NumberData] {
	return NewFilterProcessor("even-filter", func(event *engine.Event[NumberData]) bool {
		return int(event.Data.Value)%2 == 0
	})
}

// RangeFilter creates a filter for numbers in a specific range
func RangeFilter(min, max float64) *FilterProcessor[NumberData] {
	return NewFilterProcessor("range-filter", func(event *engine.Event[NumberData]) bool {
		return event.Data.Value >= min && event.Data.Value <= max
	})
}

// SquareTransform squares numeric values
func SquareTransform() *TransformProcessor[NumberData, NumberData] {
	return NewTransformProcessor("square-transform", func(event *engine.Event[NumberData]) (*engine.Event[NumberData], error) {
		result := &engine.Event[NumberData]{
			ID:        engine.GenerateID(),
			Timestamp: time.Now(),
			Data: NumberData{
				Value:  event.Data.Value * event.Data.Value,
				Source: event.Data.Source + "->squared",
			},
			Metadata: make(map[string]interface{}),
		}

		// Copy original metadata
		for k, v := range event.Metadata {
			result.Metadata[k] = v
		}
		result.Metadata["original_value"] = event.Data.Value

		return result, nil
	})
}

// StringToNumberTransform converts strings to numbers
func StringToNumberTransform() *TransformProcessor[StringData, NumberData] {
	return NewTransformProcessor("string-to-number", func(event *engine.Event[StringData]) (*engine.Event[NumberData], error) {
		// Try to parse the string as a number
		value, err := strconv.ParseFloat(strings.TrimSpace(event.Data.Text), 64)
		if err != nil {
			// If parsing fails, use string length as value
			value = float64(event.Data.Length)
		}

		result := &engine.Event[NumberData]{
			ID:        engine.GenerateID(),
			Timestamp: time.Now(),
			Data: NumberData{
				Value:  value,
				Source: "string-conversion",
			},
			Metadata: make(map[string]interface{}),
		}

		// Copy metadata
		for k, v := range event.Metadata {
			result.Metadata[k] = v
		}
		result.Metadata["original_text"] = event.Data.Text

		return result, nil
	})
}

// MovingAverageTransform calculates moving average
type MovingAverageTransform struct {
	name       string
	windowSize int
	window     []float64
	index      int
}

// NewMovingAverageTransform creates a moving average transformer
func NewMovingAverageTransform(windowSize int) *MovingAverageTransform {
	return &MovingAverageTransform{
		name:       "moving-average",
		windowSize: windowSize,
		window:     make([]float64, windowSize),
	}
}

// Process calculates moving average
func (p *MovingAverageTransform) Process(ctx context.Context, event *engine.Event[NumberData]) (*engine.Event[NumberData], error) {
	p.window[p.index%p.windowSize] = event.Data.Value
	p.index++

	// Calculate average
	var sum float64
	count := p.windowSize
	if p.index < p.windowSize {
		count = p.index
	}

	for i := 0; i < count; i++ {
		sum += p.window[i]
	}

	average := sum / float64(count)

	result := &engine.Event[NumberData]{
		ID:        engine.GenerateID(),
		Timestamp: time.Now(),
		Data: NumberData{
			Value:  average,
			Source: event.Data.Source + "->moving-avg",
		},
		Metadata: make(map[string]interface{}),
	}

	// Copy metadata
	for k, v := range event.Metadata {
		result.Metadata[k] = v
	}
	result.Metadata["window_size"] = p.windowSize
	result.Metadata["original_value"] = event.Data.Value

	return result, nil
}

// Name returns processor name
func (p *MovingAverageTransform) Name() string {
	return p.name
}

// AggregatorProcessor aggregates numeric data
type AggregatorProcessor struct {
	name      string
	batchSize int

	count  int64
	sum    float64
	min    float64
	max    float64
	values []float64
}

// NewAggregatorProcessor creates a new aggregator
func NewAggregatorProcessor(batchSize int) *AggregatorProcessor {
	return &AggregatorProcessor{
		name:      "aggregator",
		batchSize: batchSize,
		min:       math.Inf(1),
		max:       math.Inf(-1),
		values:    make([]float64, 0, batchSize),
	}
}

// Process accumulates values and emits aggregate when batch is full
func (p *AggregatorProcessor) Process(ctx context.Context, event *engine.Event[NumberData]) (*engine.Event[AggregateData], error) {
	value := event.Data.Value

	p.count++
	p.sum += value
	p.values = append(p.values, value)

	if value < p.min {
		p.min = value
	}
	if value > p.max {
		p.max = value
	}

	// Emit aggregate when batch is full
	if len(p.values) >= p.batchSize {
		return p.emitAggregate()
	}

	return nil, nil // Wait for more values
}

// emitAggregate creates aggregate result
func (p *AggregatorProcessor) emitAggregate() (*engine.Event[AggregateData], error) {
	if len(p.values) == 0 {
		return nil, nil
	}

	average := p.sum / float64(len(p.values))

	// Calculate standard deviation
	var variance float64
	for _, v := range p.values {
		diff := v - average
		variance += diff * diff
	}
	variance /= float64(len(p.values))
	stdDev := math.Sqrt(variance)

	result := &engine.Event[AggregateData]{
		ID:        engine.GenerateID(),
		Timestamp: time.Now(),
		Data: AggregateData{
			Count:   int64(len(p.values)),
			Sum:     p.sum,
			Average: average,
			Min:     p.min,
			Max:     p.max,
			StdDev:  stdDev,
		},
		Metadata: make(map[string]interface{}),
	}

	// Reset for next batch
	p.values = p.values[:0]
	p.sum = 0
	p.min = math.Inf(1)
	p.max = math.Inf(-1)

	return result, nil
}

// Name returns processor name
func (p *AggregatorProcessor) Name() string {
	return p.name
}

// --- Data Sources ---

// RandomNumberSource generates random numbers
type RandomNumberSource struct {
	name     string
	min, max float64
	rate     time.Duration
	rng      *rand.Rand
}

// NewRandomNumberSource creates a random number generator
func NewRandomNumberSource(name string, min, max float64, rate time.Duration) *RandomNumberSource {
	return &RandomNumberSource{
		name: name,
		min:  min,
		max:  max,
		rate: rate,
		rng:  rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Generate produces random number events
func (s *RandomNumberSource) Generate(ctx context.Context, output chan<- *engine.Event[NumberData]) {
	ticker := time.NewTicker(s.rate)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			value := s.min + s.rng.Float64()*(s.max-s.min)

			event := &engine.Event[NumberData]{
				ID:        engine.GenerateID(),
				Timestamp: time.Now(),
				Data: NumberData{
					Value:  value,
					Source: s.name,
				},
				Metadata: make(map[string]interface{}),
			}

			select {
			case output <- event:
			case <-ctx.Done():
				return
			}
		}
	}
}

// Name returns source name
func (s *RandomNumberSource) Name() string {
	return s.name
}
