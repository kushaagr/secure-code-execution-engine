package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// CodeTask represents the execution request payload travelling through our pipeline
type CodeTask struct {
	ID         string    `json:"id"`
	Language   string    `json:"language"`
	SourceCode string    `json:"source_code"`
	Timestamp  time.Time `json:"timestamp"`
}

// QueueManager coordinates interactions with our transient datastore
type QueueManager struct {
	client *redis.Client
	listKey string
}

func NewQueueManager() *QueueManager {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	return &QueueManager{
		client:  rdb,
		listKey: "engine_execution_tasks",
	}
}

// PushTask enqueues a new execution task onto our broker
func (q *QueueManager) PushTask(ctx context.Context, task *CodeTask) error {
	data, err := json.Marshal(task)
	if err != nil {
		return err
	}
	// Using a Redis List as a FIFO queue (First-In, First-Out)
	return q.client.LPush(ctx, q.listKey, data).Err()
}

// PopTask blocks until an execution payload becomes available (BLPop)
func (q *QueueManager) PopTask(ctx context.Context) (*CodeTask, error) {
	// 0 means block indefinitely until an item arrives
	results, err := q.client.BRPop(ctx, 0, q.listKey).Result()
	if err != nil {
		return nil, err
	}

	// results[0] is the key name, results[1] is the string payload
	var task CodeTask
	err = json.Unmarshal([]byte(results[1]), &task)
	if err != nil {
		return nil, err
	}

	return &task, nil
}

// SetResult saves execution returns with an aggressive TTL to prevent storage leaks
func (q *QueueManager) SetResult(ctx context.Context, taskID string, result interface{}) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	// Expire execution output states after 10 minutes to save memory
	return q.client.Set(ctx, "result:"+taskID, data, 10*time.Minute).Err()
}
