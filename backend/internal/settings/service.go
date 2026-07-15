package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/audit"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	modelpkg "github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/model"
	providerpkg "github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provider"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	minStorageRetentionDays = 1
	maxStorageRetentionDays = 3650
	minLogRetentionDays     = 1
	maxLogRetentionDays     = 3650
	minStorageQuotaBytes    = 1
	maxStorageQuotaBytes    = int64(109951162777600)
)

type Service struct {
	db                     *gorm.DB
	repo                   Repository
	hardCap                config.UploadConfig
	taskConcurrencyDefault TaskConcurrency
	taskConcurrencyHardCap TaskConcurrency
	globalConcurrency      int
	log                    *slog.Logger
	now                    func() time.Time
}

func NewService(db *gorm.DB, log *slog.Logger, uploadConfig config.UploadConfig, queueConfig config.QueueConfig) *Service {
	if log == nil {
		log = slog.Default()
	}
	globalConcurrency := queueConfig.GlobalConcurrency
	if globalConcurrency <= 0 {
		globalConcurrency = maxTaskConcurrencyValue(taskConcurrencyHardCapFromQueueConfig(queueConfig))
	}
	return &Service{
		db:                     db,
		repo:                   NewRepository(db),
		hardCap:                config.NormalizeUploadConfig(uploadConfig),
		taskConcurrencyDefault: taskConcurrencyFromQueueConfig(queueConfig),
		taskConcurrencyHardCap: taskConcurrencyHardCapFromQueueConfig(queueConfig),
		globalConcurrency:      globalConcurrency,
		log:                    log,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *Service) RegisterRoutes(group *gin.RouterGroup) {
	admin := group.Group("/admin")
	admin.GET("/system-settings", s.GetSystemSettings)
	admin.PATCH("/system-settings", s.PatchSystemSettings)
}

func (s *Service) GetSystemSettings(c *gin.Context) {
	principal, ok := s.requireAdminPermission(c)
	if !ok {
		return
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		s.respondError(c, err)
		return
	}

	response, err := s.EffectiveSystemSettings(c.Request.Context(), scope)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) PatchSystemSettings(c *gin.Context) {
	principal, ok := s.requireAdminPermission(c)
	if !ok {
		return
	}
	patch, err := parsePatchRequest(c.Request.Body)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "系统设置请求格式不正确。", nil)
		return
	}

	if patch.UploadPolicy != nil {
		if _, err := s.UpdateUploadPolicy(c.Request.Context(), principal, *patch.UploadPolicy, c.ClientIP(), c.Request.UserAgent()); err != nil {
			s.respondError(c, err)
			return
		}
	}
	if patch.TaskDefaults != nil {
		if _, err := s.UpdateTaskDefaults(c.Request.Context(), principal, *patch.TaskDefaults, c.ClientIP(), c.Request.UserAgent()); err != nil {
			s.respondError(c, err)
			return
		}
	}
	if patch.TaskConcurrency != nil {
		if _, err := s.UpdateTaskConcurrency(c.Request.Context(), principal, *patch.TaskConcurrency, c.ClientIP(), c.Request.UserAgent()); err != nil {
			s.respondError(c, err)
			return
		}
	}
	if patch.StorageRetention != nil {
		if _, err := s.UpdateStorageRetention(c.Request.Context(), principal, *patch.StorageRetention, c.ClientIP(), c.Request.UserAgent()); err != nil {
			s.respondError(c, err)
			return
		}
	}
	if patch.StorageQuota != nil {
		if _, err := s.UpdateStorageQuota(c.Request.Context(), principal, *patch.StorageQuota, c.ClientIP(), c.Request.UserAgent()); err != nil {
			s.respondError(c, err)
			return
		}
	}
	if patch.LogRetention != nil {
		if _, err := s.UpdateLogRetention(c.Request.Context(), principal, *patch.LogRetention, c.ClientIP(), c.Request.UserAgent()); err != nil {
			s.respondError(c, err)
			return
		}
	}

	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		s.respondError(c, err)
		return
	}
	response, err := s.EffectiveSystemSettings(c.Request.Context(), scope)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) EffectiveUploadConfig(ctx context.Context, tenantID string) (config.UploadConfig, error) {
	scope, err := tenant.NewScope(tenantID)
	if err != nil {
		return config.UploadConfig{}, err
	}
	policy, err := s.EffectiveUploadPolicy(ctx, scope)
	if err != nil {
		return config.UploadConfig{}, err
	}
	return uploadConfigFromPolicy(s.hardCap, policy), nil
}

func (s *Service) EffectiveUploadPolicy(ctx context.Context, scope tenant.Scope) (UploadPolicy, error) {
	policy := uploadPolicyFromConfig(s.hardCap)
	record, ok, err := s.repo.FindByKey(ctx, scope, KeyUploadPolicy)
	if err != nil {
		return UploadPolicy{}, err
	}
	if !ok {
		return policy, nil
	}

	stored, err := decodeStoredUploadPolicy(record.ValueJSON)
	if err != nil {
		return UploadPolicy{}, err
	}
	if err := validatePositivePolicy(stored); err != nil {
		return UploadPolicy{}, err
	}
	return clampPolicyToHardCap(stored, s.hardCap), nil
}

func (s *Service) EffectiveSystemSettings(ctx context.Context, scope tenant.Scope) (Response, error) {
	policy, err := s.EffectiveUploadPolicy(ctx, scope)
	if err != nil {
		return Response{}, err
	}
	defaults, err := LoadTaskDefaults(ctx, s.repo, scope)
	if err != nil {
		return Response{}, err
	}
	taskConcurrency, err := LoadTaskConcurrencyWithDefaults(ctx, s.repo, scope, s.taskConcurrencyDefault, s.taskConcurrencyHardCap)
	if err != nil {
		return Response{}, err
	}
	storageRetention, err := LoadStorageRetention(ctx, s.repo, scope)
	if err != nil {
		return Response{}, err
	}
	storageQuota, err := LoadStorageQuotaWithUsage(ctx, s.repo, scope)
	if err != nil {
		return Response{}, err
	}
	logRetention, err := LoadLogRetention(ctx, s.repo, scope)
	if err != nil {
		return Response{}, err
	}
	return Response{
		UploadPolicy:     policy,
		TaskDefaults:     defaults,
		TaskConcurrency:  taskConcurrency,
		StorageRetention: storageRetention,
		StorageQuota:     storageQuota,
		LogRetention:     logRetention,
		Constraints:      systemSettingsConstraints(s.hardCap, s.taskConcurrencyHardCap, s.globalConcurrency),
	}, nil
}

