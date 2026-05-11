package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestTenantRepositoryRequiresTenantScope(t *testing.T) {
	db := newTenantRepositoryTestDB(t)
	repo := NewTenantRepository(db)

	_, err := repo.Query(context.Background(), tenant.Scope{})
	if !errors.Is(err, tenant.ErrMissingTenantID) {
		t.Fatalf("Query error = %v, want ErrMissingTenantID", err)
	}
}

func TestTenantRepositoryHidesRecordsFromOtherTenants(t *testing.T) {
	db := newTenantRepositoryTestDB(t)
	repo := NewTenantRepository(db)
	now := time.Now().UTC()

	tenantA, err := tenant.NewScope("tenant-a")
	if err != nil {
		t.Fatalf("create tenant A scope: %v", err)
	}
	tenantB, err := tenant.NewScope("tenant-b")
	if err != nil {
		t.Fatalf("create tenant B scope: %v", err)
	}

	users := []User{
		{
			ID:           "user-a",
			TenantID:     tenantA.ID(),
			Email:        "shared@example.com",
			DisplayName:  "Tenant A User",
			PasswordHash: "hash-a",
			Status:       "ACTIVE",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "user-b",
			TenantID:     tenantB.ID(),
			Email:        "shared@example.com",
			DisplayName:  "Tenant B User",
			PasswordHash: "hash-b",
			Status:       "ACTIVE",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}

	var notVisible User
	err = repo.FirstByID(context.Background(), tenantA, &notVisible, "user-b")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("FirstByID cross-tenant error = %v, want record not found", err)
	}

	query, err := repo.Query(context.Background(), tenantA)
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}

	var visible []struct {
		ID       string
		TenantID string
	}
	if err := query.Model(&User{}).
		Select("id", "tenant_id").
		Where("email = ?", "shared@example.com").
		Scan(&visible).Error; err != nil {
		t.Fatalf("Find tenant-scoped users: %v", err)
	}
	if len(visible) != 1 {
		t.Fatalf("visible user count = %d, want 1", len(visible))
	}
	if visible[0].TenantID != tenantA.ID() || visible[0].ID != "user-a" {
		t.Fatalf("visible user = %#v, want tenant A user only", visible[0])
	}
}

func newTenantRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Discard,
	})
	if err != nil {
		t.Fatalf("open sqlite test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access sqlite test database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}

	return db
}
