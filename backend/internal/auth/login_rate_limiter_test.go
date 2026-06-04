package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/redis/go-redis/v9"
)

func TestRedisLoginRateLimiterBlocksAfterFailuresAndUsesOpaqueKey(t *testing.T) {
	ctx := context.Background()
	client := newLoginRateLimitFakeRedis()
	limiter := NewRedisLoginRateLimiterWithClient(client, config.AuthConfig{
		LoginRateLimitMaxFailures: 2,
		LoginRateLimitWindow:      5 * time.Minute,
	})
	identity := LoginRateLimitIdentity{TenantID: "tenant-a", Email: "Admin@Example.com", IP: "203.0.113.9"}

	if err := limiter.Check(ctx, identity); err != nil {
		t.Fatalf("initial Check returned error: %v", err)
	}
	if err := limiter.RecordFailure(ctx, identity); err != nil {
		t.Fatalf("first RecordFailure returned error: %v", err)
	}
	if err := limiter.Check(ctx, identity); err != nil {
		t.Fatalf("Check after one failure returned error: %v", err)
	}
	if err := limiter.RecordFailure(ctx, identity); err != nil {
		t.Fatalf("second RecordFailure returned error: %v", err)
	}
	if err := limiter.Check(ctx, identity); !errors.Is(err, errLoginRateLimited) {
		t.Fatalf("Check after threshold = %v, want errLoginRateLimited", err)
	}

	if len(client.keys) == 0 {
		t.Fatal("fake redis did not capture keys")
	}
	for _, key := range client.keys {
		if strings.Contains(strings.ToLower(key), "admin@example.com") || strings.Contains(key, "tenant-a") || strings.Contains(key, "203.0.113.9") {
			t.Fatalf("rate limit key leaks identity fields: %q", key)
		}
	}
	if client.expireMS != int64((5 * time.Minute).Milliseconds()) {
		t.Fatalf("expireMS = %d, want configured window", client.expireMS)
	}

	if err := limiter.Reset(ctx, identity); err != nil {
		t.Fatalf("Reset returned error: %v", err)
	}
	if err := limiter.Check(ctx, identity); err != nil {
		t.Fatalf("Check after reset returned error: %v", err)
	}
}

func TestRedisLoginRateLimiterFailsClosedOnRedisError(t *testing.T) {
	ctx := context.Background()
	client := newLoginRateLimitFakeRedis()
	client.err = errors.New("redis unavailable")
	limiter := NewRedisLoginRateLimiterWithClient(client, config.AuthConfig{
		LoginRateLimitMaxFailures: 2,
		LoginRateLimitWindow:      time.Minute,
	})

	if err := limiter.Check(ctx, LoginRateLimitIdentity{TenantID: "tenant-a", Email: "admin@example.com", IP: "127.0.0.1"}); err == nil {
		t.Fatal("Check returned nil for redis failure")
	}
}

type loginRateLimitFakeRedis struct {
	counts   map[string]int64
	keys     []string
	expireMS int64
	err      error
}

func newLoginRateLimitFakeRedis() *loginRateLimitFakeRedis {
	return &loginRateLimitFakeRedis{counts: map[string]int64{}}
}

func (r *loginRateLimitFakeRedis) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	cmd := redis.NewCmd(ctx)
	if r.err != nil {
		cmd.SetErr(r.err)
		return cmd
	}
	key := keys[0]
	r.keys = append(r.keys, key)
	switch strings.TrimSpace(script) {
	case strings.TrimSpace(redisLoginRateLimitCheckScript):
		cmd.SetVal(r.counts[key])
	case strings.TrimSpace(redisLoginRateLimitFailureScript):
		if strings.Contains(script, "ARGV[1]") && len(args) < 1 {
			cmd.SetErr(errors.New("missing ARGV[1]"))
			return cmd
		}
		if strings.Contains(script, "ARGV[2]") && len(args) < 2 {
			cmd.SetErr(errors.New("missing ARGV[2]"))
			return cmd
		}
		r.counts[key]++
		if r.counts[key] == 1 {
			r.expireMS = args[0].(int64)
		}
		cmd.SetVal(r.counts[key])
	default:
		cmd.SetErr(errors.New("unexpected script"))
	}
	return cmd
}

func (r *loginRateLimitFakeRedis) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	if r.err != nil {
		cmd.SetErr(r.err)
		return cmd
	}
	var removed int64
	for _, key := range keys {
		r.keys = append(r.keys, key)
		if _, ok := r.counts[key]; ok {
			removed++
			delete(r.counts, key)
		}
	}
	cmd.SetVal(removed)
	return cmd
}
