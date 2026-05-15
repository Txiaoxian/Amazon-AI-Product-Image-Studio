package queue

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/redis/go-redis/v9"
)

var (
	ErrNoTask        = errors.New("no queued task available")
	ErrDeadLettered  = errors.New("queued task moved to dead letter")
	ErrInvalidClaim  = errors.New("invalid queue claim")
	ErrInvalidConfig = errors.New("invalid queue configuration")
)

type TaskClaim struct {
	TaskID        string
	DeliveryCount int64
	DeadLettered  bool
}

type ReliableTaskQueue interface {
	TaskEnqueuer
	Claim(ctx context.Context) (TaskClaim, error)
	Ack(ctx context.Context, claim TaskClaim) error
	Retry(ctx context.Context, claim TaskClaim, delay time.Duration) error
	DeadLetter(ctx context.Context, claim TaskClaim, reason string) error
	RecoverStale(ctx context.Context, now time.Time, limit int) ([]string, error)
	PromoteDue(ctx context.Context, now time.Time, limit int) ([]string, error)
}

type RedisReliableTaskQueue struct {
	client            redis.Cmdable
	queue             string
	processing        string
	processingClaims  string
	delayed           string
	dead              string
	deliveries        string
	claimTimeout      time.Duration
	visibilityTimeout time.Duration
	maxDeliveries     int64
}

func NewRedisReliableTaskQueue(cfg config.QueueConfig) *RedisReliableTaskQueue {
	return NewRedisReliableTaskQueueWithClient(NewRedisClient(cfg), cfg)
}

func NewRedisReliableTaskQueueWithClient(client redis.Cmdable, cfg config.QueueConfig) *RedisReliableTaskQueue {
	queueName := strings.TrimSpace(cfg.TaskQueueName)
	return &RedisReliableTaskQueue{
		client:            client,
		queue:             queueName,
		processing:        queueName + ":processing",
		processingClaims:  queueName + ":processing_claims",
		delayed:           queueName + ":delayed",
		dead:              queueName + ":dead",
		deliveries:        queueName + ":deliveries",
		claimTimeout:      cfg.ClaimTimeout,
		visibilityTimeout: cfg.VisibilityTimeout,
		maxDeliveries:     int64(cfg.MaxDeliveries),
	}
}

func (q *RedisReliableTaskQueue) EnqueueTask(ctx context.Context, taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return ErrInvalidTask
	}
	if err := q.validate(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return q.client.RPush(ctx, q.queue, taskID).Err()
}

