package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"gorm.io/gorm"
)

type TenantRepository struct {
	db *gorm.DB
}

func NewTenantRepository(db *gorm.DB) TenantRepository {
	return TenantRepository{db: db}
}

func (r TenantRepository) Query(ctx context.Context, scope tenant.Scope) (*gorm.DB, error) {
	db := r.db
	if db == nil {
		return nil, ErrNilDB
	}
	if ctx != nil {
		db = db.WithContext(ctx)
	}
	if !scope.Valid() {
		return nil, tenant.ErrMissingTenantID
	}

	return db.Where("tenant_id = ?", scope.ID()), nil
}

func (r TenantRepository) FirstByID(ctx context.Context, scope tenant.Scope, dest any, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("id is required")
	}

	query, err := r.Query(ctx, scope)
	if err != nil {
		return err
	}

	if err := query.Where("id = ?", id).First(dest).Error; err != nil {
		return fmt.Errorf("find tenant record by id: %w", err)
	}

	return nil
}
