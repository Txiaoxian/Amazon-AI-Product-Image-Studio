package queue

import (
	"context"
	"errors"
	"testing"

	"github.com/redis/go-redis/v9"
)

// fakeDepthRedisClient implements depthInspectorRedisClient for tests.
type fakeDepthRedisClient struct {
	llenResults map[string]int64
	zcardResult int64
	err         error
}

func (f *fakeDepthRedisClient) LLen(_ context.Context, key string) *redis.IntCmd {
	cmd := redis.NewIntCmd(context.Background())
	if f.err != nil {
		cmd.SetErr(f.err)
		return cmd
	}
	cmd.SetVal(f.llenResults[key])
	return cmd
}

func (f *fakeDepthRedisClient) ZCard(_ context.Context, _ string) *redis.IntCmd {
	cmd := redis.NewIntCmd(context.Background())
	if f.err != nil {
		cmd.SetErr(f.err)
		return cmd
	}
	cmd.SetVal(f.zcardResult)
	return cmd
}

func TestRedisQueueDepthInspector_Available(t *testing.T) {
	client := &fakeDepthRedisClient{
		llenResults: map[string]int64{
			"tasks":            5,
			"tasks:processing": 2,
			"tasks:dead":       1,
		},
		zcardResult: 3,
	}
	inspector := &RedisQueueDepthInspector{
		client:     client,
		queue:      "tasks",
		processing: "tasks:processing",
		delayed:    "tasks:delayed",
		dead:       "tasks:dead",
	}

	depth := inspector.Inspect(context.Background())

	if depth.Status != "available" {
		t.Fatalf("status = %q, want available", depth.Status)
	}
	if depth.Reason != "" {
		t.Errorf("reason = %q, want empty for available", depth.Reason)
	}
	if depth.Pending != 5 {
		t.Errorf("pending = %d, want 5", depth.Pending)
	}
	if depth.Processing != 2 {
		t.Errorf("processing = %d, want 2", depth.Processing)
	}
	if depth.Delayed != 3 {
		t.Errorf("delayed = %d, want 3", depth.Delayed)
	}
	if depth.Dead != 1 {
		t.Errorf("dead = %d, want 1", depth.Dead)
	}
}

func TestRedisQueueDepthInspector_RedisError(t *testing.T) {
	client := &fakeDepthRedisClient{
		err: errors.New("connection refused"),
	}
	inspector := &RedisQueueDepthInspector{
		client:     client,
		queue:      "tasks",
		processing: "tasks:processing",
		delayed:    "tasks:delayed",
		dead:       "tasks:dead",
	}

	depth := inspector.Inspect(context.Background())

	if depth.Status != "unavailable" {
		t.Fatalf("status = %q, want unavailable", depth.Status)
	}
	if depth.Reason != "queue_unavailable" {
		t.Fatalf("reason = %q, want queue_unavailable", depth.Reason)
	}
	if depth.Pending != 0 || depth.Processing != 0 || depth.Delayed != 0 || depth.Dead != 0 {
		t.Errorf("non-zero counts on unavailable: %+v", depth)
	}
}

func TestRedisQueueDepthInspector_NilClient(t *testing.T) {
	inspector := &RedisQueueDepthInspector{
		client: nil,
		queue:  "tasks",
	}
	depth := inspector.Inspect(context.Background())
	if depth.Status != "unavailable" {
		t.Fatalf("status = %q, want unavailable", depth.Status)
	}
	if depth.Reason != "queue_unavailable" {
		t.Fatalf("reason = %q, want queue_unavailable", depth.Reason)
	}
}

func TestRedisQueueDepthInspector_NilInspector(t *testing.T) {
	var inspector *RedisQueueDepthInspector
	depth := inspector.Inspect(context.Background())
	if depth.Status != "unavailable" {
		t.Fatalf("status = %q, want unavailable", depth.Status)
	}
	if depth.Reason != "queue_unavailable" {
		t.Fatalf("reason = %q, want queue_unavailable", depth.Reason)
	}
}

func TestRedisQueueDepthInspector_EmptyQueueName(t *testing.T) {
	inspector := &RedisQueueDepthInspector{
		client: &fakeDepthRedisClient{},
		queue:  "",
	}
	depth := inspector.Inspect(context.Background())
	if depth.Status != "unavailable" {
		t.Fatalf("status = %q, want unavailable", depth.Status)
	}
	if depth.Reason != "queue_unavailable" {
		t.Fatalf("reason = %q, want queue_unavailable", depth.Reason)
	}
}

func TestRedisQueueDepthInspector_NilContext(t *testing.T) {
	client := &fakeDepthRedisClient{
		llenResults: map[string]int64{
			"tasks":            1,
			"tasks:processing": 0,
			"tasks:dead":       0,
		},
		zcardResult: 0,
	}
	inspector := &RedisQueueDepthInspector{
		client:     client,
		queue:      "tasks",
		processing: "tasks:processing",
		delayed:    "tasks:delayed",
		dead:       "tasks:dead",
	}

	//nolint:staticcheck // intentionally passing nil context
	depth := inspector.Inspect(nil)
	if depth.Status != "available" {
		t.Fatalf("status = %q, want available", depth.Status)
	}
}

func TestNilQueueDepthInspector(t *testing.T) {
	inspector := NilQueueDepthInspector{}
	depth := inspector.Inspect(context.Background())
	if depth.Status != "unavailable" {
		t.Fatalf("status = %q, want unavailable", depth.Status)
	}
	if depth.Reason != "queue_unavailable" {
		t.Fatalf("reason = %q, want queue_unavailable", depth.Reason)
	}
}

func TestRedisQueueDepthInspector_PartialRedisFailure_Processing(t *testing.T) {
	callCount := 0
	client := &partialFailDepthRedisClient{
		failOnLLenKey: "tasks:processing",
	}
	_ = callCount
	inspector := &RedisQueueDepthInspector{
		client:     client,
		queue:      "tasks",
		processing: "tasks:processing",
		delayed:    "tasks:delayed",
		dead:       "tasks:dead",
	}

	depth := inspector.Inspect(context.Background())
	if depth.Status != "unavailable" {
		t.Fatalf("status = %q, want unavailable on partial failure", depth.Status)
	}
}

func TestRedisQueueDepthInspector_PartialRedisFailure_Dead(t *testing.T) {
	client := &partialFailDepthRedisClient{
		failOnLLenKey: "tasks:dead",
	}
	inspector := &RedisQueueDepthInspector{
		client:     client,
		queue:      "tasks",
		processing: "tasks:processing",
		delayed:    "tasks:delayed",
		dead:       "tasks:dead",
	}

	depth := inspector.Inspect(context.Background())
	if depth.Status != "unavailable" {
		t.Fatalf("status = %q, want unavailable on dead list failure", depth.Status)
	}
}

// partialFailDepthRedisClient fails on a specific LLen key.
type partialFailDepthRedisClient struct {
	failOnLLenKey string
}

func (f *partialFailDepthRedisClient) LLen(_ context.Context, key string) *redis.IntCmd {
	cmd := redis.NewIntCmd(context.Background())
	if key == f.failOnLLenKey {
		cmd.SetErr(errors.New("partial fail"))
		return cmd
	}
	cmd.SetVal(1)
	return cmd
}

func (f *partialFailDepthRedisClient) ZCard(_ context.Context, _ string) *redis.IntCmd {
	cmd := redis.NewIntCmd(context.Background())
	cmd.SetVal(0)
	return cmd
}
