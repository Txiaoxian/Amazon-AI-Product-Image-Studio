package task

import (
	"context"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestWriteTaskEventUsesSequenceForStableReplayOrderWithSameTimestamp(t *testing.T) {
	db := newTaskEventTestDB(t)
	repo := NewRepository(db)
	scope, err := tenant.NewScope("tenant-a")
	if err != nil {
		t.Fatalf("create tenant scope: %v", err)
	}
	createdAt := time.Date(2026, 5, 13, 8, 0, 0, 123000000, time.UTC)
	record := database.GenerationTask{
		ID:                "task-a",
		TenantID:          scope.ID(),
		ProjectID:         "project-a",
		Type:              TypeImageGeneration,
		ProviderID:        "provider-a",
		ModelID:           "model-a",
		Status:            StatusQueued,
		Prompt:            "prompt",
		ParamsJSON:        `{}`,
		InputAssetIDsJSON: `[]`,
		Attempt:           1,
		MaxAttempts:       3,
		CreatedBy:         "user-a",
		CreatedAt:         createdAt,
		UpdatedAt:         createdAt,
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	for _, eventType := range []string{EventTaskRetried, EventTaskQueued, EventTaskFailed} {
		if err := writeTaskEvent(context.Background(), repo, scope, record, eventType, nil, createdAt); err != nil {
			t.Fatalf("write event %s: %v", eventType, err)
		}
	}

	var events []database.TaskEvent
	if err := db.Where("tenant_id = ? AND task_id = ?", scope.ID(), record.ID).Order("sequence ASC").Find(&events).Error; err != nil {
		t.Fatalf("load events by sequence: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("event count = %d, want 3", len(events))
	}
	expected := []string{EventTaskRetried, EventTaskQueued, EventTaskFailed}
	for index, event := range events {
		if event.EventType != expected[index] {
			t.Fatalf("event %d type = %q, want %q; events = %#v", index, event.EventType, expected[index], events)
		}
		if event.Sequence == 0 {
			t.Fatalf("event %d missing sequence: %#v", index, event)
		}
		if event.ID != EventIDFromSequence(event.Sequence) {
			t.Fatalf("event %d id = %q, want %q", index, event.ID, EventIDFromSequence(event.Sequence))
		}
		if index > 0 && event.Sequence <= events[index-1].Sequence {
			t.Fatalf("sequence not monotonic: previous=%d current=%d", events[index-1].Sequence, event.Sequence)
		}
		if !event.CreatedAt.Equal(createdAt) {
			t.Fatalf("event %d created_at = %s, want same timestamp %s", index, event.CreatedAt, createdAt)
		}
	}
}

func newTaskEventTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Discard,
	})
	if err != nil {
		t.Fatalf("open sqlite test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access sqlite test database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&database.GenerationTask{}); err != nil {
		t.Fatalf("migrate task event test schema: %v", err)
	}
	if err := db.Exec(`CREATE TABLE task_events (
		sequence INTEGER PRIMARY KEY AUTOINCREMENT,
		id TEXT NOT NULL UNIQUE,
		tenant_id TEXT NOT NULL,
		task_id TEXT NOT NULL,
		project_id TEXT NOT NULL,
		event_type TEXT NOT NULL,
		event_payload_json TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL
	)`).Error; err != nil {
		t.Fatalf("migrate task event test schema: %v", err)
	}
	return db
}
