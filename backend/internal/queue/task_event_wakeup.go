package queue

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/redis/go-redis/v9"
)

type TaskEventSink interface {
	PublishTaskEvent(ctx context.Context, event database.TaskEvent)
}

type TaskEventSubscriber interface {
	Run(ctx context.Context, sink TaskEventSink, log *slog.Logger) error
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
			publishTaskEventWakeup(ctx, message.Payload, sink, log)
		}
	}
}

func publishTaskEventWakeup(ctx context.Context, payload string, sink TaskEventSink, log *slog.Logger) {
	if sink == nil {
		return
	}
	var wakeup taskEventWakeup
	if err := json.Unmarshal([]byte(payload), &wakeup); err != nil {
		if log != nil {
			log.Warn("ignored malformed task event wakeup")
		}
		return
	}
	if wakeup.Sequence == 0 {
		return
	}
	sink.PublishTaskEvent(ctx, database.TaskEvent{Sequence: wakeup.Sequence})
}

func StartTaskEventSubscriber(ctx context.Context, subscriber TaskEventSubscriber, sink TaskEventSink, log *slog.Logger) <-chan struct{} {
	done := make(chan struct{})
	if subscriber == nil || sink == nil {
		close(done)
		return done
	}
	if ctx == nil {
		ctx = context.Background()
	}
	go func() {
		defer close(done)
		if err := subscriber.Run(ctx, sink, log); err != nil && ctx.Err() == nil && !errors.Is(err, context.Canceled) && log != nil {
			log.Warn("task event wakeup subscriber stopped", slog.String("error", err.Error()))
		}
	}()
	return done
}
