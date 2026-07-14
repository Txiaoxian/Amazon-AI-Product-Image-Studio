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
	"gorm.io/gorm/clause"
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

type memberRow struct {
	ID         string
	TenantID   string
	ProjectID  string
	UserID     string
	Role       string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	UserEmail  string
	UserName   string
	UserStatus string
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
		Order("projects.sort_order ASC, projects.created_at DESC, projects.id DESC").
		Limit(options.PageSize).
		Offset(offset).
		Find(&records).Error; err != nil {
		return nil, 0, err
	}

	return records, total, nil
}

func (r Repository) NextSortOrder(ctx context.Context, scope tenant.Scope) (int, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return 0, err
	}

	var maxOrder int
	if err := db.Model(&database.Project{}).
		Where("tenant_id = ? AND deleted_at IS NULL", scope.ID()).
		Select("COALESCE(MAX(sort_order), 0)").
		Scan(&maxOrder).Error; err != nil {
		return 0, err
	}
	return maxOrder + 10, nil
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

func (r Repository) ListMembers(ctx context.Context, scope tenant.Scope, projectID string) ([]MemberRecord, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(projectID) == "" {
		return nil, ErrValidation
	}

	var rows []memberRow
	err = db.Table("project_members").
		Select(strings.Join([]string{
			"project_members.id",
			"project_members.tenant_id",
			"project_members.project_id",
			"project_members.user_id",
			"project_members.role",
			"project_members.created_at",
			"project_members.updated_at",
			"users.email AS user_email",
			"users.display_name AS user_name",
			"users.status AS user_status",
		}, ", ")).
		Joins("JOIN users ON users.tenant_id = project_members.tenant_id AND users.id = project_members.user_id").
		Where("project_members.tenant_id = ? AND project_members.project_id = ?", scope.ID(), strings.TrimSpace(projectID)).
		Order("project_members.created_at ASC, users.display_name ASC, users.email ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	records := make([]MemberRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, MemberRecord{
			Member: database.ProjectMember{
				ID:        row.ID,
				TenantID:  row.TenantID,
				ProjectID: row.ProjectID,
				UserID:    row.UserID,
				Role:      row.Role,
				CreatedAt: row.CreatedAt,
				UpdatedAt: row.UpdatedAt,
			},
			UserEmail:  row.UserEmail,
			UserName:   row.UserName,
			UserStatus: row.UserStatus,
		})
	}
	return records, nil
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

func (r Repository) LockProjectOwners(ctx context.Context, scope tenant.Scope, projectID string) ([]database.ProjectMember, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, err
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, ErrValidation
	}

	query := db.Model(&database.ProjectMember{}).
		Where("tenant_id = ? AND project_id = ? AND role = ?", scope.ID(), projectID, RoleOwner).
		Order("user_id ASC")
	if db.Dialector.Name() != "sqlite" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	var records []database.ProjectMember
	if err := query.Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
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

func (r Repository) FindUser(ctx context.Context, scope tenant.Scope, userID string) (database.User, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.User{}, err
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return database.User{}, ErrValidation
	}

	var record database.User
	err = db.Model(&database.User{}).
		Where("tenant_id = ? AND id = ?", scope.ID(), userID).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return database.User{}, ErrValidation
	}
	return record, err
}

func (r Repository) FindActiveUser(ctx context.Context, scope tenant.Scope, userID string) (database.User, error) {
	record, err := r.FindUser(ctx, scope, userID)
	if err != nil {
		return database.User{}, err
	}
	if record.Status != auth.UserStatusActive {
		return database.User{}, ErrValidation
	}
	return record, nil
}

func (r Repository) ListMemberCandidates(ctx context.Context, scope tenant.Scope, projectID string, query CandidateListQuery) ([]database.User, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, err
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, ErrValidation
	}

	db = db.Model(&database.User{}).
		Where("tenant_id = ? AND status = ?", scope.ID(), auth.UserStatusActive).
		Where(
			"NOT EXISTS (SELECT 1 FROM project_members WHERE project_members.tenant_id = users.tenant_id AND project_members.project_id = ? AND project_members.user_id = users.id)",
			projectID,
		)
	if strings.TrimSpace(query.Q) != "" {
		q := "%" + strings.ToLower(strings.TrimSpace(query.Q)) + "%"
		db = db.Where("LOWER(email) LIKE ? OR LOWER(display_name) LIKE ?", q, q)
	}

	var records []database.User
	offset := (query.PageNum - 1) * query.PageSize
	if err := db.
		Order("display_name ASC, email ASC, id ASC").
		Limit(query.PageSize).
		Offset(offset).
		Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}