func systemSettingsConstraints(uploadHardCap config.UploadConfig, concurrencyHardCap TaskConcurrency, globalConcurrency int) Constraints {
	uploadHardCap = config.NormalizeUploadConfig(uploadHardCap)
	concurrencyHardCap = normalizeTaskConcurrencyHardCap(concurrencyHardCap)
	positiveRange := func(max int64) IntegerRange {
		return IntegerRange{Min: 1, Max: max}
	}
	return Constraints{
		UploadPolicy: UploadPolicyConstraints{
			MaxFileSizeBytes: positiveRange(uploadHardCap.MaxFileSizeBytes),
			MaxWidth:         positiveRange(int64(uploadHardCap.MaxWidth)),
			MaxHeight:        positiveRange(int64(uploadHardCap.MaxHeight)),
			MaxPixels:        positiveRange(uploadHardCap.MaxPixels),
		},
		TaskConcurrency: TaskConcurrencyConstraints{
			GlobalCapacity: int64(globalConcurrency),
			TenantLimit:    positiveRange(int64(concurrencyHardCap.TenantLimit)),
			UserLimit:      positiveRange(int64(concurrencyHardCap.UserLimit)),
			ProviderLimit:  positiveRange(int64(concurrencyHardCap.ProviderLimit)),
			ModelLimit:     positiveRange(int64(concurrencyHardCap.ModelLimit)),
		},
		StorageRetention: IntegerRange{Min: minStorageRetentionDays, Max: maxStorageRetentionDays},
		StorageQuota:     IntegerRange{Min: minStorageQuotaBytes, Max: maxStorageQuotaBytes},
		LogRetention:     IntegerRange{Min: minLogRetentionDays, Max: maxLogRetentionDays},
	}
}

func LoadTaskDefaults(ctx context.Context, repo Repository, scope tenant.Scope) (TaskDefaults, error) {
	record, ok, err := repo.FindByKey(ctx, scope, KeyTaskDefaults)
	if err != nil {
		return TaskDefaults{}, err
	}
	if !ok {
		return TaskDefaults{}, nil
	}
	defaults, err := decodeStoredTaskDefaults(record.ValueJSON)
	if err != nil {
		return TaskDefaults{}, ErrStoredTaskDefaultsInvalid
	}
	if (defaults.DefaultProviderID == nil) != (defaults.DefaultModelID == nil) {
		return TaskDefaults{}, ErrStoredTaskDefaultsInvalid
	}
	if defaults.DefaultProviderID != nil {
		providerID, err := cleanTaskDefaultID(*defaults.DefaultProviderID)
		if err != nil {
			return TaskDefaults{}, ErrStoredTaskDefaultsInvalid
		}
		modelID, err := cleanTaskDefaultID(*defaults.DefaultModelID)
		if err != nil {
			return TaskDefaults{}, ErrStoredTaskDefaultsInvalid
		}
		defaults.DefaultProviderID = &providerID
		defaults.DefaultModelID = &modelID
		if err := validateTaskDefaults(ctx, repo, scope, providerID, modelID); err != nil {
			return TaskDefaults{}, ErrStoredTaskDefaultsInvalid
		}
	}
	return defaults, nil
}

func LoadTaskConcurrency(ctx context.Context, repo Repository, scope tenant.Scope, hardCap TaskConcurrency) (TaskConcurrency, error) {
	return LoadTaskConcurrencyWithDefaults(ctx, repo, scope, hardCap, hardCap)
}

func LoadTaskConcurrencyWithDefaults(ctx context.Context, repo Repository, scope tenant.Scope, defaults TaskConcurrency, hardCap TaskConcurrency) (TaskConcurrency, error) {
	hardCap = normalizeTaskConcurrencyHardCap(hardCap)
	defaults = clampTaskConcurrencyToHardCap(normalizeTaskConcurrencyHardCap(defaults), hardCap)
	record, ok, err := repo.FindByKey(ctx, scope, KeyTaskConcurrency)
	if err != nil {
		return TaskConcurrency{}, err
	}
	if !ok {
		return defaults, nil
	}
	policy, err := decodeStoredTaskConcurrency(record.ValueJSON)
	if err != nil {
		return TaskConcurrency{}, ErrStoredTaskConcurrencyInvalid
	}
	if err := validateTaskConcurrencyWithinHardCap(policy, hardCap); err != nil {
		return TaskConcurrency{}, ErrStoredTaskConcurrencyInvalid
	}
	return policy, nil
}

func LoadStorageRetention(ctx context.Context, repo Repository, scope tenant.Scope) (StorageRetention, error) {
	record, ok, err := repo.FindByKey(ctx, scope, KeyStorageRetention)
	if err != nil {
		return StorageRetention{}, err
	}
	if !ok {
		return StorageRetention{}, nil
	}
	retention, err := decodeStoredStorageRetention(record.ValueJSON)
	if err != nil {
		return StorageRetention{}, ErrStoredStorageRetentionInvalid
	}
	if err := validateStorageRetention(retention); err != nil {
		return StorageRetention{}, ErrStoredStorageRetentionInvalid
	}
	return retention, nil
}

func LoadStorageQuota(ctx context.Context, repo Repository, scope tenant.Scope) (StorageQuota, error) {
	record, ok, err := repo.FindByKey(ctx, scope, KeyStorageQuota)
	if err != nil {
		return StorageQuota{}, err
	}
	if !ok {
		return StorageQuota{}, nil
	}
	quota, err := decodeStoredStorageQuota(record.ValueJSON)
	if err != nil {
		return StorageQuota{}, ErrStoredStorageQuotaInvalid
	}
	if err := validateStorageQuota(quota); err != nil {
		return StorageQuota{}, ErrStoredStorageQuotaInvalid
	}
	return quota, nil
}

func LoadLogRetention(ctx context.Context, repo Repository, scope tenant.Scope) (LogRetention, error) {
	record, ok, err := repo.FindByKey(ctx, scope, KeyLogRetention)
	if err != nil {
		return LogRetention{}, err
	}
	if !ok {
		return LogRetention{}, nil
	}
	retention, err := decodeStoredLogRetention(record.ValueJSON)
	if err != nil {
		return LogRetention{}, ErrStoredLogRetentionInvalid
	}
	if err := validateLogRetention(retention); err != nil {
		return LogRetention{}, ErrStoredLogRetentionInvalid
	}
	return retention, nil
}

func LoadStorageQuotaWithUsage(ctx context.Context, repo Repository, scope tenant.Scope) (StorageQuota, error) {
	quota, err := LoadStorageQuota(ctx, repo, scope)
	if err != nil {
		return StorageQuota{}, err
	}
	usedBytes, err := repo.StorageUsedBytes(ctx, scope)
	if err != nil {
		return StorageQuota{}, err
	}
	quota.UsedBytes = usedBytes
	return quota, nil
}

func CheckStorageQuota(ctx context.Context, repo Repository, scope tenant.Scope, pendingBytes int64) error {
	if pendingBytes < 0 {
		return ErrValidation
	}
	reservation, err := ReserveStorageQuota(ctx, repo, scope, pendingBytes)
	if err != nil {
		return err
	}
	return ReleaseStorageQuotaReservation(ctx, repo, scope, reservation)
}

