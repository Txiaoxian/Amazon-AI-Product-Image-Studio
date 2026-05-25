package model

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/audit"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/idgen"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provider"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Service struct {
	db   *gorm.DB
	repo Repository
	log  *slog.Logger
	now  func() time.Time
}

func NewService(db *gorm.DB, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		db:   db,
		repo: NewRepository(db),
		log:  log,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *Service) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/models", s.ListModels)
	group.POST("/models", s.CreateModel)
	group.GET("/models/:modelId", s.GetModel)
	group.PATCH("/models/:modelId", s.UpdateModel)
	group.DELETE("/models/:modelId", s.DeleteModel)
	group.POST("/models/:modelId/enable", s.EnableModel)
	group.POST("/models/:modelId/disable", s.DisableModel)
}

func (s *Service) ListModels(c *gin.Context) {
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

	page, err := s.listModels(c.Request.Context(), principal, query)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, page)
}

func (s *Service) CreateModel(c *gin.Context) {
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
	input, err := normalizeCreateRequest(request)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	response, err := s.createModel(c.Request.Context(), principal, input, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusCreated, response)
}

func (s *Service) GetModel(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	response, err := s.getModel(c.Request.Context(), principal, c.Param("modelId"))
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) UpdateModel(c *gin.Context) {
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
	input, changedFields, err := normalizeUpdateRequest(request)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	response, err := s.updateModel(c.Request.Context(), principal, c.Param("modelId"), input, changedFields, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) DeleteModel(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	if err := s.deleteModel(c.Request.Context(), principal, c.Param("modelId"), c.ClientIP(), c.Request.UserAgent()); err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, gin.H{"ok": true})
}

func (s *Service) EnableModel(c *gin.Context) {
	s.setModelStatus(c, StatusEnabled, "model.enable")
}

func (s *Service) DisableModel(c *gin.Context) {
	s.setModelStatus(c, StatusDisabled, "model.disable")
}

func (s *Service) setModelStatus(c *gin.Context, status string, action string) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	response, err := s.setStatus(c.Request.Context(), principal, c.Param("modelId"), status, action, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) listModels(ctx context.Context, principal auth.Principal, query ListQuery) (Page, error) {
	if !isTenantAdmin(principal) && !hasModelPermission(principal, PermissionRead) {
		return Page{}, ErrForbidden
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return Page{}, err
	}

	records, total, err := s.repo.ListModels(ctx, scope, ListOptions(query))
	if err != nil {
		return Page{}, err
	}
	responses, err := s.responsesForRecords(ctx, scope, records)
	if err != nil {
		return Page{}, err
	}
	return Page{Records: responses, Total: total, PageNum: query.PageNum, PageSize: query.PageSize}, nil
}

func (s *Service) createModel(ctx context.Context, principal auth.Principal, input CreateInput, ip string, userAgent string) (Response, error) {
	if !isTenantAdmin(principal) && !principal.HasPermission(PermissionManage) {
		return Response{}, ErrForbidden
	}
	if s.db == nil {
		return Response{}, database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return Response{}, err
	}

	var record database.AIModel
	var providerRecord database.AIProvider
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		providerRecord, err = ensureProviderUsableForModelWrite(ctx, tx, scope, input.ProviderID)
		if err != nil {
			return err
		}
		now := s.now()
		record = database.AIModel{
			ID:                         idgen.New(),
			TenantID:                   scope.ID(),
			ProviderID:                 providerRecord.ID,
			ModelName:                  input.ModelName,
			DisplayName:                input.DisplayName,
			SupportsGenerate:           input.SupportsGenerate,
			SupportsEdit:               input.SupportsEdit,
			SupportsMultiReference:     input.SupportsMultiReference,
			SupportsN:                  input.SupportsN,
			MaxOutputCount:             input.MaxOutputCount,
			SupportedSizesJSON:         input.SupportedSizesJSON,
			SupportedQualitiesJSON:     input.SupportedQualitiesJSON,
			SupportedOutputFormatsJSON: input.SupportedOutputFormatsJSON,
			PricingJSON:                input.PricingJSON,
			Status:                     input.Status,
			CreatedBy:                  principal.UserID,
			CreatedAt:                  now,
			UpdatedAt:                  now,
		}
		if err := s.repo.withDB(tx).CreateModel(ctx, scope, &record); err != nil {
			return err
		}
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       "model.create",
			ResourceType: "model",
			ResourceID:   record.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"providerId":             record.ProviderID,
				"providerName":           providerRecord.Name,
				"modelName":              record.ModelName,
				"displayName":            record.DisplayName,
				"status":                 record.Status,
				"supportsGenerate":       record.SupportsGenerate,
				"supportsEdit":           record.SupportsEdit,
				"supportsMultiReference": record.SupportsMultiReference,
				"supportsN":              record.SupportsN,
				"maxOutputCount":         record.MaxOutputCount,
			},
		})
	})
	if err != nil {
		return Response{}, err
	}

	return responseFromRecord(record, providerRecord.Name)
}

