package main

import (
	"context"
//	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"secure-exec-engine/metrics"
	"secure-exec-engine/queue"
	"secure-exec-engine/sandbox"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

func worker(id int, qm *queue.QueueManager, wg *sync.WaitGroup) {
	defer wg.Done()
	ctx := context.Background()
	fmt.Printf("[Worker %d] Engine worker active and monitoring secure execution ring...\n", id)

	for {
		task, err := qm.PopTask(ctx)
		if err != nil {
			continue
		}

		metrics.ActiveWorkersGauge.Inc()
		startTime := time.Now()

		// Executing code via our hardened sandbox
		res, err := sandbox.RunCode(task.Language, task.SourceCode)

		duration := time.Since(startTime).Seconds()
		metrics.ExecutionLatency.WithLabelValues(task.Language).Observe(duration)

		if err != nil {
			metrics.ActiveWorkersGauge.Dec()
			_ = qm.SetResult(ctx, task.ID, map[string]string{"error": err.Error()})
			continue
		}

		if res.TimedOut {
			metrics.TimeoutExceededCounter.Inc()
		}

		_ = qm.SetResult(ctx, task.ID, res)
		metrics.ActiveWorkersGauge.Dec()
	}
}

func main() {
	qm := queue.NewQueueManager()
	ctx := context.Background()

	// ─── 1. COMMAND LINE INTERFACE (CLI) MODE ──────────────────────────────────
	if len(os.Args) > 1 {
		filePath := os.Args[1]
		content, err := os.ReadFile(filePath)
		if err != nil {
			log.Fatalf("❌ Failed to read target file: %v", err)
		}

		taskID := fmt.Sprintf("cli-%d", time.Now().UnixNano()%10000)
		task := &queue.CodeTask{
			ID:         taskID,
			Language:   "python", // Defaulting to Python for CLI automation
			SourceCode: string(content),
			Timestamp:  time.Now(),
		}

		fmt.Printf("🚀 [CLI] Submitting %s to engine queue (Task ID: %s)...\n", filePath, taskID)
		if err := qm.PushTask(ctx, task); err != nil {
			log.Fatalf("❌ Submission failure: %v", err)
		}

		// Connect directly to Redis to poll for the asynchronous result
		rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
		fmt.Print("⏳ Waiting for sandbox execution results...")

		for i := 0; i < 20; i++ { // Poll for up to 10 seconds
			time.Sleep(500 * time.Millisecond)
			val, err := rdb.Get(ctx, "result:"+taskID).Result()
			if err == nil {
				fmt.Println("\n\n📥 [Execution Output Engine Return]:")
				fmt.Println(val)
				return
			}
		}
		fmt.Println("\n❌ Error: Execution timed out on the queue pipeline.")
		return
	}

	// ─── 2. SERVER / WORKER MODE (Default when run without file arguments) ──────
	fmt.Println("🛡️  Secure Execution Daemon Initialization...")

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		fmt.Println("[Telemetry] Operational metrics dashboard path open on http://localhost:2112/metrics")
		_ = http.ListenAndServe(":2112", nil)
	}()

	var wg sync.WaitGroup
	for i := 1; i <= 3; i++ {
		wg.Add(1)
		go worker(i, qm, &wg)
	}

	wg.Wait()
}
