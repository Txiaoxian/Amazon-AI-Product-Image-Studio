package asset

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/audit"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/idgen"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/project"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/storage"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Service struct {
	db                *gorm.DB
	repo              Repository
	projectAuthorizer project.Authorizer
	store             storage.ObjectStore
	storageConfig     config.StorageConfig
	uploadConfig      config.UploadConfig
	policyResolver    uploadPolicyResolver
	log               *slog.Logger
	now               func() time.Time
}

type uploadPolicyResolver interface {
	EffectiveUploadConfig(ctx context.Context, tenantID string) (config.UploadConfig, error)
}

type updateRequest struct {
	Category   *string `json:"category"`
	Filename   *string `json:"filename"`
	IsFavorite *bool   `json:"isFavorite"`
}

func NewService(db *gorm.DB, log *slog.Logger, storageConfig config.StorageConfig, uploadConfig config.UploadConfig, store storage.ObjectStore, policyResolver uploadPolicyResolver) *Service {
	if log == nil {
		log = slog.Default()
	}
	uploadConfig = config.NormalizeUploadConfig(uploadConfig)
	return &Service{
		db:                db,
		repo:              NewRepository(db),
		projectAuthorizer: project.NewAuthorizer(db),
		store:             store,
		storageConfig:     config.NormalizeStorageConfig(storageConfig),
		uploadConfig:      uploadConfig,
		policyResolver:    policyResolver,
		log:               log,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *Service) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/projects/:projectId/assets", s.ListAssets)
	group.POST("/projects/:projectId/assets/uploads", s.UploadAsset)
	group.GET("/assets/:assetId", s.GetAsset)
	group.PATCH("/assets/:assetId", s.UpdateAsset)
	group.DELETE("/assets/:assetId", s.DeleteAsset)
	group.POST("/assets/:assetId/favorite", s.FavoriteAsset)
	group.DELETE("/assets/:assetId/favorite", s.UnfavoriteAsset)
	group.GET("/assets/:assetId/download", s.DownloadAsset)
}

func (s *Service) ListAssets(c *gin.Context) {
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

	page, err := s.listAssets(c.Request.Context(), principal, c.Param("projectId"), query)
	if err != nil {
		s.respondError(c, err)
		return
	}

	httpx.JSON(c, http.StatusOK, page)
}

func (s *Service) UploadAsset(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	if _, err := s.projectAuthorizer.Authorize(c.Request.Context(), principal, c.Param("projectId"), PermissionUpload, rolesForPermission(PermissionUpload)...); err != nil {
		s.respondError(c, err)
		return
	}

	uploadConfig, err := s.effectiveUploadConfig(c.Request.Context(), principal.TenantID)
	if err != nil {
		s.respondError(c, err)
		return
	}
	validator := newUploadValidator(uploadConfig)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, validator.maxRequestBytes())
	fileHeader, err := c.FormFile("file")
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	validated, err := validator.validate(fileHeader)
	if err != nil {
		s.respondError(c, err)
		return
	}

	input, err := normalizeUploadRequest(c, validated)
	if err != nil {
		s.respondError(c, err)
		return
	}

	response, err := s.uploadAsset(c.Request.Context(), principal, c.Param("projectId"), input, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}

	httpx.JSON(c, http.StatusCreated, response)
}

func (s *Service) GetAsset(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	response, err := s.getAsset(c.Request.Context(), principal, c.Param("assetId"))
	if err != nil {
		s.respondError(c, err)
		return
	}

	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) UpdateAsset(c *gin.Context) {
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
		s.respondError(c, err)
		return
	}

	response, err := s.updateAsset(c.Request.Context(), principal, c.Param("assetId"), input, changedFields, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}

	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) DeleteAsset(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	if err := s.deleteAsset(c.Request.Context(), principal, c.Param("assetId"), c.ClientIP(), c.Request.UserAgent()); err != nil {
		s.respondError(c, err)
		return
	}

	httpx.JSON(c, http.StatusOK, gin.H{"ok": true})
}

func (s *Service) FavoriteAsset(c *gin.Context) {
	s.setFavorite(c, true)
}

func (s *Service) UnfavoriteAsset(c *gin.Context) {
	s.setFavorite(c, false)
}

func (s *Service) DownloadAsset(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	record, object, err := s.downloadAsset(c.Request.Context(), principal, c.Param("assetId"), c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}
	defer object.Body.Close()

	size := record.SizeBytes
	if object.Size > 0 {
		size = object.Size
	}
	c.DataFromReader(http.StatusOK, size, record.MimeType, object.Body, map[string]string{
		"Content-Disposition": contentDisposition(record),
	})
}

