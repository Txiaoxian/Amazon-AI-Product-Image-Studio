package useradmin

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/idgen"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
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

func (r Repository) ListUsers(ctx context.Context, scope tenant.Scope, options ListOptions) ([]database.User, int64, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, 0, err
	}

	query := db.Model(&database.User{}).Where("tenant_id = ?", scope.ID())
	if options.Status != "" {
		query = query.Where("status = ?", options.Status)
	}
	if strings.TrimSpace(options.Q) != "" {
		like := "%" + escapeLike(strings.ToLower(strings.TrimSpace(options.Q))) + "%"
		query = query.Where(
			"(LOWER(email) LIKE ? ESCAPE '\\' OR LOWER(display_name) LIKE ? ESCAPE '\\')",
			like,
			like,
		)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []database.User
	offset := (options.PageNum - 1) * options.PageSize
	if err := query.
		Order("created_at DESC, id DESC").
		Limit(options.PageSize).
		Offset(offset).
		Find(&records).Error; err != nil {
		return nil, 0, err
	}

	return records, total, nil
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
		return database.User{}, ErrNotFound
	}
	return record, err
}

func (r Repository) EmailExists(ctx context.Context, scope tenant.Scope, email string) (bool, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return false, err
	}
	var count int64
	if err := db.Model(&database.User{}).
		Where("tenant_id = ? AND email = ?", scope.ID(), strings.TrimSpace(email)).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r Repository) CreateUser(ctx context.Context, scope tenant.Scope, record *database.User) error {
	db, err := r.base(ctx, scope)
	if err != nil {
		return err
	}
	record.TenantID = scope.ID()
	if err := db.Create(record).Error; err != nil {
		if isDuplicateKeyError(err) {
			return ErrConflict
		}
		return err
	}
	return nil
}

func (r Repository) UpdateUser(ctx context.Context, scope tenant.Scope, userID string, updates map[string]any) (database.User, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.User{}, err
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return database.User{}, ErrValidation
	}
	result := db.Model(&database.User{}).
		Where("tenant_id = ? AND id = ?", scope.ID(), userID).
		Updates(updates)
	if result.Error != nil {
		return database.User{}, result.Error
	}
	if result.RowsAffected == 0 {
		return database.User{}, ErrNotFound
	}
	return r.FindUser(ctx, scope, userID)
}

func (r Repository) ListRolesForUsers(ctx context.Context, scope tenant.Scope, userIDs []string) (map[string][]database.Role, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, err
	}
	userIDs = cleanStringSet(userIDs)
	if len(userIDs) == 0 {
		return map[string][]database.Role{}, nil
	}

	var rows []struct {
		UserID      string
		ID          string
		TenantID    string
		Code        string
		Name        string
		Description string
		Status      string
		CreatedAt   time.Time
		UpdatedAt   time.Time
	}
	if err := db.Table("roles").
		Select("user_roles.user_id, roles.id, roles.tenant_id, roles.code, roles.name, roles.description, roles.status, roles.created_at, roles.updated_at").
		Joins("JOIN user_roles ON user_roles.tenant_id = roles.tenant_id AND user_roles.role_id = roles.id").
		Where("roles.tenant_id = ? AND user_roles.user_id IN ?", scope.ID(), userIDs).
		Order("roles.code ASC, roles.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[string][]database.Role, len(userIDs))
	for _, item := range rows {
		result[item.UserID] = append(result[item.UserID], database.Role{
			ID:          item.ID,
			TenantID:    item.TenantID,
			Code:        item.Code,
			Name:        item.Name,
			Description: item.Description,
			Status:      item.Status,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}
	return result, nil
}

func (r Repository) ListUserRoles(ctx context.Context, scope tenant.Scope, userID string) ([]database.Role, error) {
	rolesByUser, err := r.ListRolesForUsers(ctx, scope, []string{userID})
	if err != nil {
		return nil, err
	}
	return rolesByUser[strings.TrimSpace(userID)], nil
}

func (r Repository) ActiveRolesByIDs(ctx context.Context, scope tenant.Scope, roleIDs []string) ([]database.Role, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, err
	}
	roleIDs = cleanStringSet(roleIDs)
	if len(roleIDs) == 0 {
		return []database.Role{}, nil
	}

	var records []database.Role
	if err := db.Model(&database.Role{}).
		Where("tenant_id = ? AND id IN ? AND status = ?", scope.ID(), roleIDs, auth.RoleStatusActive).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Order("code ASC, id ASC").
		Find(&records).Error; err != nil {
		return nil, err
	}
	if len(records) != len(roleIDs) {
		return nil, ErrValidation
	}
	return records, nil
}

func (r Repository) ListRoles(ctx context.Context, scope tenant.Scope) ([]database.Role, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, err
	}
	var records []database.Role
	if err := db.Model(&database.Role{}).
		Where("tenant_id = ?", scope.ID()).
		Order("code ASC, id ASC").
		Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func (r Repository) FindTenant(ctx context.Context, scope tenant.Scope) (database.Tenant, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.Tenant{}, err
	}
	var record database.Tenant
	err = db.Model(&database.Tenant{}).Where("id = ?", scope.ID()).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return database.Tenant{}, ErrNotFound
	}
	return record, err
}

