package sse

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/project"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/task"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const DefaultHeartbeatInterval = 25 * time.Second
const DefaultMaxReplayEvents = 200

type Options struct {
	HeartbeatInterval time.Duration
	MaxReplayEvents   int
}

type Service struct {
	db                *gorm.DB
	repo              task.Repository
	broker            *Broker
	log               *slog.Logger
	heartbeatInterval time.Duration
	maxReplayEvents   int
}

type replayBatch struct {
	events      []database.TaskEvent
	maxSequence uint64
}

func NewService(db *gorm.DB, log *slog.Logger, broker *Broker, options Options) *Service {
	if log == nil {
		log = slog.Default()
	}
	if broker == nil {
		broker = NewBroker(defaultBrokerBuffer)
	}
	heartbeatInterval := options.HeartbeatInterval
	if heartbeatInterval <= 0 {
		heartbeatInterval = DefaultHeartbeatInterval
	}
	maxReplayEvents := options.MaxReplayEvents
	if maxReplayEvents <= 0 {
		maxReplayEvents = DefaultMaxReplayEvents
	}
	return &Service{
		db:                db,
		repo:              task.NewRepository(db),
		broker:            broker,
		log:               log,
		heartbeatInterval: heartbeatInterval,
		maxReplayEvents:   maxReplayEvents,
	}
}

func (s *Service) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/events/tasks", s.StreamTasks)
}

func (s *Service) StreamTasks(c *gin.Context) {
	if s.db == nil {
		httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
		return
	}
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	cursor, err := CursorFromEventIDs(c.GetHeader("Last-Event-ID"), c.Query("lastEventId"))
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}
	filter, err := s.resolveFilter(c.Request.Context(), principal, scope, c.Query("projectId"), c.Query("taskId"))
	if err != nil {
		s.respondError(c, err)
		return
	}

	notifications, unsubscribe := s.broker.Subscribe(c.Request.Context())
	defer func() {
		unsubscribe()
	}()

	batch, err := s.visibleEventsAfter(c.Request.Context(), principal, scope, cursor, filter)
	if err != nil {
		s.respondError(c, err)
		return
	}
	queryCursor := cursor
	if batch.maxSequence > queryCursor {
		queryCursor = batch.maxSequence
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
		return
	}
	if err := http.NewResponseController(c.Writer).SetWriteDeadline(time.Time{}); err != nil {
		s.log.Error("task SSE write deadline setup failed", slog.String("request_id", httpx.RequestIDFromContext(c)), slog.String("error", err.Error()))
		httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
		return
	}
	setStreamHeaders(c.Writer.Header())
	c.Status(http.StatusOK)
	flusher.Flush()

	if !s.writeEvents(c, flusher, batch.events) {
		return
	}

	ticker := time.NewTicker(s.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			nextCursor, ok := s.replayVisibleEvents(c, flusher, principal, scope, queryCursor, filter)
			if !ok {
				return
			}
			queryCursor = nextCursor
			if err := WriteHeartbeat(c.Writer); err != nil {
				return
			}
			flusher.Flush()
		case notification, ok := <-notifications:
			if !ok {
				unsubscribe()
				notifications, unsubscribe = s.broker.Subscribe(c.Request.Context())
				nextCursor, ok := s.replayVisibleEvents(c, flusher, principal, scope, queryCursor, filter)
				if !ok {
					return
				}
				queryCursor = nextCursor
				continue
			}
			if notification.Sequence != 0 && notification.Sequence <= queryCursor {
				continue
			}
			nextCursor, ok := s.replayVisibleEvents(c, flusher, principal, scope, queryCursor, filter)
			if !ok {
				return
			}
			queryCursor = nextCursor
		}
	}
}

func (s *Service) resolveFilter(ctx context.Context, principal auth.Principal, scope tenant.Scope, rawProjectID string, rawTaskID string) (task.EventFilter, error) {
	projectID := strings.TrimSpace(rawProjectID)
	taskID := strings.TrimSpace(rawTaskID)
	filter := task.EventFilter{}

	if projectID != "" {
		if _, err := project.NewAuthorizer(s.db).Authorize(ctx, principal, projectID, task.PermissionRead, project.RoleOwner, project.RoleEditor, project.RoleViewer); err != nil {
			return task.EventFilter{}, err
		}
		filter.ProjectID = projectID
	}

	if taskID == "" {
		return filter, nil
	}

	record, err := s.repo.FindTask(ctx, scope, taskID)
	if err != nil {
		return task.EventFilter{}, err
	}
	if projectID != "" && record.ProjectID != projectID {
		return task.EventFilter{}, task.ErrNotFound
	}
	if _, err := project.NewAuthorizer(s.db).Authorize(ctx, principal, record.ProjectID, task.PermissionRead, project.RoleOwner, project.RoleEditor, project.RoleViewer); err != nil {
		return task.EventFilter{}, err
	}
	filter.ProjectID = record.ProjectID
	filter.TaskID = record.ID
	return filter, nil
}

