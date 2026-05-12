package provider

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/audit"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/idgen"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Service struct {
	db        *gorm.DB
	repo      Repository
	cfg       config.ProviderConfig
	cipher    APIKeyCipher
	cipherErr error
	validator *URLValidator
	prober    Prober
	log       *slog.Logger
	now       func() time.Time
}

type Option func(*Service)

type createRequest struct {
	Type             string `json:"type"`
	Name             string `json:"name"`
	BaseURL          string `json:"baseUrl"`
	APIKey           string `json:"apiKey"`
	Status           string `json:"status"`
	TimeoutSeconds   int    `json:"timeoutSeconds"`
	ConcurrencyLimit int    `json:"concurrencyLimit"`
}

type updateRequest struct {
	Name             *string `json:"name"`
	BaseURL          *string `json:"baseUrl"`
	APIKey           *string `json:"apiKey"`
	Status           *string `json:"status"`
	TimeoutSeconds   *int    `json:"timeoutSeconds"`
	ConcurrencyLimit *int    `json:"concurrencyLimit"`
}

func WithProber(prober Prober) Option {
	return func(s *Service) {
		if prober != nil {
			s.prober = prober
		}
	}
}

func WithURLValidator(validator *URLValidator) Option {
	return func(s *Service) {
		if validator != nil {
			s.validator = validator
		}
	}
}

func NewService(db *gorm.DB, log *slog.Logger, providerConfig config.ProviderConfig, options ...Option) *Service {
	if log == nil {
		log = slog.Default()
	}
	cipher, cipherErr := NewAPIKeyCipher(providerConfig.APIKeyEncryptionKey, providerConfig.APIKeyEncryptionKeyID)
	service := &Service{
		db:        db,
		repo:      NewRepository(db),
		cfg:       providerConfig,
		cipher:    cipher,
		cipherErr: cipherErr,
		validator: NewURLValidator(nil),
		log:       log,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	for _, option := range options {
		option(service)
	}
	if service.prober == nil {
		service.prober = NewHTTPProber(nil, service.validator)
	}
	return service
}

func (s *Service) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/providers", s.ListProviders)
	group.POST("/providers", s.CreateProvider)
	group.GET("/providers/:providerId", s.GetProvider)
	group.PATCH("/providers/:providerId", s.UpdateProvider)
	group.DELETE("/providers/:providerId", s.DeleteProvider)
	group.POST("/providers/:providerId/enable", s.EnableProvider)
	group.POST("/providers/:providerId/disable", s.DisableProvider)
	group.POST("/providers/:providerId/test", s.TestProvider)
}

func (s *Service) ListProviders(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}
	query, err := parseListQuery(c)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	page, err := s.listProviders(c.Request.Context(), principal, query)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, page)
}

func (s *Service) CreateProvider(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	var request createRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.AbortWithError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}
	input, err := s.normalizeCreateRequest(c.Request.Context(), request)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	response, err := s.createProvider(c.Request.Context(), principal, input, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusCreated, response)
}

func (s *Service) GetProvider(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	response, err := s.getProvider(c.Request.Context(), principal, c.Param("providerId"))
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) UpdateProvider(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	var request updateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.AbortWithError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}
	input, changedFields, keyChanged, err := s.normalizeUpdateRequest(c.Request.Context(), request)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	response, err := s.updateProvider(c.Request.Context(), principal, c.Param("providerId"), input, changedFields, keyChanged, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) DeleteProvider(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	if err := s.deleteProvider(c.Request.Context(), principal, c.Param("providerId"), c.ClientIP(), c.Request.UserAgent()); err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, gin.H{"ok": true})
}

func (s *Service) EnableProvider(c *gin.Context) {
	s.setProviderStatus(c, StatusEnabled, "provider.enable")
}

func (s *Service) DisableProvider(c *gin.Context) {
	s.setProviderStatus(c, StatusDisabled, "provider.disable")
}

