package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	auditlog "github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/audit"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/redaction"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	adminReadDefaultPageNum  = 1
	adminReadDefaultPageSize = 20
	adminReadMaxPageSize     = 100
	adminReadMaxFilterRunes  = 128
)

type adminAuditUsageService struct {
	repo     auditlog.Repository
	log      *slog.Logger
	redactor *redaction.Redactor
}

func newAdminAuditUsageService(db *gorm.DB, log *slog.Logger, redactor *redaction.Redactor) *adminAuditUsageService {
	if log == nil {
		log = slog.Default()
	}
	if redactor == nil {
		redactor = redaction.New()
	}
	return &adminAuditUsageService{
		repo:     auditlog.NewRepository(db),
		log:      log,
		redactor: redactor,
	}
}

func (s *adminAuditUsageService) RegisterRoutes(group *gin.RouterGroup) {
	admin := group.Group("/admin")
	admin.GET("/analytics/overview", s.AnalyticsOverview)
	admin.GET("/analytics/usage", s.AnalyticsUsage)
	admin.GET("/analytics/users", s.AnalyticsUsers)
	admin.GET("/analytics/users/:id", s.AnalyticsUserDetail)
	admin.GET("/analytics/tasks", s.AnalyticsTasks)
	admin.GET("/analytics/requests", s.AnalyticsRequests)
	admin.GET("/analytics/exports/:dataset", s.ExportAnalytics)
	admin.GET("/usage/summary", s.UsageSummary)
	admin.GET("/usage/records", s.ListUsageRecords)
	admin.GET("/operation-logs", s.ListOperationLogs)
	admin.GET("/api-call-logs", s.ListAPICallLogs)
	admin.GET("/api-call-logs/:id", s.GetAPICallLog)
}

func (s *adminAuditUsageService) UsageSummary(c *gin.Context) {
	principal, ok := s.requireAdminPermission(c, auditlog.PermissionUsageRead)
	if !ok {
		return
	}
	query, err := parseUsageSummaryQuery(c)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		s.respondError(c, err)
		return
	}

	rows, total, err := s.repo.UsageSummary(c.Request.Context(), scope, query)
	if err != nil {
		s.respondError(c, err)
		return
	}
	records := make([]auditlog.UsageSummaryResponse, 0, len(rows))
	for _, row := range rows {
		records = append(records, auditlog.UsageSummaryResponseFromRow(query.Dimension, row))
	}
	httpx.JSON(c, http.StatusOK, auditlog.UsageSummaryPage{
		Records:  records,
		Total:    total,
		PageNum:  query.PageNum,
		PageSize: query.PageSize,
	})
}

func (s *adminAuditUsageService) ListUsageRecords(c *gin.Context) {
	principal, ok := s.requireAdminPermission(c, auditlog.PermissionUsageRead)
	if !ok {
		return
	}
	query, err := parseUsageRecordListQuery(c)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		s.respondError(c, err)
		return
	}

	rows, total, err := s.repo.ListUsageRecords(c.Request.Context(), scope, query)
	if err != nil {
		s.respondError(c, err)
		return
	}
	records := make([]auditlog.UsageRecordResponse, 0, len(rows))
	for _, row := range rows {
		records = append(records, auditlog.UsageRecordResponseFromRecord(row, s.redactor))
	}
	httpx.JSON(c, http.StatusOK, auditlog.UsageRecordPage{
		Records:  records,
		Total:    total,
		PageNum:  query.PageNum,
		PageSize: query.PageSize,
	})
}

func (s *adminAuditUsageService) ListOperationLogs(c *gin.Context) {
	principal, ok := s.requireAdminPermission(c, auditlog.PermissionAuditRead)
	if !ok {
		return
	}
	query, err := parseOperationLogListQuery(c)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		s.respondError(c, err)
		return
	}

	rows, total, err := s.repo.ListOperationLogs(c.Request.Context(), scope, query)
	if err != nil {
		s.respondError(c, err)
		return
	}
	records := make([]auditlog.OperationLogResponse, 0, len(rows))
	for _, row := range rows {
		records = append(records, auditlog.OperationLogResponseFromRecord(row, s.redactor))
	}
	httpx.JSON(c, http.StatusOK, auditlog.OperationLogPage{
		Records:  records,
		Total:    total,
		PageNum:  query.PageNum,
		PageSize: query.PageSize,
	})
}

func (s *adminAuditUsageService) ListAPICallLogs(c *gin.Context) {
	principal, ok := s.requireAdminPermission(c, auditlog.PermissionAuditRead)
	if !ok {
		return
	}
	query, err := parseAPICallLogListQuery(c)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		s.respondError(c, err)
		return
	}

	rows, total, err := s.repo.ListAPICallLogs(c.Request.Context(), scope, query)
	if err != nil {
		s.respondError(c, err)
		return
	}
	records := make([]auditlog.APICallLogResponse, 0, len(rows))
	for _, row := range rows {
		records = append(records, auditlog.APICallLogResponseFromRecord(row, s.redactor))
	}
	httpx.JSON(c, http.StatusOK, auditlog.APICallLogPage{
		Records:  records,
		Total:    total,
		PageNum:  query.PageNum,
		PageSize: query.PageSize,
	})
}

