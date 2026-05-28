package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	auditlog "github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/audit"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/queue"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/settings"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/task"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	diagnosticsDefaultWindowHours = 24
	diagnosticsMinWindowHours     = 1
	diagnosticsMaxWindowHours     = 720
	diagnosticsDefaultLimit       = 10
	diagnosticsMinLimit           = 1
	diagnosticsMaxLimit           = 50
	diagnosticsMaxMaintenanceOps  = 20
)

// adminDiagnosticsService provides read-only tenant-scoped diagnostics
// for admin users with audit:read permission. It aggregates task status,
// queue depth, Provider/API-call failure rates, storage usage, and
// recent maintenance results.
//
// Security: all responses are aggregate-only and sanitized. The service
// must not expose Redis keys, queue payloads, claim IDs, raw Provider
// payloads, operation/API log metadata, object keys, bucket names,
// MinIO URLs, signed URLs, image base64, Authorization, Cookie, JWT,
// or Provider secrets.
type adminDiagnosticsService struct {
	db             *gorm.DB
	log            *slog.Logger
	depthInspector queue.QueueDepthInspector
	settingsRepo   settings.Repository
}

func newAdminDiagnosticsService(db *gorm.DB, log *slog.Logger, depthInspector queue.QueueDepthInspector) *adminDiagnosticsService {
	if log == nil {
		log = slog.Default()
	}
	if depthInspector == nil {
		depthInspector = queue.NilQueueDepthInspector{}
	}
	return &adminDiagnosticsService{
		db:             db,
		log:            log,
		depthInspector: depthInspector,
		settingsRepo:   settings.NewRepository(db),
	}
}

func (s *adminDiagnosticsService) RegisterRoutes(group *gin.RouterGroup) {
	admin := group.Group("/admin")
	admin.GET("/diagnostics/summary", s.GetDiagnosticsSummary)
}

// diagnosticsSummaryResponse is the top-level response envelope for
// GET /admin/diagnostics/summary. All fields are aggregate or sanitized.
type diagnosticsSummaryResponse struct {
	Tasks       diagnosticsTaskSection        `json:"tasks"`
	Queue       queue.QueueDepth              `json:"queue"`
	Providers   diagnosticsProviderSection    `json:"providers"`
	Storage     diagnosticsStorageSection     `json:"storage"`
	Maintenance diagnosticsMaintenanceSection `json:"maintenance"`
	GeneratedAt string                        `json:"generatedAt"`
}

type diagnosticsTaskSection struct {
	StatusCounts   map[string]int64     `json:"statusCounts"`
	TotalTasks     int64                `json:"totalTasks"`
	RecentFailures []task.RecentFailure `json:"recentFailures"`
}

type diagnosticsProviderSection struct {
	WindowHours  int                             `json:"windowHours"`
	TotalCalls   int64                           `json:"totalCalls"`
	SuccessCount int64                           `json:"successCount"`
	FailureCount int64                           `json:"failureCount"`
	FailureRate  float64                         `json:"failureRate"`
	ByProvider   []diagnosticsProviderAggregate  `json:"byProvider"`
}

type diagnosticsProviderAggregate struct {
	ProviderID   string  `json:"providerId"`
	TotalCalls   int64   `json:"totalCalls"`
	SuccessCount int64   `json:"successCount"`
	FailureCount int64   `json:"failureCount"`
	FailureRate  float64 `json:"failureRate"`
}

type diagnosticsStorageSection struct {
	QuotaMaxBytes    *int64 `json:"quotaMaxBytes"`
	UsedBytes        int64  `json:"usedBytes"`
	TotalAssets      int64  `json:"totalAssets"`
	ActiveAssets     int64  `json:"activeAssets"`
	SoftDeletedAssets int64 `json:"softDeletedAssets"`
	PurgedAssets     int64  `json:"purgedAssets"`
}

type diagnosticsMaintenanceSection struct {
	RecentOperations []diagnosticsMaintenanceOp `json:"recentOperations"`
}

type diagnosticsMaintenanceOp struct {
	Action    string         `json:"action"`
	CreatedAt string         `json:"createdAt"`
	Summary   map[string]any `json:"summary"`
}

