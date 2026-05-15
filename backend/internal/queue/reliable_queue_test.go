package queue

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/redis/go-redis/v9"
)

func TestRedisReliableTaskQueueClaimAndAck(t *testing.T) {
	ctx := context.Background()
	store := newReliableQueueFakeRedis()
	q := newTestReliableQueue(store)

	if err := q.EnqueueTask(ctx, "task-a"); err != nil {
		t.Fatalf("EnqueueTask returned error: %v", err)
	}
	claim, err := q.Claim(ctx)
	if err != nil {
		t.Fatalf("Claim returned error: %v", err)
	}
	if claim.TaskID != "task-a" || claim.DeliveryCount != 1 {
		t.Fatalf("claim = %#v, want task-a delivery 1", claim)
	}
	if store.evalCalls != 1 {
		t.Fatalf("claim metadata eval calls = %d, want 1", store.evalCalls)
	}
	assertFakeList(t, store, q.queue, nil)
	assertFakeList(t, store, q.processing, []string{"task-a"})
	if store.hashes[q.processingClaims]["task-a"] == "" {
		t.Fatalf("processing claim metadata missing: %#v", store.hashes[q.processingClaims])
	}

	if err := q.Ack(ctx, claim); err != nil {
		t.Fatalf("Ack returned error: %v", err)
	}
	assertFakeList(t, store, q.processing, nil)
	if _, ok := store.hashes[q.processingClaims]["task-a"]; ok {
		t.Fatalf("processing claim metadata remained after ack: %#v", store.hashes[q.processingClaims])
	}
	if _, ok := store.hashes[q.deliveries]["task-a"]; ok {
		t.Fatalf("delivery count remained after ack: %#v", store.hashes[q.deliveries])
	}
}

func TestRedisReliableTaskQueueRetryDelayAndPromoteDue(t *testing.T) {
	ctx := context.Background()
	store := newReliableQueueFakeRedis()
	q := newTestReliableQueue(store)

	if err := q.EnqueueTask(ctx, "task-retry"); err != nil {
		t.Fatalf("EnqueueTask returned error: %v", err)
	}
	claim, err := q.Claim(ctx)
	if err != nil {
		t.Fatalf("Claim returned error: %v", err)
	}
	if err := q.Retry(ctx, claim, time.Hour); err != nil {
		t.Fatalf("Retry returned error: %v", err)
	}
	assertFakeList(t, store, q.processing, nil)
	assertFakeList(t, store, q.queue, nil)

	early, err := q.PromoteDue(ctx, time.Now().Add(30*time.Minute), 10)
	if err != nil {
		t.Fatalf("early PromoteDue returned error: %v", err)
	}
	if len(early) != 0 {
		t.Fatalf("early promoted = %#v, want none", early)
	}
	assertFakeList(t, store, q.queue, nil)

	promoted, err := q.PromoteDue(ctx, time.Now().Add(2*time.Hour), 10)
	if err != nil {
		t.Fatalf("due PromoteDue returned error: %v", err)
	}
	if !reflect.DeepEqual(promoted, []string{"task-retry"}) {
		t.Fatalf("promoted = %#v, want task-retry", promoted)
	}
	assertFakeList(t, store, q.queue, []string{"task-retry"})
}

func TestRedisReliableTaskQueueMaxDeliveriesDeadLetters(t *testing.T) {
	ctx := context.Background()
	store := newReliableQueueFakeRedis()
	q := newTestReliableQueue(store)
	q.maxDeliveries = 1

	if err := q.EnqueueTask(ctx, "task-dead"); err != nil {
		t.Fatalf("EnqueueTask returned error: %v", err)
	}
	claim, err := q.Claim(ctx)
	if err != nil {
		t.Fatalf("first Claim returned error: %v", err)
	}
	if err := q.Retry(ctx, claim, 0); err != nil {
		t.Fatalf("Retry returned error: %v", err)
	}
	deadClaim, err := q.Claim(ctx)
	if !errors.Is(err, ErrDeadLettered) {
		t.Fatalf("second Claim error = %v, want ErrDeadLettered", err)
	}
	if deadClaim.TaskID != "task-dead" || deadClaim.DeliveryCount != 2 || !deadClaim.DeadLettered {
		t.Fatalf("dead claim = %#v, want task-dead delivery 2 dead-lettered", deadClaim)
	}
	assertFakeList(t, store, q.queue, nil)
	assertFakeList(t, store, q.processing, nil)
	assertFakeList(t, store, q.dead, []string{"task-dead"})
	if store.hashes[q.dead+":reasons"]["task-dead"] != "max_deliveries_exceeded" {
		t.Fatalf("dead letter reason = %#v", store.hashes[q.dead+":reasons"])
	}
}