func (s *Service) CheckStorageQuota(ctx context.Context, tenantID string, pendingBytes int64) error {
	scope, err := tenant.NewScope(tenantID)
	if err != nil {
		return err
	}
	return CheckStorageQuota(ctx, s.repo, scope, pendingBytes)
}

func (s *Service) ReserveStorageQuota(ctx context.Context, tenantID string, pendingBytes int64) (StorageQuotaReservation, error) {
	scope, err := tenant.NewScope(tenantID)
	if err != nil {
		return StorageQuotaReservation{}, err
	}
	return ReserveStorageQuota(ctx, s.repo, scope, pendingBytes)
}

func (s *Service) FinalizeStorageQuotaReservation(ctx context.Context, tenantID string, reservation StorageQuotaReservation, finalizedBytes int64) error {
	scope, err := tenant.NewScope(tenantID)
	if err != nil {
		return err
	}
	return FinalizeStorageQuotaReservation(ctx, s.repo, scope, reservation, finalizedBytes)
}

func (s *Service) ReleaseStorageQuotaReservation(ctx context.Context, tenantID string, reservation StorageQuotaReservation) error {
	scope, err := tenant.NewScope(tenantID)
	if err != nil {
		return err
	}
	return ReleaseStorageQuotaReservation(ctx, s.repo, scope, reservation)
}

func LoadEnabledStorageRetentions(ctx context.Context, repo Repository) ([]EnabledStorageRetention, []InvalidStorageRetention, error) {
	records, err := repo.ListByKeyForActiveTenants(ctx, KeyStorageRetention)
	if err != nil {
		return nil, nil, err
	}

	enabled := make([]EnabledStorageRetention, 0, len(records))
	invalid := make([]InvalidStorageRetention, 0)
	for _, record := range records {
		retention, err := decodeStoredStorageRetention(record.ValueJSON)
		if err != nil {
			invalid = append(invalid, InvalidStorageRetention{TenantID: record.TenantID, Err: ErrStoredStorageRetentionInvalid})
			continue
		}
		if err := validateStorageRetention(retention); err != nil {
			invalid = append(invalid, InvalidStorageRetention{TenantID: record.TenantID, Err: ErrStoredStorageRetentionInvalid})
			continue
		}
		if retention.DeletedAssetRetentionDays == nil {
			continue
		}
		enabled = append(enabled, EnabledStorageRetention{
			TenantID:                  record.TenantID,
			DeletedAssetRetentionDays: *retention.DeletedAssetRetentionDays,
		})
	}
	return enabled, invalid, nil
}

func LoadEnabledLogRetentions(ctx context.Context, repo Repository) ([]EnabledLogRetention, []InvalidLogRetention, error) {
	records, err := repo.ListByKeyForActiveTenants(ctx, KeyLogRetention)
	if err != nil {
		return nil, nil, err
	}

	enabled := make([]EnabledLogRetention, 0, len(records))
	invalid := make([]InvalidLogRetention, 0)
	for _, record := range records {
		retention, err := decodeStoredLogRetention(record.ValueJSON)
		if err != nil {
			invalid = append(invalid, InvalidLogRetention{TenantID: record.TenantID, Err: ErrStoredLogRetentionInvalid})
			continue
		}
		if err := validateLogRetention(retention); err != nil {
			invalid = append(invalid, InvalidLogRetention{TenantID: record.TenantID, Err: ErrStoredLogRetentionInvalid})
			continue
		}
		if retention.OperationLogRetentionDays == nil && retention.APICallLogRetentionDays == nil && retention.TaskEventRetentionDays == nil {
			continue
		}
		enabled = append(enabled, EnabledLogRetention{
			TenantID:                  record.TenantID,
			OperationLogRetentionDays: retention.OperationLogRetentionDays,
			APICallLogRetentionDays:   retention.APICallLogRetentionDays,
			TaskEventRetentionDays:    retention.TaskEventRetentionDays,
		})
	}
	return enabled, invalid, nil
}

func (s *Service) UpdateUploadPolicy(ctx context.Context, principal auth.Principal, patch UploadPolicyPatch, ip string, userAgent string) (UploadPolicy, error) {
	if s.db == nil {
		return UploadPolicy{}, database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return UploadPolicy{}, err
	}

	current, err := s.EffectiveUploadPolicy(ctx, scope)
	if err != nil {
		return UploadPolicy{}, err
	}
	updated, changedFields, err := applyUploadPolicyPatch(current, patch, s.hardCap)
	if err != nil {
		return UploadPolicy{}, err
	}
	if len(changedFields) == 0 {
		return UploadPolicy{}, ErrValidation
	}

	valueJSON, err := json.Marshal(updated)
	if err != nil {
		return UploadPolicy{}, err
	}
	now := s.now()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.withDB(tx)
		if err := repo.Upsert(ctx, scope, KeyUploadPolicy, string(valueJSON), now); err != nil {
			return err
		}
		sort.Strings(changedFields)
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       ActionUpdateSystemSettings,
			ResourceType: "system_settings",
			ResourceID:   KeyUploadPolicy,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"key":           KeyUploadPolicy,
				"changedFields": changedFields,
				"uploadPolicy": map[string]any{
					"maxFileSizeBytes": updated.MaxFileSizeBytes,
					"maxWidth":         updated.MaxWidth,
					"maxHeight":        updated.MaxHeight,
					"maxPixels":        updated.MaxPixels,
				},
			},
		})
	})
	if err != nil {
		return UploadPolicy{}, err
	}
	return updated, nil
}

func (s *Service) UpdateTaskDefaults(ctx context.Context, principal auth.Principal, patch TaskDefaultsPatch, ip string, userAgent string) (TaskDefaults, error) {
	if s.db == nil {
		return TaskDefaults{}, database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return TaskDefaults{}, err
	}

	updated, err := normalizeTaskDefaultsPatch(patch)
	if err != nil {
		return TaskDefaults{}, err
	}
	valueJSON, err := json.Marshal(updated)
	if err != nil {
		return TaskDefaults{}, err
	}

	now := s.now()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.withDB(tx)
		if updated.DefaultProviderID != nil {
			if err := validateTaskDefaultsForUpdate(ctx, repo, scope, *updated.DefaultProviderID, *updated.DefaultModelID); err != nil {
				return err
			}
		}
		if err := repo.Upsert(ctx, scope, KeyTaskDefaults, string(valueJSON), now); err != nil {
			return err
		}
		changedFields := []string{"taskDefaults.defaultModelId", "taskDefaults.defaultProviderId"}
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       ActionUpdateSystemSettings,
			ResourceType: "system_settings",
			ResourceID:   KeyTaskDefaults,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"key":           KeyTaskDefaults,
				"changedFields": changedFields,
				"taskDefaults": map[string]any{
					"defaultProviderId": updated.DefaultProviderID,
					"defaultModelId":    updated.DefaultModelID,
				},
			},
		})
	})
	if err != nil {
		return TaskDefaults{}, err
	}
	return updated, nil
}