func (s *adminDiagnosticsService) GetDiagnosticsSummary(c *gin.Context) {
	principal, ok := s.requireAdminPermission(c)
	if !ok {
		return
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		s.respondInternalError(c)
		return
	}

	windowHours, limit, err := parseDiagnosticsParams(c)
	if err != nil {
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request.", nil)
		return
	}

	ctx := c.Request.Context()

	// Section: tasks
	taskAggregate, err := task.QueryTaskStatusAggregate(ctx, s.db, scope, limit)
	if err != nil {
		s.log.Error("diagnostics task aggregate failed",
			slog.String("request_id", httpx.RequestIDFromContext(c)))
		s.respondInternalError(c)
		return
	}

	// Section: queue (Redis unavailability is section-level, not endpoint-level)
	queueDepth := s.depthInspector.Inspect(ctx)

	// Section: providers
	providerSection, err := s.queryProviderFailureRates(ctx, scope, windowHours, limit)
	if err != nil {
		s.log.Error("diagnostics provider aggregate failed",
			slog.String("request_id", httpx.RequestIDFromContext(c)))
		s.respondInternalError(c)
		return
	}

	// Section: storage
	storageSection, err := s.queryStorageUsage(ctx, scope)
	if err != nil {
		s.log.Error("diagnostics storage query failed",
			slog.String("request_id", httpx.RequestIDFromContext(c)))
		s.respondInternalError(c)
		return
	}

	// Section: maintenance
	maintenanceSection, err := s.queryMaintenanceResults(ctx, scope, limit)
	if err != nil {
		s.log.Error("diagnostics maintenance query failed",
			slog.String("request_id", httpx.RequestIDFromContext(c)))
		s.respondInternalError(c)
		return
	}

	response := diagnosticsSummaryResponse{
		Tasks: diagnosticsTaskSection{
			StatusCounts:   taskAggregate.StatusCounts,
			TotalTasks:     taskAggregate.TotalTasks,
			RecentFailures: taskAggregate.RecentFailures,
		},
		Queue:       queueDepth,
		Providers:   providerSection,
		Storage:     storageSection,
		Maintenance: maintenanceSection,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}

	httpx.JSON(c, http.StatusOK, response)
}

func (s *adminDiagnosticsService) requireAdminPermission(c *gin.Context) (auth.Principal, bool) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
		return auth.Principal{}, false
	}
	if !isAdminPrincipal(principal) || !principal.HasPermission(auditlog.PermissionAuditRead) {
		httpx.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Forbidden.", nil)
		return auth.Principal{}, false
	}
	return principal, true
}

func (s *adminDiagnosticsService) respondInternalError(c *gin.Context) {
	httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
}

// parseDiagnosticsParams extracts and validates windowHours and limit query params.
func parseDiagnosticsParams(c *gin.Context) (windowHours int, limit int, err error) {
	windowHours = diagnosticsDefaultWindowHours
	limit = diagnosticsDefaultLimit

	if raw := strings.TrimSpace(c.Query("windowHours")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < diagnosticsMinWindowHours || parsed > diagnosticsMaxWindowHours {
			return 0, 0, auditlog.ErrValidation
		}
		windowHours = parsed
	}

	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < diagnosticsMinLimit || parsed > diagnosticsMaxLimit {
			return 0, 0, auditlog.ErrValidation
		}
		limit = parsed
	}

	return windowHours, limit, nil
}

