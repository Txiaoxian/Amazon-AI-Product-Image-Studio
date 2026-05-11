package audit

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/idgen"
	"gorm.io/gorm"
)

type Recorder struct {
	db *gorm.DB
}

type Event struct {
	TenantID     string
	ActorUserID  *string
	Action       string
	ResourceType string
	ResourceID   string
	IP           string
	UserAgent    string
	Metadata     map[string]any
}

func NewRecorder(db *gorm.DB) Recorder {
	return Recorder{db: db}
}

func (r Recorder) Record(ctx context.Context, event Event) error {
	if r.db == nil {
		return database.ErrNilDB
	}
	if ctx == nil {
		ctx = context.Background()
	}

	metadata, err := json.Marshal(sanitizeMetadata(event.Metadata))
	if err != nil {
		return err
	}

	return r.db.WithContext(ctx).Create(&database.OperationLog{
		ID:           idgen.New(),
		TenantID:     strings.TrimSpace(event.TenantID),
		ActorUserID:  event.ActorUserID,
		Action:       strings.TrimSpace(event.Action),
		ResourceType: strings.TrimSpace(event.ResourceType),
		ResourceID:   strings.TrimSpace(event.ResourceID),
		IP:           truncate(strings.TrimSpace(event.IP), 45),
		UserAgent:    truncate(strings.TrimSpace(event.UserAgent), 512),
		MetadataJSON: string(metadata),
		CreatedAt:    time.Now().UTC(),
	}).Error
}

func sanitizeMetadata(metadata map[string]any) map[string]any {
	clean := make(map[string]any, len(metadata))
	for key, value := range metadata {
		if sensitiveKey(key) {
			continue
		}
		clean[key] = value
	}
	return clean
}

func sensitiveKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, marker := range []string{"password", "token", "cookie", "authorization", "api_key", "apikey", "secret", "jwt"} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
