package project

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
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/idgen"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Service struct {
	db         *gorm.DB
	repo       Repository
	authorizer Authorizer
	log        *slog.Logger
	now        func() time.Time
}

type projectCreateRequest struct {
	Name   string `json:"name"`
	Brand  string `json:"brand"`
	ASIN   string `json:"asin"`
	Site   string `json:"site"`
	Notes  string `json:"notes"`
	Status string `json:"status"`
}

type projectUpdateRequest struct {
	Name   *string `json:"name"`
	Brand  *string `json:"brand"`
	ASIN   *string `json:"asin"`
	Site   *string `json:"site"`
	Notes  *string `json:"notes"`
	Status *string `json:"status"`
}

type memberRequest struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
}

func NewService(db *gorm.DB, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		db:         db,
		repo:       NewRepository(db),
		authorizer: NewAuthorizer(db),
		log:        log,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *Service) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/projects", s.ListProjects)
	group.POST("/projects", s.CreateProject)
	group.GET("/projects/:projectId", s.GetProject)
	group.PATCH("/projects/:projectId", s.UpdateProject)
	group.DELETE("/projects/:projectId", s.DeleteProject)

	group.GET("/projects/:projectId/members", s.ListMembers)
	group.POST("/projects/:projectId/members", s.AddMember)
	group.PATCH("/projects/:projectId/members/:userId", s.UpdateMember)
	group.DELETE("/projects/:projectId/members/:userId", s.RemoveMember)
}

func (s *Service) ListProjects(c *gin.Context) {
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

	page, err := s.listProjects(c.Request.Context(), principal, query)
	if err != nil {
		s.respondError(c, err)
		return
	}

	httpx.JSON(c, http.StatusOK, page)
}

func (s *Service) CreateProject(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	var request projectCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.AbortWithError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	input, err := normalizeCreateRequest(request)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	response, err := s.createProject(c.Request.Context(), principal, input, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}

	httpx.JSON(c, http.StatusCreated, response)
}

func (s *Service) GetProject(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	response, err := s.getProject(c.Request.Context(), principal, c.Param("projectId"))
	if err != nil {
		s.respondError(c, err)
		return
	}

	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) UpdateProject(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	var request projectUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.AbortWithError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	input, err := normalizeUpdateRequest(request)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	response, err := s.updateProject(c.Request.Context(), principal, c.Param("projectId"), input, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}

	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) DeleteProject(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	if err := s.deleteProject(c.Request.Context(), principal, c.Param("projectId"), c.ClientIP(), c.Request.UserAgent()); err != nil {
		s.respondError(c, err)
		return
	}

	httpx.JSON(c, http.StatusOK, gin.H{"ok": true})
}

func (s *Service) ListMembers(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	response, err := s.listMembers(c.Request.Context(), principal, c.Param("projectId"))
	if err != nil {
		s.respondError(c, err)
		return
	}

	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) AddMember(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	var request memberRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.AbortWithError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}
	input, err := normalizeMemberRequest(request)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	response, err := s.addMember(c.Request.Context(), principal, c.Param("projectId"), input, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}

	httpx.JSON(c, http.StatusCreated, response)
}

func (s *Service) UpdateMember(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	var request memberRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.AbortWithError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}
	request.UserID = c.Param("userId")
	input, err := normalizeMemberRequest(request)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	response, err := s.updateMember(c.Request.Context(), principal, c.Param("projectId"), input, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}

	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) RemoveMember(c *gin.Context) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	if err := s.removeMember(c.Request.Context(), principal, c.Param("projectId"), c.Param("userId"), c.ClientIP(), c.Request.UserAgent()); err != nil {
		s.respondError(c, err)
		return
	}

	httpx.JSON(c, http.StatusOK, gin.H{"ok": true})
}

func (s *Service) listProjects(ctx context.Context, principal auth.Principal, query ListQuery) (Page, error) {
	if !IsTenantAdmin(principal) && !principal.HasPermission(PermissionRead) {
		return Page{}, ErrForbidden
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return Page{}, err
	}

	memberUserID := ""
	if !IsTenantAdmin(principal) {
		memberUserID = principal.UserID
	}

	records, total, err := s.repo.ListProjects(ctx, scope, ListOptions{
		PageNum:      query.PageNum,
		PageSize:     query.PageSize,
		Status:       query.Status,
		MemberUserID: memberUserID,
	})
	if err != nil {
		return Page{}, err
	}

	responseRecords := make([]ProjectResponse, 0, len(records))
	for _, record := range records {
		responseRecords = append(responseRecords, projectResponse(record))
	}
	return Page{Records: responseRecords, Total: total, PageNum: query.PageNum, PageSize: query.PageSize}, nil
}