// queryProviderFailureRates aggregates API call success/failure counts
// from api_call_logs within the time window. The top-level totals reflect
// the full tenant window (not limited), while byProvider is capped by limit.
// Does not expose request/response payloads, error messages, or raw metadata.
func (s *adminDiagnosticsService) queryProviderFailureRates(ctx context.Context, scope tenant.Scope, windowHours int, limit int) (diagnosticsProviderSection, error) {
	if s.db == nil {
		return diagnosticsProviderSection{}, database.ErrNilDB
	}

	windowStart := time.Now().UTC().Add(-time.Duration(windowHours) * time.Hour)

	// Query 1: tenant-wide totals for the window (no limit).
	type totalRow struct {
		TotalCalls   int64 `gorm:"column:total_calls"`
		FailureCount int64 `gorm:"column:failure_count"`
	}
	var totals totalRow
	if err := s.db.WithContext(ctx).
		Model(&database.APICallLog{}).
		Select("COUNT(*) AS total_calls, SUM(CASE WHEN status = 'FAILURE' THEN 1 ELSE 0 END) AS failure_count").
		Where("tenant_id = ? AND created_at >= ?", scope.ID(), windowStart).
		Scan(&totals).Error; err != nil {
		return diagnosticsProviderSection{}, err
	}

	// Query 2: per-provider breakdown (limited).
	type providerRow struct {
		ProviderID   string `gorm:"column:provider_id"`
		TotalCalls   int64  `gorm:"column:total_calls"`
		FailureCount int64  `gorm:"column:failure_count"`
	}
	var rows []providerRow
	if err := s.db.WithContext(ctx).
		Model(&database.APICallLog{}).
		Select("provider_id, COUNT(*) AS total_calls, SUM(CASE WHEN status = 'FAILURE' THEN 1 ELSE 0 END) AS failure_count").
		Where("tenant_id = ? AND created_at >= ?", scope.ID(), windowStart).
		Group("provider_id").
		Order("total_calls DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return diagnosticsProviderSection{}, err
	}

	byProvider := make([]diagnosticsProviderAggregate, 0, len(rows))
	for _, row := range rows {
		successCount := row.TotalCalls - row.FailureCount
		byProvider = append(byProvider, diagnosticsProviderAggregate{
			ProviderID:   row.ProviderID,
			TotalCalls:   row.TotalCalls,
			SuccessCount: successCount,
			FailureCount: row.FailureCount,
			FailureRate:  safeFailureRate(row.FailureCount, row.TotalCalls),
		})
	}

	return diagnosticsProviderSection{
		WindowHours:  windowHours,
		TotalCalls:   totals.TotalCalls,
		SuccessCount: totals.TotalCalls - totals.FailureCount,
		FailureCount: totals.FailureCount,
		FailureRate:  safeFailureRate(totals.FailureCount, totals.TotalCalls),
		ByProvider:   byProvider,
	}, nil
}

// queryStorageUsage queries storage quota and asset counts.
// Does not expose object keys, bucket names, MinIO URLs, or image bytes.
func (s *adminDiagnosticsService) queryStorageUsage(ctx context.Context, scope tenant.Scope) (diagnosticsStorageSection, error) {
	if s.db == nil {
		return diagnosticsStorageSection{}, database.ErrNilDB
	}

	quota, err := settings.LoadStorageQuotaWithUsage(ctx, s.settingsRepo, scope)
	if err != nil {
		return diagnosticsStorageSection{}, err
	}

	type assetCounts struct {
		Total       int64 `gorm:"column:total"`
		Active      int64 `gorm:"column:active"`
		SoftDeleted int64 `gorm:"column:soft_deleted"`
		Purged      int64 `gorm:"column:purged"`
	}
	var counts assetCounts
	if err := s.db.WithContext(ctx).
		Unscoped().
		Model(&database.ImageAsset{}).
		Select(
			"COUNT(*) AS total",
			"SUM(CASE WHEN deleted_at IS NULL THEN 1 ELSE 0 END) AS active",
			"SUM(CASE WHEN deleted_at IS NOT NULL AND purged_at IS NULL THEN 1 ELSE 0 END) AS soft_deleted",
			"SUM(CASE WHEN purged_at IS NOT NULL THEN 1 ELSE 0 END) AS purged",
		).
		Where("tenant_id = ?", scope.ID()).
		Scan(&counts).Error; err != nil {
		return diagnosticsStorageSection{}, err
	}

	return diagnosticsStorageSection{
		QuotaMaxBytes:     quota.MaxBytes,
		UsedBytes:         quota.UsedBytes,
		TotalAssets:       counts.Total,
		ActiveAssets:      counts.Active,
		SoftDeletedAssets: counts.SoftDeleted,
		PurgedAssets:      counts.Purged,
	}, nil
}

// maintenanceActions are the operation log actions considered maintenance.
var maintenanceActions = []string{
	"storage.orphan_cleanup",
	"log_retention.cleanup",
	"storage_retention.cleanup",
}

// queryMaintenanceResults returns sanitized recent maintenance operations.
// Only whitelisted aggregate fields from metadata_json are included.
// Raw metadata, object keys, bucket names, MinIO URLs are stripped.
func (s *adminDiagnosticsService) queryMaintenanceResults(ctx context.Context, scope tenant.Scope, limit int) (diagnosticsMaintenanceSection, error) {
	if s.db == nil {
		return diagnosticsMaintenanceSection{}, database.ErrNilDB
	}
	if limit <= 0 || limit > diagnosticsMaxMaintenanceOps {
		limit = diagnosticsDefaultLimit
	}

	var logs []database.OperationLog
	if err := s.db.WithContext(ctx).
		Model(&database.OperationLog{}).
		Where("tenant_id = ? AND action IN ?", scope.ID(), maintenanceActions).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&logs).Error; err != nil {
		return diagnosticsMaintenanceSection{}, err
	}

	ops := make([]diagnosticsMaintenanceOp, 0, len(logs))
	for _, log := range logs {
		summary := sanitizeMaintenanceMetadata(log.MetadataJSON)
		ops = append(ops, diagnosticsMaintenanceOp{
			Action:    log.Action,
			CreatedAt: log.CreatedAt.UTC().Format(time.RFC3339Nano),
			Summary:   summary,
		})
	}

	return diagnosticsMaintenanceSection{RecentOperations: ops}, nil
}

