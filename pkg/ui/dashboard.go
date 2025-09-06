package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"simple-data-processing-engine/pkg/engine"
	"time"

	"github.com/fatih/color"
)

// Dashboard provides real-time monitoring interface
type Dashboard struct {
	pipeline *engine.Pipeline
	server   *http.Server
}

// NewDashboard creates a new monitoring dashboard
func NewDashboard(pipeline *engine.Pipeline) *Dashboard {
	return &Dashboard{
		pipeline: pipeline,
	}
}

// Start begins the monitoring dashboard
func (d *Dashboard) Start(ctx context.Context) {
	// Start HTTP metrics server
	go d.startHTTPServer(ctx)

	// Start terminal dashboard
	go d.startTerminalDashboard(ctx)
}

// startHTTPServer starts HTTP endpoint for metrics
func (d *Dashboard) startHTTPServer(ctx context.Context) {
	mux := http.NewServeMux()

	// Metrics endpoint
	mux.HandleFunc("/metrics", d.metricsHandler)

	// Health check endpoint
	mux.HandleFunc("/health", d.healthHandler)

	// Dashboard endpoint
	mux.HandleFunc("/dashboard", d.dashboardHandler)

	d.server = &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		if err := d.server.ListenAndServe(); err != http.ErrServerClosed {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	// Shutdown when context is cancelled
	<-ctx.Done()
	d.server.Shutdown(context.Background())
}

// metricsHandler serves JSON metrics
func (d *Dashboard) metricsHandler(w http.ResponseWriter, r *http.Request) {
	metrics := d.pipeline.GetMetrics()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	json.NewEncoder(w).Encode(metrics)
}

// healthHandler serves health check
func (d *Dashboard) healthHandler(w http.ResponseWriter, r *http.Request) {
	metrics := d.pipeline.GetMetrics()

	status := "healthy"
	httpStatus := http.StatusOK

	if metrics.OverallHealth < 50 {
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
	} else if metrics.OverallHealth < 80 {
		status = "degraded"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)

	response := map[string]interface{}{
		"status":    status,
		"health":    metrics.OverallHealth,
		"timestamp": time.Now(),
	}

	json.NewEncoder(w).Encode(response)
}

// dashboardHandler serves HTML dashboard
func (d *Dashboard) dashboardHandler(w http.ResponseWriter, r *http.Request) {
	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Data Processing Engine Dashboard</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 20px; border-radius: 10px; margin-bottom: 20px; }
        .metrics-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
        .metric-card { background: white; padding: 20px; border-radius: 10px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .metric-title { font-size: 18px; font-weight: bold; margin-bottom: 10px; color: #333; }
        .metric-value { font-size: 24px; font-weight: bold; color: #667eea; }
        .stage-list { margin-top: 20px; }
        .stage-item { background: white; padding: 15px; margin: 10px 0; border-radius: 8px; border-left: 4px solid #667eea; }
        .refresh-btn { background: #667eea; color: white; padding: 10px 20px; border: none; border-radius: 5px; cursor: pointer; }
        .health-good { color: #28a745; }
        .health-warning { color: #ffc107; }
        .health-error { color: #dc3545; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🚀 Data Processing Engine Dashboard</h1>
            <p>Real-time monitoring and metrics</p>
        </div>
        
        <button class="refresh-btn" onclick="loadMetrics()">🔄 Refresh</button>
        
        <div class="metrics-grid" id="metrics-container">
            Loading...
        </div>
        
        <div class="stage-list" id="stages-container">
        </div>
    </div>

    <script>
        function loadMetrics() {
            fetch('/metrics')
                .then(response => response.json())
                .then(data => updateDashboard(data))
                .catch(error => console.error('Error:', error));
        }
        
        function updateDashboard(metrics) {
            const container = document.getElementById('metrics-container');
            const stagesContainer = document.getElementById('stages-container');
            
            // Update main metrics
            container.innerHTML = ` + "`" + `
                <div class="metric-card">
                    <div class="metric-title">Pipeline</div>
                    <div class="metric-value">${metrics.pipeline_name}</div>
                </div>
                <div class="metric-card">
                    <div class="metric-title">Total Throughput</div>
                    <div class="metric-value">${metrics.total_throughput.toFixed(2)} /sec</div>
                </div>
                <div class="metric-card">
                    <div class="metric-title">Overall Health</div>
                    <div class="metric-value ${getHealthClass(metrics.overall_health)}">${metrics.overall_health.toFixed(1)}%</div>
                </div>
                <div class="metric-card">
                    <div class="metric-title">Total Latency</div>
                    <div class="metric-value">${formatDuration(metrics.total_latency)}</div>
                </div>
            ` + "`" + `;
            
            // Update stages
            let stagesHTML = '<h2>📊 Stage Metrics</h2>';
            metrics.stage_metrics.forEach(stage => {
                stagesHTML += ` + "`" + `
                    <div class="stage-item">
                        <h3>${stage.name}</h3>
                        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 10px;">
                            <div><strong>Input:</strong> ${stage.input_count}</div>
                            <div><strong>Output:</strong> ${stage.output_count}</div>
                            <div><strong>Errors:</strong> ${stage.error_count}</div>
                            <div><strong>Rate:</strong> ${stage.input_rate.toFixed(2)}/sec</div>
                            <div><strong>Success:</strong> <span class="${getHealthClass(stage.success_rate)}">${stage.success_rate.toFixed(1)}%</span></div>
                            <div><strong>Workers:</strong> ${stage.active_workers}/${stage.max_workers}</div>
                        </div>
                    </div>
                ` + "`" + `;
            });
            stagesContainer.innerHTML = stagesHTML;
        }
        
        function getHealthClass(health) {
            if (health >= 80) return 'health-good';
            if (health >= 60) return 'health-warning';
            return 'health-error';
        }
        
        function formatDuration(nanoseconds) {
            const ms = nanoseconds / 1000000;
            if (ms < 1) return (ms * 1000).toFixed(0) + 'μs';
            if (ms < 1000) return ms.toFixed(2) + 'ms';
            return (ms / 1000).toFixed(2) + 's';
        }
        
        // Load initial data
        loadMetrics();
        
        // Auto-refresh every 2 seconds
        setInterval(loadMetrics, 2000);
    </script>
</body>
</html>
    `

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// startTerminalDashboard shows real-time metrics in terminal
func (d *Dashboard) startTerminalDashboard(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	titleColor := color.New(color.FgCyan, color.Bold)
	successColor := color.New(color.FgGreen)
	warningColor := color.New(color.FgYellow)
	errorColor := color.New(color.FgRed)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			metrics := d.pipeline.GetMetrics()

			// Clear screen and show metrics
			fmt.Print("\033[2J\033[H") // Clear screen

			titleColor.Println("📊 Real-time Pipeline Metrics")
			fmt.Println("═══════════════════════════════")

			// Overall metrics
			fmt.Printf("🏷️  Pipeline: %s\n", metrics.PipelineName)
			fmt.Printf("⚡ Throughput: %.2f events/sec\n", metrics.TotalThroughput)
			fmt.Printf("⏱️  Latency: %v\n", metrics.TotalLatency)

			// Health indicator
			fmt.Print("💚 Health: ")
			if metrics.OverallHealth >= 80 {
				successColor.Printf("%.1f%% (Excellent)\n", metrics.OverallHealth)
			} else if metrics.OverallHealth >= 60 {
				warningColor.Printf("%.1f%% (Good)\n", metrics.OverallHealth)
			} else {
				errorColor.Printf("%.1f%% (Poor)\n", metrics.OverallHealth)
			}

			fmt.Printf("📅 Updated: %s\n", metrics.Timestamp.Format("15:04:05"))

			fmt.Println("\n📈 Stage Performance:")
			fmt.Println("─────────────────────")

			for _, stage := range metrics.StageMetrics {
				fmt.Printf("🔧 %s:\n", stage.Name)
				fmt.Printf("   📥 In: %d  📤 Out: %d  ❌ Err: %d\n",
					stage.InputCount, stage.OutputCount, stage.ErrorCount)
				fmt.Printf("   📊 Rate: %.1f/sec  ⏱️ Avg: %v\n",
					stage.InputRate, stage.AvgDuration)

				// Success rate with color
				fmt.Print("   ✅ Success: ")
				if stage.SuccessRate >= 95 {
					successColor.Printf("%.1f%%", stage.SuccessRate)
				} else if stage.SuccessRate >= 80 {
					warningColor.Printf("%.1f%%", stage.SuccessRate)
				} else {
					errorColor.Printf("%.1f%%", stage.SuccessRate)
				}

				fmt.Printf("  👥 Workers: %d/%d\n\n", stage.ActiveWorkers, stage.MaxWorkers)
			}

			fmt.Println("🌐 Web Dashboard: http://localhost:8080/dashboard")
			fmt.Println("📊 JSON Metrics: http://localhost:8080/metrics")
		}
	}
}

// Stop shuts down the dashboard
func (d *Dashboard) Stop() error {
	if d.server != nil {
		return d.server.Shutdown(context.Background())
	}
	return nil
}
