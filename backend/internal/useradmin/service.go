package useradmin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sort"
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
	db   *gorm.DB
	repo Repository
	log  *slog.Logger
	now  func() time.Time
}

type createRequest struct {
	Email       string   `json:"email"`
	DisplayName string   `json:"displayName"`
	Password    string   `json:"password"`
	RoleIDs     []string `json:"roleIds"`
}

type updateRequest struct {
	DisplayName *string `json:"displayName"`
	Status      *string `json:"status"`
}

type replaceRolesRequest struct {
	RoleIDs []string `json:"roleIds"`
}

var errMalformedJSON = errors.New("malformed json")

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
	group.GET("/users", s.ListUsers)
	group.POST("/users", s.CreateUser)
	group.GET("/users/:userId", s.GetUser)
	group.PATCH("/users/:userId", s.UpdateUser)
	group.POST("/users/:userId/disable", s.DisableUser)
	group.POST("/users/:userId/enable", s.EnableUser)
	group.POST("/users/:userId/roles", s.ReplaceUserRoles)
	group.GET("/roles", s.ListRoles)
	group.GET("/permissions", s.ListPermissions)
}

func (s *Service) ListUsers(c *gin.Context) {
	principal, ok := requirePrincipal(c)
	if !ok {
		return
	}
	query, err := parseListQuery(c)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	page, err := s.listUsers(c.Request.Context(), principal, query)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, page)
}

func (s *Service) CreateUser(c *gin.Context) {
	principal, ok := requirePrincipal(c)
	if !ok {
		return
	}
	request, err := parseCreateRequest(c.Request.Body)
	if err != nil {
		respondParseError(c, err)
		return
	}
	input, err := normalizeCreateRequest(request)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	response, err := s.createUser(c.Request.Context(), principal, input, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusCreated, response)
}

