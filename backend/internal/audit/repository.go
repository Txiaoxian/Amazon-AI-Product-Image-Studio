package audit

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/usagecost"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return Repository{db: db}
}

func (r Repository) base(ctx context.Context, scope tenant.Scope) (*gorm.DB, error) {
	if r.db == nil {
		return nil, database.ErrNilDB
	}
	if !scope.Valid() {
		return nil, tenant.ErrMissingTenantID
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return r.db.WithContext(ctx), nil
}

func (r Repository) ListUsageRecords(ctx context.Context, scope tenant.Scope, options UsageRecordListOptions) ([]database.UsageRecord, int64, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, 0, err
	}

	query := usageRecordsQuery(db, scope, options)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []database.UsageRecord
	if err := query.
		Order(createdAtOrder("usage_records", options.SortOrder)).
		Limit(options.PageSize).
		Offset(pageOffset(options.PageNum, options.PageSize)).
		Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

func (r Repository) UsageSummary(ctx context.Context, scope tenant.Scope, options UsageSummaryOptions) ([]UsageSummaryRow, int64, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, 0, err
	}
	dimensionColumn, ok := usageSummaryDimensionColumn(options.Dimension)
	if !ok {
		return nil, 0, ErrValidation
	}

	selectSQL := fmt.Sprintf(
		"%s AS dimension_id, currency, COUNT(*) AS record_count, COALESCE(SUM(input_tokens), 0) AS input_tokens, COALESCE(SUM(output_tokens), 0) AS output_tokens, COALESCE(SUM(image_count), 0) AS image_count, MAX(created_at) AS latest_created_at",
		dimensionColumn,
	)
	groupBy := dimensionColumn + ", currency"

	var total int64
	groupedCount := usageRecordsQuery(db, scope, options.UsageRecordListOptions).Select("1").Group(groupBy)
	if err := db.Table("(?) AS usage_summary", groupedCount).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []UsageSummaryRow
	groupedRows := usageRecordsQuery(db, scope, options.UsageRecordListOptions).Select(selectSQL).Group(groupBy)
	if err := groupedRows.
		Order(usageSummaryOrder(options.SortOrder)).
		Limit(options.PageSize).
		Offset(pageOffset(options.PageNum, options.PageSize)).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	for index := range rows {
		estimatedCost, err := usageSummaryEstimatedCost(db, scope, options, dimensionColumn, rows[index].DimensionID, rows[index].Currency)
		if err != nil {
			return nil, 0, err
		}
		rows[index].EstimatedCost = estimatedCost
	}
	return rows, total, nil
}

func (r Repository) ListOperationLogs(ctx context.Context, scope tenant.Scope, options OperationLogListOptions) ([]OperationLogRecord, int64, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, 0, err
	}

	query := db.Table("operation_logs").Where("operation_logs.tenant_id = ?", scope.ID())
	query = applyTimeRange(query, "operation_logs.created_at", options.TimeRange)
	if options.ActorUserID != "" {
		query = query.Where("operation_logs.actor_user_id = ?", options.ActorUserID)
	}
	if options.Action != "" {
		query = query.Where("operation_logs.action = ?", options.Action)
	}
	if options.ResourceType != "" {
		query = query.Where("operation_logs.resource_type = ?", options.ResourceType)
	}
	if options.ResourceID != "" {
		query = query.Where("operation_logs.resource_id = ?", options.ResourceID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []OperationLogRecord
	if err := query.
		Joins("LEFT JOIN users AS actor ON actor.tenant_id = operation_logs.tenant_id AND actor.id = operation_logs.actor_user_id").
		Select(`
			operation_logs.id, operation_logs.tenant_id, operation_logs.actor_user_id,
			COALESCE(NULLIF(actor.display_name, ''), actor.email, '') AS actor_name,
			COALESCE(actor.email, '') AS actor_email,
			operation_logs.action, operation_logs.resource_type, operation_logs.resource_id,
			operation_logs.ip, operation_logs.user_agent, operation_logs.metadata_json, operation_logs.created_at
		`).
		Order(createdAtOrder("operation_logs", options.SortOrder)).
		Limit(options.PageSize).
		Offset(pageOffset(options.PageNum, options.PageSize)).
		Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

func (r Repository) ListAPICallLogs(ctx context.Context, scope tenant.Scope, options APICallLogListOptions) ([]APICallLogRecord, int64, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, 0, err
	}

	query := apiCallLogsQuery(db, scope, options)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []APICallLogRecord
	if err := query.
		Joins("LEFT JOIN ai_providers AS call_provider ON call_provider.tenant_id = api_call_logs.tenant_id AND call_provider.id = api_call_logs.provider_id").
		Joins("LEFT JOIN ai_models AS call_model ON call_model.tenant_id = api_call_logs.tenant_id AND call_model.id = api_call_logs.model_id").
		Select(`
			api_call_logs.*,
			COALESCE(call_provider.name, '') AS provider_name,
			COALESCE(NULLIF(call_model.display_name, ''), call_model.model_name, '') AS model_name
		`).
		Order(createdAtOrder("api_call_logs", options.SortOrder)).
		Limit(options.PageSize).
		Offset(pageOffset(options.PageNum, options.PageSize)).
		Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

func (r Repository) FindAPICallLog(ctx context.Context, scope tenant.Scope, id string) (APICallLogRecord, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return APICallLogRecord{}, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return APICallLogRecord{}, ErrValidation
	}

	var record APICallLogRecord
	err = db.Table("api_call_logs").
		Joins("LEFT JOIN ai_providers AS call_provider ON call_provider.tenant_id = api_call_logs.tenant_id AND call_provider.id = api_call_logs.provider_id").
		Joins("LEFT JOIN ai_models AS call_model ON call_model.tenant_id = api_call_logs.tenant_id AND call_model.id = api_call_logs.model_id").
		Select(`
			api_call_logs.*,
			COALESCE(call_provider.name, '') AS provider_name,
			COALESCE(NULLIF(call_model.display_name, ''), call_model.model_name, '') AS model_name
		`).
		Where("api_call_logs.tenant_id = ? AND api_call_logs.id = ?", scope.ID(), id).
		Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return APICallLogRecord{}, ErrNotFound
	}
	return record, err
}

