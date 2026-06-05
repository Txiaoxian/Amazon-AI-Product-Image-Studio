package queue

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/idgen"
	"github.com/redis/go-redis/v9"
)

var ErrConcurrencyLimited = errors.New("task concurrency limit reached")
var ErrConcurrencyLeaseLost = errors.New("task concurrency lease lost")

type ConcurrencyDimension struct {
	Name  string
	Value string
	Limit int
}

type ConcurrencyLease struct {
	ID        string
	ExpiresAt time.Time
	Keys      []string
}

type ConcurrencyLimiter interface {
	Acquire(ctx context.Context, dimensions []ConcurrencyDimension, ttl time.Duration, now time.Time) (ConcurrencyLease, error)
	Renew(ctx context.Context, lease ConcurrencyLease, ttl time.Duration, now time.Time) (ConcurrencyLease, error)
	Release(ctx context.Context, lease ConcurrencyLease) error
	ReapStale(ctx context.Context, now time.Time) error
}

type concurrencyRedisClient interface {
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
	SAdd(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	ZRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	SMembers(ctx context.Context, key string) *redis.StringSliceCmd
	ZRemRangeByScore(ctx context.Context, key, min, max string) *redis.IntCmd
}

type RedisConcurrencyLimiter struct {
	client  concurrencyRedisClient
	prefix  string
	keysSet string
}

func NewRedisConcurrencyLimiter(cfg config.QueueConfig) *RedisConcurrencyLimiter {
	return NewRedisConcurrencyLimiterWithClient(NewRedisClient(cfg), cfg.TaskQueueName)
}

func NewRedisConcurrencyLimiterWithClient(client concurrencyRedisClient, queueName string) *RedisConcurrencyLimiter {
	prefix := strings.TrimSpace(queueName) + ":concurrency"
	return &RedisConcurrencyLimiter{
		client:  client,
		prefix:  prefix,
		keysSet: prefix + ":keys",
	}
}

func (l *RedisConcurrencyLimiter) Acquire(ctx context.Context, dimensions []ConcurrencyDimension, ttl time.Duration, now time.Time) (ConcurrencyLease, error) {
	if l == nil || l.client == nil || strings.TrimSpace(l.prefix) == "" {
		return ConcurrencyLease{}, ErrUnavailable
	}
	if ttl <= 0 {
		return ConcurrencyLease{}, fmt.Errorf("%w: non-positive concurrency lease ttl", ErrInvalidConfig)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	lease := ConcurrencyLease{
		ID:        idgen.New(),
		ExpiresAt: now.UTC().Add(ttl),
		Keys:      make([]string, 0, len(dimensions)),
	}
	for _, dimension := range dimensions {
		if dimension.Limit <= 0 {
			continue
		}
		key := l.key(dimension)
		if key == "" {
			continue
		}
		ok, err := l.acquireKey(ctx, key, lease.ID, dimension.Limit, now, lease.ExpiresAt, ttl)
		if err != nil {
			_ = l.Release(ctx, lease)
			return ConcurrencyLease{}, err
		}
		if !ok {
			_ = l.Release(ctx, lease)
			return ConcurrencyLease{}, ErrConcurrencyLimited
		}
		lease.Keys = append(lease.Keys, key)
		if err := l.client.SAdd(ctx, l.keysSet, key).Err(); err != nil {
			_ = l.Release(ctx, lease)
			return ConcurrencyLease{}, err
		}
	}
	return lease, nil
}

func (l *RedisConcurrencyLimiter) Release(ctx context.Context, lease ConcurrencyLease) error {
	if l == nil || l.client == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for _, key := range lease.Keys {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(lease.ID) == "" {
			continue
		}
		if err := l.client.ZRem(ctx, key, lease.ID).Err(); err != nil {
			return err
		}
	}
	return nil
}

func (l *RedisConcurrencyLimiter) Renew(ctx context.Context, lease ConcurrencyLease, ttl time.Duration, now time.Time) (ConcurrencyLease, error) {
	if l == nil || l.client == nil || strings.TrimSpace(l.prefix) == "" {
		return ConcurrencyLease{}, ErrUnavailable
	}
	if ttl <= 0 {
		return ConcurrencyLease{}, fmt.Errorf("%w: non-positive concurrency lease ttl", ErrInvalidConfig)
	}
	if strings.TrimSpace(lease.ID) == "" {
		return ConcurrencyLease{}, fmt.Errorf("%w: empty concurrency lease id", ErrInvalidClaim)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	keys := make([]string, 0, len(lease.Keys))
	for _, key := range lease.Keys {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		lease.ExpiresAt = now.UTC().Add(ttl)
		return lease, nil
	}
	expiresAt := now.UTC().Add(ttl)
	result, err := l.client.Eval(ctx, redisRenewSemaphoreScript, keys,
		fmt.Sprintf("%d", expiresAt.UTC().UnixMilli()),
		lease.ID,
		int64(ttl/time.Millisecond),
	).Int()
	if err != nil {
		return ConcurrencyLease{}, err
	}
	if result != 1 {
		return ConcurrencyLease{}, ErrConcurrencyLeaseLost
	}
	lease.ExpiresAt = expiresAt
	lease.Keys = keys
	return lease, nil
}

func (l *RedisConcurrencyLimiter) ReapStale(ctx context.Context, now time.Time) error {
	if l == nil || l.client == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	keys, err := l.client.SMembers(ctx, l.keysSet).Result()
	if err != nil {
		return err
	}
	score := fmt.Sprintf("%d", now.UTC().UnixMilli())
	for _, key := range keys {
		if err := l.client.ZRemRangeByScore(ctx, key, "-inf", score).Err(); err != nil {
			return err
		}
	}
	return nil
}

func (l *RedisConcurrencyLimiter) acquireKey(ctx context.Context, key string, leaseID string, limit int, now time.Time, expiresAt time.Time, ttl time.Duration) (bool, error) {
	result, err := l.client.Eval(ctx, redisAcquireSemaphoreScript, []string{key},
		fmt.Sprintf("%d", now.UTC().UnixMilli()),
		limit,
		fmt.Sprintf("%d", expiresAt.UTC().UnixMilli()),
		leaseID,
		int64(ttl/time.Millisecond),
	).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (l *RedisConcurrencyLimiter) key(dimension ConcurrencyDimension) string {
	name := normalizeKeyPart(dimension.Name)
	value := normalizeKeyPart(dimension.Value)
	if name == "" || value == "" {
		return ""
	}
	return l.prefix + ":" + name + ":" + value
}

func normalizeKeyPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer(":", "_", " ", "_", "\n", "_", "\r", "_", "\t", "_")
	return replacer.Replace(value)
}

const redisAcquireSemaphoreScript = `
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
local count = redis.call('ZCARD', KEYS[1])
if count >= tonumber(ARGV[2]) then
  return 0
end
redis.call('ZADD', KEYS[1], ARGV[3], ARGV[4])
redis.call('PEXPIRE', KEYS[1], ARGV[5])
return 1
`

const redisRenewSemaphoreScript = `
for i, key in ipairs(KEYS) do
  local score = redis.call('ZSCORE', key, ARGV[2])
  if not score then
    return 0
  end
end
for i, key in ipairs(KEYS) do
  redis.call('ZADD', key, ARGV[1], ARGV[2])
  redis.call('PEXPIRE', key, ARGV[3])
end
return 1
`
