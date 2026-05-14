package queue

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/redis/go-redis/v9"
)

var (
	ErrUnavailable = errors.New("task queue unavailable")
	ErrInvalidTask = errors.New("invalid queued task")
)

type TaskEnqueuer interface {
	EnqueueTask(ctx context.Context, taskID string) error
}

type RedisTaskEnqueuer struct {
	client  redis.Cmdable
	queue   string
	timeout time.Duration
}

func NewRedisTaskEnqueuer(cfg config.QueueConfig) *RedisTaskEnqueuer {
	return &RedisTaskEnqueuer{
		client: redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		}),
		queue:   cfg.TaskQueueName,
		timeout: cfg.EnqueueTimeout,
	}
}

func NewRedisTaskEnqueuerWithClient(client redis.Cmdable, queueName string, timeout time.Duration) *RedisTaskEnqueuer {
	return &RedisTaskEnqueuer{client: client, queue: strings.TrimSpace(queueName), timeout: timeout}
}

func (e *RedisTaskEnqueuer) EnqueueTask(ctx context.Context, taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return ErrInvalidTask
	}
	if e == nil || e.client == nil || strings.TrimSpace(e.queue) == "" {
		return ErrUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if e.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.timeout)
		defer cancel()
	}
	if err := e.client.RPush(ctx, e.queue, taskID).Err(); err != nil {
		return err
	}
	return nil
}
