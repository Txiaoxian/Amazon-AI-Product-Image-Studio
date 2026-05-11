package auth

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/audit"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/idgen"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	errAlreadyInitialized = errors.New("platform is already initialized")
	errInvalidCredentials = errors.New("invalid credentials")
	errTenantRequired     = errors.New("tenant_id is required")
)

type Service struct {
	db  *gorm.DB
	cfg config.Config
	log *slog.Logger
	now func() time.Time
}

type initAdminRequest struct {
	TenantName  string `json:"tenantName"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
}

type loginRequest struct {
	TenantID string `json:"tenantId"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type accessSnapshot struct {
	Roles       []RoleInfo
	Permissions []string
}

func NewService(db *gorm.DB, cfg config.Config, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}

	return &Service{
		db:  db,
		cfg: cfg,
		log: log,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *Service) InitAdmin(c *gin.Context) {
	var request initAdminRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.AbortWithError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	tenantName := cleanDisplayValue(request.TenantName, 255)
	displayName := cleanDisplayValue(request.DisplayName, 255)
	email, err := normalizeEmail(request.Email)
	if err != nil || tenantName == "" || displayName == "" {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	passwordHash, err := HashPassword(request.Password)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	session, err := s.initAdmin(c.Request.Context(), tenantName, email, displayName, passwordHash, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		if errors.Is(err, errAlreadyInitialized) {
			httpx.AbortWithError(c, http.StatusConflict, "CONFLICT", "Administrator has already been initialized.", nil)
			return
		}
		s.log.Error("init admin failed", slog.String("request_id", httpx.RequestIDFromContext(c)), slog.String("error", err.Error()))
		httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
		return
	}

	s.setSessionCookies(c, session)
	httpx.JSON(c, http.StatusCreated, session)
}

func (s *Service) Login(c *gin.Context) {
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.AbortWithError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	email, err := normalizeEmail(request.Email)
	if err != nil || strings.TrimSpace(request.Password) == "" {
		httpx.AbortWithError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password.", nil)
		return
	}

	session, err := s.login(c.Request.Context(), strings.TrimSpace(request.TenantID), email, request.Password, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		if errors.Is(err, errTenantRequired) {
			httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "tenantId is required when multiple tenants exist.", nil)
			return
		}
		if errors.Is(err, errInvalidCredentials) {
			httpx.AbortWithError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password.", nil)
			return
		}
		s.log.Error("login failed", slog.String("request_id", httpx.RequestIDFromContext(c)), slog.String("error", err.Error()))
		httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
		return
	}

	s.setSessionCookies(c, session)
	httpx.JSON(c, http.StatusOK, session)
}

func (s *Service) Logout(c *gin.Context) {
	principal, ok := PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	if err := s.recordOperation(c.Request.Context(), s.db, audit.Event{
		TenantID:     principal.TenantID,
		ActorUserID:  &principal.UserID,
		Action:       "auth.logout",
		ResourceType: "user",
		ResourceID:   principal.UserID,
		IP:           c.ClientIP(),
		UserAgent:    c.Request.UserAgent(),
		Metadata: map[string]any{
			"result": "success",
		},
	}); err != nil {
		s.log.Error("logout audit failed", slog.String("request_id", httpx.RequestIDFromContext(c)), slog.String("error", err.Error()))
		httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
		return
	}

	s.clearSessionCookies(c)
	httpx.JSON(c, http.StatusOK, gin.H{"ok": true})
}

func (s *Service) Me(c *gin.Context) {
	principal, ok := PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	session, err := s.currentSession(c.Request.Context(), principal.TenantID, principal.UserID, principal.CSRFToken)
	if err != nil {
		s.log.Error("load current session failed", slog.String("request_id", httpx.RequestIDFromContext(c)), slog.String("error", err.Error()))
		httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
		return
	}

	httpx.JSON(c, http.StatusOK, session)
}

func (s *Service) ChangePassword(c *gin.Context) {
	principal, ok := PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return
	}

	var request changePasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.AbortWithError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}
	if err := ValidatePassword(request.NewPassword); err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	if err := s.changePassword(c.Request.Context(), principal, request.CurrentPassword, request.NewPassword, c.ClientIP(), c.Request.UserAgent()); err != nil {
		if errors.Is(err, errInvalidCredentials) {
			httpx.AbortWithError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid current password.", nil)
			return
		}
		s.log.Error("password change failed", slog.String("request_id", httpx.RequestIDFromContext(c)), slog.String("error", err.Error()))
		httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
		return
	}

	httpx.JSON(c, http.StatusOK, gin.H{"ok": true})
}

