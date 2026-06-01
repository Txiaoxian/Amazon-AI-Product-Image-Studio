package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/audit"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/idgen"
	"gorm.io/gorm"
)

var (
	ErrInvalidTenantProvisioningInput = errors.New("invalid tenant provisioning input")
	ErrPlatformNotInitialized         = errors.New("platform must be initialized before provisioning additional tenants")
)

type TenantProvisioningInput struct {
	TenantName       string
	AdminEmail       string
	AdminDisplayName string
	AdminPassword    string
}

type TenantProvisioningResult struct {
	TenantID string
}

type normalizedTenantProvisioningInput struct {
	tenantName       string
	adminEmail       string
	adminDisplayName string
	adminPassword    string
}

func ValidateTenantProvisioningInput(input TenantProvisioningInput) error {
	_, err := normalizeTenantProvisioningInput(input)
	return err
}

func ProvisionTenant(ctx context.Context, db *gorm.DB, input TenantProvisioningInput) (TenantProvisioningResult, error) {
	return provisionTenant(ctx, db, input, nil)
}

func provisionTenant(ctx context.Context, db *gorm.DB, input TenantProvisioningInput, beforeCommit func() error) (TenantProvisioningResult, error) {
	normalized, err := normalizeTenantProvisioningInput(input)
	if err != nil {
		return TenantProvisioningResult{}, err
	}
	if db == nil {
		return TenantProvisioningResult{}, database.ErrNilDB
	}
	if ctx == nil {
		ctx = context.Background()
	}

	passwordHash, err := HashPassword(normalized.adminPassword)
	if err != nil {
		return TenantProvisioningResult{}, fmt.Errorf("hash tenant admin password: %w", err)
	}

	var result TenantProvisioningResult
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		initialized, err := (&Service{}).initialized(ctx, tx)
		if err != nil {
			return fmt.Errorf("check platform initialization: %w", err)
		}
		if !initialized {
			return ErrPlatformNotInitialized
		}

		now := time.Now().UTC()
		tenantRecord := database.Tenant{
			ID:        idgen.New(),
			Name:      normalized.tenantName,
			Status:    TenantStatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := tx.WithContext(ctx).Create(&tenantRecord).Error; err != nil {
			return fmt.Errorf("create tenant: %w", err)
		}

		rolesByCode, err := (&Service{}).seedBuiltInRoles(ctx, tx, tenantRecord.ID, now)
		if err != nil {
			return fmt.Errorf("seed built-in roles: %w", err)
		}

		adminRecord := database.User{
			ID:           idgen.New(),
			TenantID:     tenantRecord.ID,
			Email:        normalized.adminEmail,
			DisplayName:  normalized.adminDisplayName,
			PasswordHash: passwordHash,
			Status:       UserStatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := tx.WithContext(ctx).Create(&adminRecord).Error; err != nil {
			return fmt.Errorf("create tenant admin: %w", err)
		}

		adminRole, ok := rolesByCode["admin"]
		if !ok {
			return errors.New("missing built-in admin role")
		}
		if err := tx.WithContext(ctx).Create(&database.UserRole{
			ID:        idgen.New(),
			TenantID:  tenantRecord.ID,
			UserID:    adminRecord.ID,
			RoleID:    adminRole.ID,
			CreatedAt: now,
		}).Error; err != nil {
			return fmt.Errorf("assign tenant admin role: %w", err)
		}

		if err := audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     tenantRecord.ID,
			Action:       "auth.provision_tenant",
			ResourceType: "tenant",
			ResourceID:   tenantRecord.ID,
			Metadata: map[string]any{
				"result": "success",
			},
		}); err != nil {
			return fmt.Errorf("record tenant provisioning audit: %w", err)
		}

		if beforeCommit != nil {
			if err := beforeCommit(); err != nil {
				return err
			}
		}

		result.TenantID = tenantRecord.ID
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return TenantProvisioningResult{}, err
	}

	return result, nil
}

func normalizeTenantProvisioningInput(input TenantProvisioningInput) (normalizedTenantProvisioningInput, error) {
	tenantName := cleanDisplayValue(input.TenantName, 255)
	adminDisplayName := cleanDisplayValue(input.AdminDisplayName, 255)
	adminEmail, err := normalizeEmail(input.AdminEmail)
	if err != nil || tenantName == "" || adminDisplayName == "" {
		return normalizedTenantProvisioningInput{}, ErrInvalidTenantProvisioningInput
	}
	if err := ValidatePassword(input.AdminPassword); err != nil {
		return normalizedTenantProvisioningInput{}, err
	}

	return normalizedTenantProvisioningInput{
		tenantName:       tenantName,
		adminEmail:       adminEmail,
		adminDisplayName: adminDisplayName,
		adminPassword:    input.AdminPassword,
	}, nil
}
