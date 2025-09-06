package engine

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// MockProcessor for testing
type MockProcessor struct {
	name        string
	processFunc func(ctx context.Context, event *Event[int]) (*Event[int], error)
}

func (p *MockProcessor) Process(ctx context.Context, event *Event[int]) (*Event[int], error) {
	if p.processFunc != nil {
		return p.processFunc(ctx, event)
	}
	return event, nil // Default: pass through
}

func (p *MockProcessor) Name() string {
	return p.name
}

func TestStageBasicOperation(t *testing.T) {
	// Create a simple processor that doubles values
	processor := &MockProcessor{
		name: "doubler",
		processFunc: func(ctx context.Context, event *Event[int]) (*Event[int], error) {
			result := &Event[int]{
				ID:        GenerateID(),
				Data:      event.Data * 2,
				Timestamp: time.Now(),
				Metadata:  make(map[string]interface{}),
			}
			return result, nil
		},
	}

	// Create stage
	config := StageConfig{
		Workers:    2,
		BufferSize: 10,
		Timeout:    5 * time.Second,
	}
	stage := NewStage("test-stage", processor, config)

	// Create input channel and send test data
	input := make(chan *Event[int], 10)
	output := stage.Start(input)

	// Send test events
	testEvents := []*Event[int]{
		{ID: "1", Data: 5, Timestamp: time.Now(), Metadata: make(map[string]interface{})},
		{ID: "2", Data: 10, Timestamp: time.Now(), Metadata: make(map[string]interface{})},
		{ID: "3", Data: 15, Timestamp: time.Now(), Metadata: make(map[string]interface{})},
	}

	go func() {
		for _, event := range testEvents {
			input <- event
		}
		close(input)
	}()

	// Collect results
	var results []*Event[int]
	for result := range output {
		results = append(results, result)
	}

	// Verify results
	if len(results) != len(testEvents) {
		t.Fatalf("Expected %d results, got %d", len(testEvents), len(results))
	}

	expectedValues := []int{10, 20, 30}
	for i, result := range results {
		if result.Data != expectedValues[i] {
			t.Errorf("Expected result %d to be %d, got %d", i, expectedValues[i], result.Data)
		}
	}

	// Check metrics
	metrics := stage.GetMetrics()
	snapshot := metrics.GetSnapshot()

	if snapshot.InputCount != int64(len(testEvents)) {
		t.Errorf("Expected input count %d, got %d", len(testEvents), snapshot.InputCount)
	}

	if snapshot.OutputCount != int64(len(testEvents)) {
		t.Errorf("Expected output count %d, got %d", len(testEvents), snapshot.OutputCount)
	}

	// Stop stage
	if err := stage.Stop(5 * time.Second); err != nil {
		t.Errorf("Error stopping stage: %v", err)
	}
}

func TestStageWithErrors(t *testing.T) {
	// Create a processor that fails on even numbers
	processor := &MockProcessor{
		name: "error-prone",
		processFunc: func(ctx context.Context, event *Event[int]) (*Event[int], error) {
			if event.Data%2 == 0 {
				return nil, fmt.Errorf("even numbers not allowed")
			}
			return event, nil
		},
	}

	config := StageConfig{
		Workers:    1,
		BufferSize: 10,
		Timeout:    5 * time.Second,
	}
	stage := NewStage("error-test", processor, config)

	input := make(chan *Event[int], 10)
	output := stage.Start(input)

	// Send mix of even and odd numbers
	testEvents := []*Event[int]{
		{ID: "1", Data: 1, Timestamp: time.Now(), Metadata: make(map[string]interface{})}, // odd - should pass
		{ID: "2", Data: 2, Timestamp: time.Now(), Metadata: make(map[string]interface{})}, // even - should fail
		{ID: "3", Data: 3, Timestamp: time.Now(), Metadata: make(map[string]interface{})}, // odd - should pass
		{ID: "4", Data: 4, Timestamp: time.Now(), Metadata: make(map[string]interface{})}, // even - should fail
	}

	go func() {
		for _, event := range testEvents {
			input <- event
		}
		close(input)
	}()

	// Collect results
	var results []*Event[int]
	for result := range output {
		results = append(results, result)
	}

	// Should only get 2 results (odd numbers)
	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// Check metrics for errors
	metrics := stage.GetMetrics()
	snapshot := metrics.GetSnapshot()

	if snapshot.ErrorCount != 2 {
		t.Errorf("Expected 2 errors, got %d", snapshot.ErrorCount)
	}

	if snapshot.SuccessRate != 50.0 {
		t.Errorf("Expected 50%% success rate, got %.1f%%", snapshot.SuccessRate)
	}

	stage.Stop(5 * time.Second)
}