func (s *Service) setFavorite(c *gin.Context, favorite bool) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	response, err := s.setAssetFavorite(c.Request.Context(), principal, c.Param("assetId"), favorite, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}

	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) listAssets(ctx context.Context, principal auth.Principal, projectID string, query ListQuery) (Page, error) {
	projectRecord, err := s.projectAuthorizer.Authorize(ctx, principal, projectID, PermissionRead, rolesForPermission(PermissionRead)...)
	if err != nil {
		return Page{}, err
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return Page{}, err
	}

	records, total, err := s.repo.ListAssets(ctx, scope, projectRecord.ID, ListOptions(query))
	if err != nil {
		return Page{}, err
	}

	responseRecords := make([]Response, 0, len(records))
	for _, record := range records {
		responseRecords = append(responseRecords, responseFromRecord(record))
	}
	return Page{Records: responseRecords, Total: total, PageNum: query.PageNum, PageSize: query.PageSize}, nil
}

func (s *Service) uploadAsset(ctx context.Context, principal auth.Principal, projectID string, input uploadInput, ip string, userAgent string) (Response, error) {
	if input.Kind != KindReference {
		return Response{}, ErrValidation
	}
	if s.db == nil {
		return Response{}, database.ErrNilDB
	}
	if s.store == nil {
		return Response{}, ErrStorageUnavailable
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return Response{}, err
	}

	projectRecord, err := s.projectAuthorizer.Authorize(ctx, principal, projectID, PermissionUpload, rolesForPermission(PermissionUpload)...)
	if err != nil {
		return Response{}, err
	}

	now := s.now()
	record := database.ImageAsset{
		ID:         idgen.New(),
		TenantID:   scope.ID(),
		ProjectID:  projectRecord.ID,
		Kind:       input.Kind,
		Category:   input.Category,
		Filename:   input.Filename,
		MimeType:   input.MimeType,
		SizeBytes:  input.SizeBytes,
		Width:      input.Width,
		Height:     input.Height,
		SHA256:     input.SHA256,
		IsFavorite: input.IsFavorite,
		CreatedBy:  principal.UserID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	record.ObjectKey = objectKey(scope.ID(), projectRecord.ID, record.ID, input.Ext)

	if err := s.store.PutObject(ctx, s.storageConfig.BucketOriginals, record.ObjectKey, bytes.NewReader(input.Data), input.SizeBytes, input.MimeType); err != nil {
		return Response{}, err
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.withDB(tx)
		if err := repo.CreateAsset(ctx, scope, &record); err != nil {
			return err
		}
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       "asset.upload",
			ResourceType: "asset",
			ResourceID:   record.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"projectId":  record.ProjectID,
				"kind":       record.Kind,
				"category":   record.Category,
				"filename":   record.Filename,
				"mimeType":   record.MimeType,
				"fileSize":   record.SizeBytes,
				"width":      record.Width,
				"height":     record.Height,
				"isFavorite": record.IsFavorite,
			},
		})
	}); err != nil {
		if removeErr := s.store.RemoveObject(ctx, s.storageConfig.BucketOriginals, record.ObjectKey); removeErr != nil {
			s.log.Warn("uploaded asset cleanup failed", slog.String("asset_id", record.ID), slog.String("error", removeErr.Error()))
		}
		return Response{}, err
	}

	return responseFromRecord(record), nil
}

func (s *Service) getAsset(ctx context.Context, principal auth.Principal, assetID string) (Response, error) {
	record, _, err := s.authorizeAsset(ctx, principal, assetID, PermissionRead)
	if err != nil {
		return Response{}, err
	}
	return responseFromRecord(record), nil
}

func (s *Service) updateAsset(ctx context.Context, principal auth.Principal, assetID string, input UpdateInput, changedFields []string, ip string, userAgent string) (Response, error) {
	if s.db == nil {
		return Response{}, database.ErrNilDB
	}
	record, _, err := s.authorizeAsset(ctx, principal, assetID, PermissionUpdate)
	if err != nil {
		return Response{}, err
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return Response{}, err
	}

	updates := map[string]any{"updated_at": s.now()}
	if input.Category != nil {
		updates["category"] = *input.Category
	}
	if input.Filename != nil {
		updates["filename"] = *input.Filename
	}
	if input.IsFavorite != nil {
		updates["is_favorite"] = *input.IsFavorite
	}

	var updated database.ImageAsset
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.withDB(tx)
		var err error
		updated, err = repo.UpdateAsset(ctx, scope, record.ID, updates)
		if err != nil {
			return err
		}
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       "asset.update",
			ResourceType: "asset",
			ResourceID:   record.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"projectId":     record.ProjectID,
				"changedFields": changedFields,
			},
		})
	})
	if err != nil {
		return Response{}, err
	}

	return responseFromRecord(updated), nil
}