func (s *Service) TestProvider(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	response, err := s.testProvider(c.Request.Context(), principal, c.Param("providerId"), c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) setProviderStatus(c *gin.Context, status string, action string) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	response, err := s.setStatus(c.Request.Context(), principal, c.Param("providerId"), status, action, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) listProviders(ctx context.Context, principal auth.Principal, query ListQuery) (Page, error) {
	if !isTenantAdmin(principal) && !principal.HasPermission(PermissionRead) {
		return Page{}, ErrForbidden
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return Page{}, err
	}

	records, total, err := s.repo.ListProviders(ctx, scope, ListOptions(query))
	if err != nil {
		return Page{}, err
	}
	responses := make([]Response, 0, len(records))
	for _, record := range records {
		responses = append(responses, responseFromRecord(record))
	}
	return Page{Records: responses, Total: total, PageNum: query.PageNum, PageSize: query.PageSize}, nil
}

func (s *Service) createProvider(ctx context.Context, principal auth.Principal, input CreateInput, ip string, userAgent string) (Response, error) {
	if !isTenantAdmin(principal) && !principal.HasPermission(PermissionManage) {
		return Response{}, ErrForbidden
	}
	if s.db == nil {
		return Response{}, database.ErrNilDB
	}
	if s.cipherErr != nil {
		return Response{}, s.cipherErr
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return Response{}, err
	}

	encrypted, hint, err := s.cipher.Encrypt(input.APIKey)
	if err != nil {
		return Response{}, err
	}

	var record database.AIProvider
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := s.now()
		keyUpdatedAt := now
		record = database.AIProvider{
			ID:               idgen.New(),
			TenantID:         scope.ID(),
			Type:             input.Type,
			Name:             input.Name,
			BaseURL:          input.BaseURL,
			EncryptedAPIKey:  encrypted,
			APIKeyHint:       hint,
			APIKeyUpdatedAt:  &keyUpdatedAt,
			Status:           input.Status,
			TimeoutSeconds:   input.TimeoutSeconds,
			ConcurrencyLimit: input.ConcurrencyLimit,
			CreatedBy:        principal.UserID,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if err := s.repo.withDB(tx).CreateProvider(ctx, scope, &record); err != nil {
			return err
		}
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       "provider.create",
			ResourceType: "provider",
			ResourceID:   record.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"type":             record.Type,
				"name":             record.Name,
				"baseUrl":          record.BaseURL,
				"status":           record.Status,
				"timeoutSeconds":   record.TimeoutSeconds,
				"concurrencyLimit": record.ConcurrencyLimit,
			},
		})
	})
	if err != nil {
		return Response{}, err
	}

	return responseFromRecord(record), nil
}

func (s *Service) getProvider(ctx context.Context, principal auth.Principal, providerID string) (Response, error) {
	record, err := s.authorizeProvider(ctx, principal, providerID, PermissionRead)
	if err != nil {
		return Response{}, err
	}
	return responseFromRecord(record), nil
}

func (s *Service) updateProvider(ctx context.Context, principal auth.Principal, providerID string, input UpdateInput, changedFields []string, keyChanged bool, ip string, userAgent string) (Response, error) {
	if s.db == nil {
		return Response{}, database.ErrNilDB
	}
	if s.cipherErr != nil && keyChanged {
		return Response{}, s.cipherErr
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return Response{}, err
	}

	var updated database.AIProvider
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := s.authorizeProviderWithRepo(ctx, s.repo.withDB(tx), principal, providerID, PermissionManage)
		if err != nil {
			return err
		}

		updates := map[string]any{"updated_at": s.now()}
		if input.Name != nil {
			updates["name"] = *input.Name
		}
		if input.BaseURL != nil {
			updates["base_url"] = *input.BaseURL
		}
		if input.Status != nil {
			updates["status"] = *input.Status
		}
		if input.TimeoutSeconds != nil {
			updates["timeout_seconds"] = *input.TimeoutSeconds
		}
		if input.ConcurrencyLimit != nil {
			updates["concurrency_limit"] = *input.ConcurrencyLimit
		}
		if input.APIKey != nil {
			encrypted, hint, err := s.cipher.Encrypt(*input.APIKey)
			if err != nil {
				return err
			}
			keyUpdatedAt := s.now()
			updates["encrypted_api_key"] = encrypted
			updates["api_key_hint"] = hint
			updates["api_key_updated_at"] = keyUpdatedAt
		}

		updated, err = s.repo.withDB(tx).UpdateProvider(ctx, scope, current.ID, updates)
		if err != nil {
			return err
		}
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       "provider.update",
			ResourceType: "provider",
			ResourceID:   current.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"changedFields": changedFields,
				"oldStatus":     current.Status,
				"newStatus":     updated.Status,
				"keyChanged":    keyChanged,
			},
		})
	})
	if err != nil {
		return Response{}, err
	}

	return responseFromRecord(updated), nil
}