func (s *Service) GetUser(c *gin.Context) {
	principal, ok := requirePrincipal(c)
	if !ok {
		return
	}

	response, err := s.getUser(c.Request.Context(), principal, c.Param("userId"))
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) UpdateUser(c *gin.Context) {
	principal, ok := requirePrincipal(c)
	if !ok {
		return
	}
	request, err := parseUpdateRequest(c.Request.Body)
	if err != nil {
		respondParseError(c, err)
		return
	}
	input, changedFields, err := normalizeUpdateRequest(request)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	response, err := s.updateUser(c.Request.Context(), principal, c.Param("userId"), input, changedFields, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) DisableUser(c *gin.Context) {
	s.setUserStatus(c, UserStatusDisabled, "user.disable")
}

func (s *Service) EnableUser(c *gin.Context) {
	s.setUserStatus(c, UserStatusActive, "user.enable")
}

func (s *Service) ReplaceUserRoles(c *gin.Context) {
	principal, ok := requirePrincipal(c)
	if !ok {
		return
	}
	request, err := parseReplaceRolesRequest(c.Request.Body)
	if err != nil {
		respondParseError(c, err)
		return
	}
	input, err := normalizeRolesRequest(request)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	response, err := s.replaceUserRoles(c.Request.Context(), principal, c.Param("userId"), input, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) ListRoles(c *gin.Context) {
	principal, ok := requirePrincipal(c)
	if !ok {
		return
	}
	response, err := s.listRoles(c.Request.Context(), principal)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) ListPermissions(c *gin.Context) {
	principal, ok := requirePrincipal(c)
	if !ok {
		return
	}
	response, err := s.listPermissions(c.Request.Context(), principal)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) setUserStatus(c *gin.Context, status string, action string) {
	principal, ok := requirePrincipal(c)
	if !ok {
		return
	}
	response, err := s.setStatus(c.Request.Context(), principal, c.Param("userId"), status, action, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *Service) listUsers(ctx context.Context, principal auth.Principal, query ListQuery) (Page, error) {
	if !hasPermissionOrAdmin(principal, PermissionUserRead) {
		return Page{}, ErrForbidden
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return Page{}, err
	}
	records, total, err := s.repo.ListUsers(ctx, scope, ListOptions(query))
	if err != nil {
		return Page{}, err
	}
	userIDs := make([]string, 0, len(records))
	for _, record := range records {
		userIDs = append(userIDs, record.ID)
	}
	rolesByUser, err := s.repo.ListRolesForUsers(ctx, scope, userIDs)
	if err != nil {
		return Page{}, err
	}
	responses := make([]UserResponse, 0, len(records))
	for _, record := range records {
		responses = append(responses, userResponse(record, rolesByUser[record.ID]))
	}
	return Page{Records: responses, Total: total, PageNum: query.PageNum, PageSize: query.PageSize}, nil
}

func (s *Service) createUser(ctx context.Context, principal auth.Principal, input CreateInput, ip string, userAgent string) (UserResponse, error) {
	if !hasPermissionOrAdmin(principal, PermissionUserCreate) {
		return UserResponse{}, ErrForbidden
	}
	if len(input.RoleIDs) > 0 && !hasPermissionOrAdmin(principal, PermissionRoleManage) {
		return UserResponse{}, ErrForbidden
	}
	if s.db == nil {
		return UserResponse{}, database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return UserResponse{}, err
	}
	passwordHash, err := auth.HashPassword(input.Password)
	if err != nil {
		return UserResponse{}, ErrValidation
	}

	var created database.User
	var assignedRoles []database.Role
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.withDB(tx)
		exists, err := repo.EmailExists(ctx, scope, input.Email)
		if err != nil {
			return err
		}
		if exists {
			return ErrConflict
		}
		assignedRoles, err = repo.ActiveRolesByIDs(ctx, scope, input.RoleIDs)
		if err != nil {
			return err
		}

		now := s.now()
		created = database.User{
			ID:           idgen.New(),
			TenantID:     scope.ID(),
			Email:        input.Email,
			DisplayName:  input.DisplayName,
			PasswordHash: passwordHash,
			Status:       UserStatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := repo.CreateUser(ctx, scope, &created); err != nil {
			return err
		}
		if err := repo.ReplaceUserRoles(ctx, scope, created.ID, roleIDsFromRoles(assignedRoles)); err != nil {
			return err
		}
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       "user.create",
			ResourceType: "user",
			ResourceID:   created.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"email":       created.Email,
				"displayName": created.DisplayName,
				"status":      created.Status,
				"roleCodes":   roleCodesFromRoles(assignedRoles),
				"roleCount":   len(assignedRoles),
			},
		})
	})
	if err != nil {
		return UserResponse{}, err
	}
	return userResponse(created, assignedRoles), nil
}

func (s *Service) getUser(ctx context.Context, principal auth.Principal, userID string) (UserResponse, error) {
	if !hasPermissionOrAdmin(principal, PermissionUserRead) {
		return UserResponse{}, ErrForbidden
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return UserResponse{}, err
	}
	record, err := s.repo.FindUser(ctx, scope, userID)
	if err != nil {
		return UserResponse{}, err
	}
	roles, err := s.repo.ListUserRoles(ctx, scope, record.ID)
	if err != nil {
		return UserResponse{}, err
	}
	return userResponse(record, roles), nil
}

func (s *Service) updateUser(ctx context.Context, principal auth.Principal, userID string, input UpdateInput, changedFields []string, ip string, userAgent string) (UserResponse, error) {
	if !hasPermissionOrAdmin(principal, PermissionUserUpdate) {
		return UserResponse{}, ErrForbidden
	}
	if input.Status != nil && !hasPermissionOrAdmin(principal, PermissionUserDisable) {
		return UserResponse{}, ErrForbidden
	}
	if s.db == nil {
		return UserResponse{}, database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return UserResponse{}, err
	}

	var updated database.User
	var roles []database.Role
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.withDB(tx)
		current, err := repo.FindUser(ctx, scope, userID)
		if err != nil {
			return err
		}

		updates := map[string]any{"updated_at": s.now()}
		if input.DisplayName != nil {
			updates["display_name"] = *input.DisplayName
		}
		if input.Status != nil {
			if *input.Status == UserStatusDisabled && current.Status != UserStatusDisabled {
				if err := s.ensureCanDisableUser(ctx, repo, scope, principal, current); err != nil {
					return err
				}
			}
			updates["status"] = *input.Status
		}
		updated, err = repo.UpdateUser(ctx, scope, current.ID, updates)
		if err != nil {
			return err
		}
		roles, err = repo.ListUserRoles(ctx, scope, updated.ID)
		if err != nil {
			return err
		}
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       "user.update",
			ResourceType: "user",
			ResourceID:   updated.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"changedFields": changedFields,
				"oldStatus":     current.Status,
				"newStatus":     updated.Status,
			},
		})
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return UserResponse{}, err
	}
	return userResponse(updated, roles), nil
}