func (s *Service) createProject(ctx context.Context, principal auth.Principal, input CreateInput, ip string, userAgent string) (ProjectResponse, error) {
	if !IsTenantAdmin(principal) && !principal.HasPermission(PermissionCreate) {
		return ProjectResponse{}, ErrForbidden
	}
	if s.db == nil {
		return ProjectResponse{}, database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return ProjectResponse{}, err
	}

	var record database.Project
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := s.now()
		repo := s.repo.withDB(tx)
		record = database.Project{
			ID:        idgen.New(),
			TenantID:  scope.ID(),
			Name:      input.Name,
			Brand:     input.Brand,
			ASIN:      input.ASIN,
			Site:      input.Site,
			Notes:     input.Notes,
			Status:    input.Status,
			CreatedBy: principal.UserID,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := repo.CreateProject(ctx, scope, &record); err != nil {
			return err
		}

		owner := database.ProjectMember{
			ID:        idgen.New(),
			TenantID:  scope.ID(),
			ProjectID: record.ID,
			UserID:    principal.UserID,
			Role:      RoleOwner,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := repo.CreateMember(ctx, scope, &owner); err != nil {
			return err
		}

		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       "project.create",
			ResourceType: "project",
			ResourceID:   record.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"name":   record.Name,
				"status": record.Status,
				"brand":  record.Brand,
				"asin":   record.ASIN,
				"site":   record.Site,
			},
		})
	})
	if err != nil {
		return ProjectResponse{}, err
	}

	return projectResponse(record), nil
}

func (s *Service) getProject(ctx context.Context, principal auth.Principal, projectID string) (ProjectResponse, error) {
	record, err := s.authorizer.Authorize(ctx, principal, projectID, PermissionRead, rolesForPermission(PermissionRead)...)
	if err != nil {
		return ProjectResponse{}, err
	}
	return projectResponse(record), nil
}

func (s *Service) updateProject(ctx context.Context, principal auth.Principal, projectID string, input UpdateInput, ip string, userAgent string) (ProjectResponse, error) {
	if s.db == nil {
		return ProjectResponse{}, database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return ProjectResponse{}, err
	}

	var updated database.Project
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		authorizer := s.authorizer.withDB(tx)
		current, err := authorizer.Authorize(ctx, principal, projectID, PermissionUpdate, rolesForPermission(PermissionUpdate)...)
		if err != nil {
			return err
		}

		updates := map[string]any{"updated_at": s.now()}
		changedFields := make([]string, 0, 6)
		addUpdate := func(column string, value string) {
			updates[column] = value
			changedFields = append(changedFields, column)
		}
		if input.Name != nil {
			addUpdate("name", *input.Name)
		}
		if input.Brand != nil {
			addUpdate("brand", *input.Brand)
		}
		if input.ASIN != nil {
			addUpdate("asin", *input.ASIN)
		}
		if input.Site != nil {
			addUpdate("site", *input.Site)
		}
		if input.Notes != nil {
			addUpdate("notes", *input.Notes)
		}
		if input.Status != nil {
			addUpdate("status", *input.Status)
		}
		if len(changedFields) == 0 {
			return ErrValidation
		}

		repo := s.repo.withDB(tx)
		updated, err = repo.UpdateProject(ctx, scope, current.ID, updates)
		if err != nil {
			return err
		}

		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       "project.update",
			ResourceType: "project",
			ResourceID:   current.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"changedFields": changedFields,
				"oldStatus":     current.Status,
				"newStatus":     updated.Status,
			},
		})
	})
	if err != nil {
		return ProjectResponse{}, err
	}

	return projectResponse(updated), nil
}

