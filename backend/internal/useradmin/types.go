package useradmin

import (
	"errors"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
)

const (
	UserStatusActive   = "ACTIVE"
	UserStatusDisabled = "DISABLED"

	PermissionUserRead    = "user:read"
	PermissionUserCreate  = "user:create"
	PermissionUserUpdate  = "user:update"
	PermissionUserDisable = "user:disable"
	PermissionRoleRead    = "role:read"
	PermissionRoleManage  = "role:manage"

	maxPageSize          = 100
	defaultPageNum       = 1
	defaultPageSize      = 20
	maxDisplayNameRunes  = 255
	maxQueryRunes        = 128
	maxRoleIDsPerRequest = 100
)

var (
	ErrValidation = errors.New("invalid user admin request")
	ErrForbidden  = errors.New("user admin access forbidden")
	ErrNotFound   = errors.New("user admin resource not found")
	ErrConflict   = errors.New("user admin conflict")
)

type ListQuery struct {
	PageNum  int
	PageSize int
	Status   string
	Q        string
}

type ListOptions struct {
	PageNum  int
	PageSize int
	Status   string
	Q        string
}

type Page struct {
	Records  []UserResponse `json:"records"`
	Total    int64          `json:"total"`
	PageNum  int            `json:"pageNum"`
	PageSize int            `json:"pageSize"`
}

type UserResponse struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenantId"`
	Email       string         `json:"email"`
	DisplayName string         `json:"displayName"`
	Status      string         `json:"status"`
	LastLoginAt *string        `json:"lastLoginAt"`
	CreatedAt   string         `json:"createdAt"`
	UpdatedAt   string         `json:"updatedAt"`
	Roles       []RoleResponse `json:"roles"`
}

type RoleResponse struct {
	ID          string               `json:"id"`
	TenantID    string               `json:"tenantId"`
	Code        string               `json:"code"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Status      string               `json:"status"`
	Permissions []PermissionResponse `json:"permissions,omitempty"`
	CreatedAt   string               `json:"createdAt"`
	UpdatedAt   string               `json:"updatedAt"`
}

type PermissionResponse struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type CreateInput struct {
	Email       string
	DisplayName string
	Password    string
	RoleIDs     []string
}

type UpdateInput struct {
	DisplayName *string
	Status      *string
}

type RolesInput struct {
	RoleIDs []string
}

func userResponse(record database.User, roles []database.Role) UserResponse {
	return UserResponse{
		ID:          record.ID,
		TenantID:    record.TenantID,
		Email:       record.Email,
		DisplayName: record.DisplayName,
		Status:      record.Status,
		LastLoginAt: optionalTime(record.LastLoginAt),
		CreatedAt:   formatTime(record.CreatedAt),
		UpdatedAt:   formatTime(record.UpdatedAt),
		Roles:       roleResponses(roles, nil),
	}
}

func roleResponses(records []database.Role, permissionsByRoleID map[string][]database.Permission) []RoleResponse {
	responses := make([]RoleResponse, 0, len(records))
	for _, record := range records {
		permissions := []PermissionResponse(nil)
		if permissionsByRoleID != nil {
			permissions = permissionResponses(permissionsByRoleID[record.ID])
		}
		responses = append(responses, RoleResponse{
			ID:          record.ID,
			TenantID:    record.TenantID,
			Code:        record.Code,
			Name:        record.Name,
			Description: record.Description,
			Status:      record.Status,
			Permissions: permissions,
			CreatedAt:   formatTime(record.CreatedAt),
			UpdatedAt:   formatTime(record.UpdatedAt),
		})
	}
	return responses
}

func permissionResponses(records []database.Permission) []PermissionResponse {
	responses := make([]PermissionResponse, 0, len(records))
	for _, record := range records {
		responses = append(responses, PermissionResponse{
			ID:          record.ID,
			Code:        record.Code,
			Name:        record.Name,
			Description: record.Description,
		})
	}
	return responses
}

func optionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := formatTime(*value)
	return &formatted
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func validUserStatus(status string) bool {
	switch status {
	case UserStatusActive, UserStatusDisabled:
		return true
	default:
		return false
	}
}