func (s *Service) setStatus(ctx context.Context, principal auth.Principal, userID string, status string, action string, ip string, userAgent string) (UserResponse, error) {
	if !hasPermissionOrAdmin(principal, PermissionUserDisable) {
		return UserResponse{}, ErrForbidden
	}
	if s.db == nil {
		return UserResponse{}, database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return UserResponse{}, err
	}

	var updated database.User
	var roles []database.Role
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.withDB(tx)
		current, err := repo.FindUser(ctx, scope, userID)
		if err != nil {
			return err
		}
		if status == UserStatusDisabled && current.Status != UserStatusDisabled {
			if err := s.ensureCanDisableUser(ctx, repo, scope, principal, current); err != nil {
				return err
			}
		}

		updated, err = repo.UpdateUser(ctx, scope, current.ID, map[string]any{
			"status":     status,
			"updated_at": s.now(),
		})
		if err != nil {
			return err
		}
		roles, err = repo.ListUserRoles(ctx, scope, updated.ID)
		if err != nil {
			return err
		}
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       action,
			ResourceType: "user",
			ResourceID:   updated.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"oldStatus": current.Status,
				"newStatus": updated.Status,
			},
		})
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return UserResponse{}, err
	}
	return userResponse(updated, roles), nil
}

func (s *Service) replaceUserRoles(ctx context.Context, principal auth.Principal, userID string, input RolesInput, ip string, userAgent string) (UserResponse, error) {
	if !hasPermissionOrAdmin(principal, PermissionRoleManage) {
		return UserResponse{}, ErrForbidden
	}
	if s.db == nil {
		return UserResponse{}, database.ErrNilDB
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return UserResponse{}, err
	}

	var userRecord database.User
	var assignedRoles []database.Role
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.withDB(tx)
		current, err := repo.FindUser(ctx, scope, userID)
		if err != nil {
			return err
		}
		assignedRoles, err = repo.ActiveRolesByIDs(ctx, scope, input.RoleIDs)
		if err != nil {
			return err
		}
		if current.Status == UserStatusActive {
			currentIsAdmin, err := repo.UserHasActiveRoleCode(ctx, scope, current.ID, "admin")
			if err != nil {
				return err
			}
			if currentIsAdmin && !rolesContainCode(assignedRoles, "admin") {
				if err := s.ensureAnotherActiveAdminExists(ctx, repo, scope); err != nil {
					return err
				}
			}
		}
		if err := repo.ReplaceUserRoles(ctx, scope, current.ID, roleIDsFromRoles(assignedRoles)); err != nil {
			return err
		}
		userRecord = current
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     scope.ID(),
			ActorUserID:  &principal.UserID,
			Action:       "user.roles.replace",
			ResourceType: "user",
			ResourceID:   current.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"roleCodes": roleCodesFromRoles(assignedRoles),
				"roleCount": len(assignedRoles),
			},
		})
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return UserResponse{}, err
	}
	return userResponse(userRecord, assignedRoles), nil
}

func (s *Service) listRoles(ctx context.Context, principal auth.Principal) ([]RoleResponse, error) {
	if !hasPermissionOrAdmin(principal, PermissionRoleRead) {
		return nil, ErrForbidden
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return nil, err
	}
	roles, err := s.repo.ListRoles(ctx, scope)
	if err != nil {
		return nil, err
	}
	permissionsByRole, err := s.repo.ListPermissionsForRoles(ctx, scope, roleIDsFromRoles(roles))
	if err != nil {
		return nil, err
	}
	return roleResponses(roles, permissionsByRole), nil
}