func usageRecordsQuery(db *gorm.DB, scope tenant.Scope, options UsageRecordListOptions) *gorm.DB {
	query := db.Model(&database.UsageRecord{}).Where("usage_records.tenant_id = ?", scope.ID())
	query = applyTimeRange(query, "usage_records.created_at", options.TimeRange)
	if options.TaskID != "" {
		query = query.Where("usage_records.task_id = ?", options.TaskID)
	}
	if options.UserID != "" {
		query = query.Where("usage_records.user_id = ?", options.UserID)
	}
	if options.ProjectID != "" {
		query = query.Where("usage_records.project_id = ?", options.ProjectID)
	}
	if options.ProviderID != "" {
		query = query.Where("usage_records.provider_id = ?", options.ProviderID)
	}
	if options.ModelID != "" {
		query = query.Where("usage_records.model_id = ?", options.ModelID)
	}
	return query
}

func apiCallLogsQuery(db *gorm.DB, scope tenant.Scope, options APICallLogListOptions) *gorm.DB {
	query := db.Model(&database.APICallLog{}).Where("api_call_logs.tenant_id = ?", scope.ID())
	query = applyTimeRange(query, "api_call_logs.created_at", options.TimeRange)
	if options.ProjectID != "" || options.UserID != "" || options.ImageType != "" {
		query = query.Joins("JOIN generation_tasks AS gt ON gt.tenant_id = api_call_logs.tenant_id AND gt.id = api_call_logs.task_id")
	}
	if options.TaskID != "" {
		query = query.Where("api_call_logs.task_id = ?", options.TaskID)
	}
	if options.ProviderID != "" {
		query = query.Where("api_call_logs.provider_id = ?", options.ProviderID)
	}
	if options.ModelID != "" {
		query = query.Where("api_call_logs.model_id = ?", options.ModelID)
	}
	if options.Status != "" {
		query = query.Where("api_call_logs.status = ?", options.Status)
	}
	if options.RequestID != "" {
		query = query.Where("api_call_logs.request_id = ?", options.RequestID)
	}
	if options.ProjectID != "" {
		query = query.Where("gt.project_id = ?", options.ProjectID)
	}
	if options.UserID != "" {
		query = query.Where("gt.created_by = ?", options.UserID)
	}
	if options.ImageType != "" {
		query = query.Where("gt.image_type = ?", options.ImageType)
	}
	return query
}

func applyTimeRange(query *gorm.DB, column string, timeRange TimeRange) *gorm.DB {
	if timeRange.From != nil {
		query = query.Where(column+" >= ?", timeRange.From.UTC())
	}
	if timeRange.To != nil {
		if timeRange.ToExclusive {
			query = query.Where(column+" < ?", timeRange.To.UTC())
		} else {
			query = query.Where(column+" <= ?", timeRange.To.UTC())
		}
	}
	return query
}

func usageSummaryDimensionColumn(dimension string) (string, bool) {
	switch dimension {
	case UsageSummaryDimensionTenant:
		return "tenant_id", true
	case UsageSummaryDimensionUser:
		return "user_id", true
	case UsageSummaryDimensionProject:
		return "project_id", true
	case UsageSummaryDimensionProvider:
		return "provider_id", true
	case UsageSummaryDimensionModel:
		return "model_id", true
	default:
		return "", false
	}
}

func usageSummaryEstimatedCost(db *gorm.DB, scope tenant.Scope, options UsageSummaryOptions, dimensionColumn string, dimensionID string, currency string) (string, error) {
	var values []string
	err := usageRecordsQuery(db, scope, options.UsageRecordListOptions).
		Where(dimensionColumn+" = ? AND usage_records.currency = ?", dimensionID, currency).
		Pluck("usage_records.estimated_cost", &values).Error
	if err != nil {
		return "", err
	}
	return usagecost.SumDecimal8(values), nil
}

func createdAtOrder(table string, sortOrder string) string {
	direction := "DESC"
	if strings.EqualFold(sortOrder, "asc") {
		direction = "ASC"
	}
	return table + ".created_at " + direction + ", " + table + ".id " + direction
}

func usageSummaryOrder(sortOrder string) string {
	direction := "DESC"
	if strings.EqualFold(sortOrder, "asc") {
		direction = "ASC"
	}
	return "latest_created_at " + direction + ", dimension_id ASC, currency ASC"
}

func pageOffset(pageNum int, pageSize int) int {
	return (pageNum - 1) * pageSize
}
