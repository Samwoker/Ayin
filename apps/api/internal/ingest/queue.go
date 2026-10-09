package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
)

var (
	ErrQueueEmpty = errors.New("investigation queue is empty or timed out")
)

// InvestigationJob encapsulates an enqueued investigation task.
type InvestigationJob struct {
	OrgID      pgtype.UUID `json:"org_id"`
	IncidentID pgtype.UUID `json:"incident_id"`
	AlertID    pgtype.UUID `json:"alert_id,omitempty"`
	Title      string      `json:"title"`
	Service    string      `json:"service"`
	Severity   string      `json:"severity"`
	Source     string      `json:"source"`
	EnqueuedAt time.Time   `json:"enqueued_at"`
}

// Queue interface for enqueuing and dequeuing incident investigations.
type Queue interface {
	Enqueue(ctx context.Context, job InvestigationJob) error
	Dequeue(ctx context.Context, timeout time.Duration) (*InvestigationJob, error)
}

// MemoryQueue provides an in-memory Queue implementation for tests and local execution.
type MemoryQueue struct {
	ch chan InvestigationJob
	mu sync.Mutex
}

func NewMemoryQueue(bufferSize int) *MemoryQueue {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &MemoryQueue{
		ch: make(chan InvestigationJob, bufferSize),
	}
}

func (q *MemoryQueue) Enqueue(ctx context.Context, job InvestigationJob) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case q.ch <- job:
		return nil
	default:
		return errors.New("in-memory investigation queue buffer full")
	}
}

func (q *MemoryQueue) Dequeue(ctx context.Context, timeout time.Duration) (*InvestigationJob, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case job := <-q.ch:
		return &job, nil
	case <-timer.C:
		return nil, ErrQueueEmpty
	}
}

// RedisQueue implements Queue using Redis Streams.
type RedisQueue struct {
	client     *redis.Client
	streamKey  string
	groupName  string
	consumerID string
}

func NewRedisQueue(client *redis.Client, streamKey, groupName, consumerID string) *RedisQueue {
	if streamKey == "" {
		streamKey = "stream:investigations"
	}
	if groupName == "" {
		groupName = "triage-workers"
	}
	if consumerID == "" {
		consumerID = fmt.Sprintf("worker-%d", time.Now().UnixNano())
	}

	rq := &RedisQueue{
		client:     client,
		streamKey:  streamKey,
		groupName:  groupName,
		consumerID: consumerID,
	}

	// Initialize consumer group idempotently
	if client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = client.XGroupCreateMkStream(ctx, streamKey, groupName, "0").Err()
	}

	return rq
}

func (q *RedisQueue) Enqueue(ctx context.Context, job InvestigationJob) error {
	if q.client == nil {
		return errors.New("redis client not configured")
	}

	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to serialize investigation job: %w", err)
	}

	orgIDStr := fmt.Sprintf("%x-%x-%x-%x-%x", job.OrgID.Bytes[0:4], job.OrgID.Bytes[4:6], job.OrgID.Bytes[6:8], job.OrgID.Bytes[8:10], job.OrgID.Bytes[10:16])
	incidentIDStr := fmt.Sprintf("%x-%x-%x-%x-%x", job.IncidentID.Bytes[0:4], job.IncidentID.Bytes[4:6], job.IncidentID.Bytes[6:8], job.IncidentID.Bytes[8:10], job.IncidentID.Bytes[10:16])

	err = q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: q.streamKey,
		Values: map[string]interface{}{
			"org_id":      orgIDStr,
			"incident_id": incidentIDStr,
			"payload":     string(payload),
		},
	}).Err()

	if err != nil {
		return fmt.Errorf("failed to enqueue investigation to redis stream: %w", err)
	}

	return nil
}

func (q *RedisQueue) Dequeue(ctx context.Context, timeout time.Duration) (*InvestigationJob, error) {
	if q.client == nil {
		return nil, errors.New("redis client not configured")
	}

	streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    q.groupName,
		Consumer: q.consumerID,
		Streams:  []string{q.streamKey, ">"},
		Count:    1,
		Block:    timeout,
	}).Result()

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrQueueEmpty
		}
		return nil, err
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil, ErrQueueEmpty
	}

	msg := streams[0].Messages[0]
	rawPayload, ok := msg.Values["payload"].(string)
	if !ok {
		return nil, errors.New("malformed redis stream job payload")
	}

	var job InvestigationJob
	if err := json.Unmarshal([]byte(rawPayload), &job); err != nil {
		return nil, fmt.Errorf("failed to decode investigation job: %w", err)
	}

	// Acknowledge processed message
	_ = q.client.XAck(ctx, q.streamKey, q.groupName, msg.ID).Err()

	return &job, nil
}