func (s *Service) UpdateTaskConcurrency(ctx context.Context, principal auth.Principal, patch TaskConcurrencyPatch, ip string, userAgent string) (TaskConcurrency, error) {
	if s.db == nil {
		return TaskConcurrency{}, database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return TaskConcurrency{}, err
	}
	current, err := LoadTaskConcurrencyWithDefaults(ctx, s.repo, scope, s.taskConcurrencyDefault, s.taskConcurrencyHardCap)
	if err != nil {
		return TaskConcurrency{}, err
	}
	updated, changedFields, err := applyTaskConcurrencyPatch(current, patch, s.taskConcurrencyHardCap)
	if err != nil {
		return TaskConcurrency{}, err
	}
	if len(changedFields) == 0 {
		return TaskConcurrency{}, ErrValidation
	}
	valueJSON, err := json.Marshal(updated)
	if err != nil {
		return TaskConcurrency{}, err
	}

	now := s.now()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.withDB(tx)
		if err := repo.Upsert(ctx, scope, KeyTaskConcurrency, string(valueJSON), now); err != nil {
			return err
		}
		sort.Strings(changedFields)
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       ActionUpdateSystemSettings,
			ResourceType: "system_settings",
			ResourceID:   KeyTaskConcurrency,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"key":           KeyTaskConcurrency,
				"changedFields": changedFields,
				"taskConcurrency": map[string]any{
					"tenantLimit":   updated.TenantLimit,
					"userLimit":     updated.UserLimit,
					"providerLimit": updated.ProviderLimit,
					"modelLimit":    updated.ModelLimit,
				},
			},
		})
	})
	if err != nil {
		return TaskConcurrency{}, err
	}
	return updated, nil
}

func (s *Service) UpdateStorageRetention(ctx context.Context, principal auth.Principal, patch StorageRetentionPatch, ip string, userAgent string) (StorageRetention, error) {
	if s.db == nil {
		return StorageRetention{}, database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return StorageRetention{}, err
	}
	current, err := LoadStorageRetention(ctx, s.repo, scope)
	if err != nil {
		return StorageRetention{}, err
	}
	updated, changedFields, err := applyStorageRetentionPatch(current, patch)
	if err != nil {
		return StorageRetention{}, err
	}
	if len(changedFields) == 0 {
		return StorageRetention{}, ErrValidation
	}
	valueJSON, err := json.Marshal(updated)
	if err != nil {
		return StorageRetention{}, err
	}

	now := s.now()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.withDB(tx)
		if err := repo.Upsert(ctx, scope, KeyStorageRetention, string(valueJSON), now); err != nil {
			return err
		}
		sort.Strings(changedFields)
		retentionMetadata := map[string]any{
			"deletedAssetRetentionDays": updated.DeletedAssetRetentionDays,
		}
		if updated.DeletedAssetRetentionDays == nil {
			retentionMetadata["status"] = "cleared"
		}
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       ActionUpdateSystemSettings,
			ResourceType: "system_settings",
			ResourceID:   KeyStorageRetention,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"key":              KeyStorageRetention,
				"changedFields":    changedFields,
				"storageRetention": retentionMetadata,
			},
		})
	})
	if err != nil {
		return StorageRetention{}, err
	}
	return updated, nil
}

func (s *Service) UpdateStorageQuota(ctx context.Context, principal auth.Principal, patch StorageQuotaPatch, ip string, userAgent string) (StorageQuota, error) {
	if s.db == nil {
		return StorageQuota{}, database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return StorageQuota{}, err
	}
	current, err := LoadStorageQuota(ctx, s.repo, scope)
	if err != nil {
		return StorageQuota{}, err
	}
	updated, changedFields, err := applyStorageQuotaPatch(current, patch)
	if err != nil {
		return StorageQuota{}, err
	}
	if len(changedFields) == 0 {
		return StorageQuota{}, ErrValidation
	}
	valueJSON, err := json.Marshal(struct {
		MaxBytes *int64 `json:"maxBytes"`
	}{MaxBytes: updated.MaxBytes})
	if err != nil {
		return StorageQuota{}, err
	}
	usedBytes, err := s.repo.StorageUsedBytes(ctx, scope)
	if err != nil {
		return StorageQuota{}, err
	}
	updated.UsedBytes = usedBytes

	now := s.now()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.withDB(tx)
		if err := repo.Upsert(ctx, scope, KeyStorageQuota, string(valueJSON), now); err != nil {
			return err
		}
		sort.Strings(changedFields)
		quotaMetadata := map[string]any{
			"maxBytes":  updated.MaxBytes,
			"usedBytes": usedBytes,
		}
		if updated.MaxBytes == nil {
			quotaMetadata["status"] = "cleared"
		}
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       ActionUpdateSystemSettings,
			ResourceType: "system_settings",
			ResourceID:   KeyStorageQuota,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"key":           KeyStorageQuota,
				"changedFields": changedFields,
				"storageQuota":  quotaMetadata,
			},
		})
	})
	if err != nil {
		return StorageQuota{}, err
	}
	return updated, nil
}

func (s *Service) UpdateLogRetention(ctx context.Context, principal auth.Principal, patch LogRetentionPatch, ip string, userAgent string) (LogRetention, error) {
	if s.db == nil {
		return LogRetention{}, database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return LogRetention{}, err
	}
	current, err := LoadLogRetention(ctx, s.repo, scope)
	if err != nil {
		return LogRetention{}, err
	}
	updated, changedFields, err := applyLogRetentionPatch(current, patch)
	if err != nil {
		return LogRetention{}, err
	}
	if len(changedFields) == 0 {
		return LogRetention{}, ErrValidation
	}
	valueJSON, err := json.Marshal(updated)
	if err != nil {
		return LogRetention{}, err
	}

	now := s.now()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.withDB(tx)
		if err := repo.Upsert(ctx, scope, KeyLogRetention, string(valueJSON), now); err != nil {
			return err
		}
		sort.Strings(changedFields)
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       ActionUpdateSystemSettings,
			ResourceType: "system_settings",
			ResourceID:   KeyLogRetention,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"key":           KeyLogRetention,
				"changedFields": changedFields,
				"logRetention": map[string]any{
					"operationLogRetentionDays": updated.OperationLogRetentionDays,
					"apiCallLogRetentionDays":   updated.APICallLogRetentionDays,
					"taskEventRetentionDays":    updated.TaskEventRetentionDays,
				},
			},
		})
	})
	if err != nil {
		return LogRetention{}, err
	}
	return updated, nil
}

func (s *Service) requireAdminPermission(c *gin.Context) (auth.Principal, bool) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return auth.Principal{}, false
	}
	if !isTenantAdmin(principal) || !principal.HasPermission(PermissionManage) {
		httpx.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Forbidden.", nil)
		return auth.Principal{}, false
	}
	return principal, true
}