func (s *adminAuditUsageService) GetAPICallLog(c *gin.Context) {
	principal, ok := s.requireAdminPermission(c, auditlog.PermissionAuditRead)
	if !ok {
		return
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		s.respondError(c, err)
		return
	}

	record, err := s.repo.FindAPICallLog(c.Request.Context(), scope, c.Param("id"))
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, auditlog.APICallLogResponseFromRecord(record, s.redactor))
}

func (s *adminAuditUsageService) requireAdminPermission(c *gin.Context, permission string) (auth.Principal, bool) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return auth.Principal{}, false
	}
	if !isAdminPrincipal(principal) || !principal.HasPermission(permission) {
		httpx.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Forbidden.", nil)
		return auth.Principal{}, false
	}
	return principal, true
}

func (s *adminAuditUsageService) respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, auditlog.ErrValidation):
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
	case errors.Is(err, auditlog.ErrForbidden):
		httpx.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Forbidden.", nil)
	case errors.Is(err, auditlog.ErrNotFound):
		httpx.AbortWithError(c, http.StatusNotFound, "NOT_FOUND", "Resource not found.", nil)
	default:
		s.log.Error("admin audit usage read failed", slog.String("request_id", httpx.RequestIDFromContext(c)))
		httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
	}
}

func parseUsageRecordListQuery(c *gin.Context) (auditlog.UsageRecordListOptions, error) {
	page, err := parseAdminReadPageOptions(c)
	if err != nil {
		return auditlog.UsageRecordListOptions{}, err
	}
	taskID, err := cleanAdminReadFilter(c.Query("taskId"))
	if err != nil {
		return auditlog.UsageRecordListOptions{}, err
	}
	userID, err := cleanAdminReadFilter(c.Query("userId"))
	if err != nil {
		return auditlog.UsageRecordListOptions{}, err
	}
	projectID, err := cleanAdminReadFilter(c.Query("projectId"))
	if err != nil {
		return auditlog.UsageRecordListOptions{}, err
	}
	providerID, err := cleanAdminReadFilter(c.Query("providerId"))
	if err != nil {
		return auditlog.UsageRecordListOptions{}, err
	}
	modelID, err := cleanAdminReadFilter(c.Query("modelId"))
	if err != nil {
		return auditlog.UsageRecordListOptions{}, err
	}
	return auditlog.UsageRecordListOptions{
		PageOptions: page,
		TaskID:      taskID,
		UserID:      userID,
		ProjectID:   projectID,
		ProviderID:  providerID,
		ModelID:     modelID,
	}, nil
}

func parseUsageSummaryQuery(c *gin.Context) (auditlog.UsageSummaryOptions, error) {
	list, err := parseUsageRecordListQuery(c)
	if err != nil {
		return auditlog.UsageSummaryOptions{}, err
	}
	dimension := strings.ToLower(strings.TrimSpace(c.Query("dimension")))
	if dimension == "" {
		dimension = auditlog.UsageSummaryDimensionProvider
	}
	if !auditlog.ValidUsageSummaryDimension(dimension) {
		return auditlog.UsageSummaryOptions{}, auditlog.ErrValidation
	}
	return auditlog.UsageSummaryOptions{
		UsageRecordListOptions: list,
		Dimension:              dimension,
	}, nil
}

func parseOperationLogListQuery(c *gin.Context) (auditlog.OperationLogListOptions, error) {
	page, err := parseAdminReadPageOptions(c)
	if err != nil {
		return auditlog.OperationLogListOptions{}, err
	}
	actorUserID, err := cleanAdminReadFilter(c.Query("actorUserId"))
	if err != nil {
		return auditlog.OperationLogListOptions{}, err
	}
	action, err := cleanAdminReadFilter(c.Query("action"))
	if err != nil {
		return auditlog.OperationLogListOptions{}, err
	}
	resourceType, err := cleanAdminReadFilter(c.Query("resourceType"))
	if err != nil {
		return auditlog.OperationLogListOptions{}, err
	}
	resourceID, err := cleanAdminReadFilter(c.Query("resourceId"))
	if err != nil {
		return auditlog.OperationLogListOptions{}, err
	}
	return auditlog.OperationLogListOptions{
		PageOptions:  page,
		ActorUserID:  actorUserID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
	}, nil
}

