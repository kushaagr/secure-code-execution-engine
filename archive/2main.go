package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"secure-exec-engine/queue"
	"secure-exec-engine/sandbox"
)

func worker(id int, qm *queue.QueueManager, wg *sync.WaitGroup) {
	defer wg.Done()
	ctx := context.Background()
	fmt.Printf("[Worker %d] Booted and listening for untrusted execution streams...\n", id)

	for {
		// 1. Block wait for an incoming task from the Redis pipeline
		task, err := qm.PopTask(ctx)
		if err != nil {
			log.Printf("[Worker %d] Error fetching task from queue: %v", id, err)
			continue
		}

		fmt.Printf("[Worker %d] Processing Task ID: %s (%s)\n", id, task.ID, task.Language)

		// 2. Delegate execution payload to the underlying Docker Sandbox isolation engine
		res, err := sandbox.RunCode(task.Language, task.SourceCode)
		if err != nil {
			log.Printf("[Worker %d] Critical Failure executing script %s: %v", id, task.ID, err)
			_ = qm.SetResult(ctx, task.ID, map[string]string{"error": err.Error()})
			continue
		}

		// 3. Save execution state telemetry back to Redis space
		err = qm.SetResult(ctx, task.ID, res)
		if err != nil {
			log.Printf("[Worker %d] Failed saving task results to state-store: %v", id, err)
		}

		fmt.Printf("[Worker %d] Finished Task ID: %s | TimedOut=%t | ExitCode=%d\n", 
			id, task.ID, res.TimedOut, res.ExitCode)
	}
}

func main() {
	fmt.Println("🛡️ Secure Execution Pipeline Initialization...")
	qm := queue.NewQueueManager()

	// Spin up a thread pool of 3 concurrent background workers
	var wg sync.WaitGroup
	numWorkers := 3
	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go worker(i, qm, &wg)
	}

	// Simulate incoming multi-tenant traffic spikes
	ctx := context.Background()
	time.Sleep(1 * time.Second) // Let workers warm up
	fmt.Println("\n📥 Simulating rapid API submission traffic spike...")

	tasks := []*queue.CodeTask{
		{ID: "req-101", Language: "python", SourceCode: "print('Task 1 completed successfully')", Timestamp: time.Now()},
		{ID: "req-102", Language: "python", SourceCode: "import time\ntime.sleep(10)", Timestamp: time.Now()}, // Will trigger 5s timeout
		{ID: "req-103", Language: "bash", SourceCode: "echo 'Task 3 standard shell context environment verified'", Timestamp: time.Now()},
	}

	for _, t := range tasks {
		fmt.Printf("[API Gateway] Enqueued Task: %s\n", t.ID)
		if err := qm.PushTask(ctx, t); err != nil {
			log.Fatalf("Queue insertion failure: %v", err)
		}
	}

	// Keep the system main run context alive for viewing logs
	wg.Wait()
}