func (s *Service) getModel(ctx context.Context, principal auth.Principal, modelID string) (Response, error) {
	record, err := s.authorizeModel(ctx, principal, modelID, PermissionRead)
	if err != nil {
		return Response{}, err
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return Response{}, err
	}
	return s.responseForRecord(ctx, scope, record)
}

func (s *Service) updateModel(ctx context.Context, principal auth.Principal, modelID string, input UpdateInput, changedFields []string, ip string, userAgent string) (Response, error) {
	if s.db == nil {
		return Response{}, database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return Response{}, err
	}

	var updated database.AIModel
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.withDB(tx)
		current, err := s.authorizeModelWithRepo(ctx, repo, principal, modelID, PermissionManage)
		if err != nil {
			return err
		}
		if err := validateCapabilityState(applyStateUpdate(stateFromRecord(current), input)); err != nil {
			return err
		}

		updates := map[string]any{"updated_at": s.now()}
		if input.ProviderID != nil {
			if _, err := ensureProviderUsableForModelWrite(ctx, tx, scope, *input.ProviderID); err != nil {
				return err
			}
			updates["provider_id"] = *input.ProviderID
		}
		targetProviderID := current.ProviderID
		if input.ProviderID != nil {
			targetProviderID = *input.ProviderID
		}
		if input.ModelName != nil {
			updates["model_name"] = *input.ModelName
		}
		if input.DisplayName != nil {
			updates["display_name"] = *input.DisplayName
		}
		if input.SupportsGenerate != nil {
			updates["supports_generate"] = *input.SupportsGenerate
		}
		if input.SupportsEdit != nil {
			updates["supports_edit"] = *input.SupportsEdit
		}
		if input.SupportsMultiReference != nil {
			updates["supports_multi_reference"] = *input.SupportsMultiReference
		}
		if input.SupportsN != nil {
			updates["supports_n"] = *input.SupportsN
		}
		if input.MaxOutputCount != nil {
			updates["max_output_count"] = *input.MaxOutputCount
		}
		if input.SupportedSizesJSON != nil {
			updates["supported_sizes_json"] = *input.SupportedSizesJSON
		}
		if input.SupportedQualitiesJSON != nil {
			updates["supported_qualities_json"] = *input.SupportedQualitiesJSON
		}
		if input.SupportedOutputFormatsJSON != nil {
			updates["supported_output_formats_json"] = *input.SupportedOutputFormatsJSON
		}
		if input.PricingJSON != nil {
			updates["pricing_json"] = *input.PricingJSON
		}
		if input.Status != nil {
			if *input.Status == StatusEnabled {
				if _, err := ensureProviderUsableForModelWrite(ctx, tx, scope, targetProviderID); err != nil {
					return err
				}
			}
			updates["status"] = *input.Status
		}

		updated, err = repo.UpdateModel(ctx, scope, current.ID, updates)
		if err != nil {
			return err
		}
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       "model.update",
			ResourceType: "model",
			ResourceID:   current.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"providerId":    updated.ProviderID,
				"modelName":     updated.ModelName,
				"changedFields": changedFields,
				"oldStatus":     current.Status,
				"newStatus":     updated.Status,
			},
		})
	})
	if err != nil {
		return Response{}, err
	}

	return s.responseForRecord(ctx, scope, updated)
}

func (s *Service) deleteModel(ctx context.Context, principal auth.Principal, modelID string, ip string, userAgent string) error {
	if s.db == nil {
		return database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return err
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.withDB(tx)
		record, err := s.authorizeModelWithRepo(ctx, repo, principal, modelID, PermissionManage)
		if err != nil {
			return err
		}
		if err := repo.SoftDeleteModel(ctx, scope, record.ID, s.now()); err != nil {
			return err
		}
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       "model.delete",
			ResourceType: "model",
			ResourceID:   record.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"providerId":  record.ProviderID,
				"modelName":   record.ModelName,
				"displayName": record.DisplayName,
				"status":      record.Status,
			},
		})
	})
}