func (s *Service) respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrValidation):
		var validationErr *ValidationError
		if errors.As(err, &validationErr) {
			httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", validationErr.Message, validationErr.Details())
			return
		}
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "系统设置内容不符合要求，请检查后重试。", nil)
	case errors.Is(err, ErrForbidden):
		httpx.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Forbidden.", nil)
	default:
		s.log.Error("system settings request failed", slog.String("request_id", httpx.RequestIDFromContext(c)))
		httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
	}
}

func parsePatchRequest(body io.Reader) (PatchRequest, error) {
	var root map[string]json.RawMessage
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&root); err != nil {
		return PatchRequest{}, ErrValidation
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return PatchRequest{}, ErrValidation
	}
	if len(root) != 1 {
		return PatchRequest{}, ErrValidation
	}

	if rawPolicy, ok := root["uploadPolicy"]; ok {
		patch, err := parseUploadPolicyPatch(rawPolicy)
		if err != nil {
			return PatchRequest{}, err
		}
		return PatchRequest{UploadPolicy: &patch}, nil
	}
	if rawDefaults, ok := root["taskDefaults"]; ok {
		patch, err := parseTaskDefaultsPatch(rawDefaults)
		if err != nil {
			return PatchRequest{}, err
		}
		return PatchRequest{TaskDefaults: &patch}, nil
	}
	if rawConcurrency, ok := root["taskConcurrency"]; ok {
		patch, err := parseTaskConcurrencyPatch(rawConcurrency)
		if err != nil {
			return PatchRequest{}, err
		}
		return PatchRequest{TaskConcurrency: &patch}, nil
	}
	if rawRetention, ok := root["storageRetention"]; ok {
		patch, err := parseStorageRetentionPatch(rawRetention)
		if err != nil {
			return PatchRequest{}, err
		}
		return PatchRequest{StorageRetention: &patch}, nil
	}
	if rawQuota, ok := root["storageQuota"]; ok {
		patch, err := parseStorageQuotaPatch(rawQuota)
		if err != nil {
			return PatchRequest{}, err
		}
		return PatchRequest{StorageQuota: &patch}, nil
	}
	if rawRetention, ok := root["logRetention"]; ok {
		patch, err := parseLogRetentionPatch(rawRetention)
		if err != nil {
			return PatchRequest{}, err
		}
		return PatchRequest{LogRetention: &patch}, nil
	}
	return PatchRequest{}, ErrValidation
}

func parseUploadPolicyPatch(raw json.RawMessage) (UploadPolicyPatch, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return UploadPolicyPatch{}, ErrValidation
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return UploadPolicyPatch{}, ErrValidation
	}
	if len(fields) == 0 {
		return UploadPolicyPatch{}, ErrValidation
	}

	var patch UploadPolicyPatch
	for key, value := range fields {
		parsed, err := decodeJSONInt64(value)
		if err != nil {
			return UploadPolicyPatch{}, err
		}
		switch key {
		case "maxFileSizeBytes":
			patch.MaxFileSizeBytes = &parsed
		case "maxWidth":
			patch.MaxWidth = &parsed
		case "maxHeight":
			patch.MaxHeight = &parsed
		case "maxPixels":
			patch.MaxPixels = &parsed
		default:
			return UploadPolicyPatch{}, ErrValidation
		}
	}
	return patch, nil
}

func parseTaskDefaultsPatch(raw json.RawMessage) (TaskDefaultsPatch, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return TaskDefaultsPatch{}, ErrValidation
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return TaskDefaultsPatch{}, ErrValidation
	}
	if len(fields) != 2 {
		return TaskDefaultsPatch{}, ErrValidation
	}
	rawProviderID, hasProviderID := fields["defaultProviderId"]
	rawModelID, hasModelID := fields["defaultModelId"]
	if !hasProviderID || !hasModelID {
		return TaskDefaultsPatch{}, ErrValidation
	}
	for key := range fields {
		if key != "defaultProviderId" && key != "defaultModelId" {
			return TaskDefaultsPatch{}, ErrValidation
		}
	}

	providerID, providerNull, err := decodeNullableTaskDefaultID(rawProviderID)
	if err != nil {
		return TaskDefaultsPatch{}, err
	}
	modelID, modelNull, err := decodeNullableTaskDefaultID(rawModelID)
	if err != nil {
		return TaskDefaultsPatch{}, err
	}
	if providerNull != modelNull {
		return TaskDefaultsPatch{}, ErrValidation
	}
	if providerNull {
		return TaskDefaultsPatch{}, nil
	}
	return TaskDefaultsPatch{DefaultProviderID: &providerID, DefaultModelID: &modelID}, nil
}

func parseTaskConcurrencyPatch(raw json.RawMessage) (TaskConcurrencyPatch, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return TaskConcurrencyPatch{}, ErrValidation
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return TaskConcurrencyPatch{}, ErrValidation
	}
	if len(fields) == 0 {
		return TaskConcurrencyPatch{}, ErrValidation
	}
	var patch TaskConcurrencyPatch
	for key, value := range fields {
		parsed, err := decodeJSONInt64(value)
		if err != nil {
			return TaskConcurrencyPatch{}, err
		}
		switch key {
		case "tenantLimit":
			patch.TenantLimit = &parsed
		case "userLimit":
			patch.UserLimit = &parsed
		case "providerLimit":
			patch.ProviderLimit = &parsed
		case "modelLimit":
			patch.ModelLimit = &parsed
		default:
			return TaskConcurrencyPatch{}, ErrValidation
		}
	}
	return patch, nil
}

func parseStorageRetentionPatch(raw json.RawMessage) (StorageRetentionPatch, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return StorageRetentionPatch{}, ErrValidation
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return StorageRetentionPatch{}, ErrValidation
	}
	if len(fields) == 0 {
		return StorageRetentionPatch{}, ErrValidation
	}
	rawDays, ok := fields["deletedAssetRetentionDays"]
	if !ok || len(fields) != 1 {
		return StorageRetentionPatch{}, ErrValidation
	}
	if bytes.Equal(bytes.TrimSpace(rawDays), []byte("null")) {
		return StorageRetentionPatch{
			ClearDeletedAssetRetentionDays: true,
			HasDeletedAssetRetentionDays:   true,
		}, nil
	}
	parsed, err := decodeJSONInt64(rawDays)
	if err != nil {
		return StorageRetentionPatch{}, err
	}
	return StorageRetentionPatch{
		DeletedAssetRetentionDays:    &parsed,
		HasDeletedAssetRetentionDays: true,
	}, nil
}

func parseStorageQuotaPatch(raw json.RawMessage) (StorageQuotaPatch, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return StorageQuotaPatch{}, ErrValidation
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return StorageQuotaPatch{}, ErrValidation
	}
	if len(fields) == 0 {
		return StorageQuotaPatch{}, ErrValidation
	}
	rawMaxBytes, ok := fields["maxBytes"]
	if !ok || len(fields) != 1 {
		return StorageQuotaPatch{}, ErrValidation
	}
	if bytes.Equal(bytes.TrimSpace(rawMaxBytes), []byte("null")) {
		return StorageQuotaPatch{
			ClearMaxBytes: true,
			HasMaxBytes:   true,
		}, nil
	}
	parsed, err := decodeJSONInt64(rawMaxBytes)
	if err != nil {
		return StorageQuotaPatch{}, err
	}
	return StorageQuotaPatch{
		MaxBytes:    &parsed,
		HasMaxBytes: true,
	}, nil
}