func (r Repository) UpdateTenant(ctx context.Context, scope tenant.Scope, name string, updatedAt time.Time) (database.Tenant, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.Tenant{}, err
	}
	result := db.Model(&database.Tenant{}).
		Where("id = ?", scope.ID()).
		Updates(map[string]any{"name": name, "updated_at": updatedAt})
	if result.Error != nil {
		return database.Tenant{}, result.Error
	}
	if result.RowsAffected == 0 {
		return database.Tenant{}, ErrNotFound
	}
	return r.FindTenant(ctx, scope)
}

func (r Repository) FindRole(ctx context.Context, scope tenant.Scope, roleID string) (database.Role, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.Role{}, err
	}
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return database.Role{}, ErrValidation
	}
	var record database.Role
	err = db.Model(&database.Role{}).
		Where("tenant_id = ? AND id = ?", scope.ID(), roleID).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return database.Role{}, ErrNotFound
	}
	return record, err
}

func (r Repository) RoleCodeExists(ctx context.Context, scope tenant.Scope, code string) (bool, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return false, err
	}
	var count int64
	if err := db.Model(&database.Role{}).
		Where("tenant_id = ? AND code = ?", scope.ID(), strings.TrimSpace(code)).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r Repository) CreateRole(ctx context.Context, scope tenant.Scope, record *database.Role) error {
	db, err := r.base(ctx, scope)
	if err != nil {
		return err
	}
	record.TenantID = scope.ID()
	if err := db.Create(record).Error; err != nil {
		if isDuplicateKeyError(err) {
			return ErrConflict
		}
		return err
	}
	return nil
}

func (r Repository) UpdateRole(ctx context.Context, scope tenant.Scope, roleID string, updates map[string]any) (database.Role, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.Role{}, err
	}
	result := db.Model(&database.Role{}).
		Where("tenant_id = ? AND id = ?", scope.ID(), strings.TrimSpace(roleID)).
		Updates(updates)
	if result.Error != nil {
		return database.Role{}, result.Error
	}
	if result.RowsAffected == 0 {
		return database.Role{}, ErrNotFound
	}
	return r.FindRole(ctx, scope, roleID)
}