func (s *Service) deleteProvider(ctx context.Context, principal auth.Principal, providerID string, ip string, userAgent string) error {
	if s.db == nil {
		return database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return err
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.withDB(tx)
		record, err := s.authorizeProviderWithRepo(ctx, repo, principal, providerID, PermissionManage)
		if err != nil {
			return err
		}
		if err := repo.SoftDeleteProvider(ctx, scope, record.ID, s.now()); err != nil {
			return err
		}
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       "provider.delete",
			ResourceType: "provider",
			ResourceID:   record.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"type":   record.Type,
				"name":   record.Name,
				"status": record.Status,
			},
		})
	})
}

func (s *Service) setStatus(ctx context.Context, principal auth.Principal, providerID string, status string, action string, ip string, userAgent string) (Response, error) {
	if s.db == nil {
		return Response{}, database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return Response{}, err
	}

	var updated database.AIProvider
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.withDB(tx)
		current, err := s.authorizeProviderWithRepo(ctx, repo, principal, providerID, PermissionManage)
		if err != nil {
			return err
		}
		updated, err = repo.UpdateProvider(ctx, scope, current.ID, map[string]any{
			"status":     status,
			"updated_at": s.now(),
		})
		if err != nil {
			return err
		}
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       action,
			ResourceType: "provider",
			ResourceID:   current.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"type":      current.Type,
				"name":      current.Name,
				"oldStatus": current.Status,
				"newStatus": status,
			},
		})
	})
	if err != nil {
		return Response{}, err
	}
	return responseFromRecord(updated), nil
}

func (s *Service) testProvider(ctx context.Context, principal auth.Principal, providerID string, ip string, userAgent string) (TestResponse, error) {
	if s.db == nil {
		return TestResponse{}, database.ErrNilDB
	}
	if s.cipherErr != nil {
		return TestResponse{}, s.cipherErr
	}
	record, err := s.authorizeProvider(ctx, principal, providerID, PermissionManage)
	if err != nil {
		return TestResponse{}, err
	}
	apiKey, err := s.cipher.Decrypt(record.EncryptedAPIKey)
	if err != nil {
		_ = s.recordTestResult(ctx, principal, record, ProbeResult{
			Status:    TestStatusFailure,
			CheckedAt: s.now(),
			Message:   "Provider test failed.",
		}, ip, userAgent)
		return TestResponse{}, err
	}
	if _, err := s.validator.Validate(ctx, record.BaseURL); err != nil {
		_ = s.recordTestResult(ctx, principal, record, ProbeResult{
			Status:    TestStatusFailure,
			CheckedAt: s.now(),
			Message:   "Provider base URL failed security validation.",
		}, ip, userAgent)
		return TestResponse{}, err
	}

	result, probeErr := s.prober.Test(ctx, ProbeConfig{
		Type:           record.Type,
		BaseURL:        record.BaseURL,
		APIKey:         apiKey,
		TimeoutSeconds: record.TimeoutSeconds,
	})
	if probeErr != nil && result.Message == "" {
		result = ProbeResult{
			Status:    TestStatusFailure,
			CheckedAt: s.now(),
			Message:   "Provider test failed.",
		}
	}
	if result.Status == "" {
		result.Status = TestStatusFailure
	}
	if result.CheckedAt.IsZero() {
		result.CheckedAt = s.now()
	}
	result.Message = cleanProbeMessage(result.Message)

	if err := s.recordTestResult(ctx, principal, record, result, ip, userAgent); err != nil {
		return TestResponse{}, err
	}

	return TestResponse{
		Status:     result.Status,
		DurationMs: result.DurationMs,
		CheckedAt:  formatTime(result.CheckedAt),
		HTTPStatus: result.HTTPStatus,
		RequestID:  result.RequestID,
		Message:    result.Message,
	}, nil
}