func (s *Service) initAdmin(ctx context.Context, tenantName string, email string, displayName string, passwordHash string, ip string, userAgent string) (SessionResponse, error) {
	if s.db == nil {
		return SessionResponse{}, database.ErrNilDB
	}

	var session SessionResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		initialized, err := s.initialized(ctx, tx)
		if err != nil {
			return err
		}
		if initialized {
			return errAlreadyInitialized
		}

		now := s.now()
		tenantRecord := database.Tenant{
			ID:        idgen.New(),
			Name:      tenantName,
			Status:    TenantStatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := tx.Create(&tenantRecord).Error; err != nil {
			return err
		}

		rolesByCode, err := s.seedBuiltInRoles(ctx, tx, tenantRecord.ID, now)
		if err != nil {
			return err
		}

		userRecord := database.User{
			ID:           idgen.New(),
			TenantID:     tenantRecord.ID,
			Email:        email,
			DisplayName:  displayName,
			PasswordHash: passwordHash,
			Status:       UserStatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := tx.Create(&userRecord).Error; err != nil {
			return err
		}

		adminRole, ok := rolesByCode["admin"]
		if !ok {
			return errors.New("missing built-in admin role")
		}
		if err := tx.Create(&database.UserRole{
			ID:        idgen.New(),
			TenantID:  tenantRecord.ID,
			UserID:    userRecord.ID,
			RoleID:    adminRole.ID,
			CreatedAt: now,
		}).Error; err != nil {
			return err
		}

		csrfToken, err := newCSRFToken()
		if err != nil {
			return err
		}
		session, err = s.sessionResponse(ctx, tx, tenantRecord, userRecord, csrfToken)
		if err != nil {
			return err
		}

		if err := s.recordOperation(ctx, tx, audit.Event{
			TenantID:     tenantRecord.ID,
			ActorUserID:  &userRecord.ID,
			Action:       "auth.init_admin",
			ResourceType: "tenant",
			ResourceID:   tenantRecord.ID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"result": "success",
			},
		}); err != nil {
			return err
		}

		return nil
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return SessionResponse{}, err
	}

	return session, nil
}

func (s *Service) initialized(ctx context.Context, tx *gorm.DB) (bool, error) {
	var userCount int64
	if err := tx.WithContext(ctx).Model(&database.User{}).Count(&userCount).Error; err != nil {
		return false, err
	}
	if userCount > 0 {
		return true, nil
	}

	var adminCount int64
	if err := tx.WithContext(ctx).Table("user_roles").
		Joins("JOIN roles ON roles.tenant_id = user_roles.tenant_id AND roles.id = user_roles.role_id").
		Where("roles.code = ?", "admin").
		Count(&adminCount).Error; err != nil {
		return false, err
	}
	return adminCount > 0, nil
}

