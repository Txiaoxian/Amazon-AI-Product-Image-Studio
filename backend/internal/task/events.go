package task

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/idgen"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
)

func writeTaskEvent(ctx context.Context, repo Repository, scope tenant.Scope, record database.GenerationTask, eventType string, payload map[string]any, createdAt time.Time) error {
	payload = safeEventPayload(record, payload)
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return repo.CreateEvent(ctx, scope, &database.TaskEvent{
		TenantID:         scope.ID(),
		TaskID:           record.ID,
		ProjectID:        record.ProjectID,
		EventType:        eventType,
		EventPayloadJSON: string(encoded),
		CreatedAt:        createdAt.UTC(),
	})
}

func safeEventPayload(record database.GenerationTask, extra map[string]any) map[string]any {
	payload := map[string]any{
		"taskId":    record.ID,
		"projectId": record.ProjectID,
		"status":    record.Status,
		"attempt":   record.Attempt,
	}
	for key, value := range extra {
		key = strings.TrimSpace(key)
		if key == "" || eventSensitiveKey(key) {
			continue
		}
		payload[key] = value
	}
	return payload
}

func eventSensitiveKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, marker := range []string{"password", "token", "cookie", "authorization", "api_key", "apikey", "secret", "base64", "providerresponse"} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}

func EventIDFromSequence(sequence uint64) string {
	return fmt.Sprintf("evt_%020d", sequence)
}

func pendingTaskEventID() string {
	return "evt_pending_" + idgen.New()
}
