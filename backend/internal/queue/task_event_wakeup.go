package queue

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/redis/go-redis/v9"
)

type TaskEventSink interface {
	PublishTaskEvent(ctx context.Context, event database.TaskEvent)
}

type taskEventWakeup struct {
	Sequence uint64 `json:"sequence"`
}

type RedisTaskEventPublisher struct {
	client  redis.Cmdable
	channel string
}

func NewRedisTaskEventPublisher(cfg config.QueueConfig) *RedisTaskEventPublisher {
	return NewRedisTaskEventPublisherWithClient(NewRedisClient(cfg), TaskEventChannel(cfg.TaskQueueName))
}

func NewRedisTaskEventPublisherWithClient(client redis.Cmdable, channel string) *RedisTaskEventPublisher {
	return &RedisTaskEventPublisher{client: client, channel: strings.TrimSpace(channel)}
}

func (p *RedisTaskEventPublisher) PublishTaskEvent(ctx context.Context, event database.TaskEvent) {
	if p == nil || p.client == nil || strings.TrimSpace(p.channel) == "" || event.Sequence == 0 {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	payload, ok := taskEventWakeupPayload(event)
	if !ok {
		return
	}
	_ = p.client.Publish(ctx, p.channel, payload).Err()
}

type RedisTaskEventSubscriber struct {
	client  *redis.Client
	channel string
}

func NewRedisTaskEventSubscriber(cfg config.QueueConfig) *RedisTaskEventSubscriber {
	return &RedisTaskEventSubscriber{client: NewRedisClient(cfg), channel: TaskEventChannel(cfg.TaskQueueName)}
}

func TaskEventChannel(queueName string) string {
	queueName = strings.TrimSpace(queueName)
	if queueName == "" {
		queueName = "image-tasks"
	}
	return queueName + ":task-events"
}

func taskEventWakeupPayload(event database.TaskEvent) (string, bool) {
	if event.Sequence == 0 {
		return "", false
	}
	payload, err := json.Marshal(taskEventWakeup{Sequence: event.Sequence})
	if err != nil {
		return "", false
	}
	return string(payload), true
}

func (s *RedisTaskEventSubscriber) Run(ctx context.Context, sink TaskEventSink, log *slog.Logger) error {
	if s == nil || s.client == nil || strings.TrimSpace(s.channel) == "" || sink == nil {
		return ErrUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	pubsub := s.client.Subscribe(ctx, s.channel)
	defer func() {
		_ = pubsub.Close()
	}()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case message, ok := <-ch:
			if !ok {
				return nil
			}
			var wakeup taskEventWakeup
			if err := json.Unmarshal([]byte(message.Payload), &wakeup); err != nil {
				if log != nil {
					log.Warn("ignored malformed task event wakeup")
				}
				continue
			}
			if wakeup.Sequence == 0 {
				continue
			}
			sink.PublishTaskEvent(ctx, database.TaskEvent{Sequence: wakeup.Sequence})
		}
	}
}

func StartTaskEventSubscriber(ctx context.Context, subscriber *RedisTaskEventSubscriber, sink TaskEventSink, log *slog.Logger) {
	if subscriber == nil || sink == nil {
		return
	}
	go func() {
		if err := subscriber.Run(ctx, sink, log); err != nil && ctx.Err() == nil && log != nil {
			log.Warn("task event wakeup subscriber stopped", slog.String("error", err.Error()))
		}
	}()
}