func (s *Service) recordTestResult(ctx context.Context, principal auth.Principal, record database.AIProvider, result ProbeResult, ip string, userAgent string) error {
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"last_test_status": result.Status,
			"last_tested_at":   result.CheckedAt.UTC(),
			"last_test_error":  "",
			"updated_at":       s.now(),
		}
		if result.Status != TestStatusSuccess {
			updates["last_test_error"] = cleanProbeMessage(result.Message)
		}
		if _, err := s.repo.withDB(tx).UpdateProvider(ctx, scope, record.ID, updates); err != nil {
			return err
		}

		metadata := map[string]any{
			"type":       record.Type,
			"name":       record.Name,
			"status":     result.Status,
			"durationMs": result.DurationMs,
			"message":    cleanProbeMessage(result.Message),
		}
		if result.HTTPStatus != nil {
			metadata["httpStatus"] = *result.HTTPStatus
		}
		if result.RequestID != "" {
			metadata["providerRequestId"] = result.RequestID
		}
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       "provider.test",
			ResourceType: "provider",
			ResourceID:   record.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata:     metadata,
		})
	})
}

func (s *Service) authorizeProvider(ctx context.Context, principal auth.Principal, providerID string, permission string) (database.AIProvider, error) {
	return s.authorizeProviderWithRepo(ctx, s.repo, principal, providerID, permission)
}

func (s *Service) authorizeProviderWithRepo(ctx context.Context, repo Repository, principal auth.Principal, providerID string, permission string) (database.AIProvider, error) {
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return database.AIProvider{}, err
	}
	record, err := repo.FindProvider(ctx, scope, providerID)
	if err != nil {
		return database.AIProvider{}, err
	}
	if isTenantAdmin(principal) {
		return record, nil
	}
	if !principal.HasPermission(permission) {
		return database.AIProvider{}, ErrForbidden
	}
	return record, nil
}

func (s *Service) normalizeCreateRequest(ctx context.Context, request createRequest) (CreateInput, error) {
	providerType := cleanEnum(request.Type)
	if !validType(providerType) {
		return CreateInput{}, ErrValidation
	}
	name, err := cleanRequired(request.Name, maxProviderNameRunes)
	if err != nil {
		return CreateInput{}, err
	}
	baseURL, err := s.normalizeBaseURL(ctx, providerType, request.BaseURL)
	if err != nil {
		return CreateInput{}, err
	}
	apiKey, err := cleanAPIKey(request.APIKey)
	if err != nil {
		return CreateInput{}, err
	}
	status, err := normalizeStatus(request.Status, StatusEnabled)
	if err != nil {
		return CreateInput{}, err
	}
	timeoutSeconds, err := normalizeTimeoutSeconds(request.TimeoutSeconds, int(s.defaultTimeout()/time.Second))
	if err != nil {
		return CreateInput{}, err
	}
	concurrencyLimit, err := normalizeConcurrencyLimit(request.ConcurrencyLimit)
	if err != nil {
		return CreateInput{}, err
	}
	return CreateInput{
		Type:             providerType,
		Name:             name,
		BaseURL:          baseURL,
		APIKey:           apiKey,
		Status:           status,
		TimeoutSeconds:   timeoutSeconds,
		ConcurrencyLimit: concurrencyLimit,
	}, nil
}

func (s *Service) normalizeUpdateRequest(ctx context.Context, request updateRequest) (UpdateInput, []string, bool, error) {
	input := UpdateInput{}
	changedFields := make([]string, 0, 5)
	keyChanged := false

	if request.Name != nil {
		value, err := cleanRequired(*request.Name, maxProviderNameRunes)
		if err != nil {
			return UpdateInput{}, nil, false, err
		}
		input.Name = &value
		changedFields = append(changedFields, "name")
	}
	if request.BaseURL != nil {
		value, err := s.validator.Validate(ctx, *request.BaseURL)
		if err != nil {
			return UpdateInput{}, nil, false, err
		}
		input.BaseURL = &value
		changedFields = append(changedFields, "baseUrl")
	}
	if request.APIKey != nil {
		value, err := cleanAPIKey(*request.APIKey)
		if err != nil {
			return UpdateInput{}, nil, false, err
		}
		input.APIKey = &value
		keyChanged = true
	}
	if request.Status != nil {
		value, err := normalizeStatus(*request.Status, "")
		if err != nil {
			return UpdateInput{}, nil, false, err
		}
		input.Status = &value
		changedFields = append(changedFields, "status")
	}
	if request.TimeoutSeconds != nil {
		value, err := normalizeTimeoutSeconds(*request.TimeoutSeconds, int(s.defaultTimeout()/time.Second))
		if err != nil {
			return UpdateInput{}, nil, false, err
		}
		input.TimeoutSeconds = &value
		changedFields = append(changedFields, "timeoutSeconds")
	}
	if request.ConcurrencyLimit != nil {
		value, err := normalizeConcurrencyLimit(*request.ConcurrencyLimit)
		if err != nil {
			return UpdateInput{}, nil, false, err
		}
		input.ConcurrencyLimit = &value
		changedFields = append(changedFields, "concurrencyLimit")
	}
	if len(changedFields) == 0 && !keyChanged {
		return UpdateInput{}, nil, false, ErrValidation
	}
	return input, changedFields, keyChanged, nil
}