func (s *Service) visibleEventsAfter(ctx context.Context, principal auth.Principal, scope tenant.Scope, cursor uint64, filter task.EventFilter) (replayBatch, error) {
	events, err := s.repo.ListEventsAfter(ctx, scope, cursor, filter, s.maxReplayEvents)
	if err != nil {
		return replayBatch{}, err
	}

	batch := replayBatch{events: make([]database.TaskEvent, 0, len(events))}
	for _, event := range events {
		if event.Sequence > batch.maxSequence {
			batch.maxSequence = event.Sequence
		}
		visible, err := s.canSeeEvent(ctx, principal, event, filter)
		if err != nil {
			return replayBatch{}, err
		}
		if visible {
			batch.events = append(batch.events, event)
		}
	}
	return batch, nil
}

func (s *Service) replayVisibleEvents(c *gin.Context, flusher http.Flusher, principal auth.Principal, scope tenant.Scope, cursor uint64, filter task.EventFilter) (uint64, bool) {
	if !s.principalStillCurrent(c.Request.Context(), principal, c) {
		return cursor, false
	}
	batch, err := s.visibleEventsAfter(c.Request.Context(), principal, scope, cursor, filter)
	if err != nil {
		s.log.Error("task SSE replay failed", slog.String("request_id", httpx.RequestIDFromContext(c)), slog.String("error", err.Error()))
		return cursor, false
	}
	if batch.maxSequence > cursor {
		cursor = batch.maxSequence
	}
	if !s.writeEvents(c, flusher, batch.events) {
		return cursor, false
	}
	return cursor, true
}

func (s *Service) principalStillCurrent(ctx context.Context, principal auth.Principal, c *gin.Context) bool {
	var userRecord database.User
	err := s.db.WithContext(ctx).
		Select("id", "tenant_id", "status", "session_version").
		Where("tenant_id = ? AND id = ? AND status = ?", principal.TenantID, principal.UserID, auth.UserStatusActive).
		First(&userRecord).Error
	if err == nil {
		return userRecord.SessionVersion > 0 && userRecord.SessionVersion == principal.SessionVersion
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false
	}
	s.log.Error("task SSE session revalidation failed", slog.String("request_id", httpx.RequestIDFromContext(c)), slog.String("error", err.Error()))
	return false
}

func (s *Service) canSeeEvent(ctx context.Context, principal auth.Principal, event database.TaskEvent, filter task.EventFilter) (bool, error) {
	if event.TenantID != principal.TenantID {
		return false, nil
	}
	if filter.ProjectID != "" && event.ProjectID != filter.ProjectID {
		return false, nil
	}
	if filter.TaskID != "" && event.TaskID != filter.TaskID {
		return false, nil
	}

	_, err := project.NewAuthorizer(s.db).Authorize(ctx, principal, event.ProjectID, task.PermissionRead, project.RoleOwner, project.RoleEditor, project.RoleViewer)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, project.ErrForbidden) || errors.Is(err, project.ErrNotFound) {
		return false, nil
	}
	return false, err
}

func (s *Service) writeEvents(c *gin.Context, flusher http.Flusher, events []database.TaskEvent) bool {
	for _, event := range events {
		err := WriteFrame(c.Writer, Frame{
			ID:    event.ID,
			Event: event.EventType,
			Data:  event.EventPayloadJSON,
		})
		if err != nil {
			return false
		}
		flusher.Flush()
	}
	return true
}

func setStreamHeaders(header http.Header) {
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
	header.Set("X-Accel-Buffering", "no")
}

func (s *Service) respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, task.ErrValidation), errors.Is(err, ErrInvalidEventID):
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
	case errors.Is(err, task.ErrForbidden), errors.Is(err, project.ErrForbidden):
		httpx.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Forbidden.", nil)
	case errors.Is(err, task.ErrNotFound), errors.Is(err, project.ErrNotFound):
		httpx.AbortWithError(c, http.StatusNotFound, "NOT_FOUND", "Resource not found.", nil)
	default:
		s.log.Error("task SSE request failed", slog.String("request_id", httpx.RequestIDFromContext(c)), slog.String("error", err.Error()))
		httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
	}
}