func (s *Service) setStatus(ctx context.Context, principal auth.Principal, modelID string, status string, action string, ip string, userAgent string) (Response, error) {
	if s.db == nil {
		return Response{}, database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return Response{}, err
	}

	var updated database.AIModel
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.withDB(tx)
		current, err := s.authorizeModelWithRepo(ctx, repo, principal, modelID, PermissionManage)
		if err != nil {
			return err
		}
		if status == StatusEnabled {
			if _, err := ensureProviderUsableForModelWrite(ctx, tx, scope, current.ProviderID); err != nil {
				return err
			}
		}
		updated, err = repo.UpdateModel(ctx, scope, current.ID, map[string]any{
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
			ResourceType: "model",
			ResourceID:   current.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"providerId": updated.ProviderID,
				"modelName":  updated.ModelName,
				"oldStatus":  current.Status,
				"newStatus":  status,
			},
		})
	})
	if err != nil {
		return Response{}, err
	}
	return s.responseForRecord(ctx, scope, updated)
}

func (s *Service) authorizeModel(ctx context.Context, principal auth.Principal, modelID string, permission string) (database.AIModel, error) {
	return s.authorizeModelWithRepo(ctx, s.repo, principal, modelID, permission)
}

func (s *Service) authorizeModelWithRepo(ctx context.Context, repo Repository, principal auth.Principal, modelID string, permission string) (database.AIModel, error) {
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return database.AIModel{}, err
	}
	record, err := repo.FindModel(ctx, scope, modelID)
	if err != nil {
		return database.AIModel{}, err
	}
	if isTenantAdmin(principal) {
		return record, nil
	}
	if !hasModelPermission(principal, permission) {
		return database.AIModel{}, ErrForbidden
	}
	return record, nil
}

func (s *Service) responsesForRecords(ctx context.Context, scope tenant.Scope, records []database.AIModel) ([]Response, error) {
	providerIDs := make([]string, 0, len(records))
	for _, record := range records {
		providerIDs = append(providerIDs, record.ProviderID)
	}
	providerNames, err := s.repo.ProviderNames(ctx, scope, providerIDs)
	if err != nil {
		return nil, err
	}

	responses := make([]Response, 0, len(records))
	for _, record := range records {
		response, err := responseFromRecord(record, providerNames[record.ProviderID])
		if err != nil {
			return nil, err
		}
		responses = append(responses, response)
	}
	return responses, nil
}

func (s *Service) responseForRecord(ctx context.Context, scope tenant.Scope, record database.AIModel) (Response, error) {
	responses, err := s.responsesForRecords(ctx, scope, []database.AIModel{record})
	if err != nil {
		return Response{}, err
	}
	if len(responses) != 1 {
		return Response{}, ErrNotFound
	}
	return responses[0], nil
}

func ensureProviderUsableForModelWrite(ctx context.Context, db *gorm.DB, scope tenant.Scope, providerID string) (database.AIProvider, error) {
	record, err := provider.NewRepository(db).LockProvider(ctx, scope, providerID)
	if errors.Is(err, provider.ErrNotFound) || errors.Is(err, provider.ErrValidation) {
		return database.AIProvider{}, ErrValidation
	}
	if err != nil {
		return database.AIProvider{}, err
	}
	if record.Status != provider.StatusEnabled {
		return database.AIProvider{}, ErrValidation
	}
	return record, nil
}

func isTenantAdmin(principal auth.Principal) bool {
	for _, role := range principal.Roles {
		if role.Code == "admin" {
			return true
		}
	}
	return false
}

func hasModelPermission(principal auth.Principal, permission string) bool {
	if permission == PermissionRead {
		return principal.HasPermission(PermissionRead) || principal.HasPermission(PermissionManage)
	}
	return principal.HasPermission(permission)
}

func (s *Service) respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrValidation):
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
	case errors.Is(err, ErrForbidden):
		httpx.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Forbidden.", nil)
	case errors.Is(err, ErrNotFound):
		httpx.AbortWithError(c, http.StatusNotFound, "NOT_FOUND", "Resource not found.", nil)
	default:
		s.log.Error("model request failed", slog.String("request_id", httpx.RequestIDFromContext(c)), slog.String("error", err.Error()))
		httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
	}
}
