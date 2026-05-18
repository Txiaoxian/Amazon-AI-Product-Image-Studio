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

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/audit"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Service struct {
	db      *gorm.DB
	repo    Repository
	hardCap config.UploadConfig
	log     *slog.Logger
	now     func() time.Time
}

func NewService(db *gorm.DB, log *slog.Logger, uploadConfig config.UploadConfig) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		db:      db,
		repo:    NewRepository(db),
		hardCap: config.NormalizeUploadConfig(uploadConfig),
		log:     log,
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

	policy, err := s.EffectiveUploadPolicy(c.Request.Context(), scope)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, Response{UploadPolicy: policy})
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

	policy, err := s.UpdateUploadPolicy(c.Request.Context(), principal, patch, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, Response{UploadPolicy: policy})
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

func parsePatchRequest(body io.Reader) (UploadPolicyPatch, error) {
	var root map[string]json.RawMessage
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&root); err != nil {
		return UploadPolicyPatch{}, ErrValidation
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return UploadPolicyPatch{}, ErrValidation
	}
	if len(root) != 1 {
		return UploadPolicyPatch{}, ErrValidation
	}

	rawPolicy, ok := root["uploadPolicy"]
	if !ok {
		return UploadPolicyPatch{}, ErrValidation
	}
	return parseUploadPolicyPatch(rawPolicy)
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