func parseLogRetentionPatch(raw json.RawMessage) (LogRetentionPatch, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return LogRetentionPatch{}, ErrValidation
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return LogRetentionPatch{}, ErrValidation
	}
	if len(fields) == 0 {
		return LogRetentionPatch{}, ErrValidation
	}
	var patch LogRetentionPatch
	for key, value := range fields {
		isNull := bytes.Equal(bytes.TrimSpace(value), []byte("null"))
		var parsed int64
		var err error
		if !isNull {
			parsed, err = decodeJSONInt64(value)
			if err != nil {
				return LogRetentionPatch{}, err
			}
		}
		switch key {
		case "operationLogRetentionDays":
			patch.HasOperationLogRetentionDays = true
			if isNull {
				patch.ClearOperationLogRetentionDays = true
			} else {
				patch.OperationLogRetentionDays = &parsed
			}
		case "apiCallLogRetentionDays":
			patch.HasAPICallLogRetentionDays = true
			if isNull {
				patch.ClearAPICallLogRetentionDays = true
			} else {
				patch.APICallLogRetentionDays = &parsed
			}
		case "taskEventRetentionDays":
			patch.HasTaskEventRetentionDays = true
			if isNull {
				patch.ClearTaskEventRetentionDays = true
			} else {
				patch.TaskEventRetentionDays = &parsed
			}
		default:
			return LogRetentionPatch{}, ErrValidation
		}
	}
	return patch, nil
}

func decodeJSONInt64(raw json.RawMessage) (int64, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return 0, ErrValidation
	}
	var value int64
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&value); err != nil {
		return 0, ErrValidation
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return 0, ErrValidation
	}
	return value, nil
}

