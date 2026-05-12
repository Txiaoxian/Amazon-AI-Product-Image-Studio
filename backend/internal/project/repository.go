package project

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

type ListOptions struct {
	PageNum      int
	PageSize     int
	Status       string
	MemberUserID string
}

func NewRepository(db *gorm.DB) Repository {
	return Repository{db: db}
}

func (r Repository) withDB(db *gorm.DB) Repository {
	return Repository{db: db}
}

func (r Repository) base(ctx context.Context, scope tenant.Scope) (*gorm.DB, error) {
	if r.db == nil {
		return nil, database.ErrNilDB
	}
	if !scope.Valid() {
		return nil, tenant.ErrMissingTenantID
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return r.db.WithContext(ctx), nil
}

func (r Repository) ListProjects(ctx context.Context, scope tenant.Scope, options ListOptions) ([]database.Project, int64, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, 0, err
	}

	query := db.Model(&database.Project{}).
		Where("projects.tenant_id = ? AND projects.deleted_at IS NULL", scope.ID())
	if options.Status != "" {
		query = query.Where("projects.status = ?", options.Status)
	}
	if strings.TrimSpace(options.MemberUserID) != "" {
		query = query.Joins(
			"JOIN project_members ON project_members.tenant_id = projects.tenant_id AND project_members.project_id = projects.id AND project_members.user_id = ?",
			strings.TrimSpace(options.MemberUserID),
		).Where("project_members.tenant_id = ?", scope.ID())
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []database.Project
	offset := (options.PageNum - 1) * options.PageSize
	if err := query.
		Order("projects.created_at DESC, projects.id DESC").
		Limit(options.PageSize).
		Offset(offset).
		Find(&records).Error; err != nil {
		return nil, 0, err
	}

	return records, total, nil
}

func (r Repository) FindProject(ctx context.Context, scope tenant.Scope, projectID string) (database.Project, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.Project{}, err
	}

	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return database.Project{}, ErrValidation
	}

	var record database.Project
	err = db.Model(&database.Project{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", scope.ID(), projectID).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return database.Project{}, ErrNotFound
	}
	return record, err
}

func (r Repository) CreateProject(ctx context.Context, scope tenant.Scope, record *database.Project) error {
	db, err := r.base(ctx, scope)
	if err != nil {
		return err
	}
	record.TenantID = scope.ID()
	return db.Create(record).Error
}

func (r Repository) UpdateProject(ctx context.Context, scope tenant.Scope, projectID string, updates map[string]any) (database.Project, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.Project{}, err
	}
	if strings.TrimSpace(projectID) == "" {
		return database.Project{}, ErrValidation
	}

	result := db.Model(&database.Project{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", scope.ID(), strings.TrimSpace(projectID)).
		Updates(updates)
	if result.Error != nil {
		return database.Project{}, result.Error
	}

	return r.FindProject(ctx, scope, projectID)
}

func (r Repository) SoftDeleteProject(ctx context.Context, scope tenant.Scope, projectID string, deletedAt time.Time) error {
	db, err := r.base(ctx, scope)
	if err != nil {
		return err
	}
	if strings.TrimSpace(projectID) == "" {
		return ErrValidation
	}

	result := db.Model(&database.Project{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", scope.ID(), strings.TrimSpace(projectID)).
		Update("deleted_at", deletedAt.UTC())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r Repository) ListMembers(ctx context.Context, scope tenant.Scope, projectID string) ([]database.ProjectMember, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(projectID) == "" {
		return nil, ErrValidation
	}

	var records []database.ProjectMember
	err = db.Model(&database.ProjectMember{}).
		Where("tenant_id = ? AND project_id = ?", scope.ID(), strings.TrimSpace(projectID)).
		Order("created_at ASC, user_id ASC").
		Find(&records).Error
	return records, err
}

func (r Repository) FindMember(ctx context.Context, scope tenant.Scope, projectID string, userID string) (database.ProjectMember, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.ProjectMember{}, err
	}
	projectID = strings.TrimSpace(projectID)
	userID = strings.TrimSpace(userID)
	if projectID == "" || userID == "" {
		return database.ProjectMember{}, ErrValidation
	}

	var record database.ProjectMember
	err = db.Model(&database.ProjectMember{}).
		Where("tenant_id = ? AND project_id = ? AND user_id = ?", scope.ID(), projectID, userID).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return database.ProjectMember{}, ErrNotFound
	}
	return record, err
}

func (r Repository) CreateMember(ctx context.Context, scope tenant.Scope, record *database.ProjectMember) error {
	db, err := r.base(ctx, scope)
	if err != nil {
		return err
	}
	record.TenantID = scope.ID()
	return db.Create(record).Error
}

func (r Repository) UpdateMember(ctx context.Context, scope tenant.Scope, projectID string, userID string, role string, updatedAt time.Time) (database.ProjectMember, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.ProjectMember{}, err
	}
	projectID = strings.TrimSpace(projectID)
	userID = strings.TrimSpace(userID)
	if projectID == "" || userID == "" {
		return database.ProjectMember{}, ErrValidation
	}

	result := db.Model(&database.ProjectMember{}).
		Where("tenant_id = ? AND project_id = ? AND user_id = ?", scope.ID(), projectID, userID).
		Updates(map[string]any{"role": role, "updated_at": updatedAt.UTC()})
	if result.Error != nil {
		return database.ProjectMember{}, result.Error
	}

	return r.FindMember(ctx, scope, projectID, userID)
}

func (r Repository) DeleteMember(ctx context.Context, scope tenant.Scope, projectID string, userID string) error {
	db, err := r.base(ctx, scope)
	if err != nil {
		return err
	}
	projectID = strings.TrimSpace(projectID)
	userID = strings.TrimSpace(userID)
	if projectID == "" || userID == "" {
		return ErrValidation
	}

	result := db.Where("tenant_id = ? AND project_id = ? AND user_id = ?", scope.ID(), projectID, userID).
		Delete(&database.ProjectMember{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r Repository) ActiveUserExists(ctx context.Context, scope tenant.Scope, userID string) (bool, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return false, err
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false, ErrValidation
	}

	var count int64
	err = db.Model(&database.User{}).
		Where("tenant_id = ? AND id = ? AND status = ?", scope.ID(), userID, auth.UserStatusActive).
		Count(&count).Error
	return count > 0, err
}