func (s *Service) deleteProject(ctx context.Context, principal auth.Principal, projectID string, ip string, userAgent string) error {
	if s.db == nil {
		return database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return err
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		authorizer := s.authorizer.withDB(tx)
		record, err := authorizer.Authorize(ctx, principal, projectID, PermissionDelete, rolesForPermission(PermissionDelete)...)
		if err != nil {
			return err
		}

		repo := s.repo.withDB(tx)
		if err := repo.SoftDeleteProject(ctx, scope, record.ID, s.now()); err != nil {
			return err
		}

		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       "project.delete",
			ResourceType: "project",
			ResourceID:   record.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"name":   record.Name,
				"status": record.Status,
			},
		})
	})
}

func (s *Service) listMembers(ctx context.Context, principal auth.Principal, projectID string) ([]MemberResponse, error) {
	record, err := s.authorizer.Authorize(ctx, principal, projectID, PermissionMemberManage, rolesForPermission(PermissionMemberManage)...)
	if err != nil {
		return nil, err
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return nil, err
	}

	members, err := s.repo.ListMembers(ctx, scope, record.ID)
	if err != nil {
		return nil, err
	}

	response := make([]MemberResponse, 0, len(members))
	for _, member := range members {
		response = append(response, memberResponse(member))
	}
	return response, nil
}

func (s *Service) addMember(ctx context.Context, principal auth.Principal, projectID string, input MemberInput, ip string, userAgent string) (MemberResponse, error) {
	if s.db == nil {
		return MemberResponse{}, database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return MemberResponse{}, err
	}

	var created database.ProjectMember
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		authorizer := s.authorizer.withDB(tx)
		record, err := authorizer.Authorize(ctx, principal, projectID, PermissionMemberManage, rolesForPermission(PermissionMemberManage)...)
		if err != nil {
			return err
		}
		repo := s.repo.withDB(tx)
		if err := ensureTargetUser(ctx, repo, scope, input.UserID); err != nil {
			return err
		}
		if _, err := repo.FindMember(ctx, scope, record.ID, input.UserID); err == nil {
			return ErrConflict
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}

		now := s.now()
		created = database.ProjectMember{
			ID:        idgen.New(),
			TenantID:  scope.ID(),
			ProjectID: record.ID,
			UserID:    input.UserID,
			Role:      input.Role,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := repo.CreateMember(ctx, scope, &created); err != nil {
			return err
		}

		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       "project_member.create",
			ResourceType: "project_member",
			ResourceID:   created.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"projectId": record.ID,
				"userId":    created.UserID,
				"role":      created.Role,
			},
		})
	})
	if err != nil {
		return MemberResponse{}, err
	}

	return memberResponse(created), nil
}

func (s *Service) updateMember(ctx context.Context, principal auth.Principal, projectID string, input MemberInput, ip string, userAgent string) (MemberResponse, error) {
	if s.db == nil {
		return MemberResponse{}, database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return MemberResponse{}, err
	}

	var updated database.ProjectMember
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		authorizer := s.authorizer.withDB(tx)
		record, err := authorizer.Authorize(ctx, principal, projectID, PermissionMemberManage, rolesForPermission(PermissionMemberManage)...)
		if err != nil {
			return err
		}
		repo := s.repo.withDB(tx)
		current, err := repo.FindMember(ctx, scope, record.ID, input.UserID)
		if err != nil {
			return err
		}

		updated, err = repo.UpdateMember(ctx, scope, record.ID, input.UserID, input.Role, s.now())
		if err != nil {
			return err
		}

		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       "project_member.update",
			ResourceType: "project_member",
			ResourceID:   current.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"projectId": record.ID,
				"userId":    current.UserID,
				"oldRole":   current.Role,
				"newRole":   updated.Role,
			},
		})
	})
	if err != nil {
		return MemberResponse{}, err
	}

	return memberResponse(updated), nil
}

func (s *Service) removeMember(ctx context.Context, principal auth.Principal, projectID string, userID string, ip string, userAgent string) error {
	if s.db == nil {
		return database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return err
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ErrValidation
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		authorizer := s.authorizer.withDB(tx)
		record, err := authorizer.Authorize(ctx, principal, projectID, PermissionMemberManage, rolesForPermission(PermissionMemberManage)...)
		if err != nil {
			return err
		}
		repo := s.repo.withDB(tx)
		current, err := repo.FindMember(ctx, scope, record.ID, userID)
		if err != nil {
			return err
		}
		if err := repo.DeleteMember(ctx, scope, record.ID, userID); err != nil {
			return err
		}

		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       "project_member.delete",
			ResourceType: "project_member",
			ResourceID:   current.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"projectId": record.ID,
				"userId":    current.UserID,
				"role":      current.Role,
			},
		})
	})
}

