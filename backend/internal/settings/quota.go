package settings

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/idgen"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	storageQuotaReservationStatusReserved  = "RESERVED"
	storageQuotaReservationStatusFinalized = "FINALIZED"
	storageQuotaReservationStatusReleased  = "RELEASED"
	storageQuotaReservationTTL             = 24 * time.Hour
)

type StorageQuotaReservation struct {
	ID    string
	Bytes int64
}

func ReserveStorageQuota(ctx context.Context, repo Repository, scope tenant.Scope, pendingBytes int64) (StorageQuotaReservation, error) {
	if repo.db == nil {
		return StorageQuotaReservation{}, database.ErrNilDB
	}
	if pendingBytes < 0 {
		return StorageQuotaReservation{}, ErrValidation
	}
	if pendingBytes == 0 {
		return StorageQuotaReservation{}, nil
	}
	quota, err := LoadStorageQuota(ctx, repo, scope)
	if err != nil {
		return StorageQuotaReservation{}, err
	}
	now := time.Now().UTC()
	var reservation StorageQuotaReservation
	var quotaExceeded bool
	err = repo.db.WithContext(normalizeContext(ctx)).Transaction(func(tx *gorm.DB) error {
		txRepo := repo.withDB(tx)
		counter, err := ensureStorageQuotaCounter(ctx, txRepo, scope, now, true)
		if err != nil {
			return err
		}
		if quota.MaxBytes != nil {
			if counter.UsedBytes > *quota.MaxBytes || counter.ReservedBytes > *quota.MaxBytes-counter.UsedBytes || pendingBytes > *quota.MaxBytes-counter.UsedBytes-counter.ReservedBytes {
				quotaExceeded = true
				return nil
			}
		}
		record := database.StorageQuotaReservation{
			ID:        idgen.New(),
			TenantID:  scope.ID(),
			Bytes:     pendingBytes,
			Status:    storageQuotaReservationStatusReserved,
			ExpiresAt: now.Add(storageQuotaReservationTTL),
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		if err := addStorageQuotaReservedBytes(tx, scope, pendingBytes, now); err != nil {
			return err
		}
		reservation = StorageQuotaReservation{ID: record.ID, Bytes: record.Bytes}
		return nil
	})
	if err != nil {
		return StorageQuotaReservation{}, err
	}
	if quotaExceeded {
		return StorageQuotaReservation{}, ErrStorageQuotaExceeded
	}
	return reservation, nil
}

func FinalizeStorageQuotaReservation(ctx context.Context, repo Repository, scope tenant.Scope, reservation StorageQuotaReservation, finalizedBytes int64) error {
	if repo.db == nil {
		return database.ErrNilDB
	}
	if reservation.ID == "" && reservation.Bytes == 0 && finalizedBytes == 0 {
		return nil
	}
	if finalizedBytes < 0 || finalizedBytes > reservation.Bytes {
		return ErrValidation
	}
	now := time.Now().UTC()
	return repo.db.WithContext(normalizeContext(ctx)).Transaction(func(tx *gorm.DB) error {
		txRepo := repo.withDB(tx)
		if _, err := ensureStorageQuotaCounter(ctx, txRepo, scope, now, true); err != nil {
			return err
		}
		record, err := lockStorageQuotaReservation(ctx, tx, scope, reservation.ID)
		if err != nil {
			return err
		}
		if record.Bytes < 0 || record.FinalizedBytes < 0 {
			return ErrStorageQuotaReservationInvalid
		}
		if record.Bytes != reservation.Bytes {
			return ErrStorageQuotaReservationInvalid
		}
		if finalizedBytes > record.Bytes {
			return ErrValidation
		}
		switch record.Status {
		case storageQuotaReservationStatusFinalized:
			if record.FinalizedBytes == finalizedBytes {
				return nil
			}
			return ErrStorageQuotaReservationInvalid
		case storageQuotaReservationStatusReleased:
			if finalizedBytes == 0 {
				return nil
			}
			return ErrStorageQuotaReservationInvalid
		case storageQuotaReservationStatusReserved:
		default:
			return ErrStorageQuotaReservationInvalid
		}
		status := storageQuotaReservationStatusFinalized
		if finalizedBytes == 0 {
			status = storageQuotaReservationStatusReleased
		}
		if err := tx.Model(&database.StorageQuotaReservation{}).
			Where("tenant_id = ? AND id = ? AND status = ?", scope.ID(), record.ID, storageQuotaReservationStatusReserved).
			Updates(map[string]any{
				"status":          status,
				"finalized_bytes": finalizedBytes,
				"updated_at":      now,
			}).Error; err != nil {
			return err
		}
		return applyStorageQuotaCounterDelta(tx, scope, finalizedBytes, -record.Bytes, now)
	})
}

func ReleaseStorageQuotaReservation(ctx context.Context, repo Repository, scope tenant.Scope, reservation StorageQuotaReservation) error {
	if repo.db == nil {
		return database.ErrNilDB
	}
	if reservation.ID == "" && reservation.Bytes == 0 {
		return nil
	}
	now := time.Now().UTC()
	return repo.db.WithContext(normalizeContext(ctx)).Transaction(func(tx *gorm.DB) error {
		txRepo := repo.withDB(tx)
		if _, err := ensureStorageQuotaCounter(ctx, txRepo, scope, now, true); err != nil {
			return err
		}
		record, err := lockStorageQuotaReservation(ctx, tx, scope, reservation.ID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if record.Status != storageQuotaReservationStatusReserved {
			return nil
		}
		if record.Bytes < 0 {
			return ErrStorageQuotaReservationInvalid
		}
		if err := tx.Model(&database.StorageQuotaReservation{}).
			Where("tenant_id = ? AND id = ? AND status = ?", scope.ID(), record.ID, storageQuotaReservationStatusReserved).
			Updates(map[string]any{
				"status":     storageQuotaReservationStatusReleased,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}
		return applyStorageQuotaCounterDelta(tx, scope, 0, -record.Bytes, now)
	})
}

func ReconcileStorageQuotaCounter(ctx context.Context, repo Repository, scope tenant.Scope) error {
	if repo.db == nil {
		return database.ErrNilDB
	}
	now := time.Now().UTC()
	return repo.db.WithContext(normalizeContext(ctx)).Transaction(func(tx *gorm.DB) error {
		txRepo := repo.withDB(tx)
		if _, err := ensureStorageQuotaCounter(ctx, txRepo, scope, now, true); err != nil {
			return err
		}
		if err := tx.Model(&database.StorageQuotaReservation{}).
			Where("tenant_id = ? AND status = ? AND expires_at <= ?", scope.ID(), storageQuotaReservationStatusReserved, now).
			Updates(map[string]any{
				"status":     storageQuotaReservationStatusReleased,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}
		usedBytes, err := metadataStorageUsedBytes(ctx, tx, scope)
		if err != nil {
			return err
		}
		reservedBytes, err := activeStorageQuotaReservedBytes(ctx, tx, scope, now)
		if err != nil {
			return err
		}
		return tx.Model(&database.StorageQuotaCounter{}).
			Where("tenant_id = ?", scope.ID()).
			Updates(map[string]any{
				"used_bytes":     usedBytes,
				"reserved_bytes": reservedBytes,
				"reconciled_at":  now,
				"updated_at":     now,
			}).Error
	})
}

func DecrementStorageQuotaUsedBytes(ctx context.Context, repo Repository, scope tenant.Scope, bytes int64) error {
	if repo.db == nil {
		return database.ErrNilDB
	}
	if bytes < 0 {
		return ErrValidation
	}
	if bytes == 0 {
		return nil
	}
	now := time.Now().UTC()
	return repo.db.WithContext(normalizeContext(ctx)).Transaction(func(tx *gorm.DB) error {
		txRepo := repo.withDB(tx)
		counter, err := ensureStorageQuotaCounter(ctx, txRepo, scope, now, true)
		if err != nil {
			return err
		}
		if bytes > counter.UsedBytes {
			return ErrStorageQuotaCounterInvalid
		}
		return applyStorageQuotaCounterDelta(tx, scope, -bytes, 0, now)
	})
}

func storageQuotaCounterUsedBytes(ctx context.Context, repo Repository, scope tenant.Scope) (int64, error) {
	if repo.db == nil {
		return 0, database.ErrNilDB
	}
	now := time.Now().UTC()
	var usedBytes int64
	err := repo.db.WithContext(normalizeContext(ctx)).Transaction(func(tx *gorm.DB) error {
		counter, err := ensureStorageQuotaCounter(ctx, repo.withDB(tx), scope, now, true)
		if err != nil {
			return err
		}
		usedBytes = counter.UsedBytes
		return nil
	})
	return usedBytes, err
}

func ensureStorageQuotaCounter(ctx context.Context, repo Repository, scope tenant.Scope, now time.Time, lock bool) (database.StorageQuotaCounter, error) {
	db, err := repo.base(ctx, scope)
	if err != nil {
		return database.StorageQuotaCounter{}, err
	}
	var counter database.StorageQuotaCounter
	query := db.Model(&database.StorageQuotaCounter{}).
		Select("id, tenant_id, used_bytes, reserved_bytes").
		Where("tenant_id = ?", scope.ID())
	if lock {
		query = withStorageQuotaLock(query)
	}
	err = query.First(&counter).Error
	if err == nil {
		return validateStorageQuotaCounter(counter)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return database.StorageQuotaCounter{}, err
	}
	usedBytes, err := metadataStorageUsedBytes(ctx, db, scope)
	if err != nil {
		return database.StorageQuotaCounter{}, err
	}
	counter = database.StorageQuotaCounter{
		ID:        idgen.New(),
		TenantID:  scope.ID(),
		UsedBytes: usedBytes,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&counter).Error; err != nil {
		return database.StorageQuotaCounter{}, err
	}
	query = db.Model(&database.StorageQuotaCounter{}).
		Select("id, tenant_id, used_bytes, reserved_bytes").
		Where("tenant_id = ?", scope.ID())
	if lock {
		query = withStorageQuotaLock(query)
	}
	if err := query.First(&counter).Error; err != nil {
		return database.StorageQuotaCounter{}, err
	}
	return validateStorageQuotaCounter(counter)
}

func validateStorageQuotaCounter(counter database.StorageQuotaCounter) (database.StorageQuotaCounter, error) {
	if counter.TenantID == "" || counter.UsedBytes < 0 || counter.ReservedBytes < 0 {
		return database.StorageQuotaCounter{}, ErrStorageQuotaCounterInvalid
	}
	return counter, nil
}

func lockStorageQuotaReservation(ctx context.Context, db *gorm.DB, scope tenant.Scope, reservationID string) (database.StorageQuotaReservation, error) {
	if db == nil {
		return database.StorageQuotaReservation{}, database.ErrNilDB
	}
	if !scope.Valid() {
		return database.StorageQuotaReservation{}, tenant.ErrMissingTenantID
	}
	if reservationID == "" {
		return database.StorageQuotaReservation{}, ErrStorageQuotaReservationInvalid
	}
	var record database.StorageQuotaReservation
	err := withStorageQuotaLock(db.WithContext(normalizeContext(ctx)).
		Model(&database.StorageQuotaReservation{}).
		Select("id, tenant_id, bytes, finalized_bytes, status").
		Where("tenant_id = ? AND id = ?", scope.ID(), reservationID)).
		First(&record).Error
	return record, err
}

func addStorageQuotaReservedBytes(db *gorm.DB, scope tenant.Scope, bytes int64, now time.Time) error {
	return applyStorageQuotaCounterDelta(db, scope, 0, bytes, now)
}

func applyStorageQuotaCounterDelta(db *gorm.DB, scope tenant.Scope, usedDelta int64, reservedDelta int64, now time.Time) error {
	updates := map[string]any{
		"updated_at": now,
	}
	if usedDelta < 0 {
		updates["used_bytes"] = gorm.Expr("used_bytes - ?", -usedDelta)
	} else {
		updates["used_bytes"] = gorm.Expr("used_bytes + ?", usedDelta)
	}
	if reservedDelta < 0 {
		updates["reserved_bytes"] = gorm.Expr("reserved_bytes - ?", -reservedDelta)
	} else {
		updates["reserved_bytes"] = gorm.Expr("reserved_bytes + ?", reservedDelta)
	}
	query := db.Model(&database.StorageQuotaCounter{}).Where("tenant_id = ?", scope.ID())
	if usedDelta < 0 {
		query = query.Where("used_bytes >= ?", -usedDelta)
	}
	if reservedDelta < 0 {
		query = query.Where("reserved_bytes >= ?", -reservedDelta)
	}
	result := query.
		Updates(map[string]any{
			"used_bytes":     updates["used_bytes"],
			"reserved_bytes": updates["reserved_bytes"],
			"updated_at":     updates["updated_at"],
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrStorageQuotaCounterInvalid
	}
	return nil
}

func metadataStorageUsedBytes(ctx context.Context, db *gorm.DB, scope tenant.Scope) (int64, error) {
	var total sql.NullInt64
	err := db.WithContext(normalizeContext(ctx)).
		Unscoped().
		Model(&database.ImageAsset{}).
		Select("COALESCE(SUM(size_bytes), 0)").
		Where("tenant_id = ? AND purged_at IS NULL", scope.ID()).
		Scan(&total).Error
	if err != nil {
		return 0, err
	}
	if !total.Valid {
		return 0, nil
	}
	if total.Int64 < 0 {
		return 0, ErrStorageQuotaCounterInvalid
	}
	return total.Int64, nil
}

func activeStorageQuotaReservedBytes(ctx context.Context, db *gorm.DB, scope tenant.Scope, now time.Time) (int64, error) {
	var total sql.NullInt64
	err := db.WithContext(normalizeContext(ctx)).
		Model(&database.StorageQuotaReservation{}).
		Select("COALESCE(SUM(bytes), 0)").
		Where("tenant_id = ? AND status = ? AND expires_at > ?", scope.ID(), storageQuotaReservationStatusReserved, now).
		Scan(&total).Error
	if err != nil {
		return 0, err
	}
	if !total.Valid {
		return 0, nil
	}
	if total.Int64 < 0 {
		return 0, ErrStorageQuotaCounterInvalid
	}
	return total.Int64, nil
}

func withStorageQuotaLock(db *gorm.DB) *gorm.DB {
	if db != nil && db.Dialector != nil && db.Dialector.Name() == "sqlite" {
		return db
	}
	return db.Clauses(clause.Locking{Strength: "UPDATE"})
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
