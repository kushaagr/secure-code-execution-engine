package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"secure-exec-engine/metrics"
	"secure-exec-engine/queue"
	"secure-exec-engine/sandbox"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func worker(id int, qm *queue.QueueManager, wg *sync.WaitGroup) {
	defer wg.Done()
	ctx := context.Background()
	fmt.Printf("[Worker %d] Booted and recording live telemetry...\n", id)

	for {
		task, err := qm.PopTask(ctx)
		if err != nil {
			log.Printf("[Worker %d] Error fetching task: %v", id, err)
			continue
		}

		// Metric Increment: Worker transitions into an active tracking phase
		metrics.ActiveWorkersGauge.Inc()
		startTime := time.Now()

		fmt.Printf("[Worker %d] Processing Task ID: %s (%s)\n", id, task.ID, task.Language)
		res, err := sandbox.RunCode(task.Language, task.SourceCode)
		
		// Metric Duration Record: Record sandbox processing latency
		duration := time.Since(startTime).Seconds()
		metrics.ExecutionLatency.WithLabelValues(task.Language).Observe(duration)

		if err != nil {
			metrics.ActiveWorkersGauge.Dec()
			_ = qm.SetResult(ctx, task.ID, map[string]string{"error": err.Error()})
			continue
		}

		// Metric Tracking: Check if the application context hit a security timeout limit
		if res.TimedOut {
			metrics.TimeoutExceededCounter.Inc()
		}

		_ = qm.SetResult(ctx, task.ID, res)
		metrics.ActiveWorkersGauge.Dec() // Metric Decrement: Worker goes back to idle

		fmt.Printf("[Worker %d] Finished Task ID: %s | Latency: %.2fs\n", id, task.ID, duration)
	}
}

func main() {
	fmt.Println("🛡️ Secure Execution Pipeline Initialization...")
	qm := queue.NewQueueManager()

	// 1. Launch standalone background HTTP server to expose Prometheus telemetry scraper path
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		fmt.Println("[Telemetry] Exposing internal metrics dashboard path on http://localhost:2112/metrics")
		if err := http.ListenAndServe(":2112", nil); err != nil {
			log.Fatalf("Failed to initialize metrics scraping engine handler: %v", err)
		}
	}()

	// 2. Spin up a thread pool of 3 concurrent background workers
	var wg sync.WaitGroup
	numWorkers := 3
	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go worker(i, qm, &wg)
	}

	// 3. Keep injecting mock high-traffic scripts indefinitely to generate dashboard data
	ctx := context.Background()
	for {
		time.Sleep(2 * time.Second)
		_ = qm.PushTask(ctx, &queue.CodeTask{
			ID:         fmt.Sprintf("req-%d", time.Now().UnixNano()%10000),
			Language:   "python",
			SourceCode: "print('Telemetry logging verify tick')",
			Timestamp:  time.Now(),
		})
	}
}