func (r Repository) DeleteRole(ctx context.Context, scope tenant.Scope, roleID string) error {
	db, err := r.base(ctx, scope)
	if err != nil {
		return err
	}
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return ErrValidation
	}
	if err := db.Where("tenant_id = ? AND role_id = ?", scope.ID(), roleID).Delete(&database.RolePermission{}).Error; err != nil {
		return err
	}
	result := db.Where("tenant_id = ? AND id = ?", scope.ID(), roleID).Delete(&database.Role{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r Repository) RoleAssignmentCount(ctx context.Context, scope tenant.Scope, roleID string) (int64, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := db.Model(&database.UserRole{}).
		Where("tenant_id = ? AND role_id = ?", scope.ID(), strings.TrimSpace(roleID)).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r Repository) ListPermissions(ctx context.Context) ([]database.Permission, error) {
	if r.db == nil {
		return nil, database.ErrNilDB
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var records []database.Permission
	if err := r.db.WithContext(ctx).Model(&database.Permission{}).
		Order("code ASC, id ASC").
		Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func (r Repository) PermissionsByIDs(ctx context.Context, permissionIDs []string) ([]database.Permission, error) {
	if r.db == nil {
		return nil, database.ErrNilDB
	}
	if ctx == nil {
		ctx = context.Background()
	}
	permissionIDs = cleanStringSet(permissionIDs)
	if len(permissionIDs) == 0 {
		return []database.Permission{}, nil
	}
	var records []database.Permission
	if err := r.db.WithContext(ctx).Model(&database.Permission{}).
		Where("id IN ?", permissionIDs).
		Order("code ASC, id ASC").
		Find(&records).Error; err != nil {
		return nil, err
	}
	if len(records) != len(permissionIDs) {
		return nil, ErrValidation
	}
	return records, nil
}

func (r Repository) ListPermissionsForRoles(ctx context.Context, scope tenant.Scope, roleIDs []string) (map[string][]database.Permission, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, err
	}
	roleIDs = cleanStringSet(roleIDs)
	if len(roleIDs) == 0 {
		return map[string][]database.Permission{}, nil
	}

	type row struct {
		RoleID      string
		ID          string
		Code        string
		Name        string
		Description string
	}
	var rows []row
	if err := db.Table("permissions").
		Select("role_permissions.role_id, permissions.id, permissions.code, permissions.name, permissions.description").
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Where("role_permissions.tenant_id = ? AND role_permissions.role_id IN ?", scope.ID(), roleIDs).
		Order("permissions.code ASC, permissions.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string][]database.Permission, len(roleIDs))
	for _, item := range rows {
		result[item.RoleID] = append(result[item.RoleID], database.Permission{
			ID:          item.ID,
			Code:        item.Code,
			Name:        item.Name,
			Description: item.Description,
		})
	}
	return result, nil
}

func (r Repository) ReplaceUserRoles(ctx context.Context, scope tenant.Scope, userID string, roleIDs []string) error {
	db, err := r.base(ctx, scope)
	if err != nil {
		return err
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ErrValidation
	}
	if err := db.Where("tenant_id = ? AND user_id = ?", scope.ID(), userID).Delete(&database.UserRole{}).Error; err != nil {
		return err
	}
	if len(roleIDs) == 0 {
		return nil
	}
	return db.Create(userRoleRecords(scope.ID(), userID, roleIDs)).Error
}

func (r Repository) ListUserModelAccessIDs(ctx context.Context, scope tenant.Scope, userID string) ([]string, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, err
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrValidation
	}
	var modelIDs []string
	if err := db.Table("user_model_access_grants").
		Select("user_model_access_grants.model_id").
		Joins("JOIN ai_models ON ai_models.tenant_id = user_model_access_grants.tenant_id AND ai_models.id = user_model_access_grants.model_id AND ai_models.deleted_at IS NULL").
		Where("user_model_access_grants.tenant_id = ? AND user_model_access_grants.user_id = ?", scope.ID(), userID).
		Order("user_model_access_grants.model_id ASC").
		Pluck("user_model_access_grants.model_id", &modelIDs).Error; err != nil {
		return nil, err
	}
	return modelIDs, nil
}

func (r Repository) ModelsByIDs(ctx context.Context, scope tenant.Scope, modelIDs []string) ([]database.AIModel, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, err
	}
	modelIDs = cleanStringSet(modelIDs)
	if len(modelIDs) == 0 {
		return []database.AIModel{}, nil
	}
	var records []database.AIModel
	if err := db.Model(&database.AIModel{}).
		Where("tenant_id = ? AND id IN ? AND deleted_at IS NULL", scope.ID(), modelIDs).
		Order("id ASC").
		Find(&records).Error; err != nil {
		return nil, err
	}
	if len(records) != len(modelIDs) {
		return nil, ErrValidation
	}
	return records, nil
}

func (r Repository) ReplaceUserModelAccess(ctx context.Context, scope tenant.Scope, userID string, modelIDs []string, grantedBy string, now time.Time) error {
	db, err := r.base(ctx, scope)
	if err != nil {
		return err
	}
	userID = strings.TrimSpace(userID)
	grantedBy = strings.TrimSpace(grantedBy)
	if userID == "" || grantedBy == "" {
		return ErrValidation
	}
	if err := db.Where("tenant_id = ? AND user_id = ?", scope.ID(), userID).Delete(&database.UserModelAccessGrant{}).Error; err != nil {
		return err
	}
	modelIDs = cleanStringSet(modelIDs)
	if len(modelIDs) == 0 {
		return nil
	}
	records := make([]database.UserModelAccessGrant, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		actorID := grantedBy
		records = append(records, database.UserModelAccessGrant{
			ID:        newID(),
			TenantID:  scope.ID(),
			UserID:    userID,
			ModelID:   modelID,
			GrantedBy: &actorID,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	return db.Create(&records).Error
}

func (r Repository) ReplaceRolePermissions(ctx context.Context, scope tenant.Scope, roleID string, permissionIDs []string) error {
	db, err := r.base(ctx, scope)
	if err != nil {
		return err
	}
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return ErrValidation
	}
	if err := db.Where("tenant_id = ? AND role_id = ?", scope.ID(), roleID).Delete(&database.RolePermission{}).Error; err != nil {
		return err
	}
	if len(permissionIDs) == 0 {
		return nil
	}
	records := make([]database.RolePermission, 0, len(permissionIDs))
	for _, permissionID := range permissionIDs {
		records = append(records, database.RolePermission{
			ID:           newID(),
			TenantID:     scope.ID(),
			RoleID:       roleID,
			PermissionID: permissionID,
			CreatedAt:    nowUTC(),
		})
	}
	return db.Create(records).Error
}

func (r Repository) ActiveAdminCount(ctx context.Context, scope tenant.Scope) (int64, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := db.Table("users").
		Joins("JOIN user_roles ON user_roles.tenant_id = users.tenant_id AND user_roles.user_id = users.id").
		Joins("JOIN roles ON roles.tenant_id = user_roles.tenant_id AND roles.id = user_roles.role_id").
		Where("users.tenant_id = ? AND users.status = ? AND roles.status = ? AND roles.code = ?", scope.ID(), UserStatusActive, auth.RoleStatusActive, "admin").
		Distinct("users.id").
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r Repository) UserHasActiveRoleCode(ctx context.Context, scope tenant.Scope, userID string, roleCode string) (bool, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return false, err
	}
	var count int64
	if err := db.Table("user_roles").
		Joins("JOIN roles ON roles.tenant_id = user_roles.tenant_id AND roles.id = user_roles.role_id").
		Where("user_roles.tenant_id = ? AND user_roles.user_id = ? AND roles.status = ? AND roles.code = ?", scope.ID(), strings.TrimSpace(userID), auth.RoleStatusActive, strings.TrimSpace(roleCode)).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func userRoleRecords(tenantID string, userID string, roleIDs []string) []database.UserRole {
	records := make([]database.UserRole, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		records = append(records, database.UserRole{
			ID:        newID(),
			TenantID:  tenantID,
			UserID:    userID,
			RoleID:    roleID,
			CreatedAt: nowUTC(),
		})
	}
	return records
}

func newID() string {
	return idgen.New()
}

func nowUTC() time.Time {
	return time.Now().UTC()
}

func cleanStringSet(values []string) []string {
	seen := map[string]bool{}
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		cleaned = append(cleaned, value)
	}
	return cleaned
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}

func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate") || strings.Contains(message, "unique constraint")
}
