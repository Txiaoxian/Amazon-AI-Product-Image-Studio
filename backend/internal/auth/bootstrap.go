package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/idgen"
	"gorm.io/gorm"
)

type builtInRole struct {
	Code        string
	Name        string
	Description string
	Permissions []string
}

type builtInPermission struct {
	Code        string
	Name        string
	Description string
}

var builtInPermissions = []builtInPermission{
	{Code: "user:read", Name: "查看用户", Description: "查看当前租户的用户列表与用户详情。"},
	{Code: "user:create", Name: "创建用户", Description: "在当前租户中创建普通用户账号。"},
	{Code: "user:update", Name: "编辑用户", Description: "修改当前租户用户的显示名称等安全字段。"},
	{Code: "user:disable", Name: "启用或禁用用户", Description: "启用或禁用当前租户的用户账号。"},
	{Code: "role:read", Name: "查看角色与权限", Description: "查看当前租户的角色以及系统权限目录。"},
	{Code: "role:manage", Name: "管理角色与权限", Description: "创建、编辑、删除自定义角色并配置角色权限。"},
	{Code: "project:read", Name: "查看产品", Description: "查看当前租户中有权限访问的产品。"},
	{Code: "project:create", Name: "创建产品", Description: "在当前租户中创建新的产品。"},
	{Code: "project:update", Name: "编辑产品", Description: "修改有权限管理的产品资料。"},
	{Code: "project:delete", Name: "删除产品", Description: "删除有权限管理的产品。"},
	{Code: "project:member:manage", Name: "管理产品成员", Description: "添加、修改或移除产品成员。"},
	{Code: "asset:read", Name: "查看产品素材", Description: "查看有权限访问的产品素材。"},
	{Code: "asset:upload", Name: "上传产品素材", Description: "向有编辑权限的产品上传参考图片。"},
	{Code: "asset:update", Name: "编辑产品素材", Description: "修改产品素材的名称、分类和收藏状态。"},
	{Code: "asset:delete", Name: "删除产品素材", Description: "删除有权限管理的产品素材。"},
	{Code: "asset:download", Name: "下载产品素材", Description: "通过后端授权下载产品素材。"},
	{Code: "task:read", Name: "查看生图任务", Description: "查看有权限访问的生图任务和生成记录。"},
	{Code: "task:create", Name: "创建生图任务", Description: "使用已分配的模型创建生图或图片编辑任务。"},
	{Code: "task:cancel", Name: "取消生图任务", Description: "取消有权限操作且尚未结束的生图任务。"},
	{Code: "task:retry", Name: "重试生图任务", Description: "重试有权限操作且允许重试的生图任务。"},
	{Code: "provider:read", Name: "查看中转站", Description: "仅供管理员查看当前租户的 AI 中转站配置。"},
	{Code: "provider:manage", Name: "管理中转站", Description: "仅供管理员创建、编辑、删除、启用、禁用和测试 AI 中转站。"},
	{Code: "model:read", Name: "查看模型", Description: "查看当前账号获准使用的 AI 模型及能力。"},
	{Code: "model:manage", Name: "管理模型", Description: "仅供管理员创建、编辑、删除、启用或禁用 AI 模型配置。"},
	{Code: "usage:read", Name: "查看用量", Description: "查看当前租户的模型调用用量与费用统计。"},
	{Code: "audit:read", Name: "查看审计记录", Description: "查看当前租户的操作日志和接口调用记录。"},
	{Code: "system:settings:manage", Name: "管理系统设置", Description: "查看和修改当前租户的运行参数与系统设置。"},
}

var builtInPermissionCodes = permissionCodes(builtInPermissions)

var builtInRoles = []builtInRole{
	{
		Code:        "admin",
		Name:        "管理员",
		Description: "管理当前租户的用户、权限、中转站、模型和系统设置。",
		Permissions: builtInPermissionCodes,
	},
	{
		Code:        "seller",
		Name:        "普通用户",
		Description: "创建产品图片并管理有权限访问的产品和素材。",
		Permissions: []string{
			"project:read",
			"project:create",
			"project:update",
			"asset:read",
			"asset:upload",
			"asset:update",
			"asset:delete",
			"asset:download",
			"task:read",
			"task:create",
			"task:cancel",
			"task:retry",
			"model:read",
			"usage:read",
		},
	},
	{
		Code:        "viewer",
		Name:        "只读用户",
		Description: "只读查看已分配的产品、素材和任务记录。",
		Permissions: []string{
			"project:read",
			"asset:read",
			"asset:download",
			"task:read",
		},
	},
}

func ReconcileBuiltInRoles(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return database.ErrNilDB
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var tenants []database.Tenant
	if err := db.WithContext(ctx).Select("id").Order("id ASC").Find(&tenants).Error; err != nil {
		return fmt.Errorf("list tenants for built-in role reconciliation: %w", err)
	}

	now := time.Now().UTC()
	service := &Service{}
	for _, tenant := range tenants {
		if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			_, err := service.seedBuiltInRoles(ctx, tx, tenant.ID, now)
			return err
		}); err != nil {
			return fmt.Errorf("reconcile tenant built-in roles: %w", err)
		}
	}

	return nil
}