func (s *Service) login(ctx context.Context, tenantID string, email string, password string, ip string, userAgent string) (SessionResponse, error) {
	if s.db == nil {
		return SessionResponse{}, database.ErrNilDB
	}

	tenantRecord, err := s.resolveLoginTenant(ctx, tenantID)
	if err != nil {
		return SessionResponse{}, err
	}

	var userRecord database.User
	err = s.db.WithContext(ctx).
		Where("tenant_id = ? AND email = ? AND status = ?", tenantRecord.ID, email, UserStatusActive).
		First(&userRecord).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		CheckPassword("", password)
		_ = s.recordLoginFailure(ctx, tenantRecord.ID, ip, userAgent)
		return SessionResponse{}, errInvalidCredentials
	}
	if err != nil {
		return SessionResponse{}, err
	}

	if !CheckPassword(userRecord.PasswordHash, password) {
		_ = s.recordLoginFailure(ctx, tenantRecord.ID, ip, userAgent)
		return SessionResponse{}, errInvalidCredentials
	}

	now := s.now()
	if err := s.db.WithContext(ctx).
		Model(&database.User{}).
		Where("tenant_id = ? AND id = ?", tenantRecord.ID, userRecord.ID).
		Updates(map[string]any{
			"last_login_at": now,
			"updated_at":    now,
		}).Error; err != nil {
		return SessionResponse{}, err
	}
	userRecord.LastLoginAt = &now
	userRecord.UpdatedAt = now

	csrfToken, err := newCSRFToken()
	if err != nil {
		return SessionResponse{}, err
	}
	session, err := s.sessionResponse(ctx, s.db, tenantRecord, userRecord, csrfToken)
	if err != nil {
		return SessionResponse{}, err
	}

	if err := s.recordOperation(ctx, s.db, audit.Event{
		TenantID:     tenantRecord.ID,
		ActorUserID:  &userRecord.ID,
		Action:       "auth.login",
		ResourceType: "user",
		ResourceID:   userRecord.ID,
		IP:           ip,
		UserAgent:    userAgent,
		Metadata: map[string]any{
			"result": "success",
		},
	}); err != nil {
		return SessionResponse{}, err
	}

	return session, nil
}

func (s *Service) resolveLoginTenant(ctx context.Context, tenantID string) (database.Tenant, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID != "" {
		var tenantRecord database.Tenant
		err := s.db.WithContext(ctx).
			Where("id = ? AND status = ?", tenantID, TenantStatusActive).
			First(&tenantRecord).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return database.Tenant{}, errInvalidCredentials
		}
		return tenantRecord, err
	}

	var tenants []database.Tenant
	if err := s.db.WithContext(ctx).
		Where("status = ?", TenantStatusActive).
		Order("created_at ASC").
		Limit(2).
		Find(&tenants).Error; err != nil {
		return database.Tenant{}, err
	}
	if len(tenants) == 0 {
		return database.Tenant{}, errInvalidCredentials
	}
	if len(tenants) > 1 {
		return database.Tenant{}, errTenantRequired
	}

	return tenants[0], nil
}

func (s *Service) currentSession(ctx context.Context, tenantID string, userID string, csrfToken string) (SessionResponse, error) {
	var tenantRecord database.Tenant
	if err := s.db.WithContext(ctx).
		Where("id = ? AND status = ?", tenantID, TenantStatusActive).
		First(&tenantRecord).Error; err != nil {
		return SessionResponse{}, err
	}

	var userRecord database.User
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ? AND status = ?", tenantID, userID, UserStatusActive).
		First(&userRecord).Error; err != nil {
		return SessionResponse{}, err
	}

	return s.sessionResponse(ctx, s.db, tenantRecord, userRecord, csrfToken)
}

func (s *Service) changePassword(ctx context.Context, principal Principal, currentPassword string, newPassword string, ip string, userAgent string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var userRecord database.User
		if err := tx.
			Where("tenant_id = ? AND id = ? AND status = ?", principal.TenantID, principal.UserID, UserStatusActive).
			First(&userRecord).Error; err != nil {
			return err
		}

		if !CheckPassword(userRecord.PasswordHash, currentPassword) {
			_ = s.recordOperation(ctx, tx, audit.Event{
				TenantID:     principal.TenantID,
				ActorUserID:  &principal.UserID,
				Action:       "auth.password_change",
				ResourceType: "user",
				ResourceID:   principal.UserID,
				IP:           ip,
				UserAgent:    userAgent,
				Metadata: map[string]any{
					"result": "failure",
					"reason": "invalid_current_credential",
				},
			})
			return errInvalidCredentials
		}

		passwordHash, err := HashPassword(newPassword)
		if err != nil {
			return err
		}

		now := s.now()
		if err := tx.Model(&database.User{}).
			Where("tenant_id = ? AND id = ?", principal.TenantID, principal.UserID).
			Updates(map[string]any{
				"password_hash": passwordHash,
				"updated_at":    now,
			}).Error; err != nil {
			return err
		}

		return s.recordOperation(ctx, tx, audit.Event{
			TenantID:     principal.TenantID,
			ActorUserID:  &principal.UserID,
			Action:       "auth.password_change",
			ResourceType: "user",
			ResourceID:   principal.UserID,
			IP:           ip,
			UserAgent:    userAgent,
			Metadata: map[string]any{
				"result": "success",
			},
		})
	})
}

