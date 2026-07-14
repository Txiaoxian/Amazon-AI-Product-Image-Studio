package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/audit"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/idgen"
	modelpkg "github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/model"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/project"
	providerpkg "github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provider"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/queue"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/settings"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Service struct {
	db             *gorm.DB
	repo           Repository
	enqueuer       queue.TaskEnqueuer
	eventPublisher EventPublisher
	log            *slog.Logger
	now            func() time.Time
}

type Option func(*Service)

func WithEventPublisher(publisher EventPublisher) Option {
	return func(s *Service) {
		s.eventPublisher = publisher
	}
}

func NewService(db *gorm.DB, log *slog.Logger, enqueuer queue.TaskEnqueuer, options ...Option) *Service {
	if log == nil {
		log = slog.Default()
	}
	service := &Service{
		db:       db,
		repo:     NewRepository(db),
		enqueuer: enqueuer,
		log:      log,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

func (s *Service) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/projects/:projectId/tasks", s.CreateTask)
	group.GET("/projects/:projectId/tasks", s.ListTasks)
	group.GET("/projects/:projectId/history", s.ListProjectHistory)
	group.GET("/tasks/:taskId", s.GetTask)
	group.POST("/tasks/:taskId/cancel", s.CancelTask)
	group.POST("/tasks/:taskId/retry", s.RetryTask)
}

func (s *Service) CreateTask(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	request, err := bindCreateRequest(c)
	if err != nil {
		if errors.Is(err, ErrMalformedRequest) {
			httpx.AbortWithError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request.", nil)
			return
		}
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	response, err := s.createTask(c.Request.Context(), principal, c.Param("projectId"), request, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusCreated, response)
}

func (s *Service) ListTasks(c *gin.Context) {
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

	page, err := s.listTasks(c.Request.Context(), principal, c.Param("projectId"), query)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, page)
}

func (s *Service) ListProjectHistory(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}
	query, err := parseHistoryQuery(c)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	page, err := s.listProjectHistory(c.Request.Context(), principal, c.Param("projectId"), query)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, page)
}

func (s *Service) GetTask(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	response, err := s.getTask(c.Request.Context(), principal, c.Param("taskId"))
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) CancelTask(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	response, err := s.cancelTask(c.Request.Context(), principal, c.Param("taskId"), c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) RetryTask(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	response, err := s.retryTask(c.Request.Context(), principal, c.Param("taskId"), c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) listTasks(ctx context.Context, principal auth.Principal, projectID string, query ListQuery) (Page, error) {
	projectRecord, err := project.NewAuthorizer(s.db).Authorize(ctx, principal, projectID, PermissionRead, rolesForPermission(PermissionRead)...)
	if err != nil {
		return Page{}, err
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return Page{}, err
	}

	records, total, err := s.repo.ListTasks(ctx, scope, projectRecord.ID, ListOptions(query))
	if err != nil {
		return Page{}, err
	}
	responses := make([]Response, 0, len(records))
	for _, record := range records {
		response, err := s.responseForRecord(ctx, scope, record)
		if err != nil {
			return Page{}, err
		}
		responses = append(responses, response)
	}
	return Page{Records: responses, Total: total, PageNum: query.PageNum, PageSize: query.PageSize}, nil
}

func (s *Service) listProjectHistory(ctx context.Context, principal auth.Principal, projectID string, query HistoryQuery) (HistoryPage, error) {
	projectRecord, err := project.NewAuthorizer(s.db).Authorize(ctx, principal, projectID, PermissionRead, rolesForPermission(PermissionRead)...)
	if err != nil {
		return HistoryPage{}, err
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return HistoryPage{}, err
	}

	pairs, total, err := s.repo.ListHistoryPairs(ctx, scope, projectRecord.ID, HistoryOptions(query))
	if err != nil {
		return HistoryPage{}, err
	}

	assetIDs := make([]string, 0, len(pairs))
	taskIDs := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		assetIDs = append(assetIDs, pair.AssetID)
		taskIDs = append(taskIDs, pair.TaskID)
	}
	assetsByID, err := s.repo.FindHistoryAssets(ctx, scope, projectRecord.ID, assetIDs)
	if err != nil {
		return HistoryPage{}, err
	}
	tasksByID, err := s.repo.FindHistoryTasks(ctx, scope, projectRecord.ID, taskIDs)
	if err != nil {
		return HistoryPage{}, err
	}
	outputAssetIDsByTaskID, err := s.repo.VisibleHistoryOutputAssetIDsByTask(ctx, scope, projectRecord.ID, taskIDs)
	if err != nil {
		return HistoryPage{}, err
	}

	records := make([]HistoryRecord, 0, len(pairs))
	for _, pair := range pairs {
		assetRecord, ok := assetsByID[pair.AssetID]
		if !ok {
			return HistoryPage{}, ErrNotFound
		}
		taskRecord, ok := tasksByID[pair.TaskID]
		if !ok {
			return HistoryPage{}, ErrNotFound
		}
		taskResponse, err := responseFromRecord(taskRecord, outputAssetIDsByTaskID[taskRecord.ID])
		if err != nil {
			return HistoryPage{}, err
		}
		records = append(records, HistoryRecord{
			Asset: assetResponseFromRecord(assetRecord),
			Task:  taskResponse,
		})
	}
	return HistoryPage{Records: records, Total: total, PageNum: query.PageNum, PageSize: query.PageSize}, nil
}

func (s *Service) getTask(ctx context.Context, principal auth.Principal, taskID string) (Response, error) {
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return Response{}, err
	}
	record, err := s.authorizeTask(ctx, s.repo, principal, scope, taskID, PermissionRead)
	if err != nil {
		return Response{}, err
	}
	return s.responseForRecord(ctx, scope, record)
}

