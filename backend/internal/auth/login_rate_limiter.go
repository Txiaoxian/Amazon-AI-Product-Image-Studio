package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/redis/go-redis/v9"
)

var (
	errLoginRateLimited = errors.New("login rate limit exceeded")
	ErrLoginRateLimited = errLoginRateLimited
)

type LoginRateLimitIdentity struct {
	TenantID string
	Email    string
	IP       string
}

type LoginRateLimiter interface {
	Check(ctx context.Context, identity LoginRateLimitIdentity) error
	RecordFailure(ctx context.Context, identity LoginRateLimitIdentity) error
	Reset(ctx context.Context, identity LoginRateLimitIdentity) error
}

type redisLoginRateLimitClient interface {
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
}

type RedisLoginRateLimiter struct {
	client      redisLoginRateLimitClient
	maxFailures int
	windowMS    int64
	keyPrefix   string
}

func NewRedisLoginRateLimiter(queueCfg config.QueueConfig, authCfg config.AuthConfig) *RedisLoginRateLimiter {
	client := redis.NewClient(&redis.Options{
		Addr:     queueCfg.RedisAddr,
		Password: queueCfg.RedisPassword,
		DB:       queueCfg.RedisDB,
	})
	return NewRedisLoginRateLimiterWithClient(client, authCfg)
}

func NewRedisLoginRateLimiterWithClient(client redisLoginRateLimitClient, authCfg config.AuthConfig) *RedisLoginRateLimiter {
	maxFailures := authCfg.LoginRateLimitMaxFailures
	if maxFailures <= 0 {
		maxFailures = 5
	}
	window := authCfg.LoginRateLimitWindow
	if window <= 0 {
		window = 10 * time.Minute
	}
	return &RedisLoginRateLimiter{
		client:      client,
		maxFailures: maxFailures,
		windowMS:    window.Milliseconds(),
		keyPrefix:   "auth:login:fail:",
	}
}

func (l *RedisLoginRateLimiter) Check(ctx context.Context, identity LoginRateLimitIdentity) error {
	if l == nil || l.client == nil {
		return nil
	}
	count, err := l.evalInt(ctx, redisLoginRateLimitCheckScript, identity)
	if err != nil {
		return err
	}
	if count >= int64(l.maxFailures) {
		return errLoginRateLimited
	}
	return nil
}

func (l *RedisLoginRateLimiter) RecordFailure(ctx context.Context, identity LoginRateLimitIdentity) error {
	if l == nil || l.client == nil {
		return nil
	}
	_, err := l.evalInt(ctx, redisLoginRateLimitFailureScript, identity, l.windowMS)
	return err
}

func (l *RedisLoginRateLimiter) Reset(ctx context.Context, identity LoginRateLimitIdentity) error {
	if l == nil || l.client == nil {
		return nil
	}
	return l.client.Del(ctx, l.key(identity)).Err()
}

func (l *RedisLoginRateLimiter) evalInt(ctx context.Context, script string, identity LoginRateLimitIdentity, args ...interface{}) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	result, err := l.client.Eval(ctx, script, []string{l.key(identity)}, args...).Result()
	if err != nil {
		return 0, err
	}
	switch value := result.(type) {
	case int64:
		return value, nil
	case int:
		return int64(value), nil
	case string:
		return strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	default:
		return 0, errors.New("unexpected login rate limit result")
	}
}

func (l *RedisLoginRateLimiter) key(identity LoginRateLimitIdentity) string {
	normalized := strings.Join([]string{
		strings.TrimSpace(identity.TenantID),
		strings.ToLower(strings.TrimSpace(identity.Email)),
		strings.TrimSpace(identity.IP),
	}, "\x00")
	sum := sha256.Sum256([]byte(normalized))
	return l.keyPrefix + hex.EncodeToString(sum[:])
}

const redisLoginRateLimitCheckScript = `
return tonumber(redis.call('GET', KEYS[1]) or '0')
`

const redisLoginRateLimitFailureScript = `
local current = redis.call('INCR', KEYS[1])
if current == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return current
`