func (s *Service) normalizeBaseURL(ctx context.Context, providerType string, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = defaultBaseURL(providerType)
	}
	if raw == "" {
		return "", ErrValidation
	}
	return s.validator.Validate(ctx, raw)
}

func (s *Service) defaultTimeout() time.Duration {
	if s.cfg.DefaultTimeout <= 0 {
		return defaultRequestTimeout * time.Second
	}
	return s.cfg.DefaultTimeout
}

func parseListQuery(c *gin.Context) (ListQuery, error) {
	pageNum := parsePositiveInt(c.Query("pageNum"), 1)
	pageSize := parsePositiveInt(c.Query("pageSize"), 20)
	if pageSize > 100 {
		pageSize = 100
	}
	providerType := cleanEnum(c.Query("type"))
	if providerType != "" && !validType(providerType) {
		return ListQuery{}, ErrValidation
	}
	status := cleanEnum(c.Query("status"))
	if status != "" && !validStatus(status) {
		return ListQuery{}, ErrValidation
	}
	return ListQuery{PageNum: pageNum, PageSize: pageSize, Type: providerType, Status: status}, nil
}

func parsePositiveInt(raw string, fallback int) int {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func normalizeStatus(status string, defaultStatus string) (string, error) {
	status = cleanEnum(status)
	if status == "" {
		status = defaultStatus
	}
	if !validStatus(status) {
		return "", ErrValidation
	}
	return status, nil
}

func normalizeTimeoutSeconds(value int, fallback int) (int, error) {
	if value == 0 {
		value = fallback
	}
	if value < minTimeoutSeconds || value > maxTimeoutSeconds {
		return 0, ErrValidation
	}
	return value, nil
}

func normalizeConcurrencyLimit(value int) (int, error) {
	if value < 0 || value > maxConcurrencyLimit {
		return 0, ErrValidation
	}
	return value, nil
}

func cleanRequired(value string, limit int) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > limit {
		return "", ErrValidation
	}
	return value, nil
}

func cleanAPIKey(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 4096 {
		return "", ErrValidation
	}
	return value, nil
}

func cleanProbeMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "Provider test failed."
	}
	normalized := strings.ToLower(message)
	for _, marker := range []string{"authorization", "cookie", "api_key", "apikey", "api key", "password", "secret", "token", "jwt", "bearer ", "sk-"} {
		if strings.Contains(normalized, marker) {
			return "Provider test failed."
		}
	}
	runes := []rune(message)
	if len(runes) > maxProbeMessageRunes {
		return string(runes[:maxProbeMessageRunes])
	}
	return message
}

func isTenantAdmin(principal auth.Principal) bool {
	for _, role := range principal.Roles {
		if role.Code == "admin" {
			return true
		}
	}
	return false
}

func (s *Service) respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrValidation):
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
	case errors.Is(err, ErrForbidden):
		httpx.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Forbidden.", nil)
	case errors.Is(err, ErrNotFound):
		httpx.AbortWithError(c, http.StatusNotFound, "NOT_FOUND", "Resource not found.", nil)
	case errors.Is(err, ErrEncryption), errors.Is(err, ErrProbeUnavailable):
		s.log.Error("provider security setup failed", slog.String("request_id", httpx.RequestIDFromContext(c)))
		httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
	default:
		s.log.Error("provider request failed", slog.String("request_id", httpx.RequestIDFromContext(c)), slog.String("error", err.Error()))
		httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
	}
}