func (s *Service) listPermissions(ctx context.Context, principal auth.Principal) ([]PermissionResponse, error) {
	if !hasPermissionOrAdmin(principal, PermissionRoleRead) {
		return nil, ErrForbidden
	}
	permissions, err := s.repo.ListPermissions(ctx)
	if err != nil {
		return nil, err
	}
	return permissionResponses(permissions), nil
}

func (s *Service) ensureCanDisableUser(ctx context.Context, repo Repository, scope tenant.Scope, principal auth.Principal, user database.User) error {
	if user.ID == principal.UserID {
		return ErrConflict
	}
	isAdmin, err := repo.UserHasActiveRoleCode(ctx, scope, user.ID, "admin")
	if err != nil {
		return err
	}
	if isAdmin {
		return s.ensureAnotherActiveAdminExists(ctx, repo, scope)
	}
	return nil
}

func (s *Service) ensureAnotherActiveAdminExists(ctx context.Context, repo Repository, scope tenant.Scope) error {
	count, err := repo.ActiveAdminCount(ctx, scope)
	if err != nil {
		return err
	}
	if count <= 1 {
		return ErrConflict
	}
	return nil
}

func requirePrincipal(c *gin.Context) (auth.Principal, bool) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return auth.Principal{}, false
	}
	return principal, true
}

func hasPermissionOrAdmin(principal auth.Principal, permission string) bool {
	if isTenantAdmin(principal) {
		return true
	}
	return principal.HasPermission(permission)
}

func isTenantAdmin(principal auth.Principal) bool {
	for _, role := range principal.Roles {
		if role.Code == "admin" {
			return true
		}
	}
	return false
}

func parseListQuery(c *gin.Context) (ListQuery, error) {
	pageNum, err := parsePositiveIntQuery(c.Query("pageNum"), defaultPageNum, 0)
	if err != nil {
		return ListQuery{}, err
	}
	pageSize, err := parsePositiveIntQuery(c.Query("pageSize"), defaultPageSize, maxPageSize)
	if err != nil {
		return ListQuery{}, err
	}
	status := strings.ToUpper(strings.TrimSpace(c.Query("status")))
	if status != "" && !validUserStatus(status) {
		return ListQuery{}, ErrValidation
	}
	q := strings.TrimSpace(c.Query("q"))
	if utf8.RuneCountInString(q) > maxQueryRunes {
		return ListQuery{}, ErrValidation
	}
	return ListQuery{PageNum: pageNum, PageSize: pageSize, Status: status, Q: q}, nil
}

func parsePositiveIntQuery(raw string, fallback int, max int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, ErrValidation
	}
	if max > 0 && value > max {
		return 0, ErrValidation
	}
	return value, nil
}

func parseCreateRequest(body io.Reader) (createRequest, error) {
	var request createRequest
	if err := decodeStrictObject(body, map[string]bool{
		"email":       true,
		"displayName": true,
		"password":    true,
		"roleIds":     true,
	}, &request); err != nil {
		return createRequest{}, err
	}
	return request, nil
}

func parseUpdateRequest(body io.Reader) (updateRequest, error) {
	var request updateRequest
	if err := decodeStrictObject(body, map[string]bool{
		"displayName": true,
		"status":      true,
	}, &request); err != nil {
		return updateRequest{}, err
	}
	return request, nil
}

func parseReplaceRolesRequest(body io.Reader) (replaceRolesRequest, error) {
	fields, err := decodeRawObject(body)
	if err != nil {
		return replaceRolesRequest{}, err
	}
	for key := range fields {
		if key != "roleIds" {
			return replaceRolesRequest{}, ErrValidation
		}
	}
	raw, ok := fields["roleIds"]
	if !ok || strings.TrimSpace(string(raw)) == "null" {
		return replaceRolesRequest{}, ErrValidation
	}
	var request replaceRolesRequest
	if err := json.Unmarshal(raw, &request.RoleIDs); err != nil {
		return replaceRolesRequest{}, errMalformedJSON
	}
	return request, nil
}