func parseAPICallLogListQuery(c *gin.Context) (auditlog.APICallLogListOptions, error) {
	page, err := parseAdminReadPageOptions(c)
	if err != nil {
		return auditlog.APICallLogListOptions{}, err
	}
	taskID, err := cleanAdminReadFilter(c.Query("taskId"))
	if err != nil {
		return auditlog.APICallLogListOptions{}, err
	}
	userID, err := cleanAdminReadFilter(c.Query("userId"))
	if err != nil {
		return auditlog.APICallLogListOptions{}, err
	}
	projectID, err := cleanAdminReadFilter(c.Query("projectId"))
	if err != nil {
		return auditlog.APICallLogListOptions{}, err
	}
	providerID, err := cleanAdminReadFilter(c.Query("providerId"))
	if err != nil {
		return auditlog.APICallLogListOptions{}, err
	}
	modelID, err := cleanAdminReadFilter(c.Query("modelId"))
	if err != nil {
		return auditlog.APICallLogListOptions{}, err
	}
	imageType, err := cleanAdminReadFilter(c.Query("imageType"))
	if err != nil {
		return auditlog.APICallLogListOptions{}, err
	}
	status, err := cleanAPICallStatus(c.Query("status"))
	if err != nil {
		return auditlog.APICallLogListOptions{}, err
	}
	requestID, err := cleanAdminReadFilter(c.Query("requestId"))
	if err != nil {
		return auditlog.APICallLogListOptions{}, err
	}
	return auditlog.APICallLogListOptions{
		PageOptions: page,
		TaskID:      taskID,
		UserID:      userID,
		ProjectID:   projectID,
		ProviderID:  providerID,
		ModelID:     modelID,
		ImageType:   imageType,
		Status:      status,
		RequestID:   requestID,
	}, nil
}

func parseAdminReadPageOptions(c *gin.Context) (auditlog.PageOptions, error) {
	pageNum, err := parseBoundedPositiveInt(c.Query("pageNum"), adminReadDefaultPageNum, 0)
	if err != nil {
		return auditlog.PageOptions{}, err
	}
	pageSize, err := parseBoundedPositiveInt(c.Query("pageSize"), adminReadDefaultPageSize, adminReadMaxPageSize)
	if err != nil {
		return auditlog.PageOptions{}, err
	}
	sortBy := strings.TrimSpace(c.Query("sortBy"))
	if sortBy != "" && sortBy != "createdAt" {
		return auditlog.PageOptions{}, auditlog.ErrValidation
	}
	sortOrder := strings.ToLower(strings.TrimSpace(c.Query("sortOrder")))
	if sortOrder == "" {
		sortOrder = "desc"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		return auditlog.PageOptions{}, auditlog.ErrValidation
	}
	timeRange, err := parseAdminReadTimeRange(c)
	if err != nil {
		return auditlog.PageOptions{}, err
	}
	return auditlog.PageOptions{
		PageNum:   pageNum,
		PageSize:  pageSize,
		SortOrder: sortOrder,
		TimeRange: timeRange,
	}, nil
}

func parseBoundedPositiveInt(raw string, fallback int, maxValue int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, auditlog.ErrValidation
	}
	if maxValue > 0 && value > maxValue {
		return 0, auditlog.ErrValidation
	}
	return value, nil
}

func parseAdminReadTimeRange(c *gin.Context) (auditlog.TimeRange, error) {
	from, _, err := parseAdminReadTime(c.Query("createdAtFrom"), false)
	if err != nil {
		return auditlog.TimeRange{}, err
	}
	to, toExclusive, err := parseAdminReadTime(c.Query("createdAtTo"), true)
	if err != nil {
		return auditlog.TimeRange{}, err
	}
	if from != nil && to != nil {
		if toExclusive {
			if !from.Before(*to) {
				return auditlog.TimeRange{}, auditlog.ErrValidation
			}
		} else if from.After(*to) {
			return auditlog.TimeRange{}, auditlog.ErrValidation
		}
	}
	return auditlog.TimeRange{From: from, To: to, ToExclusive: toExclusive}, nil
}

func parseAdminReadTime(raw string, endOfDate bool) (*time.Time, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false, nil
	}
	if value, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		utc := value.UTC()
		return &utc, false, nil
	}
	if value, err := time.Parse("2006-01-02", raw); err == nil {
		utc := value.UTC()
		if endOfDate {
			utc = utc.AddDate(0, 0, 1)
			return &utc, true, nil
		}
		return &utc, false, nil
	}
	return nil, false, auditlog.ErrValidation
}

func cleanAdminReadFilter(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	if utf8.RuneCountInString(value) > adminReadMaxFilterRunes {
		return "", auditlog.ErrValidation
	}
	for _, item := range value {
		if item < 32 || item == 127 {
			return "", auditlog.ErrValidation
		}
	}
	return value, nil
}

func cleanAPICallStatus(raw string) (string, error) {
	value := strings.ToUpper(strings.TrimSpace(raw))
	if value == "" {
		return "", nil
	}
	switch value {
	case "SUCCESS", "FAILURE":
		return value, nil
	default:
		return "", auditlog.ErrValidation
	}
}

func isAdminPrincipal(principal auth.Principal) bool {
	for _, role := range principal.Roles {
		if role.Code == "admin" {
			return true
		}
	}
	return false
}
