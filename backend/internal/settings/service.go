package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	minStorageQuotaBytes    = 1
	maxStorageQuotaBytes    = int64(109951162777600)
)

type Service struct {
	db                     *gorm.DB
	repo                   Repository
	hardCap                config.UploadConfig
	taskConcurrencyHardCap TaskConcurrency
	log                    *slog.Logger
	now                    func() time.Time
}

func NewService(db *gorm.DB, log *slog.Logger, uploadConfig config.UploadConfig, queueConfig config.QueueConfig) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		db:                     db,
		repo:                   NewRepository(db),
		hardCap:                config.NormalizeUploadConfig(uploadConfig),
		taskConcurrencyHardCap: taskConcurrencyFromQueueConfig(queueConfig),
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
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
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
	taskConcurrency, err := LoadTaskConcurrency(ctx, s.repo, scope, s.taskConcurrencyHardCap)
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
	return Response{UploadPolicy: policy, TaskDefaults: defaults, TaskConcurrency: taskConcurrency, StorageRetention: storageRetention, StorageQuota: storageQuota}, nil
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
	hardCap = normalizeTaskConcurrencyHardCap(hardCap)
	record, ok, err := repo.FindByKey(ctx, scope, KeyTaskConcurrency)
	if err != nil {
		return TaskConcurrency{}, err
	}
	if !ok {
		return hardCap, nil
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
	quota, err := LoadStorageQuota(ctx, repo, scope)
	if err != nil {
		return err
	}
	if quota.MaxBytes == nil {
		return nil
	}
	usedBytes, err := repo.StorageUsedBytes(ctx, scope)
	if err != nil {
		return err
	}
	if usedBytes > *quota.MaxBytes || pendingBytes > *quota.MaxBytes-usedBytes {
		return ErrStorageQuotaExceeded
	}
	return nil
}

func (s *Service) CheckStorageQuota(ctx context.Context, tenantID string, pendingBytes int64) error {
	scope, err := tenant.NewScope(tenantID)
	if err != nil {
		return err
	}
	return CheckStorageQuota(ctx, s.repo, scope, pendingBytes)
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
			if err := validateTaskDefaults(ctx, repo, scope, *updated.DefaultProviderID, *updated.DefaultModelID); err != nil {
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
	current, err := LoadTaskConcurrency(ctx, s.repo, scope, s.taskConcurrencyHardCap)
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
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
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
	if policy.MaxFileSizeBytes > hardCap.MaxFileSizeBytes ||
		policy.MaxWidth > hardCap.MaxWidth ||
		policy.MaxHeight > hardCap.MaxHeight ||
		policy.MaxPixels > hardCap.MaxPixels {
		return ErrValidation
	}
	return nil
}

func validatePositivePolicy(policy UploadPolicy) error {
	if policy.MaxFileSizeBytes <= 0 || policy.MaxWidth <= 0 || policy.MaxHeight <= 0 || policy.MaxPixels <= 0 {
		return ErrValidation
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
	if policy.TenantLimit <= 0 || policy.UserLimit <= 0 || policy.ProviderLimit <= 0 || policy.ModelLimit <= 0 {
		return ErrValidation
	}
	if policy.TenantLimit > hardCap.TenantLimit ||
		policy.UserLimit > hardCap.UserLimit ||
		policy.ProviderLimit > hardCap.ProviderLimit ||
		policy.ModelLimit > hardCap.ModelLimit {
		return ErrValidation
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
		return ErrValidation
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
		return ErrValidation
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
		return err
	}
	if providerRecord.Status != providerpkg.StatusEnabled {
		return ErrValidation
	}
	modelRecord, err := repo.FindModel(ctx, scope, modelID)
	if err != nil {
		return err
	}
	if modelRecord.Status != modelpkg.StatusEnabled || modelRecord.ProviderID != providerRecord.ID {
		return ErrValidation
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