func normalizeCreateRequest(request createRequest) (CreateInput, error) {
	email, err := auth.NormalizeEmail(request.Email)
	if err != nil {
		return CreateInput{}, ErrValidation
	}
	displayName, err := cleanRequired(request.DisplayName, maxDisplayNameRunes)
	if err != nil {
		return CreateInput{}, err
	}
	if err := auth.ValidatePassword(request.Password); err != nil {
		return CreateInput{}, ErrValidation
	}
	roleIDs, err := normalizeRoleIDs(request.RoleIDs)
	if err != nil {
		return CreateInput{}, err
	}
	return CreateInput{Email: email, DisplayName: displayName, Password: request.Password, RoleIDs: roleIDs}, nil
}

func normalizeUpdateRequest(request updateRequest) (UpdateInput, []string, error) {
	input := UpdateInput{}
	changedFields := make([]string, 0, 2)
	if request.DisplayName != nil {
		value, err := cleanRequired(*request.DisplayName, maxDisplayNameRunes)
		if err != nil {
			return UpdateInput{}, nil, err
		}
		input.DisplayName = &value
		changedFields = append(changedFields, "displayName")
	}
	if request.Status != nil {
		value := strings.ToUpper(strings.TrimSpace(*request.Status))
		if !validUserStatus(value) {
			return UpdateInput{}, nil, ErrValidation
		}
		input.Status = &value
		changedFields = append(changedFields, "status")
	}
	if len(changedFields) == 0 {
		return UpdateInput{}, nil, ErrValidation
	}
	return input, changedFields, nil
}

func normalizeRolesRequest(request replaceRolesRequest) (RolesInput, error) {
	roleIDs, err := normalizeRoleIDs(request.RoleIDs)
	if err != nil {
		return RolesInput{}, err
	}
	return RolesInput{RoleIDs: roleIDs}, nil
}

func normalizeRoleIDs(values []string) ([]string, error) {
	if len(values) > maxRoleIDsPerRequest {
		return nil, ErrValidation
	}
	seen := map[string]bool{}
	roleIDs := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 128 || seen[value] {
			return nil, ErrValidation
		}
		seen[value] = true
		roleIDs = append(roleIDs, value)
	}
	return roleIDs, nil
}

func cleanRequired(value string, max int) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > max {
		return "", ErrValidation
	}
	return value, nil
}

func decodeStrictObject(body io.Reader, allowed map[string]bool, target any) error {
	fields, err := decodeRawObject(body)
	if err != nil {
		return err
	}
	for key := range fields {
		if !allowed[key] {
			return ErrValidation
		}
	}
	payload, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return errMalformedJSON
	}
	return nil
}

func decodeRawObject(body io.Reader) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(body)
	var fields map[string]json.RawMessage
	if err := decoder.Decode(&fields); err != nil {
		return nil, errMalformedJSON
	}
	if fields == nil {
		return nil, ErrValidation
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errMalformedJSON
	}
	return fields, nil
}

func respondParseError(c *gin.Context, err error) {
	if errors.Is(err, errMalformedJSON) {
		httpx.AbortWithError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}
	httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
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
		httpx.AbortWithError(c, http.StatusConflict, "CONFLICT", "Request conflicts with current state.", nil)
	default:
		s.log.Error("user admin request failed", slog.String("request_id", httpx.RequestIDFromContext(c)), slog.String("error", err.Error()))
		httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
	}
}

func roleIDsFromRoles(roles []database.Role) []string {
	roleIDs := make([]string, 0, len(roles))
	for _, role := range roles {
		roleIDs = append(roleIDs, role.ID)
	}
	sort.Strings(roleIDs)
	return roleIDs
}

func roleCodesFromRoles(roles []database.Role) []string {
	codes := make([]string, 0, len(roles))
	for _, role := range roles {
		codes = append(codes, role.Code)
	}
	sort.Strings(codes)
	return codes
}

func rolesContainCode(roles []database.Role, code string) bool {
	for _, role := range roles {
		if role.Code == code {
			return true
		}
	}
	return false
}