func (s *Service) deleteAsset(ctx context.Context, principal auth.Principal, assetID string, ip string, userAgent string) error {
	if s.db == nil {
		return database.ErrNilDB
	}
	record, _, err := s.authorizeAsset(ctx, principal, assetID, PermissionDelete)
	if err != nil {
		return err
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return err
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.withDB(tx)
		if err := repo.SoftDeleteAsset(ctx, scope, record.ID, s.now()); err != nil {
			return err
		}
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       "asset.delete",
			ResourceType: "asset",
			ResourceID:   record.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"projectId": record.ProjectID,
				"kind":      record.Kind,
				"category":  record.Category,
				"filename":  record.Filename,
			},
		})
	})
}

func (s *Service) setAssetFavorite(ctx context.Context, principal auth.Principal, assetID string, favorite bool, ip string, userAgent string) (Response, error) {
	if s.db == nil {
		return Response{}, database.ErrNilDB
	}
	record, _, err := s.authorizeAsset(ctx, principal, assetID, PermissionUpdate)
	if err != nil {
		return Response{}, err
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return Response{}, err
	}

	var updated database.ImageAsset
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.withDB(tx)
		var err error
		updated, err = repo.UpdateAsset(ctx, scope, record.ID, map[string]any{
			"is_favorite": favorite,
			"updated_at":  s.now(),
		})
		if err != nil {
			return err
		}
		action := "asset.favorite"
		if !favorite {
			action = "asset.unfavorite"
		}
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       action,
			ResourceType: "asset",
			ResourceID:   record.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"projectId":  record.ProjectID,
				"isFavorite": favorite,
			},
		})
	})
	if err != nil {
		return Response{}, err
	}

	return responseFromRecord(updated), nil
}

func (s *Service) downloadAsset(ctx context.Context, principal auth.Principal, assetID string, ip string, userAgent string) (database.ImageAsset, storage.Object, error) {
	if s.db == nil {
		return database.ImageAsset{}, storage.Object{}, database.ErrNilDB
	}
	if s.store == nil {
		return database.ImageAsset{}, storage.Object{}, ErrStorageUnavailable
	}
	record, _, err := s.authorizeAsset(ctx, principal, assetID, PermissionDownload)
	if err != nil {
		return database.ImageAsset{}, storage.Object{}, err
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return database.ImageAsset{}, storage.Object{}, err
	}

	object, err := s.store.GetObject(ctx, bucketForKind(s.storageConfig, record.Kind), record.ObjectKey)
	if err != nil {
		return database.ImageAsset{}, storage.Object{}, err
	}

	if err := audit.NewRecorder(s.db).Record(ctx, audit.Event{
		TenantID:     scope.ID(),
		ActorUserID:  &principal.UserID,
		Action:       "asset.download",
		ResourceType: "asset",
		ResourceID:   record.ID,
		IP:           ip,
		UserAgent:    userAgent,
		Metadata: map[string]any{
			"projectId": record.ProjectID,
			"kind":      record.Kind,
			"filename":  record.Filename,
			"mimeType":  record.MimeType,
			"fileSize":  record.SizeBytes,
		},
	}); err != nil {
		_ = object.Body.Close()
		return database.ImageAsset{}, storage.Object{}, err
	}

	return record, object, nil
}

func (s *Service) authorizeAsset(ctx context.Context, principal auth.Principal, assetID string, permission string) (database.ImageAsset, database.Project, error) {
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return database.ImageAsset{}, database.Project{}, err
	}
	record, err := s.repo.FindAsset(ctx, scope, assetID)
	if err != nil {
		return database.ImageAsset{}, database.Project{}, err
	}
	projectRecord, err := s.projectAuthorizer.Authorize(ctx, principal, record.ProjectID, permission, rolesForPermission(permission)...)
	if err != nil {
		return database.ImageAsset{}, database.Project{}, err
	}
	return record, projectRecord, nil
}

func (s *Service) effectiveUploadConfig(ctx context.Context, tenantID string) (config.UploadConfig, error) {
	if s.policyResolver == nil {
		return s.uploadConfig, nil
	}
	return s.policyResolver.EffectiveUploadConfig(ctx, tenantID)
}

