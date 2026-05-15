package sse

import (
	"strings"
	"testing"
)

func TestWriteFrameIncludesIDEventAndData(t *testing.T) {
	var out strings.Builder
	err := WriteFrame(&out, Frame{
		ID:    "evt_00000000000000000001",
		Event: "TASK_QUEUED",
		Data:  `{"taskId":"task-a","projectId":"project-a"}`,
	})
	if err != nil {
		t.Fatalf("write frame: %v", err)
	}
	want := "id: evt_00000000000000000001\nevent: TASK_QUEUED\ndata: {\"taskId\":\"task-a\",\"projectId\":\"project-a\"}\n\n"
	if out.String() != want {
		t.Fatalf("frame = %q, want %q", out.String(), want)
	}
}

func TestWriteHeartbeatDoesNotLeakTaskMetadata(t *testing.T) {
	var out strings.Builder
	if err := WriteHeartbeat(&out); err != nil {
		t.Fatalf("write heartbeat: %v", err)
	}
	frame := out.String()
	if !strings.Contains(frame, "event: HEARTBEAT\n") || !strings.Contains(frame, "data: {}\n") {
		t.Fatalf("heartbeat frame = %q, want HEARTBEAT with empty JSON data", frame)
	}
	for _, forbidden := range []string{"taskId", "projectId", "tenantId", "prompt"} {
		if strings.Contains(frame, forbidden) {
			t.Fatalf("heartbeat leaked %s: %q", forbidden, frame)
		}
	}
}