func TestRedisReliableTaskQueueRecoverStaleClaim(t *testing.T) {
	ctx := context.Background()
	store := newReliableQueueFakeRedis()
	q := newTestReliableQueue(store)
	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)

	if err := q.EnqueueTask(ctx, "task-stale"); err != nil {
		t.Fatalf("EnqueueTask returned error: %v", err)
	}
	if _, err := q.Claim(ctx); err != nil {
		t.Fatalf("Claim returned error: %v", err)
	}
	store.hashes[q.processingClaims]["task-stale"] = strconv.FormatInt(now.Add(-2*q.visibilityTimeout).UnixMilli(), 10)

	recovered, err := q.RecoverStale(ctx, now, 10)
	if err != nil {
		t.Fatalf("RecoverStale returned error: %v", err)
	}
	if !reflect.DeepEqual(recovered, []string{"task-stale"}) {
		t.Fatalf("recovered = %#v, want task-stale", recovered)
	}
	assertFakeList(t, store, q.processing, nil)
	assertFakeList(t, store, q.queue, []string{"task-stale"})
	if _, ok := store.hashes[q.processingClaims]["task-stale"]; ok {
		t.Fatalf("stale claim metadata remained: %#v", store.hashes[q.processingClaims])
	}
}

func TestRedisReliableTaskQueueRecoverMalformedClaim(t *testing.T) {
	ctx := context.Background()
	store := newReliableQueueFakeRedis()
	q := newTestReliableQueue(store)

	store.lists[q.processing] = []string{"task-malformed"}
	store.hashes[q.processingClaims] = map[string]string{"task-malformed": "not-a-timestamp"}
	recovered, err := q.RecoverStale(ctx, time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("RecoverStale returned error: %v", err)
	}
	if !reflect.DeepEqual(recovered, []string{"task-malformed"}) {
		t.Fatalf("recovered = %#v, want task-malformed", recovered)
	}
	assertFakeList(t, store, q.processing, nil)
	assertFakeList(t, store, q.queue, []string{"task-malformed"})
}

func TestRedisReliableTaskQueueRecoverOrphanProcessingEntry(t *testing.T) {
	ctx := context.Background()
	store := newReliableQueueFakeRedis()
	q := newTestReliableQueue(store)

	store.lists[q.processing] = []string{"task-orphan"}
	recovered, err := q.RecoverStale(ctx, time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("RecoverStale returned error: %v", err)
	}
	if !reflect.DeepEqual(recovered, []string{"task-orphan"}) {
		t.Fatalf("recovered = %#v, want task-orphan", recovered)
	}
	assertFakeList(t, store, q.processing, nil)
	assertFakeList(t, store, q.queue, []string{"task-orphan"})
	if _, ok := store.hashes[q.processingClaims]["task-orphan"]; ok {
		t.Fatalf("orphan claim metadata unexpectedly remained: %#v", store.hashes[q.processingClaims])
	}
}

func newTestReliableQueue(store *reliableQueueFakeRedis) *RedisReliableTaskQueue {
	return newRedisReliableTaskQueueWithClient(store, config.QueueConfig{
		TaskQueueName:     "test-tasks",
		ClaimTimeout:      time.Millisecond,
		VisibilityTimeout: time.Minute,
		MaxDeliveries:     5,
	})
}

func assertFakeList(t *testing.T, store *reliableQueueFakeRedis, key string, expected []string) {
	t.Helper()
	actual := append([]string(nil), store.lists[key]...)
	if expected == nil {
		expected = []string{}
	}
	if actual == nil {
		actual = []string{}
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("list %s = %#v, want %#v", key, actual, expected)
	}
}

type reliableQueueFakeRedis struct {
	lists     map[string][]string
	hashes    map[string]map[string]string
	zsets     map[string]map[string]float64
	evalCalls int
}

func newReliableQueueFakeRedis() *reliableQueueFakeRedis {
	return &reliableQueueFakeRedis{
		lists:  map[string][]string{},
		hashes: map[string]map[string]string{},
		zsets:  map[string]map[string]float64{},
	}
}

func (r *reliableQueueFakeRedis) RPush(ctx context.Context, key string, values ...interface{}) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	for _, value := range values {
		r.lists[key] = append(r.lists[key], strings.TrimSpace(value.(string)))
	}
	cmd.SetVal(int64(len(r.lists[key])))
	return cmd
}

func (r *reliableQueueFakeRedis) BLMove(ctx context.Context, source, destination, srcpos, destpos string, _ time.Duration) *redis.StringCmd {
	cmd := redis.NewStringCmd(ctx)
	if len(r.lists[source]) == 0 {
		cmd.SetErr(redis.Nil)
		return cmd
	}
	value := r.lists[source][0]
	r.lists[source] = r.lists[source][1:]
	r.lists[destination] = append(r.lists[destination], value)
	cmd.SetVal(value)
	return cmd
}