func parseListQuery(c *gin.Context) (ListQuery, error) {
	pageNum := parsePositiveInt(c.Query("pageNum"), 1)
	pageSize := parsePositiveInt(c.Query("pageSize"), 20)
	if pageSize > 100 {
		pageSize = 100
	}

	kind := strings.ToUpper(strings.TrimSpace(c.Query("kind")))
	if kind != "" && !validKind(kind) {
		return ListQuery{}, ErrValidation
	}
	category, err := cleanOptional(c.Query("category"), maxCategoryRunes)
	if err != nil {
		return ListQuery{}, err
	}
	var favorite *bool
	if raw := strings.TrimSpace(c.Query("favorite")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return ListQuery{}, ErrValidation
		}
		favorite = &value
	}

	return ListQuery{PageNum: pageNum, PageSize: pageSize, Kind: kind, Category: category, Favorite: favorite}, nil
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

func normalizeUploadRequest(c *gin.Context, validated validatedUpload) (uploadInput, error) {
	kind := strings.ToUpper(strings.TrimSpace(c.PostForm("kind")))
	if kind == "" {
		kind = KindReference
	}
	if kind != KindReference {
		return uploadInput{}, ErrValidation
	}
	category, err := cleanOptional(c.PostForm("category"), maxCategoryRunes)
	if err != nil {
		return uploadInput{}, err
	}
	filename := validated.Filename
	if rawFilename := strings.TrimSpace(c.PostForm("filename")); rawFilename != "" {
		filename = sanitizeFilename(rawFilename, validated.Ext)
	}
	isFavorite, err := parseOptionalBool(c.PostForm("isFavorite"), false)
	if err != nil {
		return uploadInput{}, err
	}

	return uploadInput{
		Kind:       kind,
		Category:   category,
		Filename:   filename,
		IsFavorite: isFavorite,
		Data:       validated.Data,
		MimeType:   validated.MimeType,
		Ext:        validated.Ext,
		SizeBytes:  validated.SizeBytes,
		Width:      validated.Width,
		Height:     validated.Height,
		SHA256:     validated.SHA256,
	}, nil
}

func normalizeUpdateRequest(request updateRequest) (UpdateInput, []string, error) {
	input := UpdateInput{}
	changedFields := make([]string, 0, 3)
	if request.Category != nil {
		value, err := cleanOptional(*request.Category, maxCategoryRunes)
		if err != nil {
			return UpdateInput{}, nil, err
		}
		input.Category = &value
		changedFields = append(changedFields, "category")
	}
	if request.Filename != nil {
		value := sanitizeFilename(*request.Filename, "img")
		input.Filename = &value
		changedFields = append(changedFields, "filename")
	}
	if request.IsFavorite != nil {
		input.IsFavorite = request.IsFavorite
		changedFields = append(changedFields, "isFavorite")
	}
	if len(changedFields) == 0 {
		return UpdateInput{}, nil, ErrValidation
	}
	return input, changedFields, nil
}

func parseOptionalBool(raw string, fallback bool) (bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, ErrValidation
	}
	return value, nil
}

func objectKey(tenantID string, projectID string, assetID string, ext string) string {
	return "tenants/" + tenantID + "/projects/" + projectID + "/assets/" + assetID + "/original." + ext
}

func bucketForKind(storageConfig config.StorageConfig, kind string) string {
	storageConfig = config.NormalizeStorageConfig(storageConfig)
	switch kind {
	case KindGenerated, KindEdited:
		return storageConfig.BucketGenerated
	default:
		return storageConfig.BucketOriginals
	}
}

func contentDisposition(record database.ImageAsset) string {
	filename := strings.TrimSpace(record.Filename)
	if filename == "" {
		filename = "asset-" + record.ID + "." + extensionForMIME(record.MimeType)
	}
	return mime.FormatMediaType("attachment", map[string]string{"filename": filename})
}

func extensionForMIME(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	default:
		return "img"
	}
}

func (s *Service) respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrValidation), errors.Is(err, project.ErrValidation):
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
	case errors.Is(err, ErrForbidden), errors.Is(err, project.ErrForbidden):
		httpx.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Forbidden.", nil)
	case errors.Is(err, ErrNotFound), errors.Is(err, project.ErrNotFound), errors.Is(err, storage.ErrNotFound):
		httpx.AbortWithError(c, http.StatusNotFound, "NOT_FOUND", "Resource not found.", nil)
	case errors.Is(err, ErrStorageUnavailable), errors.Is(err, storage.ErrUnavailable):
		s.log.Error("asset storage unavailable", slog.String("request_id", httpx.RequestIDFromContext(c)))
		httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
	default:
		s.log.Error("asset request failed", slog.String("request_id", httpx.RequestIDFromContext(c)), slog.String("error", err.Error()))
		httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
	}
}