// sanitizeMaintenanceMetadata extracts only whitelisted safe fields from
// maintenance operation metadata. Only numeric, boolean, and safe status
// string values survive. Arrays and nested maps are dropped entirely.
func sanitizeMaintenanceMetadata(metadataJSON string) map[string]any {
	raw := strings.TrimSpace(metadataJSON)
	if raw == "" {
		return map[string]any{}
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return map[string]any{}
	}

	return filterSafeFields(decoded)
}

// Whitelisted maintenance metadata fields for safe diagnostics output.
// Only aggregate counts, boolean flags, and status indicators are allowed.
// The "errors" field is intentionally excluded because historical operation
// logs may contain unsanitized error strings with object keys, Authorization
// headers, or other sensitive content.
var maintenanceMetadataSafeFields = map[string]bool{
	"processed":      true,
	"deleted":        true,
	"failed":         true,
	"candidates":     true,
	"scanned":        true,
	"status":         true,
	"completedAt":    true,
	"totalProcessed": true,
	"totalDeleted":   true,
	"totalFailed":    true,
	"skipped":        true,
	"dryRun":         true,
}

// maintenanceMetadataSafeStatusValues are the allowed string values for
// whitelisted maintenance metadata fields. Any string not in this set
// is replaced with "redacted" to prevent leaking arbitrary content.
var maintenanceMetadataSafeStatusValues = map[string]bool{
	"completed": true, "failed": true, "skipped": true,
	"success": true, "error": true, "partial": true,
	"running": true, "pending": true, "dry_run": true,
}

// maintenanceMetadataMaxStringLen is the max rune length for status strings.
const maintenanceMetadataMaxStringLen = 64

// maintenanceSensitiveMarkers are substrings that must never appear in
// maintenance diagnostics output. If a string value contains any of these
// markers, it is replaced with "redacted".
var maintenanceSensitiveMarkers = []string{
	"bearer", "token", "authorization", "cookie", "jwt",
	"api_key", "apikey", "secret", "password",
	"minio", "s3://", "http://", "https://",
	"object_key", "objectkey", "bucket",
}

// filterSafeFields extracts only whitelisted keys with safe value types.
// - Only keys in maintenanceMetadataSafeFields are kept.
// - Non-whitelisted keys are completely dropped (no nested recursion
//   through unsafe keys, which would leak the key name itself).
// - Allowed value types: float64 (JSON number), bool, nil.
// - String values must pass sensitive-marker check and length cap.
// - Arrays and all other types are dropped.
func filterSafeFields(data map[string]any) map[string]any {
	safe := make(map[string]any)
	for key, value := range data {
		if !maintenanceMetadataSafeFields[key] {
			continue
		}
		switch v := value.(type) {
		case float64:
			safe[key] = v
		case bool:
			safe[key] = v
		case nil:
			safe[key] = v
		case string:
			safe[key] = sanitizeMaintenanceString(v)
		default:
			// Arrays, nested maps, and unknown types are dropped.
			// This prevents leaking array contents like object keys
			// or error messages.
		}
	}
	return safe
}

// sanitizeMaintenanceString validates a string value from maintenance
// metadata. Returns the original if it is a known safe status value,
// otherwise checks length and sensitive markers.
func sanitizeMaintenanceString(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	if maintenanceMetadataSafeStatusValues[lower] {
		return lower
	}
	if len([]rune(s)) > maintenanceMetadataMaxStringLen {
		return "redacted"
	}
	for _, marker := range maintenanceSensitiveMarkers {
		if strings.Contains(lower, marker) {
			return "redacted"
		}
	}
	return s
}

// safeFailureRate returns the failure rate as a float64 rounded to 4 decimal
// places. Returns 0.0 when total is zero to avoid division by zero.
func safeFailureRate(failures, total int64) float64 {
	if total <= 0 {
		return 0.0
	}
	rate := float64(failures) / float64(total)
	return math.Round(rate*10000) / 10000
}