func (q *RedisReliableTaskQueue) Claim(ctx context.Context) (TaskClaim, error) {
	if err := q.validate(); err != nil {
		return TaskClaim{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	now := time.Now().UTC()
	if _, err := q.PromoteDue(ctx, now, 100); err != nil {
		return TaskClaim{}, err
	}

	taskID, err := q.client.BLMove(ctx, q.queue, q.processing, "LEFT", "RIGHT", q.claimTimeout).Result()
	if errors.Is(err, redis.Nil) {
		return TaskClaim{}, ErrNoTask
	}
	if err != nil {
		return TaskClaim{}, err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return TaskClaim{}, ErrInvalidTask
	}

	deliveryCount, err := q.client.HIncrBy(ctx, q.deliveries, taskID, 1).Result()
	if err != nil {
		return TaskClaim{}, err
	}
	if err := q.client.HSet(ctx, q.processingClaims, taskID, strconv.FormatInt(now.UnixMilli(), 10)).Err(); err != nil {
		return TaskClaim{}, err
	}

	claim := TaskClaim{TaskID: taskID, DeliveryCount: deliveryCount}
	if q.maxDeliveries > 0 && deliveryCount > q.maxDeliveries {
		if err := q.DeadLetter(ctx, claim, "max_deliveries_exceeded"); err != nil {
			return TaskClaim{}, err
		}
		claim.DeadLettered = true
		return claim, ErrDeadLettered
	}
	return claim, nil
}

func (q *RedisReliableTaskQueue) Ack(ctx context.Context, claim TaskClaim) error {
	taskID, err := taskIDFromClaim(claim)
	if err != nil {
		return err
	}
	if err := q.validate(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := q.client.LRem(ctx, q.processing, 1, taskID).Err(); err != nil {
		return err
	}
	if err := q.client.HDel(ctx, q.processingClaims, taskID).Err(); err != nil {
		return err
	}
	return q.client.HDel(ctx, q.deliveries, taskID).Err()
}

func (q *RedisReliableTaskQueue) Retry(ctx context.Context, claim TaskClaim, delay time.Duration) error {
	taskID, err := taskIDFromClaim(claim)
	if err != nil {
		return err
	}
	if err := q.validate(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := q.client.LRem(ctx, q.processing, 1, taskID).Err(); err != nil {
		return err
	}
	if err := q.client.HDel(ctx, q.processingClaims, taskID).Err(); err != nil {
		return err
	}
	if delay <= 0 {
		return q.client.RPush(ctx, q.queue, taskID).Err()
	}
	return q.client.ZAdd(ctx, q.delayed, redis.Z{
		Score:  float64(time.Now().UTC().Add(delay).UnixMilli()),
		Member: taskID,
	}).Err()
}

func (q *RedisReliableTaskQueue) DeadLetter(ctx context.Context, claim TaskClaim, reason string) error {
	taskID, err := taskIDFromClaim(claim)
	if err != nil {
		return err
	}
	if err := q.validate(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := q.client.LRem(ctx, q.processing, 1, taskID).Err(); err != nil {
		return err
	}
	if err := q.client.HDel(ctx, q.processingClaims, taskID).Err(); err != nil {
		return err
	}
	if err := q.client.RPush(ctx, q.dead, taskID).Err(); err != nil {
		return err
	}
	if strings.TrimSpace(reason) != "" {
		return q.client.HSet(ctx, q.dead+":reasons", taskID, strings.TrimSpace(reason)).Err()
	}
	return nil
}

func (q *RedisReliableTaskQueue) RecoverStale(ctx context.Context, now time.Time, limit int) ([]string, error) {
	if err := q.validate(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		limit = 100
	}
	before := now.UTC().Add(-q.visibilityTimeout).UnixMilli()
	claims, err := q.client.HGetAll(ctx, q.processingClaims).Result()
	if err != nil {
		return nil, err
	}
	recovered := make([]string, 0)
	for taskID, rawClaimedAt := range claims {
		if len(recovered) >= limit {
			break
		}
		claimedAt, err := strconv.ParseInt(rawClaimedAt, 10, 64)
		if err != nil || claimedAt > before {
			continue
		}
		if err := q.client.LRem(ctx, q.processing, 1, taskID).Err(); err != nil {
			return recovered, err
		}
		if err := q.client.HDel(ctx, q.processingClaims, taskID).Err(); err != nil {
			return recovered, err
		}
		if err := q.client.RPush(ctx, q.queue, taskID).Err(); err != nil {
			return recovered, err
		}
		recovered = append(recovered, taskID)
	}
	return recovered, nil
}

func (q *RedisReliableTaskQueue) PromoteDue(ctx context.Context, now time.Time, limit int) ([]string, error) {
	if err := q.validate(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		limit = 100
	}
	taskIDs, err := q.client.ZRangeByScore(ctx, q.delayed, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    strconv.FormatInt(now.UTC().UnixMilli(), 10),
		Offset: 0,
		Count:  int64(limit),
	}).Result()
	if err != nil {
		return nil, err
	}
	for _, taskID := range taskIDs {
		removed, err := q.client.ZRem(ctx, q.delayed, taskID).Result()
		if err != nil {
			return nil, err
		}
		if removed == 0 {
			continue
		}
		if err := q.client.RPush(ctx, q.queue, taskID).Err(); err != nil {
			return nil, err
		}
	}
	return taskIDs, nil
}

func (q *RedisReliableTaskQueue) validate() error {
	if q == nil || q.client == nil || strings.TrimSpace(q.queue) == "" {
		return ErrUnavailable
	}
	if q.claimTimeout <= 0 || q.visibilityTimeout <= 0 {
		return fmt.Errorf("%w: non-positive queue timeout", ErrInvalidConfig)
	}
	return nil
}

func taskIDFromClaim(claim TaskClaim) (string, error) {
	taskID := strings.TrimSpace(claim.TaskID)
	if taskID == "" {
		return "", ErrInvalidClaim
	}
	return taskID, nil
}