func decodeStoredUploadPolicy(valueJSON string) (UploadPolicy, error) {
	var policy UploadPolicy
	decoder := json.NewDecoder(strings.NewReader(valueJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		return UploadPolicy{}, ErrValidation
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return UploadPolicy{}, ErrValidation
	}
	return policy, nil
}

func decodeStoredTaskDefaults(valueJSON string) (TaskDefaults, error) {
	var defaults TaskDefaults
	decoder := json.NewDecoder(strings.NewReader(valueJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&defaults); err != nil {
		return TaskDefaults{}, ErrValidation
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return TaskDefaults{}, ErrValidation
	}
	return defaults, nil
}

func decodeStoredTaskConcurrency(valueJSON string) (TaskConcurrency, error) {
	var policy TaskConcurrency
	decoder := json.NewDecoder(strings.NewReader(valueJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		return TaskConcurrency{}, ErrValidation
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return TaskConcurrency{}, ErrValidation
	}
	return policy, nil
}

func decodeStoredStorageRetention(valueJSON string) (StorageRetention, error) {
	var retention StorageRetention
	decoder := json.NewDecoder(strings.NewReader(valueJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&retention); err != nil {
		return StorageRetention{}, ErrValidation
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return StorageRetention{}, ErrValidation
	}
	return retention, nil
}

func decodeStoredStorageQuota(valueJSON string) (StorageQuota, error) {
	var stored struct {
		MaxBytes *int64 `json:"maxBytes"`
	}
	decoder := json.NewDecoder(strings.NewReader(valueJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&stored); err != nil {
		return StorageQuota{}, ErrValidation
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return StorageQuota{}, ErrValidation
	}
	return StorageQuota{MaxBytes: stored.MaxBytes}, nil
}

func decodeStoredLogRetention(valueJSON string) (LogRetention, error) {
	var retention LogRetention
	decoder := json.NewDecoder(strings.NewReader(valueJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&retention); err != nil {
		return LogRetention{}, ErrValidation
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return LogRetention{}, ErrValidation
	}
	return retention, nil
}

func applyUploadPolicyPatch(current UploadPolicy, patch UploadPolicyPatch, hardCap config.UploadConfig) (UploadPolicy, []string, error) {
	hardCap = config.NormalizeUploadConfig(hardCap)
	updated := current
	changedFields := make([]string, 0, 4)

	if patch.MaxFileSizeBytes != nil {
		updated.MaxFileSizeBytes = *patch.MaxFileSizeBytes
		changedFields = append(changedFields, "uploadPolicy.maxFileSizeBytes")
	}
	if patch.MaxWidth != nil {
		if *patch.MaxWidth > maxIntValue() {
			return UploadPolicy{}, nil, ErrValidation
		}
		updated.MaxWidth = int(*patch.MaxWidth)
		changedFields = append(changedFields, "uploadPolicy.maxWidth")
	}
	if patch.MaxHeight != nil {
		if *patch.MaxHeight > maxIntValue() {
			return UploadPolicy{}, nil, ErrValidation
		}
		updated.MaxHeight = int(*patch.MaxHeight)
		changedFields = append(changedFields, "uploadPolicy.maxHeight")
	}
	if patch.MaxPixels != nil {
		updated.MaxPixels = *patch.MaxPixels
		changedFields = append(changedFields, "uploadPolicy.maxPixels")
	}

	if err := validatePolicyWithinHardCap(updated, hardCap); err != nil {
		return UploadPolicy{}, nil, err
	}
	return updated, changedFields, nil
}

func validatePolicyWithinHardCap(policy UploadPolicy, hardCap config.UploadConfig) error {
	hardCap = config.NormalizeUploadConfig(hardCap)
	if err := validatePositivePolicy(policy); err != nil {
		return err
	}
	if policy.MaxFileSizeBytes > hardCap.MaxFileSizeBytes {
		return newRangeValidationError("uploadPolicy.maxFileSizeBytes", "最大文件大小", 1, hardCap.MaxFileSizeBytes)
	}
	if policy.MaxWidth > hardCap.MaxWidth {
		return newRangeValidationError("uploadPolicy.maxWidth", "最大宽度", 1, int64(hardCap.MaxWidth))
	}
	if policy.MaxHeight > hardCap.MaxHeight {
		return newRangeValidationError("uploadPolicy.maxHeight", "最大高度", 1, int64(hardCap.MaxHeight))
	}
	if policy.MaxPixels > hardCap.MaxPixels {
		return newRangeValidationError("uploadPolicy.maxPixels", "最大像素数", 1, hardCap.MaxPixels)
	}
	return nil
}

func validatePositivePolicy(policy UploadPolicy) error {
	if policy.MaxFileSizeBytes <= 0 {
		return newMinimumValidationError("uploadPolicy.maxFileSizeBytes", "最大文件大小", 1)
	}
	if policy.MaxWidth <= 0 {
		return newMinimumValidationError("uploadPolicy.maxWidth", "最大宽度", 1)
	}
	if policy.MaxHeight <= 0 {
		return newMinimumValidationError("uploadPolicy.maxHeight", "最大高度", 1)
	}
	if policy.MaxPixels <= 0 {
		return newMinimumValidationError("uploadPolicy.maxPixels", "最大像素数", 1)
	}
	return nil
}

func applyTaskConcurrencyPatch(current TaskConcurrency, patch TaskConcurrencyPatch, hardCap TaskConcurrency) (TaskConcurrency, []string, error) {
	updated := current
	changedFields := make([]string, 0, 4)
	if patch.TenantLimit != nil {
		if *patch.TenantLimit > maxIntValue() {
			return TaskConcurrency{}, nil, ErrValidation
		}
		updated.TenantLimit = int(*patch.TenantLimit)
		changedFields = append(changedFields, "taskConcurrency.tenantLimit")
	}
	if patch.UserLimit != nil {
		if *patch.UserLimit > maxIntValue() {
			return TaskConcurrency{}, nil, ErrValidation
		}
		updated.UserLimit = int(*patch.UserLimit)
		changedFields = append(changedFields, "taskConcurrency.userLimit")
	}
	if patch.ProviderLimit != nil {
		if *patch.ProviderLimit > maxIntValue() {
			return TaskConcurrency{}, nil, ErrValidation
		}
		updated.ProviderLimit = int(*patch.ProviderLimit)
		changedFields = append(changedFields, "taskConcurrency.providerLimit")
	}
	if patch.ModelLimit != nil {
		if *patch.ModelLimit > maxIntValue() {
			return TaskConcurrency{}, nil, ErrValidation
		}
		updated.ModelLimit = int(*patch.ModelLimit)
		changedFields = append(changedFields, "taskConcurrency.modelLimit")
	}
	if err := validateTaskConcurrencyWithinHardCap(updated, hardCap); err != nil {
		return TaskConcurrency{}, nil, err
	}
	return updated, changedFields, nil
}

func validateTaskConcurrencyWithinHardCap(policy TaskConcurrency, hardCap TaskConcurrency) error {
	hardCap = normalizeTaskConcurrencyHardCap(hardCap)
	if policy.TenantLimit <= 0 {
		return newRangeValidationError("taskConcurrency.tenantLimit", "租户并发上限", 1, int64(hardCap.TenantLimit))
	}
	if policy.UserLimit <= 0 {
		return newRangeValidationError("taskConcurrency.userLimit", "用户并发上限", 1, int64(hardCap.UserLimit))
	}
	if policy.ProviderLimit <= 0 {
		return newRangeValidationError("taskConcurrency.providerLimit", "Provider 并发上限", 1, int64(hardCap.ProviderLimit))
	}
	if policy.ModelLimit <= 0 {
		return newRangeValidationError("taskConcurrency.modelLimit", "模型并发上限", 1, int64(hardCap.ModelLimit))
	}
	if policy.TenantLimit > hardCap.TenantLimit {
		return newRangeValidationError("taskConcurrency.tenantLimit", "租户并发上限", 1, int64(hardCap.TenantLimit))
	}
	if policy.UserLimit > hardCap.UserLimit {
		return newRangeValidationError("taskConcurrency.userLimit", "用户并发上限", 1, int64(hardCap.UserLimit))
	}
	if policy.ProviderLimit > hardCap.ProviderLimit {
		return newRangeValidationError("taskConcurrency.providerLimit", "Provider 并发上限", 1, int64(hardCap.ProviderLimit))
	}
	if policy.ModelLimit > hardCap.ModelLimit {
		return newRangeValidationError("taskConcurrency.modelLimit", "模型并发上限", 1, int64(hardCap.ModelLimit))
	}
	return nil
}

func applyStorageRetentionPatch(current StorageRetention, patch StorageRetentionPatch) (StorageRetention, []string, error) {
	if !patch.HasDeletedAssetRetentionDays {
		return StorageRetention{}, nil, ErrValidation
	}
	updated := current
	changedFields := []string{"storageRetention.deletedAssetRetentionDays"}
	if patch.ClearDeletedAssetRetentionDays {
		updated.DeletedAssetRetentionDays = nil
		return updated, changedFields, nil
	}
	if patch.DeletedAssetRetentionDays == nil || *patch.DeletedAssetRetentionDays > maxIntValue() {
		return StorageRetention{}, nil, ErrValidation
	}
	days := int(*patch.DeletedAssetRetentionDays)
	updated.DeletedAssetRetentionDays = &days
	if err := validateStorageRetention(updated); err != nil {
		return StorageRetention{}, nil, err
	}
	return updated, changedFields, nil
}

func validateStorageRetention(retention StorageRetention) error {
	if retention.DeletedAssetRetentionDays == nil {
		return nil
	}
	if *retention.DeletedAssetRetentionDays < minStorageRetentionDays || *retention.DeletedAssetRetentionDays > maxStorageRetentionDays {
		return newRangeValidationError("storageRetention.deletedAssetRetentionDays", "删除资产保留天数", minStorageRetentionDays, maxStorageRetentionDays)
	}
	return nil
}

func applyStorageQuotaPatch(current StorageQuota, patch StorageQuotaPatch) (StorageQuota, []string, error) {
	if !patch.HasMaxBytes {
		return StorageQuota{}, nil, ErrValidation
	}
	updated := current
	changedFields := []string{"storageQuota.maxBytes"}
	if patch.ClearMaxBytes {
		updated.MaxBytes = nil
		updated.UsedBytes = 0
		return updated, changedFields, nil
	}
	if patch.MaxBytes == nil {
		return StorageQuota{}, nil, ErrValidation
	}
	updated.MaxBytes = patch.MaxBytes
	if err := validateStorageQuota(updated); err != nil {
		return StorageQuota{}, nil, err
	}
	return updated, changedFields, nil
}

func validateStorageQuota(quota StorageQuota) error {
	if quota.MaxBytes == nil {
		return nil
	}
	if *quota.MaxBytes < minStorageQuotaBytes || *quota.MaxBytes > maxStorageQuotaBytes {
		return newRangeValidationError("storageQuota.maxBytes", "最大存储容量", minStorageQuotaBytes, maxStorageQuotaBytes)
	}
	return nil
}

func applyLogRetentionPatch(current LogRetention, patch LogRetentionPatch) (LogRetention, []string, error) {
	updated := current
	changedFields := make([]string, 0, 3)
	if patch.HasOperationLogRetentionDays {
		changedFields = append(changedFields, "logRetention.operationLogRetentionDays")
		if patch.ClearOperationLogRetentionDays {
			updated.OperationLogRetentionDays = nil
		} else {
			if patch.OperationLogRetentionDays == nil || *patch.OperationLogRetentionDays > maxIntValue() {
				return LogRetention{}, nil, ErrValidation
			}
			days := int(*patch.OperationLogRetentionDays)
			updated.OperationLogRetentionDays = &days
		}
	}
	if patch.HasAPICallLogRetentionDays {
		changedFields = append(changedFields, "logRetention.apiCallLogRetentionDays")
		if patch.ClearAPICallLogRetentionDays {
			updated.APICallLogRetentionDays = nil
		} else {
			if patch.APICallLogRetentionDays == nil || *patch.APICallLogRetentionDays > maxIntValue() {
				return LogRetention{}, nil, ErrValidation
			}
			days := int(*patch.APICallLogRetentionDays)
			updated.APICallLogRetentionDays = &days
		}
	}
	if patch.HasTaskEventRetentionDays {
		changedFields = append(changedFields, "logRetention.taskEventRetentionDays")
		if patch.ClearTaskEventRetentionDays {
			updated.TaskEventRetentionDays = nil
		} else {
			if patch.TaskEventRetentionDays == nil || *patch.TaskEventRetentionDays > maxIntValue() {
				return LogRetention{}, nil, ErrValidation
			}
			days := int(*patch.TaskEventRetentionDays)
			updated.TaskEventRetentionDays = &days
		}
	}
	if len(changedFields) == 0 {
		return LogRetention{}, nil, ErrValidation
	}
	if err := validateLogRetention(updated); err != nil {
		return LogRetention{}, nil, err
	}
	return updated, changedFields, nil
}

func validateLogRetention(retention LogRetention) error {
	fields := []struct {
		field string
		label string
		days  *int
	}{
		{field: "logRetention.operationLogRetentionDays", label: "操作日志保留天数", days: retention.OperationLogRetentionDays},
		{field: "logRetention.apiCallLogRetentionDays", label: "API 调用日志保留天数", days: retention.APICallLogRetentionDays},
		{field: "logRetention.taskEventRetentionDays", label: "任务事件保留天数", days: retention.TaskEventRetentionDays},
	}
	for _, item := range fields {
		if item.days == nil {
			continue
		}
		if *item.days < minLogRetentionDays || *item.days > maxLogRetentionDays {
			return newRangeValidationError(item.field, item.label, minLogRetentionDays, maxLogRetentionDays)
		}
	}
	return nil
}

func clampPolicyToHardCap(policy UploadPolicy, hardCap config.UploadConfig) UploadPolicy {
	capPolicy := uploadPolicyFromConfig(hardCap)
	if policy.MaxFileSizeBytes > capPolicy.MaxFileSizeBytes {
		policy.MaxFileSizeBytes = capPolicy.MaxFileSizeBytes
	}
	if policy.MaxWidth > capPolicy.MaxWidth {
		policy.MaxWidth = capPolicy.MaxWidth
	}
	if policy.MaxHeight > capPolicy.MaxHeight {
		policy.MaxHeight = capPolicy.MaxHeight
	}
	if policy.MaxPixels > capPolicy.MaxPixels {
		policy.MaxPixels = capPolicy.MaxPixels
	}
	return policy
}

func normalizeTaskDefaultsPatch(patch TaskDefaultsPatch) (TaskDefaults, error) {
	if (patch.DefaultProviderID == nil) != (patch.DefaultModelID == nil) {
		return TaskDefaults{}, ErrValidation
	}
	if patch.DefaultProviderID == nil {
		return TaskDefaults{}, nil
	}
	providerID, err := cleanTaskDefaultID(*patch.DefaultProviderID)
	if err != nil {
		return TaskDefaults{}, err
	}
	modelID, err := cleanTaskDefaultID(*patch.DefaultModelID)
	if err != nil {
		return TaskDefaults{}, err
	}
	return TaskDefaults{DefaultProviderID: &providerID, DefaultModelID: &modelID}, nil
}

func validateTaskDefaults(ctx context.Context, repo Repository, scope tenant.Scope, providerID string, modelID string) error {
	providerRecord, err := repo.FindProvider(ctx, scope, providerID)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			return &ValidationError{Field: "taskDefaults.defaultProviderId", Message: "默认 Provider 不存在或不可用。"}
		}
		return err
	}
	if providerRecord.Status != providerpkg.StatusEnabled {
		return &ValidationError{Field: "taskDefaults.defaultProviderId", Message: "默认 Provider 必须处于启用状态。"}
	}
	modelRecord, err := repo.FindModel(ctx, scope, modelID)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			return &ValidationError{Field: "taskDefaults.defaultModelId", Message: "默认模型不存在或不可用。"}
		}
		return err
	}
	if modelRecord.Status != modelpkg.StatusEnabled || modelRecord.ProviderID != providerRecord.ID {
		return &ValidationError{Field: "taskDefaults.defaultModelId", Message: "默认模型必须已启用并且属于所选 Provider。"}
	}
	return nil
}

func validateTaskDefaultsForUpdate(ctx context.Context, repo Repository, scope tenant.Scope, providerID string, modelID string) error {
	providerRecord, err := repo.LockProvider(ctx, scope, providerID)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			return &ValidationError{Field: "taskDefaults.defaultProviderId", Message: "默认 Provider 不存在或不可用。"}
		}
		return err
	}
	if providerRecord.Status != providerpkg.StatusEnabled {
		return &ValidationError{Field: "taskDefaults.defaultProviderId", Message: "默认 Provider 必须处于启用状态。"}
	}
	modelRecord, err := repo.LockModel(ctx, scope, modelID)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			return &ValidationError{Field: "taskDefaults.defaultModelId", Message: "默认模型不存在或不可用。"}
		}
		return err
	}
	if modelRecord.Status != modelpkg.StatusEnabled || modelRecord.ProviderID != providerRecord.ID {
		return &ValidationError{Field: "taskDefaults.defaultModelId", Message: "默认模型必须已启用并且属于所选 Provider。"}
	}
	return nil
}

func decodeNullableTaskDefaultID(raw json.RawMessage) (string, bool, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", true, nil
	}
	var value string
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&value); err != nil {
		return "", false, ErrValidation
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return "", false, ErrValidation
	}
	value, err := cleanTaskDefaultID(value)
	if err != nil {
		return "", false, err
	}
	return value, false, nil
}

func cleanTaskDefaultID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > 128 {
		return "", ErrValidation
	}
	return value, nil
}

func newRangeValidationError(field string, label string, min int64, max int64) error {
	return &ValidationError{
		Field:   field,
		Message: fmt.Sprintf("%s必须在 %d 到 %d 之间。", label, min, max),
		Min:     &min,
		Max:     &max,
	}
}

func newMinimumValidationError(field string, label string, min int64) error {
	return &ValidationError{
		Field:   field,
		Message: fmt.Sprintf("%s不能小于 %d。", label, min),
		Min:     &min,
	}
}

func maxIntValue() int64 {
	return int64(^uint(0) >> 1)
}

func isTenantAdmin(principal auth.Principal) bool {
	for _, role := range principal.Roles {
		if role.Code == "admin" {
			return true
		}
	}
	return false
}