func TestPipelineIntegration(t *testing.T) {
	// Create a simple pipeline: input -> double -> add_one

	doubler := &MockProcessor{
		name: "doubler",
		processFunc: func(ctx context.Context, event *Event[int]) (*Event[int], error) {
			return &Event[int]{
				ID:        GenerateID(),
				Data:      event.Data * 2,
				Timestamp: time.Now(),
				Metadata:  make(map[string]interface{}),
			}, nil
		},
	}

	adder := &MockProcessor{
		name: "adder",
		processFunc: func(ctx context.Context, event *Event[int]) (*Event[int], error) {
			return &Event[int]{
				ID:        GenerateID(),
				Data:      event.Data + 1,
				Timestamp: time.Now(),
				Metadata:  make(map[string]interface{}),
			}, nil
		},
	}

	config := PipelineConfig{
		Name:            "test-pipeline",
		ShutdownTimeout: 10 * time.Second,
		ErrorBufferSize: 100,
	}

	pipeline := NewPipeline("test", config)

	// Add stages
	stageConfig := StageConfig{Workers: 1, BufferSize: 10, Timeout: 5 * time.Second}

	doublerStage := NewStage("doubler", doubler, stageConfig)
	adderStage := NewStage("adder", adder, stageConfig)

	pipeline.AddStage(NewGenericStage(doublerStage))
	pipeline.AddStage(NewGenericStage(adderStage))

	// Create source
	source := make(chan interface{}, 10)

	// Start pipeline
	if err := pipeline.Start(source); err != nil {
		t.Fatalf("Failed to start pipeline: %v", err)
	}

	// Send test data
	testData := []*Event[int]{
		{ID: "1", Data: 5, Timestamp: time.Now(), Metadata: make(map[string]interface{})},
		{ID: "2", Data: 10, Timestamp: time.Now(), Metadata: make(map[string]interface{})},
	}

	go func() {
		for _, event := range testData {
			source <- event
		}
		close(source)
	}()

	// Wait a bit for processing
	time.Sleep(100 * time.Millisecond)

	// Stop pipeline
	if err := pipeline.Stop(); err != nil {
		t.Errorf("Error stopping pipeline: %v", err)
	}

	// Check pipeline metrics
	metrics := pipeline.GetMetrics()

	if len(metrics.StageMetrics) != 2 {
		t.Errorf("Expected 2 stages in metrics, got %d", len(metrics.StageMetrics))
	}

	// Verify each stage processed the data
	for _, stageMetric := range metrics.StageMetrics {
		if stageMetric.InputCount == 0 {
			t.Errorf("Stage %s should have processed some input", stageMetric.Name)
		}
	}
}

func BenchmarkStageProcessing(b *testing.B) {
	processor := &MockProcessor{
		name: "pass-through",
		processFunc: func(ctx context.Context, event *Event[int]) (*Event[int], error) {
			return event, nil
		},
	}

	config := StageConfig{
		Workers:    4,
		BufferSize: 1000,
		Timeout:    5 * time.Second,
	}
	stage := NewStage("benchmark", processor, config)

	input := make(chan *Event[int], 1000)
	output := stage.Start(input)

	b.ResetTimer()

	go func() {
		for i := 0; i < b.N; i++ {
			event := &Event[int]{
				ID:        GenerateID(),
				Data:      i,
				Timestamp: time.Now(),
				Metadata:  make(map[string]interface{}),
			}
			input <- event
		}
		close(input)
	}()

	// Consume all output
	count := 0
	for range output {
		count++
	}

	stage.Stop(5 * time.Second)

	if count != b.N {
		b.Errorf("Expected %d events, got %d", b.N, count)
	}
}
