package queue

import (
	"context"
	"strings"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/redis/go-redis/v9"
)

// QueueDepth holds sanitized aggregate counts for the reliable task queue.
// It intentionally omits Redis key names, queue payloads, claim IDs, and
// task payload bodies.
type QueueDepth struct {
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
	Pending    int64  `json:"pending"`
	Processing int64  `json:"processing"`
	Delayed    int64  `json:"delayed"`
	Dead       int64  `json:"dead"`
}

// QueueDepthInspector is a read-only interface for inspecting aggregate queue
// depth without exposing internal Redis keys, payloads, or claim details.
type QueueDepthInspector interface {
	Inspect(ctx context.Context) QueueDepth
}

// depthInspectorRedisClient is the minimal Redis command surface needed by the
// depth inspector. It is intentionally separate from the reliableQueueRedisClient
// interface used by the mutable queue operations.
type depthInspectorRedisClient interface {
	LLen(ctx context.Context, key string) *redis.IntCmd
	ZCard(ctx context.Context, key string) *redis.IntCmd
}

// RedisQueueDepthInspector reads aggregate queue depth from Redis.
type RedisQueueDepthInspector struct {
	client     depthInspectorRedisClient
	queue      string
	processing string
	delayed    string
	dead       string
}

// NewRedisQueueDepthInspector creates a depth inspector using the provided
// queue configuration. The caller may also use NewRedisQueueDepthInspectorWithClient
// for testing with a fake Redis client.
func NewRedisQueueDepthInspector(cfg config.QueueConfig) *RedisQueueDepthInspector {
	return NewRedisQueueDepthInspectorWithClient(NewRedisClient(cfg), cfg)
}

// NewRedisQueueDepthInspectorWithClient creates a depth inspector with an
// explicit Redis client, useful for testing.
func NewRedisQueueDepthInspectorWithClient(client redis.Cmdable, cfg config.QueueConfig) *RedisQueueDepthInspector {
	queueName := strings.TrimSpace(cfg.TaskQueueName)
	return &RedisQueueDepthInspector{
		client:     client,
		queue:      queueName,
		processing: queueName + ":processing",
		delayed:    queueName + ":delayed",
		dead:       queueName + ":dead",
	}
}

// Inspect returns the current aggregate queue depth. If Redis is unavailable
// or misconfigured, it returns a section-level status="unavailable" with a
// sanitized reason code instead of leaking connection strings or raw errors.
func (i *RedisQueueDepthInspector) Inspect(ctx context.Context) QueueDepth {
	if i == nil || i.client == nil || strings.TrimSpace(i.queue) == "" {
		return unavailableQueueDepth()
	}
	if ctx == nil {
		ctx = context.Background()
	}

	pending, err := i.client.LLen(ctx, i.queue).Result()
	if err != nil {
		return unavailableQueueDepth()
	}
	processing, err := i.client.LLen(ctx, i.processing).Result()
	if err != nil {
		return unavailableQueueDepth()
	}
	delayed, err := i.client.ZCard(ctx, i.delayed).Result()
	if err != nil {
		return unavailableQueueDepth()
	}
	dead, err := i.client.LLen(ctx, i.dead).Result()
	if err != nil {
		return unavailableQueueDepth()
	}

	return QueueDepth{
		Status:     "available",
		Pending:    pending,
		Processing: processing,
		Delayed:    delayed,
		Dead:       dead,
	}
}

func unavailableQueueDepth() QueueDepth {
	return QueueDepth{Status: "unavailable", Reason: "queue_unavailable"}
}

// NilQueueDepthInspector always returns an unavailable queue depth.
// Used when no Redis connection is available.
type NilQueueDepthInspector struct{}

// Inspect always returns status="unavailable".
func (NilQueueDepthInspector) Inspect(_ context.Context) QueueDepth {
	return unavailableQueueDepth()
}
