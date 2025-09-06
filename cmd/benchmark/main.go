package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"simple-data-processing-engine/pkg/engine"
	"simple-data-processing-engine/pkg/processors"
	"strings"
	"syscall"
	"time"
)

// Benchmark tests the pipeline performance
func main() {
	fmt.Println("🚀 Performance Benchmark - Data Processing Engine")
	fmt.Println(strings.Repeat("=", 60))

	// Create high-performance pipeline
	pipeline := createBenchmarkPipeline()
	
	// Create high-volume data source
	source := make(chan interface{}, 10000)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start high-speed number generator
	go generateHighVolumeData(ctx, source)

	// Start performance monitoring
	go monitorPerformance(ctx, pipeline)

	fmt.Println("⏱️ Starting benchmark...")
	start := time.Now()
	
	// Start pipeline
	if err := pipeline.Start(source); err != nil {
		fmt.Printf("❌ Failed to start pipeline: %v\n", err)
		return
	}

	// Run benchmark for specified duration
	benchmarkDuration := 30 * time.Second
	fmt.Printf("🔥 Running benchmark for %v...\n", benchmarkDuration)
	
	timer := time.NewTimer(benchmarkDuration)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-timer.C:
		fmt.Println("⏰ Benchmark duration completed")
	case <-sigChan:
		fmt.Println("🛑 Benchmark interrupted by user")
	}

	// Stop pipeline and show results
	fmt.Println("🏁 Stopping pipeline...")
	cancel()
	
	if err := pipeline.Stop(); err != nil {
		fmt.Printf("❌ Shutdown error: %v\n", err)
	}

	elapsed := time.Since(start)
	printBenchmarkResults(pipeline, elapsed)
}

func createBenchmarkPipeline() *engine.Pipeline {
	config := engine.PipelineConfig{
		Name:                "benchmark-pipeline",
		ShutdownTimeout:     10 * time.Second,
		ErrorBufferSize:     1000,
		HealthCheckInterval: 1 * time.Second,
	}

	builder := engine.NewPipelineBuilder("Benchmark Pipeline").
		WithConfig(config)

	// High-performance stages with more workers
	
	// Stage 1: Range filter with 8 workers
	rangeFilter := createHighPerfFilterStage("range-filter", 
		processors.RangeFilter(1, 500), 8)
	builder.AddStage(rangeFilter)

	// Stage 2: Even filter with 8 workers
	evenFilter := createHighPerfFilterStage("even-filter", 
		processors.EvenNumberFilter(), 8)
	builder.AddStage(evenFilter)

	// Stage 3: Square transform with 12 workers
	squareTransform := createHighPerfTransformStage("square-transform", 
		processors.SquareTransform(), 12)
	builder.AddStage(squareTransform)

	// Stage 4: Moving average with 4 workers
	movingAvg := createHighPerfMovingAvgStage("moving-average", 5, 4)
	builder.AddStage(movingAvg)

	return builder.Build()
}

func createHighPerfFilterStage(name string, processor *processors.FilterProcessor[processors.NumberData], workers int) engine.StageRunner {
	config := engine.StageConfig{
		Workers:    workers,
		BufferSize: 1000,
		Timeout:    1 * time.Second,
	}
	stage := engine.NewStage(name, processor, config)
	return engine.NewGenericStage(stage)
}

func createHighPerfTransformStage(name string, processor *processors.TransformProcessor[processors.NumberData, processors.NumberData], workers int) engine.StageRunner {
	config := engine.StageConfig{
		Workers:    workers,
		BufferSize: 1000,
		Timeout:    1 * time.Second,
	}
	stage := engine.NewStage(name, processor, config)
	return engine.NewGenericStage(stage)
}

func createHighPerfMovingAvgStage(name string, windowSize int, workers int) engine.StageRunner {
	config := engine.StageConfig{
		Workers:    workers,
		BufferSize: 500,
		Timeout:    1 * time.Second,
	}
	processor := processors.NewMovingAverageTransform(windowSize)
	stage := engine.NewStage(name, processor, config)
	return engine.NewGenericStage(stage)
}

