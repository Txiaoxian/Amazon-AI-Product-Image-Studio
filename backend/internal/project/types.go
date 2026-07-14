package project

import (
	"errors"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
)

const (
	StatusActive   = "ACTIVE"
	StatusArchived = "ARCHIVED"

	RoleOwner  = "OWNER"
	RoleEditor = "EDITOR"
	RoleViewer = "VIEWER"

	PermissionRead         = "project:read"
	PermissionCreate       = "project:create"
	PermissionUpdate       = "project:update"
	PermissionDelete       = "project:delete"
	PermissionMemberManage = "project:member:manage"
)

var (
	ErrValidation = errors.New("invalid project request")
	ErrForbidden  = errors.New("project access forbidden")
	ErrNotFound   = errors.New("project not found")
	ErrConflict   = errors.New("project conflict")
)

type ListQuery struct {
	PageNum  int
	PageSize int
	Status   string
}

type CandidateListQuery struct {
	PageNum  int
	PageSize int
	Q        string
}

type Page struct {
	Records  []ProjectResponse `json:"records"`
	Total    int64             `json:"total"`
	PageNum  int               `json:"pageNum"`
	PageSize int               `json:"pageSize"`
}

type ProjectResponse struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenantId"`
	Name      string `json:"name"`
	Brand     string `json:"brand"`
	ASIN      string `json:"asin"`
	Site      string `json:"site"`
	Notes     string `json:"notes"`
	Status    string `json:"status"`
	SortOrder int    `json:"sortOrder"`
	CreatedBy string `json:"createdBy"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type MemberResponse struct {
	ID         string `json:"id"`
	TenantID   string `json:"tenantId"`
	ProjectID  string `json:"projectId"`
	UserID     string `json:"userId"`
	UserEmail  string `json:"userEmail"`
	UserName   string `json:"userName"`
	UserStatus string `json:"userStatus"`
	Role       string `json:"role"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

type MemberCandidateResponse struct {
	UserID    string `json:"userId"`
	UserEmail string `json:"userEmail"`
	UserName  string `json:"userName"`
	Status    string `json:"status"`
}

type CreateInput struct {
	Name      string
	Brand     string
	ASIN      string
	Site      string
	Notes     string
	Status    string
	SortOrder *int
}

type UpdateInput struct {
	Name      *string
	Brand     *string
	ASIN      *string
	Site      *string
	Notes     *string
	Status    *string
	SortOrder *int
}

type MemberInput struct {
	UserID string
	Role   string
}

func projectResponse(record database.Project) ProjectResponse {
	return ProjectResponse{
		ID:        record.ID,
		TenantID:  record.TenantID,
		Name:      record.Name,
		Brand:     record.Brand,
		ASIN:      record.ASIN,
		Site:      record.Site,
		Notes:     record.Notes,
		Status:    record.Status,
		SortOrder: record.SortOrder,
		CreatedBy: record.CreatedBy,
		CreatedAt: formatTime(record.CreatedAt),
		UpdatedAt: formatTime(record.UpdatedAt),
	}
}

type MemberRecord struct {
	Member     database.ProjectMember
	UserEmail  string
	UserName   string
	UserStatus string
}

func memberResponse(record MemberRecord) MemberResponse {
	return MemberResponse{
		ID:         record.Member.ID,
		TenantID:   record.Member.TenantID,
		ProjectID:  record.Member.ProjectID,
		UserID:     record.Member.UserID,
		UserEmail:  record.UserEmail,
		UserName:   record.UserName,
		UserStatus: record.UserStatus,
		Role:       record.Member.Role,
		CreatedAt:  formatTime(record.Member.CreatedAt),
		UpdatedAt:  formatTime(record.Member.UpdatedAt),
	}
}

func memberResponseFromRecord(record database.ProjectMember, user database.User) MemberResponse {
	return memberResponse(MemberRecord{
		Member:     record,
		UserEmail:  user.Email,
		UserName:   user.DisplayName,
		UserStatus: user.Status,
	})
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func validStatus(status string) bool {
	switch status {
	case StatusActive, StatusArchived:
		return true
	default:
		return false
	}
}

func validRole(role string) bool {
	switch role {
	case RoleOwner, RoleEditor, RoleViewer:
		return true
	default:
		return false
	}
}

func normalizeStatus(status string, defaultStatus string) (string, error) {
	status = strings.ToUpper(strings.TrimSpace(status))
	if status == "" {
		status = defaultStatus
	}
	if !validStatus(status) {
		return "", ErrValidation
	}
	return status, nil
}

func normalizeRole(role string) (string, error) {
	role = strings.ToUpper(strings.TrimSpace(role))
	if !validRole(role) {
		return "", ErrValidation
	}
	return role, nil
}
