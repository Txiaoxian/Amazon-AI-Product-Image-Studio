package queue

import (
	"strings"
	"testing"

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