func (s *Service) sessionResponse(ctx context.Context, db *gorm.DB, tenantRecord database.Tenant, userRecord database.User, csrfToken string) (SessionResponse, error) {
	access, err := s.loadAccess(ctx, db, tenantRecord.ID, userRecord.ID)
	if err != nil {
		return SessionResponse{}, err
	}

	return SessionResponse{
		User:        publicUser(userRecord),
		Tenant:      publicTenant(tenantRecord),
		Roles:       access.Roles,
		Permissions: access.Permissions,
		CSRFToken:   csrfToken,
	}, nil
}

func (s *Service) loadAccess(ctx context.Context, db *gorm.DB, tenantID string, userID string) (accessSnapshot, error) {
	var roles []RoleInfo
	if err := db.WithContext(ctx).Table("roles").
		Select("roles.id, roles.code, roles.name, roles.description").
		Joins("JOIN user_roles ON user_roles.tenant_id = roles.tenant_id AND user_roles.role_id = roles.id").
		Where("roles.tenant_id = ? AND user_roles.user_id = ? AND roles.status = ?", tenantID, userID, RoleStatusActive).
		Order("roles.code ASC").
		Scan(&roles).Error; err != nil {
		return accessSnapshot{}, err
	}

	var permissions []string
	if err := db.WithContext(ctx).Table("permissions").
		Select("DISTINCT permissions.code").
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Joins("JOIN roles ON roles.tenant_id = role_permissions.tenant_id AND roles.id = role_permissions.role_id").
		Joins("JOIN user_roles ON user_roles.tenant_id = role_permissions.tenant_id AND user_roles.role_id = role_permissions.role_id").
		Where("role_permissions.tenant_id = ? AND user_roles.user_id = ? AND roles.status = ?", tenantID, userID, RoleStatusActive).
		Order("permissions.code ASC").
		Pluck("permissions.code", &permissions).Error; err != nil {
		return accessSnapshot{}, err
	}

	return accessSnapshot{Roles: roles, Permissions: permissions}, nil
}

func (s *Service) recordLoginFailure(ctx context.Context, tenantID string, ip string, userAgent string) error {
	return s.recordOperation(ctx, s.db, audit.Event{
		TenantID:     tenantID,
		Action:       "auth.login",
		ResourceType: "tenant",
		ResourceID:   tenantID,
		IP:           ip,
		UserAgent:    userAgent,
		Metadata: map[string]any{
			"result": "failure",
		},
	})
}

func (s *Service) recordOperation(ctx context.Context, db *gorm.DB, event audit.Event) error {
	return audit.NewRecorder(db).Record(ctx, event)
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || len(email) > 255 {
		return "", errors.New("invalid email")
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || address.Name != "" {
		return "", errors.New("invalid email")
	}
	return email, nil
}

func cleanDisplayValue(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) > limit {
		value = value[:limit]
	}
	return value
}

func publicTenant(tenantRecord database.Tenant) TenantInfo {
	return TenantInfo{
		ID:     tenantRecord.ID,
		Name:   tenantRecord.Name,
		Status: tenantRecord.Status,
	}
}

func publicUser(userRecord database.User) UserInfo {
	var lastLoginAt *string
	if userRecord.LastLoginAt != nil {
		formatted := userRecord.LastLoginAt.UTC().Format(time.RFC3339Nano)
		lastLoginAt = &formatted
	}

	return UserInfo{
		ID:          userRecord.ID,
		Email:       userRecord.Email,
		DisplayName: userRecord.DisplayName,
		Status:      userRecord.Status,
		LastLoginAt: lastLoginAt,
		CreatedAt:   userRecord.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:   userRecord.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}