func (s *Service) createTask(ctx context.Context, principal auth.Principal, projectID string, request createRequest, ip string, userAgent string) (Response, error) {
	if s.db == nil {
		return Response{}, database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return Response{}, err
	}

	var created database.GenerationTask
	var events []database.TaskEvent
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		projectRecord, err := project.NewAuthorizer(tx).Authorize(ctx, principal, projectID, PermissionCreate, rolesForPermission(PermissionCreate)...)
		if err != nil {
			return err
		}

		repo := s.repo.withDB(tx)
		request, err = resolveTaskDefaultProviderModel(ctx, tx, scope, request)
		if err != nil {
			return err
		}
		providerRecord, err := repo.FindProvider(ctx, scope, request.ProviderID)
		if err != nil {
			return err
		}
		if providerRecord.Status != providerpkg.StatusEnabled {
			return ErrValidation
		}
		modelRecord, err := repo.FindModel(ctx, scope, request.ModelID)
		if err != nil {
			return err
		}
		if !isTenantAdmin(principal) {
			allowed, err := repo.UserCanAccessModel(ctx, scope, principal.UserID, modelRecord.ID)
			if err != nil {
				return err
			}
			if !allowed {
				return ErrForbidden
			}
		}
		if modelRecord.ProviderID != providerRecord.ID || modelRecord.Status != modelpkg.StatusEnabled {
			return ErrValidation
		}

		inputAssetIDs, err := normalizeInputAssetIDs(cleanEnum(request.Type), request.ReferenceAssetIDs, request.EditSourceAssetID)
		if err != nil {
			return err
		}
		assets, err := repo.FindInputAssets(ctx, scope, projectRecord.ID, inputAssetIDs)
		if err != nil {
			return err
		}
		input, err := normalizeCreateRequest(request, modelRecord, assets)
		if err != nil {
			return err
		}

		paramsJSON, err := json.Marshal(input.Parameters)
		if err != nil {
			return err
		}
		inputAssetsJSON, err := json.Marshal(input.InputAssetIDs)
		if err != nil {
			return err
		}

		now := s.now()
		timeoutAt := now.Add(defaultTaskTimeout)
		queuedAt := now
		created = database.GenerationTask{
			ID:                idgen.New(),
			TenantID:          scope.ID(),
			ProjectID:         projectRecord.ID,
			Type:              input.Type,
			ProviderID:        providerRecord.ID,
			ModelID:           modelRecord.ID,
			Status:            StatusQueued,
			Prompt:            input.Prompt,
			ImageType:         input.ImageType,
			ParamsJSON:        string(paramsJSON),
			InputAssetIDsJSON: string(inputAssetsJSON),
			Attempt:           1,
			MaxAttempts:       defaultMaxAttempts,
			QueuedAt:          &queuedAt,
			TimeoutAt:         &timeoutAt,
			CreatedBy:         principal.UserID,
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		if err := repo.CreateTask(ctx, scope, &created); err != nil {
			return err
		}
		event, err := writeTaskEvent(ctx, repo, scope, created, EventTaskQueued, map[string]any{
			"queuedAt": formatTime(queuedAt),
		}, now)
		if err != nil {
			return err
		}
		events = append(events, event)
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       "task.create",
			ResourceType: "generation_task",
			ResourceID:   created.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"projectId":  created.ProjectID,
				"providerId": created.ProviderID,
				"modelId":    created.ModelID,
				"type":       created.Type,
				"status":     created.Status,
				"attempt":    created.Attempt,
			},
		})
	})
	if err != nil {
		return Response{}, err
	}
	s.publishTaskEvents(ctx, events)

	if err := s.enqueueTask(ctx, created.ID); err != nil {
		_ = s.markQueueFailure(ctx, scope, created.ID)
		return Response{}, err
	}

	return s.responseForRecord(ctx, scope, created)
}

