package asset

import (
	"errors"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/project"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/thumbnail"
)

const (
	KindReference = "REFERENCE"
	KindGenerated = "GENERATED"
	KindEdited    = "EDITED"

	PermissionRead     = "asset:read"
	PermissionUpload   = "asset:upload"
	PermissionUpdate   = "asset:update"
	PermissionDelete   = "asset:delete"
	PermissionDownload = "asset:download"
)

var (
	ErrValidation         = errors.New("invalid asset request")
	ErrForbidden          = errors.New("asset access forbidden")
	ErrNotFound           = errors.New("asset not found")
	ErrStorageUnavailable = errors.New("asset storage unavailable")
	ErrUploadFailed       = errors.New("asset upload failed")
	ErrCleanupFailed      = errors.New("asset cleanup failed")
)

type ListQuery struct {
	PageNum  int
	PageSize int
	Kind     string
	Category string
	Favorite *bool
}

type ListOptions struct {
	PageNum  int
	PageSize int
	Kind     string
	Category string
	Favorite *bool
}

type Page struct {
	Records  []Response `json:"records"`
	Total    int64      `json:"total"`
	PageNum  int        `json:"pageNum"`
	PageSize int        `json:"pageSize"`
}

type Response struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenantId"`
	ProjectID    string `json:"projectId"`
	Kind         string `json:"kind"`
	Category     string `json:"category"`
	Filename     string `json:"filename"`
	MimeType     string `json:"mimeType"`
	FileSize     int64  `json:"fileSize"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	ThumbnailURL string `json:"thumbnailUrl"`
	PreviewURL   string `json:"previewUrl"`
	IsFavorite   bool   `json:"isFavorite"`
	CreatedBy    string `json:"createdBy"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type UpdateInput struct {
	Category   *string
	Filename   *string
	IsFavorite *bool
}

type uploadInput struct {
	Kind       string
	Category   string
	Filename   string
	IsFavorite bool
	Data       []byte
	MimeType   string
	Ext        string
	SizeBytes  int64
	Width      int
	Height     int
	SHA256     string
}

func responseFromRecord(record database.ImageAsset) Response {
	return Response{
		ID:           record.ID,
		TenantID:     record.TenantID,
		ProjectID:    record.ProjectID,
		Kind:         record.Kind,
		Category:     record.Category,
		Filename:     record.Filename,
		MimeType:     record.MimeType,
		FileSize:     record.SizeBytes,
		Width:        record.Width,
		Height:       record.Height,
		ThumbnailURL: thumbnailURL(record),
		PreviewURL:   "/api/v1/assets/" + record.ID + "/download",
		IsFavorite:   record.IsFavorite,
		CreatedBy:    record.CreatedBy,
		CreatedAt:    formatTime(record.CreatedAt),
		UpdatedAt:    formatTime(record.UpdatedAt),
	}
}

func thumbnailURL(record database.ImageAsset) string {
	if record.ThumbnailObjectKey == nil || strings.TrimSpace(*record.ThumbnailObjectKey) == "" {
		return ""
	}
	return thumbnail.URL(record.ID)
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func validKind(kind string) bool {
	switch kind {
	case KindReference, KindGenerated, KindEdited:
		return true
	default:
		return false
	}
}

func rolesForPermission(permission string) []string {
	switch permission {
	case PermissionDelete:
		return []string{project.RoleOwner}
	case PermissionUpload, PermissionUpdate:
		return []string{project.RoleOwner, project.RoleEditor}
	default:
		return []string{project.RoleOwner, project.RoleEditor, project.RoleViewer}
	}
}
