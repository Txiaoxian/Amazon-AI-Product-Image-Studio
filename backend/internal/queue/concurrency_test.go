package queue

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisConcurrencyLimiterRenewExtendsHeldLease(t *testing.T) {
	ctx := context.Background()
	client := newConcurrencyFakeRedis()
	limiter := NewRedisConcurrencyLimiterWithClient(client, "tasks")
	lease := ConcurrencyLease{
		ID:   "lease-renew",
		Keys: []string{"tasks:concurrency:tenant:t1", "tasks:concurrency:user:u1"},
	}
	client.zsets[lease.Keys[0]] = map[string]float64{lease.ID: 100}
	client.zsets[lease.Keys[1]] = map[string]float64{lease.ID: 100}
	now := time.Date(2026, 6, 5, 8, 0, 0, 0, time.UTC)

	renewed, err := limiter.Renew(ctx, lease, 30*time.Second, now)
	if err != nil {
		t.Fatalf("Renew returned error: %v", err)
	}
	wantScore := float64(now.Add(30 * time.Second).UnixMilli())
	if renewed.ExpiresAt.UTC() != now.Add(30*time.Second) {
		t.Fatalf("ExpiresAt = %s, want %s", renewed.ExpiresAt, now.Add(30*time.Second))
	}
	for _, key := range lease.Keys {
		if client.zsets[key][lease.ID] != wantScore {
			t.Fatalf("score for %s = %.0f, want %.0f", key, client.zsets[key][lease.ID], wantScore)
		}
		if client.ttls[key] != 30*time.Second {
			t.Fatalf("ttl for %s = %s, want 30s", key, client.ttls[key])
		}
	}
}

func TestRedisConcurrencyLimiterRenewFailsWhenLeaseIsMissing(t *testing.T) {
	ctx := context.Background()
	client := newConcurrencyFakeRedis()
	limiter := NewRedisConcurrencyLimiterWithClient(client, "tasks")
	lease := ConcurrencyLease{
		ID:   "lease-missing",
		Keys: []string{"tasks:concurrency:tenant:t1", "tasks:concurrency:user:u1"},
	}
	client.zsets[lease.Keys[0]] = map[string]float64{lease.ID: 100}
	client.zsets[lease.Keys[1]] = map[string]float64{}

	if _, err := limiter.Renew(ctx, lease, 30*time.Second, time.Now().UTC()); !errors.Is(err, ErrConcurrencyLeaseLost) {
		t.Fatalf("Renew error = %v, want ErrConcurrencyLeaseLost", err)
	}
}

type concurrencyFakeRedis struct {
	zsets map[string]map[string]float64
	sets  map[string]map[string]struct{}
	ttls  map[string]time.Duration
}

func newConcurrencyFakeRedis() *concurrencyFakeRedis {
	return &concurrencyFakeRedis{
		zsets: map[string]map[string]float64{},
		sets:  map[string]map[string]struct{}{},
		ttls:  map[string]time.Duration{},
	}
}

func (r *concurrencyFakeRedis) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	cmd := redis.NewCmd(ctx)
	switch strings.TrimSpace(script) {
	case strings.TrimSpace(redisRenewSemaphoreScript):
		r.evalRenew(cmd, keys, args...)
	case strings.TrimSpace(redisAcquireSemaphoreScript):
		r.evalAcquire(cmd, keys, args...)
	default:
		cmd.SetErr(errors.New("unexpected concurrency script"))
	}
	return cmd
}

func (r *concurrencyFakeRedis) evalRenew(cmd *redis.Cmd, keys []string, args ...interface{}) {
	expiresAt, _ := strconv.ParseFloat(strings.TrimSpace(fmtArg(args[0])), 64)
	leaseID := strings.TrimSpace(fmtArg(args[1]))
	ttl, _ := strconv.ParseInt(strings.TrimSpace(fmtArg(args[2])), 10, 64)
	for _, key := range keys {
		if _, ok := r.zsets[key][leaseID]; !ok {
			cmd.SetVal(int64(0))
			return
		}
	}
	for _, key := range keys {
		if r.zsets[key] == nil {
			r.zsets[key] = map[string]float64{}
		}
		r.zsets[key][leaseID] = expiresAt
		r.ttls[key] = time.Duration(ttl) * time.Millisecond
	}
	cmd.SetVal(int64(1))
}

func (r *concurrencyFakeRedis) evalAcquire(cmd *redis.Cmd, keys []string, args ...interface{}) {
	if len(keys) != 1 {
		cmd.SetErr(errors.New("unexpected acquire keys"))
		return
	}
	now, _ := strconv.ParseFloat(strings.TrimSpace(fmtArg(args[0])), 64)
	limit, _ := strconv.Atoi(strings.TrimSpace(fmtArg(args[1])))
	expiresAt, _ := strconv.ParseFloat(strings.TrimSpace(fmtArg(args[2])), 64)
	leaseID := strings.TrimSpace(fmtArg(args[3]))
	ttl, _ := strconv.ParseInt(strings.TrimSpace(fmtArg(args[4])), 10, 64)
	key := keys[0]
	for member, score := range r.zsets[key] {
		if score <= now {
			delete(r.zsets[key], member)
		}
	}
	if len(r.zsets[key]) >= limit {
		cmd.SetVal(int64(0))
		return
	}
	if r.zsets[key] == nil {
		r.zsets[key] = map[string]float64{}
	}
	r.zsets[key][leaseID] = expiresAt
	r.ttls[key] = time.Duration(ttl) * time.Millisecond
	cmd.SetVal(int64(1))
}

func (r *concurrencyFakeRedis) SAdd(ctx context.Context, key string, members ...interface{}) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	if r.sets[key] == nil {
		r.sets[key] = map[string]struct{}{}
	}
	var added int64
	for _, member := range members {
		value := strings.TrimSpace(fmtArg(member))
		if _, exists := r.sets[key][value]; !exists {
			added++
		}
		r.sets[key][value] = struct{}{}
	}
	cmd.SetVal(added)
	return cmd
}

func (r *concurrencyFakeRedis) ZRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	var removed int64
	for _, member := range members {
		value := strings.TrimSpace(fmtArg(member))
		if _, exists := r.zsets[key][value]; exists {
			delete(r.zsets[key], value)
			removed++
		}
	}
	cmd.SetVal(removed)
	return cmd
}

func (r *concurrencyFakeRedis) SMembers(ctx context.Context, key string) *redis.StringSliceCmd {
	cmd := redis.NewStringSliceCmd(ctx)
	values := make([]string, 0, len(r.sets[key]))
	for member := range r.sets[key] {
		values = append(values, member)
	}
	cmd.SetVal(values)
	return cmd
}

func (r *concurrencyFakeRedis) ZRemRangeByScore(ctx context.Context, key, _, max string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	maxScore, _ := strconv.ParseFloat(max, 64)
	var removed int64
	for member, score := range r.zsets[key] {
		if score <= maxScore {
			delete(r.zsets[key], member)
			removed++
		}
	}
	cmd.SetVal(removed)
	return cmd
}

func fmtArg(value interface{}) string {
	return strings.TrimSpace(fmt.Sprint(value))
}

var _ concurrencyRedisClient = (*concurrencyFakeRedis)(nil)
