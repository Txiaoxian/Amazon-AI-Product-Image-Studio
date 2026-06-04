package provider

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrAPIKeyRotationFailed    = errors.New("provider api key rotation failed")
	ErrAPIKeyRotationSameKeyID = errors.New("provider api key rotation requires a new key id")
)

type APIKeyRotationService struct {
	db *gorm.DB
}

type APIKeyRotationSummary struct {
	ProviderCount             int
	DeletedProviderEraseCount int
	Applied                   bool
}

func NewAPIKeyRotationService(db *gorm.DB) *APIKeyRotationService {
	return &APIKeyRotationService{db: db}
}

func (s *APIKeyRotationService) Rotate(ctx context.Context, oldCipher APIKeyCipher, newCipher APIKeyCipher, apply bool) (APIKeyRotationSummary, error) {
	if s == nil || s.db == nil {
		return APIKeyRotationSummary{}, database.ErrNilDB
	}
	if oldCipher.aead == nil || newCipher.aead == nil || oldCipher.keyID == "" || newCipher.keyID == "" {
		return APIKeyRotationSummary{}, ErrAPIKeyRotationFailed
	}
	if oldCipher.keyID == newCipher.keyID {
		return APIKeyRotationSummary{}, ErrAPIKeyRotationSameKeyID
	}
	if ctx == nil {
		ctx = context.Background()
	}

	summary := APIKeyRotationSummary{Applied: apply}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var records []database.AIProvider
		query := tx.Unscoped().Model(&database.AIProvider{}).
			Order("tenant_id ASC, id ASC")
		if tx.Dialector.Name() != "sqlite" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.Find(&records).Error; err != nil {
			return ErrAPIKeyRotationFailed
		}

		for _, record := range records {
			if record.DeletedAt.Valid {
				if record.EncryptedAPIKey == "" && record.APIKeyHint == "" && record.APIKeyUpdatedAt == nil {
					continue
				}
				summary.DeletedProviderEraseCount++
				if !apply {
					continue
				}
				result := tx.Unscoped().Model(&database.AIProvider{}).
					Where("tenant_id = ? AND id = ? AND deleted_at IS NOT NULL", record.TenantID, record.ID).
					Updates(map[string]any{
						"encrypted_api_key":  "",
						"api_key_hint":       "",
						"api_key_updated_at": nil,
					})
				if result.Error != nil || result.RowsAffected != 1 {
					return ErrAPIKeyRotationFailed
				}
				continue
			}

			summary.ProviderCount++
			plain, err := oldCipher.Decrypt(record.EncryptedAPIKey)
			if err != nil {
				return ErrAPIKeyRotationFailed
			}
			encrypted, _, err := newCipher.Encrypt(plain)
			if err != nil {
				return ErrAPIKeyRotationFailed
			}
			if !apply {
				continue
			}

			result := tx.Model(&database.AIProvider{}).
				Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", record.TenantID, record.ID).
				UpdateColumn("encrypted_api_key", encrypted)
			if result.Error != nil || result.RowsAffected != 1 {
				return ErrAPIKeyRotationFailed
			}
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelSerializable}); err != nil {
		return APIKeyRotationSummary{}, ErrAPIKeyRotationFailed
	}

	return summary, nil
}