func (r *reliableQueueFakeRedis) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	cmd := redis.NewCmd(ctx)
	if strings.TrimSpace(script) != strings.TrimSpace(redisRecordClaimScript) {
		cmd.SetErr(errors.New("unexpected script"))
		return cmd
	}
	r.evalCalls++
	taskID := strings.TrimSpace(args[0].(string))
	claimedAt := strings.TrimSpace(args[1].(string))
	if r.hashes[keys[0]] == nil {
		r.hashes[keys[0]] = map[string]string{}
	}
	current, _ := strconv.ParseInt(r.hashes[keys[0]][taskID], 10, 64)
	current++
	r.hashes[keys[0]][taskID] = strconv.FormatInt(current, 10)
	if r.hashes[keys[1]] == nil {
		r.hashes[keys[1]] = map[string]string{}
	}
	r.hashes[keys[1]][taskID] = claimedAt
	cmd.SetVal(current)
	return cmd
}

func (r *reliableQueueFakeRedis) LRem(ctx context.Context, key string, count int64, value interface{}) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	target := strings.TrimSpace(value.(string))
	removed := int64(0)
	next := make([]string, 0, len(r.lists[key]))
	for _, item := range r.lists[key] {
		if item == target && (count == 0 || removed < count) {
			removed++
			continue
		}
		next = append(next, item)
	}
	r.lists[key] = next
	cmd.SetVal(removed)
	return cmd
}

func (r *reliableQueueFakeRedis) HDel(ctx context.Context, key string, fields ...string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	var removed int64
	for _, field := range fields {
		if _, ok := r.hashes[key][field]; ok {
			delete(r.hashes[key], field)
			removed++
		}
	}
	cmd.SetVal(removed)
	return cmd
}

func (r *reliableQueueFakeRedis) HSet(ctx context.Context, key string, values ...interface{}) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	if r.hashes[key] == nil {
		r.hashes[key] = map[string]string{}
	}
	var added int64
	for index := 0; index+1 < len(values); index += 2 {
		field := strings.TrimSpace(values[index].(string))
		if _, exists := r.hashes[key][field]; !exists {
			added++
		}
		r.hashes[key][field] = strings.TrimSpace(values[index+1].(string))
	}
	cmd.SetVal(added)
	return cmd
}

func (r *reliableQueueFakeRedis) HGet(ctx context.Context, key, field string) *redis.StringCmd {
	cmd := redis.NewStringCmd(ctx)
	value, ok := r.hashes[key][field]
	if !ok {
		cmd.SetErr(redis.Nil)
		return cmd
	}
	cmd.SetVal(value)
	return cmd
}

func (r *reliableQueueFakeRedis) LRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd {
	cmd := redis.NewStringSliceCmd(ctx)
	values := append([]string(nil), r.lists[key]...)
	if start == 0 && stop == -1 {
		cmd.SetVal(values)
		return cmd
	}
	if start < 0 {
		start = int64(len(values)) + start
	}
	if stop < 0 {
		stop = int64(len(values)) + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= int64(len(values)) {
		stop = int64(len(values)) - 1
	}
	if start > stop || start >= int64(len(values)) {
		cmd.SetVal([]string{})
		return cmd
	}
	cmd.SetVal(values[start : stop+1])
	return cmd
}

func (r *reliableQueueFakeRedis) ZAdd(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	if r.zsets[key] == nil {
		r.zsets[key] = map[string]float64{}
	}
	var added int64
	for _, member := range members {
		name := strings.TrimSpace(member.Member.(string))
		if _, ok := r.zsets[key][name]; !ok {
			added++
		}
		r.zsets[key][name] = member.Score
	}
	cmd.SetVal(added)
	return cmd
}

func (r *reliableQueueFakeRedis) ZRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.StringSliceCmd {
	cmd := redis.NewStringSliceCmd(ctx)
	max, err := strconv.ParseFloat(opt.Max, 64)
	if err != nil {
		cmd.SetErr(err)
		return cmd
	}
	var values []string
	for member, score := range r.zsets[key] {
		if score <= max {
			values = append(values, member)
		}
	}
	sort.Strings(values)
	if opt.Count > 0 && int64(len(values)) > opt.Count {
		values = values[:opt.Count]
	}
	cmd.SetVal(values)
	return cmd
}

func (r *reliableQueueFakeRedis) ZRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	var removed int64
	for _, member := range members {
		name := strings.TrimSpace(member.(string))
		if _, ok := r.zsets[key][name]; ok {
			delete(r.zsets[key], name)
			removed++
		}
	}
	cmd.SetVal(removed)
	return cmd
}

var _ reliableQueueRedisClient = (*reliableQueueFakeRedis)(nil)