func isTenantAdmin(principal auth.Principal) bool {
	for _, role := range principal.Roles {
		if role.Code == "admin" {
			return true
		}
	}
	return false
}

func resolveTaskDefaultProviderModel(ctx context.Context, tx *gorm.DB, scope tenant.Scope, request createRequest) (createRequest, error) {
	if request.providerIDSet && request.modelIDSet {
		return request, nil
	}
	if request.providerIDSet != request.modelIDSet {
		return createRequest{}, ErrValidation
	}

	defaults, err := settings.LoadTaskDefaults(ctx, settings.NewRepository(tx), scope)
	if err != nil {
		if errors.Is(err, settings.ErrStoredTaskDefaultsInvalid) {
			return createRequest{}, ErrValidation
		}
		return createRequest{}, err
	}
	if defaults.DefaultProviderID == nil || defaults.DefaultModelID == nil {
		return createRequest{}, ErrValidation
	}
	request.ProviderID = *defaults.DefaultProviderID
	request.ModelID = *defaults.DefaultModelID
	request.providerIDSet = true
	request.modelIDSet = true
	return request, nil
}

func (s *Service) cancelTask(ctx context.Context, principal auth.Principal, taskID string, ip string, userAgent string) (Response, error) {
	if s.db == nil {
		return Response{}, database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return Response{}, err
	}

	var updated database.GenerationTask
	var events []database.TaskEvent
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.withDB(tx)
		current, err := s.authorizeTask(ctx, repo, principal, scope, taskID, PermissionCancel)
		if err != nil {
			return err
		}
		if terminalStatus(current.Status) {
			return ErrConflict
		}

		now := s.now()
		finishedAt := now
		updated, err = repo.UpdateTask(ctx, scope, current.ID, []string{StatusQueued, StatusRunning, StatusRetrying}, map[string]any{
			"status":        StatusCancelled,
			"finished_at":   &finishedAt,
			"error_code":    "",
			"error_message": "",
			"updated_at":    now,
		})
		if err != nil {
			return err
		}
		event, err := writeTaskEvent(ctx, repo, scope, updated, EventTaskCancelled, map[string]any{
			"finishedAt": formatTime(finishedAt),
		}, now)
		if err != nil {
			return err
		}
		events = append(events, event)
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       "task.cancel",
			ResourceType: "generation_task",
			ResourceID:   updated.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"projectId": updated.ProjectID,
				"oldStatus": current.Status,
				"newStatus": updated.Status,
				"attempt":   updated.Attempt,
			},
		})
	})
	if err != nil {
		return Response{}, err
	}
	s.publishTaskEvents(ctx, events)
	return s.responseForRecord(ctx, scope, updated)
}

func (s *Service) retryTask(ctx context.Context, principal auth.Principal, taskID string, ip string, userAgent string) (Response, error) {
	if s.db == nil {
		return Response{}, database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return Response{}, err
	}

	var queued database.GenerationTask
	var events []database.TaskEvent
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.withDB(tx)
		current, err := s.authorizeTask(ctx, repo, principal, scope, taskID, PermissionRetry)
		if err != nil {
			return err
		}
		if !retryableStatus(current.Status) || current.Attempt >= current.MaxAttempts {
			return ErrConflict
		}

		now := s.now()
		timeoutAt := now.Add(defaultTaskTimeout)
		retrying, err := repo.UpdateTask(ctx, scope, current.ID, []string{StatusFailed, StatusCancelled, StatusTimedOut}, map[string]any{
			"status":        StatusRetrying,
			"attempt":       current.Attempt + 1,
			"queued_at":     nil,
			"started_at":    nil,
			"finished_at":   nil,
			"timeout_at":    &timeoutAt,
			"error_code":    "",
			"error_message": "",
			"updated_at":    now,
		})
		if err != nil {
			return err
		}
		event, err := writeTaskEvent(ctx, repo, scope, retrying, EventTaskRetried, map[string]any{
			"previousStatus": current.Status,
		}, now)
		if err != nil {
			return err
		}
		events = append(events, event)

		queuedAt := now
		queued, err = repo.UpdateTask(ctx, scope, retrying.ID, []string{StatusRetrying}, map[string]any{
			"status":     StatusQueued,
			"queued_at":  &queuedAt,
			"updated_at": now,
		})
		if err != nil {
			return err
		}
		event, err = writeTaskEvent(ctx, repo, scope, queued, EventTaskQueued, map[string]any{
			"queuedAt": formatTime(queuedAt),
		}, now)
		if err != nil {
			return err
		}
		events = append(events, event)
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       "task.retry",
			ResourceType: "generation_task",
			ResourceID:   queued.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"projectId":      queued.ProjectID,
				"previousStatus": current.Status,
				"newStatus":      queued.Status,
				"attempt":        queued.Attempt,
			},
		})
	})
	if err != nil {
		return Response{}, err
	}
	s.publishTaskEvents(ctx, events)

	if err := s.enqueueTask(ctx, queued.ID); err != nil {
		_ = s.markQueueFailure(ctx, scope, queued.ID)
		return Response{}, err
	}

	return s.responseForRecord(ctx, scope, queued)
}