func (s *Service) seedBuiltInRoles(ctx context.Context, tx *gorm.DB, tenantID string, now time.Time) (map[string]database.Role, error) {
	permissionsByCode := make(map[string]database.Permission, len(builtInPermissions))
	for _, builtIn := range builtInPermissions {
		permission, err := ensurePermission(ctx, tx, builtIn, now)
		if err != nil {
			return nil, err
		}
		permissionsByCode[builtIn.Code] = permission
	}

	rolesByCode := make(map[string]database.Role, len(builtInRoles))
	for _, builtIn := range builtInRoles {
		role, err := ensureRole(ctx, tx, tenantID, builtIn, now)
		if err != nil {
			return nil, err
		}
		rolesByCode[builtIn.Code] = role

		permissionIDs := make([]string, 0, len(builtIn.Permissions))
		for _, code := range builtIn.Permissions {
			permission, ok := permissionsByCode[code]
			if !ok {
				return nil, errors.New("missing built-in permission " + code)
			}
			if err := ensureRolePermission(ctx, tx, tenantID, role.ID, permission.ID, now); err != nil {
				return nil, err
			}
			permissionIDs = append(permissionIDs, permission.ID)
		}
		if err := reconcileBuiltInRolePermissions(ctx, tx, tenantID, role.ID, permissionIDs); err != nil {
			return nil, err
		}
	}

	return rolesByCode, nil
}

func reconcileBuiltInRolePermissions(ctx context.Context, tx *gorm.DB, tenantID string, roleID string, permissionIDs []string) error {
	query := tx.WithContext(ctx).Where("tenant_id = ? AND role_id = ?", tenantID, roleID)
	if len(permissionIDs) > 0 {
		query = query.Where("permission_id NOT IN ?", permissionIDs)
	}
	return query.Delete(&database.RolePermission{}).Error
}

func ensurePermission(ctx context.Context, tx *gorm.DB, builtIn builtInPermission, now time.Time) (database.Permission, error) {
	code := strings.TrimSpace(builtIn.Code)
	var permission database.Permission
	err := tx.WithContext(ctx).Where("code = ?", code).First(&permission).Error
	if err == nil {
		if permission.Name != builtIn.Name || permission.Description != builtIn.Description {
			if err := tx.WithContext(ctx).Model(&database.Permission{}).
				Where("id = ?", permission.ID).
				Updates(map[string]any{"name": builtIn.Name, "description": builtIn.Description, "updated_at": now}).Error; err != nil {
				return database.Permission{}, err
			}
			permission.Name = builtIn.Name
			permission.Description = builtIn.Description
			permission.UpdatedAt = now
		}
		return permission, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return database.Permission{}, err
	}

	permission = database.Permission{
		ID:          idgen.New(),
		Code:        code,
		Name:        builtIn.Name,
		Description: builtIn.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return permission, tx.WithContext(ctx).Create(&permission).Error
}

func ensureRole(ctx context.Context, tx *gorm.DB, tenantID string, builtIn builtInRole, now time.Time) (database.Role, error) {
	var role database.Role
	err := tx.WithContext(ctx).Where("tenant_id = ? AND code = ?", tenantID, builtIn.Code).First(&role).Error
	if err == nil {
		if role.Name != builtIn.Name || role.Description != builtIn.Description {
			if err := tx.WithContext(ctx).Model(&database.Role{}).
				Where("tenant_id = ? AND id = ?", tenantID, role.ID).
				Updates(map[string]any{"name": builtIn.Name, "description": builtIn.Description, "updated_at": now}).Error; err != nil {
				return database.Role{}, err
			}
			role.Name = builtIn.Name
			role.Description = builtIn.Description
			role.UpdatedAt = now
		}
		return role, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return database.Role{}, err
	}

	role = database.Role{
		ID:          idgen.New(),
		TenantID:    tenantID,
		Code:        builtIn.Code,
		Name:        builtIn.Name,
		Description: builtIn.Description,
		Status:      RoleStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return role, tx.WithContext(ctx).Create(&role).Error
}

func permissionCodes(permissions []builtInPermission) []string {
	codes := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		codes = append(codes, permission.Code)
	}
	return codes
}

func ensureRolePermission(ctx context.Context, tx *gorm.DB, tenantID string, roleID string, permissionID string, now time.Time) error {
	var existing database.RolePermission
	err := tx.WithContext(ctx).
		Where("tenant_id = ? AND role_id = ? AND permission_id = ?", tenantID, roleID, permissionID).
		First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	return tx.WithContext(ctx).Create(&database.RolePermission{
		ID:           idgen.New(),
		TenantID:     tenantID,
		RoleID:       roleID,
		PermissionID: permissionID,
		CreatedAt:    now,
	}).Error
}