func generateHighVolumeData(ctx context.Context, output chan<- interface{}) {
	defer close(output)
	
	ticker := time.NewTicker(1 * time.Millisecond) // Very high frequency
	defer ticker.Stop()
	
	counter := 1
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			event := &engine.Event[processors.NumberData]{
				ID:        engine.GenerateID(),
				Timestamp: time.Now(),
				Data: processors.NumberData{
					Value:  float64(counter % 1000),
					Source: "benchmark-generator",
				},
				Metadata: make(map[string]interface{}),
			}
			
			select {
			case output <- event:
				counter++
			case <-ctx.Done():
				return
			default:
				// Channel full, continue
			}
		}
	}
}

func monitorPerformance(ctx context.Context, pipeline *engine.Pipeline) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			metrics := pipeline.GetMetrics()
			
			fmt.Printf("\n📊 Performance Update:\n")
			fmt.Printf("   Throughput: %.0f events/sec\n", metrics.TotalThroughput)
			fmt.Printf("   Health: %.1f%%\n", metrics.OverallHealth)
			
			// Show top performing stage
			var bestStage string
			var bestThroughput float64
			for _, stage := range metrics.StageMetrics {
				if stage.Throughput > bestThroughput {
					bestThroughput = stage.Throughput
					bestStage = stage.Name
				}
			}
			fmt.Printf("   Best Stage: %s (%.0f/sec)\n", bestStage, bestThroughput)
		}
	}
}

func printBenchmarkResults(pipeline *engine.Pipeline, duration time.Duration) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("🏆 BENCHMARK RESULTS")
	fmt.Println(strings.Repeat("=", 60))
	
	metrics := pipeline.GetMetrics()
	
	fmt.Printf("⏱️  Duration: %v\n", duration)
	fmt.Printf("⚡ Peak Throughput: %.0f events/sec\n", metrics.TotalThroughput)
	fmt.Printf("🎯 Overall Health: %.1f%%\n", metrics.OverallHealth)
	fmt.Printf("📊 Total Latency: %v\n", metrics.TotalLatency)
	
	fmt.Println("\n📈 Stage Performance:")
	for _, stage := range metrics.StageMetrics {
		fmt.Printf("  🔧 %s:\n", stage.Name)
		fmt.Printf("     📥 Processed: %d events\n", stage.InputCount)
		fmt.Printf("     📤 Output: %d events\n", stage.OutputCount)
		fmt.Printf("     ⚡ Peak Rate: %.0f/sec\n", stage.InputRate)
		fmt.Printf("     ⏱️  Avg Latency: %v\n", stage.AvgDuration)
		fmt.Printf("     ✅ Success Rate: %.1f%%\n", stage.SuccessRate)
		fmt.Printf("     👥 Workers Used: %d\n\n", stage.MaxWorkers)
	}
	
	// Calculate total events processed
	var totalProcessed int64
	for _, stage := range metrics.StageMetrics {
		if stage.InputCount > totalProcessed {
			totalProcessed = stage.InputCount
		}
	}
	
	fmt.Printf("🎊 Total Events Processed: %d\n", totalProcessed)
	fmt.Printf("📊 Average Rate: %.0f events/sec\n", float64(totalProcessed)/duration.Seconds())
	
	// Performance rating
	avgThroughput := float64(totalProcessed) / duration.Seconds()
	var rating string
	if avgThroughput > 10000 {
		rating = "🚀 EXCELLENT"
	} else if avgThroughput > 5000 {
		rating = "🔥 VERY GOOD"
	} else if avgThroughput > 1000 {
		rating = "✅ GOOD"
	} else {
		rating = "⚠️  NEEDS OPTIMIZATION"
	}
	
	fmt.Printf("\n🏅 Performance Rating: %s\n", rating)
	fmt.Println(strings.Repeat("=", 60))
}
