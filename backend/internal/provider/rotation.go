package provider

import (
	"context"
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
	ProviderCount int
	Applied       bool
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
		query := tx.Model(&database.AIProvider{}).
			Where("deleted_at IS NULL").
			Order("tenant_id ASC, id ASC")
		if tx.Dialector.Name() != "sqlite" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.Find(&records).Error; err != nil {
			return ErrAPIKeyRotationFailed
		}

		summary.ProviderCount = len(records)
		for _, record := range records {
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
	}); err != nil {
		return APIKeyRotationSummary{}, ErrAPIKeyRotationFailed
	}

	return summary, nil
}