func ensureTargetUser(ctx context.Context, repo Repository, scope tenant.Scope, userID string) error {
	exists, err := repo.ActiveUserExists(ctx, scope, userID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrValidation
	}
	return nil
}

func parseListQuery(c *gin.Context) (ListQuery, error) {
	pageNum := parsePositiveInt(c.Query("pageNum"), 1)
	pageSize := parsePositiveInt(c.Query("pageSize"), 20)
	if pageSize > 100 {
		pageSize = 100
	}

	status := strings.ToUpper(strings.TrimSpace(c.Query("status")))
	if status != "" && !validStatus(status) {
		return ListQuery{}, ErrValidation
	}

	return ListQuery{PageNum: pageNum, PageSize: pageSize, Status: status}, nil
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

func normalizeCreateRequest(request projectCreateRequest) (CreateInput, error) {
	name, err := cleanRequired(request.Name, 255)
	if err != nil {
		return CreateInput{}, err
	}
	brand, err := cleanOptional(request.Brand, 255)
	if err != nil {
		return CreateInput{}, err
	}
	asin, err := cleanOptional(request.ASIN, 32)
	if err != nil {
		return CreateInput{}, err
	}
	site, err := cleanOptional(request.Site, 64)
	if err != nil {
		return CreateInput{}, err
	}
	notes, err := cleanOptional(request.Notes, 10000)
	if err != nil {
		return CreateInput{}, err
	}
	status, err := normalizeStatus(request.Status, StatusActive)
	if err != nil {
		return CreateInput{}, err
	}
	return CreateInput{Name: name, Brand: brand, ASIN: asin, Site: site, Notes: notes, Status: status}, nil
}

func normalizeUpdateRequest(request projectUpdateRequest) (UpdateInput, error) {
	input := UpdateInput{}
	if request.Name != nil {
		value, err := cleanRequired(*request.Name, 255)
		if err != nil {
			return UpdateInput{}, err
		}
		input.Name = &value
	}
	if request.Brand != nil {
		value, err := cleanOptional(*request.Brand, 255)
		if err != nil {
			return UpdateInput{}, err
		}
		input.Brand = &value
	}
	if request.ASIN != nil {
		value, err := cleanOptional(*request.ASIN, 32)
		if err != nil {
			return UpdateInput{}, err
		}
		input.ASIN = &value
	}
	if request.Site != nil {
		value, err := cleanOptional(*request.Site, 64)
		if err != nil {
			return UpdateInput{}, err
		}
		input.Site = &value
	}
	if request.Notes != nil {
		value, err := cleanOptional(*request.Notes, 10000)
		if err != nil {
			return UpdateInput{}, err
		}
		input.Notes = &value
	}
	if request.Status != nil {
		value, err := normalizeStatus(*request.Status, "")
		if err != nil {
			return UpdateInput{}, err
		}
		input.Status = &value
	}
	return input, nil
}

func normalizeMemberRequest(request memberRequest) (MemberInput, error) {
	userID := strings.TrimSpace(request.UserID)
	if userID == "" {
		return MemberInput{}, ErrValidation
	}
	role, err := normalizeRole(request.Role)
	if err != nil {
		return MemberInput{}, err
	}
	return MemberInput{UserID: userID, Role: role}, nil
}

func cleanRequired(value string, max int) (string, error) {
	cleaned, err := cleanOptional(value, max)
	if err != nil {
		return "", err
	}
	if cleaned == "" {
		return "", ErrValidation
	}
	return cleaned, nil
}

func cleanOptional(value string, max int) (string, error) {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > max {
		return "", ErrValidation
	}
	return value, nil
}

func (s *Service) respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrValidation):
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
	case errors.Is(err, ErrForbidden):
		httpx.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Forbidden.", nil)
	case errors.Is(err, ErrNotFound):
		httpx.AbortWithError(c, http.StatusNotFound, "NOT_FOUND", "Resource not found.", nil)
	case errors.Is(err, ErrConflict):
		httpx.AbortWithError(c, http.StatusConflict, "CONFLICT", "Resource already exists.", nil)
	default:
		s.log.Error("project request failed", slog.String("request_id", httpx.RequestIDFromContext(c)), slog.String("error", err.Error()))
		httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
	}
}