func (s *Service) authorizeTask(ctx context.Context, repo Repository, principal auth.Principal, scope tenant.Scope, taskID string, permission string) (database.GenerationTask, error) {
	record, err := repo.FindTask(ctx, scope, taskID)
	if err != nil {
		return database.GenerationTask{}, err
	}
	if _, err := project.NewAuthorizer(repo.db).Authorize(ctx, principal, record.ProjectID, permission, rolesForPermission(permission)...); err != nil {
		return database.GenerationTask{}, err
	}
	return record, nil
}

func (s *Service) responseForRecord(ctx context.Context, scope tenant.Scope, record database.GenerationTask) (Response, error) {
	outputAssetIDs, err := s.repo.OutputAssetIDs(ctx, scope, record.ID)
	if err != nil {
		return Response{}, err
	}
	return responseFromRecord(record, outputAssetIDs)
}

func (s *Service) enqueueTask(ctx context.Context, taskID string) error {
	if s.enqueuer == nil {
		return ErrQueueUnavailable
	}
	if err := s.enqueuer.EnqueueTask(ctx, taskID); err != nil {
		return fmt.Errorf("%w: %v", ErrQueueUnavailable, err)
	}
	return nil
}

func (s *Service) markQueueFailure(ctx context.Context, scope tenant.Scope, taskID string) error {
	if s.db == nil {
		return database.ErrNilDB
	}
	var events []database.TaskEvent
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.withDB(tx)
		now := s.now()
		finishedAt := now
		failed, err := repo.UpdateTask(ctx, scope, taskID, []string{StatusQueued, StatusRetrying}, map[string]any{
			"status":        StatusFailed,
			"finished_at":   &finishedAt,
			"error_code":    "ENQUEUE_FAILED",
			"error_message": "Task could not be queued.",
			"updated_at":    now,
		})
		if err != nil {
			if errors.Is(err, ErrInvalidTransition) {
				return nil
			}
			return err
		}
		event, err := writeTaskEvent(ctx, repo, scope, failed, EventTaskFailed, map[string]any{
			"errorCode": "ENQUEUE_FAILED",
			"message":   "Task could not be queued.",
		}, now)
		if err != nil {
			return err
		}
		events = append(events, event)
		return nil
	})
	if err != nil {
		return err
	}
	s.publishTaskEvents(ctx, events)
	return nil
}

func (s *Service) publishTaskEvents(ctx context.Context, events []database.TaskEvent) {
	if s.eventPublisher == nil {
		return
	}
	for _, event := range events {
		s.eventPublisher.PublishTaskEvent(ctx, event)
	}
}

func rolesForPermission(permission string) []string {
	switch permission {
	case PermissionCreate, PermissionCancel, PermissionRetry:
		return []string{project.RoleOwner, project.RoleEditor}
	default:
		return []string{project.RoleOwner, project.RoleEditor, project.RoleViewer}
	}
}

func (s *Service) respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrMalformedRequest):
		httpx.AbortWithError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request.", nil)
	case errors.Is(err, ErrValidation):
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
	case errors.Is(err, ErrForbidden), errors.Is(err, project.ErrForbidden):
		httpx.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Forbidden.", nil)
	case errors.Is(err, ErrNotFound), errors.Is(err, project.ErrNotFound):
		httpx.AbortWithError(c, http.StatusNotFound, "NOT_FOUND", "Resource not found.", nil)
	case errors.Is(err, ErrConflict), errors.Is(err, ErrInvalidTransition):
		httpx.AbortWithError(c, http.StatusConflict, "CONFLICT", "Task state conflict.", nil)
	case errors.Is(err, ErrQueueUnavailable):
		s.log.Error("task enqueue failed", slog.String("request_id", httpx.RequestIDFromContext(c)), slog.String("error", err.Error()))
		httpx.AbortWithError(c, http.StatusInternalServerError, "TASK_QUEUE_ERROR", "Task could not be queued.", nil)
	default:
		s.log.Error("task request failed", slog.String("request_id", httpx.RequestIDFromContext(c)), slog.String("error", err.Error()))
		httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
	}
}
