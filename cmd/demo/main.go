package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"simple-data-processing-engine/pkg/engine"
	"simple-data-processing-engine/pkg/processors"
	"simple-data-processing-engine/pkg/ui"
	"syscall"
	"time"

	"github.com/fatih/color"
)

func main() {
	// Setup colors
	titleColor := color.New(color.FgCyan, color.Bold)
	successColor := color.New(color.FgGreen, color.Bold)
	warningColor := color.New(color.FgYellow, color.Bold)
	errorColor := color.New(color.FgRed, color.Bold)

	// Print banner
	printBanner()

	// Create pipeline
	fmt.Print("🔧 Building data processing pipeline... ")
	pipeline := buildAdvancedPipeline()
	successColor.Println("✅ Done!")

	// Create data source
	fmt.Print("📊 Setting up data source... ")
	source := make(chan interface{}, 1000)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start number generator
	numberSource := processors.NewRandomNumberSource("random-generator", 1, 1000, 50*time.Millisecond)
	go func() {
		eventChan := make(chan *engine.Event[processors.NumberData], 1000)
		go numberSource.Generate(ctx, eventChan)
		
		for event := range eventChan {
			select {
			case source <- event:
			case <-ctx.Done():
				return
			}
		}
	}()
	successColor.Println("✅ Done!")

	// Start monitoring dashboard
	fmt.Print("📈 Starting monitoring dashboard... ")
	dashboard := ui.NewDashboard(pipeline)
	go dashboard.Start(ctx)
	successColor.Println("✅ Done!")

	// Start pipeline
	fmt.Print("🚀 Starting pipeline... ")
	if err := pipeline.Start(source); err != nil {
		errorColor.Printf("❌ Failed to start pipeline: %v\n", err)
		return
	}
	successColor.Println("✅ Pipeline running!")

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	titleColor.Println("\n🎯 Pipeline Status: RUNNING")
	fmt.Println("📊 Monitor: http://localhost:8080/metrics")
	fmt.Println("🔍 Dashboard: Real-time metrics updating...")
	warningColor.Println("📝 Press Ctrl+C to gracefully shutdown")

	// Wait for shutdown signal
	<-sigChan

	// Graceful shutdown
	fmt.Print("\n🛑 Shutting down pipeline... ")
	cancel()
	
	if err := pipeline.Stop(); err != nil {
		errorColor.Printf("❌ Shutdown error: %v\n", err)
	} else {
		successColor.Println("✅ Pipeline stopped gracefully!")
	}

	// Print final metrics
	printFinalMetrics(pipeline)
}

func buildAdvancedPipeline() *engine.Pipeline {
	// Create pipeline with configuration
	config := engine.PipelineConfig{
		Name:                "advanced-data-processor",
		ShutdownTimeout:     30 * time.Second,
		ErrorBufferSize:     1000,
		HealthCheckInterval: 5 * time.Second,
	}

	builder := engine.NewPipelineBuilder("Advanced Data Processing Engine").
		WithConfig(config)

	// Add startup hook
	builder.AddStartHook(func() error {
		fmt.Println("🎬 Pipeline starting - initializing resources...")
		return nil
	})

	// Add shutdown hook
	builder.AddStopHook(func() error {
		fmt.Println("🏁 Pipeline stopping - cleaning up resources...")
		return nil
	})

	// Build complex pipeline with multiple stages
	
	// Stage 1: Range Filter (1-500)
	rangeFilter := createFilterStage("range-filter", processors.RangeFilter(1, 500))
	builder.AddStage(rangeFilter)

	// Stage 2: Even Number Filter
	evenFilter := createFilterStage("even-filter", processors.EvenNumberFilter())
	builder.AddStage(evenFilter)

	// Stage 3: Square Transform
	squareTransform := createTransformStage("square-transform", processors.SquareTransform())
	builder.AddStage(squareTransform)

	// Stage 4: Moving Average (window size 10)
	movingAvg := createMovingAverageStage("moving-average", 10)
	builder.AddStage(movingAvg)

	// Stage 5: Aggregator (batch size 20)
	aggregator := createAggregatorStage("aggregator", 20)
	builder.AddStage(aggregator)

	return builder.Build()
}

