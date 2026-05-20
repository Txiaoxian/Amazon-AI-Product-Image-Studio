package queue

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
)

func TestTaskEventWakeupPayloadContainsOnlySequence(t *testing.T) {
	payload, ok := taskEventWakeupPayload(database.TaskEvent{
		Sequence:         42,
		ID:               "evt_00000000000000000042",
		TenantID:         "tenant-a",
		TaskID:           "task-a",
		ProjectID:        "project-a",
		EventType:        "TASK_PROGRESS",
		EventPayloadJSON: `{"taskId":"task-a","Authorization":"Bearer secret","base64":"hidden"}`,
	})
	if !ok {
		t.Fatal("taskEventWakeupPayload returned false")
	}
	if payload != `{"sequence":42}` {
		t.Fatalf("payload = %s, want sequence-only JSON", payload)
	}
	for _, forbidden := range []string{"task-a", "project-a", "tenant-a", "Authorization", "secret", "base64", "TASK_PROGRESS"} {
		if strings.Contains(payload, forbidden) {
			t.Fatalf("wakeup payload leaked %q: %s", forbidden, payload)
		}
	}
}

func TestTaskEventChannelUsesQueueName(t *testing.T) {
	if got := TaskEventChannel("image-tasks"); got != "image-tasks:task-events" {
		t.Fatalf("TaskEventChannel = %q, want image-tasks:task-events", got)
	}
	if got := TaskEventChannel(" "); got != "image-tasks:task-events" {
		t.Fatalf("default TaskEventChannel = %q, want image-tasks:task-events", got)
	}
}

func TestTaskEventWakeupIgnoresMalformedAndZeroSequence(t *testing.T) {
	sink := newRecordingTaskEventSink()
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	publishTaskEventWakeup(context.Background(), "not-json", sink, logger)
	publishTaskEventWakeup(context.Background(), `{"sequence":0}`, sink, logger)
	assertNoTaskEvent(t, sink.events)

	publishTaskEventWakeup(context.Background(), `{"sequence":7}`, sink, logger)
	event := receiveTaskEvent(t, sink.events)
	if event.Sequence != 7 {
		t.Fatalf("published sequence = %d, want 7", event.Sequence)
	}
	if strings.Contains(logs.String(), "not-json") {
		t.Fatalf("malformed wakeup log leaked raw payload: %s", logs.String())
	}
}

func TestStartTaskEventSubscriberStopsOnContextCancellationWithoutUnexpectedLog(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	subscriber := &blockingTaskEventSubscriber{started: make(chan struct{})}
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	done := StartTaskEventSubscriber(ctx, subscriber, newRecordingTaskEventSink(), logger)
	waitForTaskEventSubscriberStart(t, subscriber.started)
	cancel()
	waitForTaskEventSubscriberDone(t, done)

	if strings.Contains(logs.String(), "task event wakeup subscriber stopped") || strings.Contains(logs.String(), context.Canceled.Error()) {
		t.Fatalf("context cancellation was logged as unexpected failure: %s", logs.String())
	}
}

func TestStartTaskEventSubscriberLogsUnexpectedError(t *testing.T) {
	expectedErr := errors.New("subscriber boom")
	subscriber := &failingTaskEventSubscriber{err: expectedErr}
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	done := StartTaskEventSubscriber(context.Background(), subscriber, newRecordingTaskEventSink(), logger)
	waitForTaskEventSubscriberDone(t, done)

	if !strings.Contains(logs.String(), "task event wakeup subscriber stopped") || !strings.Contains(logs.String(), expectedErr.Error()) {
		t.Fatalf("unexpected subscriber error was not logged: %s", logs.String())
	}
}

type recordingTaskEventSink struct {
	events chan database.TaskEvent
}

func newRecordingTaskEventSink() *recordingTaskEventSink {
	return &recordingTaskEventSink{events: make(chan database.TaskEvent, 4)}
}

func (s *recordingTaskEventSink) PublishTaskEvent(_ context.Context, event database.TaskEvent) {
	s.events <- event
}

type blockingTaskEventSubscriber struct {
	started chan struct{}
}

func (s *blockingTaskEventSubscriber) Run(ctx context.Context, _ TaskEventSink, _ *slog.Logger) error {
	close(s.started)
	<-ctx.Done()
	return ctx.Err()
}

type failingTaskEventSubscriber struct {
	err error
}

func (s *failingTaskEventSubscriber) Run(context.Context, TaskEventSink, *slog.Logger) error {
	return s.err
}

func receiveTaskEvent(t *testing.T, ch <-chan database.TaskEvent) database.TaskEvent {
	t.Helper()
	select {
	case event := <-ch:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for task event")
		return database.TaskEvent{}
	}
}

func assertNoTaskEvent(t *testing.T, ch <-chan database.TaskEvent) {
	t.Helper()
	select {
	case event := <-ch:
		t.Fatalf("unexpected task event: %#v", event)
	default:
	}
}

func waitForTaskEventSubscriberStart(t *testing.T, started <-chan struct{}) {
	t.Helper()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for task event subscriber to start")
	}
}

func waitForTaskEventSubscriberDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for task event subscriber to stop")
	}
}
