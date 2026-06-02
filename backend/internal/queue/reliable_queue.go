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

type reliableQueueRedisClient interface {
	RPush(ctx context.Context, key string, values ...interface{}) *redis.IntCmd
	BLMove(ctx context.Context, source, destination, srcpos, destpos string, timeout time.Duration) *redis.StringCmd
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
	LRem(ctx context.Context, key string, count int64, value interface{}) *redis.IntCmd
	HDel(ctx context.Context, key string, fields ...string) *redis.IntCmd
	HSet(ctx context.Context, key string, values ...interface{}) *redis.IntCmd
	HGet(ctx context.Context, key, field string) *redis.StringCmd
	LRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd
	ZAdd(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd
	ZRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.StringSliceCmd
	ZRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
}

type RedisReliableTaskQueue struct {
	client            reliableQueueRedisClient
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
	return newRedisReliableTaskQueueWithClient(NewRedisClient(cfg), cfg)
}

func NewRedisReliableTaskQueueWithClient(client redis.Cmdable, cfg config.QueueConfig) *RedisReliableTaskQueue {
	return newRedisReliableTaskQueueWithClient(client, cfg)
}

func newRedisReliableTaskQueueWithClient(client reliableQueueRedisClient, cfg config.QueueConfig) *RedisReliableTaskQueue {
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

	deliveryCount, err := q.recordClaim(ctx, taskID, now)
	if err != nil {
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
	_, err = q.evalMoved(ctx, redisAckScript, []string{q.processing, q.processingClaims, q.deliveries}, taskID)
	return err
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
	if delay <= 0 {
		_, err = q.evalMoved(ctx, redisRetryImmediateScript, []string{q.processing, q.processingClaims, q.queue}, taskID)
		return err
	}
	_, err = q.evalMoved(ctx, redisRetryDelayedScript, []string{q.processing, q.processingClaims, q.delayed}, taskID, strconv.FormatInt(time.Now().UTC().Add(delay).UnixMilli(), 10))
	return err
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
	_, err = q.evalMoved(ctx, redisDeadLetterScript, []string{q.processing, q.processingClaims, q.dead, q.dead + ":reasons"}, taskID, strings.TrimSpace(reason))
	return err
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
	processingTasks, err := q.client.LRange(ctx, q.processing, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}
	recovered := make([]string, 0)
	for _, taskID := range processingTasks {
		taskID = strings.TrimSpace(taskID)
		if taskID == "" {
			continue
		}
		moved, err := q.evalMoved(ctx, redisRecoverStaleScript, []string{q.processing, q.processingClaims, q.queue}, taskID, strconv.FormatInt(before, 10))
		if err != nil {
			return recovered, err
		}
		if !moved {
			continue
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
	promoted := make([]string, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		moved, err := q.evalMoved(ctx, redisPromoteDueScript, []string{q.delayed, q.queue}, taskID)
		if err != nil {
			return nil, err
		}
		if !moved {
			continue
		}
		promoted = append(promoted, taskID)
	}
	return promoted, nil
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

func (q *RedisReliableTaskQueue) recordClaim(ctx context.Context, taskID string, now time.Time) (int64, error) {
	result, err := q.client.Eval(ctx, redisRecordClaimScript, []string{q.deliveries, q.processingClaims}, taskID, strconv.FormatInt(now.UTC().UnixMilli(), 10)).Result()
	if err != nil {
		return 0, err
	}
	return redisIntegerResult(result)
}

func (q *RedisReliableTaskQueue) evalMoved(ctx context.Context, script string, keys []string, args ...interface{}) (bool, error) {
	result, err := q.client.Eval(ctx, script, keys, args...).Result()
	if err != nil {
		return false, err
	}
	moved, err := redisIntegerResult(result)
	if err != nil {
		return false, err
	}
	return moved != 0, nil
}

func redisIntegerResult(result interface{}) (int64, error) {
	switch value := result.(type) {
	case int64:
		return value, nil
	case int:
		return int64(value), nil
	case string:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("%w: unexpected redis integer result %T", ErrInvalidConfig, result)
	}
}

func taskIDFromClaim(claim TaskClaim) (string, error) {
	taskID := strings.TrimSpace(claim.TaskID)
	if taskID == "" {
		return "", ErrInvalidClaim
	}
	return taskID, nil
}

const redisRecordClaimScript = `
redis.call('HGET', KEYS[1], ARGV[1])
redis.call('HGET', KEYS[2], ARGV[1])
local delivery = redis.call('HINCRBY', KEYS[1], ARGV[1], 1)
redis.call('HSET', KEYS[2], ARGV[1], ARGV[2])
return delivery
`

const redisAckScript = `
if not redis.call('LPOS', KEYS[1], ARGV[1]) then
  return 0
end
redis.call('HGET', KEYS[2], ARGV[1])
redis.call('HGET', KEYS[3], ARGV[1])
redis.call('LREM', KEYS[1], 1, ARGV[1])
redis.call('HDEL', KEYS[2], ARGV[1])
redis.call('HDEL', KEYS[3], ARGV[1])
return 1
`

const redisRetryImmediateScript = `
if not redis.call('LPOS', KEYS[1], ARGV[1]) then
  return 0
end
redis.call('HGET', KEYS[2], ARGV[1])
redis.call('RPUSH', KEYS[3], ARGV[1])
redis.call('LREM', KEYS[1], 1, ARGV[1])
redis.call('HDEL', KEYS[2], ARGV[1])
return 1
`

const redisRetryDelayedScript = `
if not redis.call('LPOS', KEYS[1], ARGV[1]) then
  return 0
end
redis.call('HGET', KEYS[2], ARGV[1])
redis.call('ZADD', KEYS[3], ARGV[2], ARGV[1])
redis.call('LREM', KEYS[1], 1, ARGV[1])
redis.call('HDEL', KEYS[2], ARGV[1])
return 1
`

const redisDeadLetterScript = `
if not redis.call('LPOS', KEYS[1], ARGV[1]) then
  return 0
end
redis.call('HGET', KEYS[2], ARGV[1])
redis.call('HGET', KEYS[4], ARGV[1])
if ARGV[2] ~= '' then
  redis.call('HSET', KEYS[4], ARGV[1], ARGV[2])
end
redis.call('RPUSH', KEYS[3], ARGV[1])
redis.call('LREM', KEYS[1], 1, ARGV[1])
redis.call('HDEL', KEYS[2], ARGV[1])
return 1
`

const redisRecoverStaleScript = `
if not redis.call('LPOS', KEYS[1], ARGV[1]) then
  return 0
end
local claimed_at = redis.call('HGET', KEYS[2], ARGV[1])
if claimed_at and tonumber(claimed_at) and tonumber(claimed_at) > tonumber(ARGV[2]) then
  return 0
end
redis.call('RPUSH', KEYS[3], ARGV[1])
redis.call('LREM', KEYS[1], 1, ARGV[1])
redis.call('HDEL', KEYS[2], ARGV[1])
return 1
`

const redisPromoteDueScript = `
if not redis.call('ZSCORE', KEYS[1], ARGV[1]) then
  return 0
end
redis.call('RPUSH', KEYS[2], ARGV[1])
redis.call('ZREM', KEYS[1], ARGV[1])
return 1
`