// Helper functions to create typed stages

func createFilterStage(name string, processor *processors.FilterProcessor[processors.NumberData]) engine.StageRunner {
	config := engine.StageConfig{
		Workers:    4,
		BufferSize: 200,
		Timeout:    5 * time.Second,
	}
	return engine.NewStage(name, processor, config)
}

func createTransformStage(name string, processor *processors.TransformProcessor[processors.NumberData, processors.NumberData]) engine.StageRunner {
	config := engine.StageConfig{
		Workers:    6,
		BufferSize: 300,
		Timeout:    5 * time.Second,
	}
	return engine.NewStage(name, processor, config)
}

func createMovingAverageStage(name string, windowSize int) engine.StageRunner {
	config := engine.StageConfig{
		Workers:    2,
		BufferSize: 100,
		Timeout:    5 * time.Second,
	}
	processor := processors.NewMovingAverageTransform(windowSize)
	return engine.NewStage(name, processor, config)
}

func createAggregatorStage(name string, batchSize int) engine.StageRunner {
	config := engine.StageConfig{
		Workers:    1,
		BufferSize: 50,
		Timeout:    5 * time.Second,
	}
	processor := processors.NewAggregatorProcessor(batchSize)
	return engine.NewStage(name, processor, config)
}

func printBanner() {
	banner := `
╔══════════════════════════════════════════════════════════════╗
║                                                              ║
║   🚀 SIMPLE DATA PROCESSING ENGINE 🚀                       ║
║                                                              ║
║   High-Performance • Concurrent • Production-Ready          ║
║   Built with Go's Powerful Concurrency Primitives          ║
║                                                              ║
╚══════════════════════════════════════════════════════════════╝
`
	titleColor := color.New(color.FgCyan, color.Bold)
	titleColor.Println(banner)
}

func printFinalMetrics(pipeline *engine.Pipeline) {
	fmt.Println("\n📊 Final Pipeline Metrics:")
	fmt.Println("═════════════════════════════")
	
	metrics := pipeline.GetMetrics()
	
	infoColor := color.New(color.FgBlue)
	valueColor := color.New(color.FgGreen, color.Bold)
	
	infoColor.Printf("Pipeline: ")
	valueColor.Println(metrics.PipelineName)
	
	infoColor.Printf("Total Throughput: ")
	valueColor.Printf("%.2f events/sec\n", metrics.TotalThroughput)
	
	infoColor.Printf("Total Latency: ")
	valueColor.Printf("%v\n", metrics.TotalLatency)
	
	infoColor.Printf("Overall Health: ")
	if metrics.OverallHealth >= 80 {
		color.Green("%.2f%%", metrics.OverallHealth)
	} else if metrics.OverallHealth >= 60 {
		color.Yellow("%.2f%%", metrics.OverallHealth)
	} else {
		color.Red("%.2f%%", metrics.OverallHealth)
	}
	fmt.Println()
	
	fmt.Println("\n📈 Stage Breakdown:")
	for _, stage := range metrics.StageMetrics {
		fmt.Printf("  • %s:\n", stage.Name)
		fmt.Printf("    Input: %d | Output: %d | Errors: %d\n", 
			stage.InputCount, stage.OutputCount, stage.ErrorCount)
		fmt.Printf("    Throughput: %.2f/sec | Avg Duration: %v\n", 
			stage.Throughput, stage.AvgDuration)
		fmt.Printf("    Success Rate: %.2f%% | Workers: %d/%d\n", 
			stage.SuccessRate, stage.ActiveWorkers, stage.MaxWorkers)
		fmt.Println()
	}
}
