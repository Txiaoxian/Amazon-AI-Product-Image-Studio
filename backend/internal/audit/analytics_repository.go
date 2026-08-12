package audit

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/usagecost"
	"gorm.io/gorm"
)

var analyticsLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

type taskMetricAggregate struct {
	TaskCount          int64
	TerminalTaskCount  int64
	SucceededTaskCount int64
	ActiveUserCount    int64
}

type analyticsUsageRow struct {
	UserID        string
	ProjectID     string
	ProviderID    string
	ModelID       string
	ImageType     string
	UserName      string
	UserEmail     string
	ProjectName   string
	ProviderName  string
	ModelName     string
	InputTokens   int64
	OutputTokens  int64
	ImageCount    int64
	EstimatedCost string
	Currency      string
	CostStatus    string
	CreatedAt     time.Time
}

type namedCountRow struct {
	DimensionID   string
	Name          string
	SecondaryName string
	TaskCount     int64
	TerminalCount int64
	SuccessCount  int64
}

func (r Repository) AnalyticsOverview(ctx context.Context, scope tenant.Scope, options AnalyticsOptions, includeAudit bool, now time.Time) (AnalyticsOverviewResponse, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return AnalyticsOverviewResponse{}, err
	}
	current, err := analyticsMetricSet(db, scope, options, includeAudit)
	if err != nil {
		return AnalyticsOverviewResponse{}, err
	}
	previous := AnalyticsMetricSet{}
	if options.Compare {
		previous, err = analyticsMetricSet(db, scope, options.Previous(), includeAudit)
		if err != nil {
			return AnalyticsOverviewResponse{}, err
		}
	}
	costs, currentCostRows, err := analyticsCostMetrics(db, scope, options)
	if err != nil {
		return AnalyticsOverviewResponse{}, err
	}
	if options.Compare {
		previousCosts, _, previousErr := analyticsCostMetrics(db, scope, options.Previous())
		if previousErr != nil {
			return AnalyticsOverviewResponse{}, previousErr
		}
		costs = mergePreviousCosts(costs, previousCosts)
	}
	trend, err := analyticsTaskTrend(db, scope, options, includeAudit)
	if err != nil {
		return AnalyticsOverviewResponse{}, err
	}
	costTrend := analyticsCostTrend(currentCostRows, options.Granularity)
	rankings, err := analyticsRankings(db, scope, options)
	if err != nil {
		return AnalyticsOverviewResponse{}, err
	}
	errorGroups := []AnalyticsErrorGroup{}
	if includeAudit {
		errorGroups, err = analyticsErrorGroups(db, scope, options)
		if err != nil {
			return AnalyticsOverviewResponse{}, err
		}
	}
	return AnalyticsOverviewResponse{
		Meta:        analyticsMeta(options, now),
		Current:     current,
		Previous:    previous,
		Changes:     metricChanges(current, previous, options.Compare),
		Costs:       costs,
		Trend:       trend,
		CostTrend:   costTrend,
		Rankings:    rankings,
		ErrorGroups: errorGroups,
	}, nil
}

func (r Repository) AnalyticsUsage(ctx context.Context, scope tenant.Scope, options AnalyticsOptions, now time.Time) (AnalyticsUsageResponse, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return AnalyticsUsageResponse{}, err
	}
	costs, rows, err := analyticsCostMetrics(db, scope, options)
	if err != nil {
		return AnalyticsUsageResponse{}, err
	}
	if options.Compare {
		previousCosts, _, previousErr := analyticsCostMetrics(db, scope, options.Previous())
		if previousErr != nil {
			return AnalyticsUsageResponse{}, previousErr
		}
		costs = mergePreviousCosts(costs, previousCosts)
	}
	breakdowns, err := analyticsUsageBreakdowns(db, scope, options, rows)
	if err != nil {
		return AnalyticsUsageResponse{}, err
	}
	outputCount, err := analyticsOutputCount(db, scope, options)
	if err != nil {
		return AnalyticsUsageResponse{}, err
	}
	unitCosts := analyticsUnitCosts(costs, outputCount)
	return AnalyticsUsageResponse{
		Meta:        analyticsMeta(options, now),
		Costs:       costs,
		OutputCount: outputCount,
		UnitCosts:   unitCosts,
		CostTrend:   analyticsCostTrend(rows, options.Granularity),
		Breakdowns:  breakdowns,
	}, nil
}

func analyticsUnitCosts(costs []AnalyticsCostMetric, outputCount int64) []AnalyticsUnitCost {
	result := make([]AnalyticsUnitCost, 0, len(costs))
	for _, cost := range costs {
		result = append(result, AnalyticsUnitCost{
			Currency: cost.Currency, Amount: usagecost.DivideDecimal8(cost.Amount, outputCount), OutputCount: outputCount, Available: outputCount > 0,
		})
	}
	return result
}

func analyticsMetricSet(db *gorm.DB, scope tenant.Scope, options AnalyticsOptions, includeLogin bool) (AnalyticsMetricSet, error) {
	query := analyticsTaskQuery(db, scope, options)
	var aggregate taskMetricAggregate
	err := query.Select(`
		COUNT(*) AS task_count,
		COALESCE(SUM(CASE WHEN gt.status IN ('SUCCEEDED','FAILED','CANCELLED','TIMED_OUT') THEN 1 ELSE 0 END), 0) AS terminal_task_count,
		COALESCE(SUM(CASE WHEN gt.status = 'SUCCEEDED' THEN 1 ELSE 0 END), 0) AS succeeded_task_count,
		COUNT(DISTINCT gt.created_by) AS active_user_count
	`).Scan(&aggregate).Error
	if err != nil {
		return AnalyticsMetricSet{}, err
	}
	outputCount, err := analyticsOutputCount(db, scope, options)
	if err != nil {
		return AnalyticsMetricSet{}, err
	}
	loginActive := int64(0)
	if includeLogin {
		loginActive, err = analyticsLoginActiveUsers(db, scope, options)
		if err != nil {
			return AnalyticsMetricSet{}, err
		}
	}
	p95, err := analyticsTaskPercentile(db, scope, options, 0.95)
	if err != nil {
		return AnalyticsMetricSet{}, err
	}
	successRate := rate(aggregate.SucceededTaskCount, aggregate.TerminalTaskCount)
	return AnalyticsMetricSet{
		TaskCount:            aggregate.TaskCount,
		OutputCount:          outputCount,
		TerminalTaskCount:    aggregate.TerminalTaskCount,
		SucceededTaskCount:   aggregate.SucceededTaskCount,
		TaskSuccessRate:      successRate,
		ActiveUserCount:      aggregate.ActiveUserCount,
		LoginActiveUserCount: loginActive,
		P95DurationMs:        p95,
	}, nil
}

func analyticsTaskQuery(db *gorm.DB, scope tenant.Scope, options AnalyticsOptions) *gorm.DB {
	query := db.Table("generation_tasks AS gt").Where("gt.tenant_id = ?", scope.ID())
	query = applyTimeRange(query, "gt.created_at", options.TimeRange)
	return applyAnalyticsTaskDimensions(query, options, "gt")
}

func applyAnalyticsTaskDimensions(query *gorm.DB, options AnalyticsOptions, alias string) *gorm.DB {
	column := func(name string) string { return alias + "." + name }
	if options.UserID != "" {
		query = query.Where(column("created_by")+" = ?", options.UserID)
	}
	if options.ProjectID != "" {
		query = query.Where(column("project_id")+" = ?", options.ProjectID)
	}
	if options.ProviderID != "" {
		query = query.Where(column("provider_id")+" = ?", options.ProviderID)
	}
	if options.ModelID != "" {
		query = query.Where(column("model_id")+" = ?", options.ModelID)
	}
	if options.Status != "" {
		query = query.Where(column("status")+" = ?", options.Status)
	}
	if options.ImageType != "" {
		query = query.Where(column("image_type")+" = ?", options.ImageType)
	}
	return query
}

func analyticsOutputCount(db *gorm.DB, scope tenant.Scope, options AnalyticsOptions) (int64, error) {
	query := db.Table("task_outputs AS output").
		Joins("JOIN generation_tasks AS gt ON gt.tenant_id = output.tenant_id AND gt.id = output.task_id").
		Where("output.tenant_id = ?", scope.ID())
	query = applyTimeRange(query, "output.created_at", options.TimeRange)
	query = applyAnalyticsTaskDimensions(query, options, "gt")
	var count int64
	err := query.Count(&count).Error
	return count, err
}

func analyticsLoginActiveUsers(db *gorm.DB, scope tenant.Scope, options AnalyticsOptions) (int64, error) {
	query := db.Table("operation_logs").Where("tenant_id = ? AND action = ?", scope.ID(), "auth.login")
	query = applyTimeRange(query, "created_at", options.TimeRange)
	if options.UserID != "" {
		query = query.Where("actor_user_id = ?", options.UserID)
	}
	var count int64
	err := query.Distinct("actor_user_id").Count(&count).Error
	return count, err
}

func analyticsTaskPercentile(db *gorm.DB, scope tenant.Scope, options AnalyticsOptions, percentile float64) (int64, error) {
	query := analyticsTaskQuery(db, scope, options).
		Where("gt.started_at IS NOT NULL AND gt.finished_at IS NOT NULL").
		Where("gt.status IN ?", []string{"SUCCEEDED", "FAILED", "CANCELLED", "TIMED_OUT"})
	var count int64
	if err := query.Count(&count).Error; err != nil || count == 0 {
		return 0, err
	}
	offset := int(math.Ceil(float64(count)*percentile)) - 1
	if offset < 0 {
		offset = 0
	}
	expression := "CAST(ROUND((julianday(gt.finished_at) - julianday(gt.started_at)) * 86400000) AS INTEGER)"
	if strings.EqualFold(db.Dialector.Name(), "mysql") {
		expression = "TIMESTAMPDIFF(MICROSECOND, gt.started_at, gt.finished_at) DIV 1000"
	}
	var row struct{ DurationMs int64 }
	err := query.Select(expression + " AS duration_ms").Order("duration_ms ASC").Limit(1).Offset(offset).Scan(&row).Error
	if row.DurationMs < 0 {
		row.DurationMs = 0
	}
	return row.DurationMs, err
}

func analyticsUsageQuery(db *gorm.DB, scope tenant.Scope, options AnalyticsOptions) *gorm.DB {
	query := db.Table("usage_records AS ur").Where("ur.tenant_id = ?", scope.ID())
	query = applyTimeRange(query, "ur.created_at", options.TimeRange)
	if options.UserID != "" {
		query = query.Where("ur.user_id = ?", options.UserID)
	}
	if options.ProjectID != "" {
		query = query.Where("ur.project_id = ?", options.ProjectID)
	}
	if options.ProviderID != "" {
		query = query.Where("ur.provider_id = ?", options.ProviderID)
	}
	if options.ModelID != "" {
		query = query.Where("ur.model_id = ?", options.ModelID)
	}
	if options.Status != "" || options.ImageType != "" {
		query = query.Joins("JOIN generation_tasks AS gt_filter ON gt_filter.tenant_id = ur.tenant_id AND gt_filter.id = ur.task_id")
		if options.Status != "" {
			query = query.Where("gt_filter.status = ?", options.Status)
		}
		if options.ImageType != "" {
			query = query.Where("gt_filter.image_type = ?", options.ImageType)
		}
	}
	return query
}

func analyticsUsageRows(db *gorm.DB, scope tenant.Scope, options AnalyticsOptions) ([]analyticsUsageRow, error) {
	query := analyticsUsageQuery(db, scope, options).
		Joins("LEFT JOIN users AS u ON u.tenant_id = ur.tenant_id AND u.id = ur.user_id").
		Joins("LEFT JOIN projects AS p ON p.tenant_id = ur.tenant_id AND p.id = ur.project_id").
		Joins("LEFT JOIN ai_providers AS ap ON ap.tenant_id = ur.tenant_id AND ap.id = ur.provider_id").
		Joins("LEFT JOIN ai_models AS am ON am.tenant_id = ur.tenant_id AND am.id = ur.model_id").
		Joins("LEFT JOIN generation_tasks AS gt_name ON gt_name.tenant_id = ur.tenant_id AND gt_name.id = ur.task_id")
	var rows []analyticsUsageRow
	err := query.Select(`
		ur.user_id, ur.project_id, ur.provider_id, ur.model_id,
		COALESCE(gt_name.image_type, '') AS image_type,
		COALESCE(u.display_name, '') AS user_name,
		COALESCE(u.email, '') AS user_email,
		COALESCE(p.name, '') AS project_name,
		COALESCE(ap.name, '') AS provider_name,
		COALESCE(NULLIF(am.display_name, ''), am.model_name, '') AS model_name,
		ur.input_tokens, ur.output_tokens, ur.image_count, ur.estimated_cost,
		ur.currency, COALESCE(ur.cost_status, 'LEGACY_UNKNOWN') AS cost_status, ur.created_at
	`).Scan(&rows).Error
	return rows, err
}

func analyticsCostMetrics(db *gorm.DB, scope tenant.Scope, options AnalyticsOptions) ([]AnalyticsCostMetric, []analyticsUsageRow, error) {
	rows, err := analyticsUsageRows(db, scope, options)
	if err != nil {
		return nil, nil, err
	}
	metrics := costMetricsFromRows(rows)
	return metrics, rows, nil
}

func costMetricsFromRows(rows []analyticsUsageRow) []AnalyticsCostMetric {
	type accumulator struct {
		values []string
		total  int64
		priced int64
	}
	byCurrency := map[string]*accumulator{}
	for _, row := range rows {
		currency := normalizeAnalyticsCurrency(row.Currency)
		current := byCurrency[currency]
		if current == nil {
			current = &accumulator{}
			byCurrency[currency] = current
		}
		current.values = append(current.values, row.EstimatedCost)
		current.total++
		if strings.EqualFold(row.CostStatus, usagecost.StatusCalculated) {
			current.priced++
		}
	}
	currencies := make([]string, 0, len(byCurrency))
	for currency := range byCurrency {
		currencies = append(currencies, currency)
	}
	sort.Strings(currencies)
	metrics := make([]AnalyticsCostMetric, 0, len(currencies))
	for _, currency := range currencies {
		value := byCurrency[currency]
		metrics = append(metrics, AnalyticsCostMetric{
			Currency:          currency,
			Amount:            usagecost.SumDecimal8(value.values),
			RecordCount:       value.total,
			PricedRecordCount: value.priced,
			PricingCoverage:   rate(value.priced, value.total),
		})
	}
	return metrics
}

func mergePreviousCosts(current []AnalyticsCostMetric, previous []AnalyticsCostMetric) []AnalyticsCostMetric {
	previousByCurrency := map[string]AnalyticsCostMetric{}
	for _, item := range previous {
		previousByCurrency[item.Currency] = item
	}
	seen := map[string]bool{}
	for index := range current {
		before := previousByCurrency[current[index].Currency]
		current[index].PreviousAmount = before.Amount
		current[index].ChangePercent = decimalChangePercent(current[index].Amount, before.Amount)
		seen[current[index].Currency] = true
	}
	for _, before := range previous {
		if seen[before.Currency] {
			continue
		}
		current = append(current, AnalyticsCostMetric{
			Currency:       before.Currency,
			Amount:         "0.00000000",
			PreviousAmount: before.Amount,
			ChangePercent:  decimalChangePercent("0", before.Amount),
		})
	}
	sort.Slice(current, func(i, j int) bool { return current[i].Currency < current[j].Currency })
	return current
}

func analyticsTaskTrend(db *gorm.DB, scope tenant.Scope, options AnalyticsOptions, includeLogin bool) ([]AnalyticsTrendPoint, error) {
	taskBucket := analyticsBucketExpression(db, "gt.created_at", options.Granularity)
	var taskRows []AnalyticsTrendPoint
	err := analyticsTaskQuery(db, scope, options).Select(fmt.Sprintf(`
		%s AS bucket,
		COUNT(*) AS task_count,
		COALESCE(SUM(CASE WHEN gt.status = 'SUCCEEDED' THEN 1 ELSE 0 END), 0) AS succeeded_count,
		COALESCE(SUM(CASE WHEN gt.status = 'FAILED' THEN 1 ELSE 0 END), 0) AS failed_count,
		COALESCE(SUM(CASE WHEN gt.status = 'TIMED_OUT' THEN 1 ELSE 0 END), 0) AS timed_out_count,
		COALESCE(SUM(CASE WHEN gt.status = 'CANCELLED' THEN 1 ELSE 0 END), 0) AS cancelled_count,
		COUNT(DISTINCT gt.created_by) AS active_user_count
	`, taskBucket)).Group("bucket").Order("bucket ASC").Scan(&taskRows).Error
	if err != nil {
		return nil, err
	}
	points := map[string]*AnalyticsTrendPoint{}
	for index := range taskRows {
		row := taskRows[index]
		points[row.Bucket] = &row
	}
	outputBucket := analyticsBucketExpression(db, "output.created_at", options.Granularity)
	var outputRows []struct {
		Bucket string
		Count  int64
	}
	outputQuery := db.Table("task_outputs AS output").
		Joins("JOIN generation_tasks AS gt ON gt.tenant_id = output.tenant_id AND gt.id = output.task_id").
		Where("output.tenant_id = ?", scope.ID())
	outputQuery = applyTimeRange(outputQuery, "output.created_at", options.TimeRange)
	outputQuery = applyAnalyticsTaskDimensions(outputQuery, options, "gt")
	if err := outputQuery.Select(outputBucket + " AS bucket, COUNT(*) AS count").Group("bucket").Scan(&outputRows).Error; err != nil {
		return nil, err
	}
	for _, row := range outputRows {
		point := points[row.Bucket]
		if point == nil {
			point = &AnalyticsTrendPoint{Bucket: row.Bucket}
			points[row.Bucket] = point
		}
		point.OutputCount = row.Count
	}
	if includeLogin {
		loginBucket := analyticsBucketExpression(db, "created_at", options.Granularity)
		var loginRows []struct {
			Bucket string
			Count  int64
		}
		loginQuery := db.Table("operation_logs").Where("tenant_id = ? AND action = ?", scope.ID(), "auth.login")
		loginQuery = applyTimeRange(loginQuery, "created_at", options.TimeRange)
		if options.UserID != "" {
			loginQuery = loginQuery.Where("actor_user_id = ?", options.UserID)
		}
		if err := loginQuery.Select(loginBucket + " AS bucket, COUNT(DISTINCT actor_user_id) AS count").Group("bucket").Scan(&loginRows).Error; err != nil {
			return nil, err
		}
		for _, row := range loginRows {
			point := points[row.Bucket]
			if point == nil {
				point = &AnalyticsTrendPoint{Bucket: row.Bucket}
				points[row.Bucket] = point
			}
			point.LoginActiveUsers = row.Count
		}
	}
	result := make([]AnalyticsTrendPoint, 0, len(points))
	for _, point := range points {
		result = append(result, *point)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Bucket < result[j].Bucket })
	return result, nil
}

func analyticsCostTrend(rows []analyticsUsageRow, granularity string) []AnalyticsCostTrendPoint {
	values := map[string]map[string][]string{}
	for _, row := range rows {
		bucket := analyticsBucket(row.CreatedAt, granularity)
		currency := normalizeAnalyticsCurrency(row.Currency)
		if values[bucket] == nil {
			values[bucket] = map[string][]string{}
		}
		values[bucket][currency] = append(values[bucket][currency], row.EstimatedCost)
	}
	result := make([]AnalyticsCostTrendPoint, 0)
	for bucket, currencies := range values {
		for currency, amounts := range currencies {
			result = append(result, AnalyticsCostTrendPoint{Bucket: bucket, Currency: currency, EstimatedCost: usagecost.SumDecimal8(amounts)})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Bucket == result[j].Bucket {
			return result[i].Currency < result[j].Currency
		}
		return result[i].Bucket < result[j].Bucket
	})
	return result
}

func analyticsRankings(db *gorm.DB, scope tenant.Scope, options AnalyticsOptions) ([]AnalyticsRankingItem, error) {
	dimensions := []string{AnalyticsGroupUser, AnalyticsGroupProject, AnalyticsGroupProvider, AnalyticsGroupModel}
	result := make([]AnalyticsRankingItem, 0, 20)
	usageRows, err := analyticsUsageRows(db, scope, options)
	if err != nil {
		return nil, err
	}
	for _, dimension := range dimensions {
		rows, err := analyticsNamedTaskCounts(db, scope, options, dimension, 5)
		if err != nil {
			return nil, err
		}
		outputCounts, err := analyticsOutputCountsByDimension(db, scope, options, dimension)
		if err != nil {
			return nil, err
		}
		costs := analyticsCostsByDimension(usageRows, dimension)
		for _, row := range rows {
			result = append(result, AnalyticsRankingItem{
				Dimension:     dimension,
				DimensionID:   row.DimensionID,
				Name:          fallbackAnalyticsName(row.Name, row.DimensionID),
				SecondaryName: row.SecondaryName,
				TaskCount:     row.TaskCount,
				OutputCount:   outputCounts[row.DimensionID],
				SuccessRate:   rate(row.SuccessCount, row.TerminalCount),
				Costs:         costs[row.DimensionID],
			})
		}
	}
	return result, nil
}

func analyticsNamedTaskCounts(db *gorm.DB, scope tenant.Scope, options AnalyticsOptions, dimension string, limit int) ([]namedCountRow, error) {
	column, nameExpression, secondaryExpression, joins, ok := analyticsDimensionSQL(dimension)
	if !ok {
		return nil, ErrValidation
	}
	query := analyticsTaskQuery(db, scope, options)
	for _, join := range joins {
		query = query.Joins(join)
	}
	var rows []namedCountRow
	selectSQL := fmt.Sprintf(`
		%s AS dimension_id,
		%s AS name,
		%s AS secondary_name,
		COUNT(*) AS task_count,
		COALESCE(SUM(CASE WHEN gt.status IN ('SUCCEEDED','FAILED','CANCELLED','TIMED_OUT') THEN 1 ELSE 0 END), 0) AS terminal_count,
		COALESCE(SUM(CASE WHEN gt.status = 'SUCCEEDED' THEN 1 ELSE 0 END), 0) AS success_count
	`, column, nameExpression, secondaryExpression)
	err := query.Select(selectSQL).Group(column + ", " + nameExpression + ", " + secondaryExpression).
		Order("task_count DESC").Limit(limit).Scan(&rows).Error
	return rows, err
}

func analyticsDimensionSQL(dimension string) (column string, nameExpression string, secondaryExpression string, joins []string, ok bool) {
	switch dimension {
	case AnalyticsGroupUser:
		return "gt.created_by", "COALESCE(NULLIF(u.display_name, ''), u.email, '')", "COALESCE(u.email, '')", []string{"LEFT JOIN users AS u ON u.tenant_id = gt.tenant_id AND u.id = gt.created_by"}, true
	case AnalyticsGroupProject:
		return "gt.project_id", "COALESCE(p.name, '')", "''", []string{"LEFT JOIN projects AS p ON p.tenant_id = gt.tenant_id AND p.id = gt.project_id"}, true
	case AnalyticsGroupProvider:
		return "gt.provider_id", "COALESCE(ap.name, '')", "''", []string{"LEFT JOIN ai_providers AS ap ON ap.tenant_id = gt.tenant_id AND ap.id = gt.provider_id"}, true
	case AnalyticsGroupModel:
		return "gt.model_id", "COALESCE(NULLIF(am.display_name, ''), am.model_name, '')", "COALESCE(am.model_name, '')", []string{"LEFT JOIN ai_models AS am ON am.tenant_id = gt.tenant_id AND am.id = gt.model_id"}, true
	default:
		return "", "", "", nil, false
	}
}

func analyticsOutputCountsByDimension(db *gorm.DB, scope tenant.Scope, options AnalyticsOptions, dimension string) (map[string]int64, error) {
	column, _, _, joins, ok := analyticsDimensionSQL(dimension)
	if !ok {
		return nil, ErrValidation
	}
	query := db.Table("task_outputs AS output").
		Joins("JOIN generation_tasks AS gt ON gt.tenant_id = output.tenant_id AND gt.id = output.task_id").
		Where("output.tenant_id = ?", scope.ID())
	query = applyTimeRange(query, "output.created_at", options.TimeRange)
	query = applyAnalyticsTaskDimensions(query, options, "gt")
	for _, join := range joins {
		query = query.Joins(join)
	}
	var rows []struct {
		DimensionID string
		Count       int64
	}
	err := query.Select(column + " AS dimension_id, COUNT(*) AS count").Group(column).Scan(&rows).Error
	result := map[string]int64{}
	for _, row := range rows {
		result[row.DimensionID] = row.Count
	}
	return result, err
}

func analyticsCostsByDimension(rows []analyticsUsageRow, dimension string) map[string][]AnalyticsCostMetric {
	grouped := map[string][]analyticsUsageRow{}
	for _, row := range rows {
		id := analyticsUsageDimensionID(row, dimension)
		if id != "" {
			grouped[id] = append(grouped[id], row)
		}
	}
	result := map[string][]AnalyticsCostMetric{}
	for id, items := range grouped {
		result[id] = costMetricsFromRows(items)
	}
	return result
}

func analyticsUsageBreakdowns(db *gorm.DB, scope tenant.Scope, options AnalyticsOptions, rows []analyticsUsageRow) ([]AnalyticsUsageBreakdown, error) {
	type accumulator struct {
		name         string
		rows         []analyticsUsageRow
		recordCount  int64
		inputTokens  int64
		outputTokens int64
		imageCount   int64
	}
	grouped := map[string]*accumulator{}
	for _, row := range rows {
		id := analyticsUsageDimensionID(row, options.GroupBy)
		if id == "" {
			continue
		}
		current := grouped[id]
		if current == nil {
			current = &accumulator{name: analyticsUsageDimensionName(row, options.GroupBy)}
			grouped[id] = current
		}
		current.rows = append(current.rows, row)
		current.recordCount++
		current.inputTokens += row.InputTokens
		current.outputTokens += row.OutputTokens
		current.imageCount += row.ImageCount
	}
	outputCounts := map[string]int64{}
	if options.GroupBy != AnalyticsGroupImageType {
		var err error
		outputCounts, err = analyticsOutputCountsByDimension(db, scope, options, options.GroupBy)
		if err != nil {
			return nil, err
		}
	} else {
		var rows []struct {
			DimensionID string
			Count       int64
		}
		query := db.Table("task_outputs AS output").
			Joins("JOIN generation_tasks AS gt ON gt.tenant_id = output.tenant_id AND gt.id = output.task_id").
			Where("output.tenant_id = ?", scope.ID())
		query = applyTimeRange(query, "output.created_at", options.TimeRange)
		query = applyAnalyticsTaskDimensions(query, options, "gt")
		if err := query.Select("COALESCE(gt.image_type, '') AS dimension_id, COUNT(*) AS count").Group("gt.image_type").Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			outputCounts[row.DimensionID] = row.Count
		}
	}
	result := make([]AnalyticsUsageBreakdown, 0, len(grouped))
	for id, value := range grouped {
		result = append(result, AnalyticsUsageBreakdown{
			Dimension:        options.GroupBy,
			DimensionID:      id,
			Name:             fallbackAnalyticsName(value.name, id),
			RecordCount:      value.recordCount,
			InputTokens:      value.inputTokens,
			OutputTokens:     value.outputTokens,
			BilledImageCount: value.imageCount,
			OutputCount:      outputCounts[id],
			Costs:            costMetricsFromRows(value.rows),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].BilledImageCount == result[j].BilledImageCount {
			return result[i].Name < result[j].Name
		}
		return result[i].BilledImageCount > result[j].BilledImageCount
	})
	return result, nil
}

func analyticsUsageDimensionID(row analyticsUsageRow, dimension string) string {
	switch dimension {
	case AnalyticsGroupUser:
		return row.UserID
	case AnalyticsGroupProject:
		return row.ProjectID
	case AnalyticsGroupProvider:
		return row.ProviderID
	case AnalyticsGroupModel:
		return row.ModelID
	case AnalyticsGroupImageType:
		return row.ImageType
	default:
		return ""
	}
}

func analyticsUsageDimensionName(row analyticsUsageRow, dimension string) string {
	switch dimension {
	case AnalyticsGroupUser:
		if row.UserName != "" {
			return row.UserName
		}
		return row.UserEmail
	case AnalyticsGroupProject:
		return row.ProjectName
	case AnalyticsGroupProvider:
		return row.ProviderName
	case AnalyticsGroupModel:
		return row.ModelName
	case AnalyticsGroupImageType:
		return row.ImageType
	default:
		return ""
	}
}

func (r Repository) AnalyticsUsers(ctx context.Context, scope tenant.Scope, options AnalyticsOptions, now time.Time) (AnalyticsUserPage, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return AnalyticsUserPage{}, err
	}
	query := db.Model(&database.User{}).Where("tenant_id = ?", scope.ID())
	if options.UserID != "" {
		query = query.Where("id = ?", options.UserID)
	}
	if options.Status != "" {
		query = query.Where("status = ?", options.Status)
	}
	if options.Search != "" {
		like := "%" + strings.ToLower(options.Search) + "%"
		query = query.Where("(LOWER(email) LIKE ? OR LOWER(display_name) LIKE ?)", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return AnalyticsUserPage{}, err
	}
	var users []database.User
	if err := query.Order("COALESCE(last_login_at, created_at) DESC, id ASC").Limit(options.PageSize).Offset(pageOffset(options.PageNum, options.PageSize)).Find(&users).Error; err != nil {
		return AnalyticsUserPage{}, err
	}
	records, err := analyticsUserRecords(db, scope, options, users)
	if err != nil {
		return AnalyticsUserPage{}, err
	}
	return AnalyticsUserPage{Meta: analyticsMeta(options, now), Records: records, Total: total, PageNum: options.PageNum, PageSize: options.PageSize}, nil
}

func analyticsUserRecords(db *gorm.DB, scope tenant.Scope, options AnalyticsOptions, users []database.User) ([]AnalyticsUserRecord, error) {
	if len(users) == 0 {
		return []AnalyticsUserRecord{}, nil
	}
	ids := make([]string, 0, len(users))
	for _, user := range users {
		ids = append(ids, user.ID)
	}
	var tasks []struct {
		UserID    string
		Status    string
		CreatedAt time.Time
	}
	taskQuery := db.Table("generation_tasks AS gt").Where("gt.tenant_id = ? AND gt.created_by IN ?", scope.ID(), ids)
	taskQuery = applyTimeRange(taskQuery, "gt.created_at", options.TimeRange)
	if options.ProjectID != "" {
		taskQuery = taskQuery.Where("gt.project_id = ?", options.ProjectID)
	}
	if options.ProviderID != "" {
		taskQuery = taskQuery.Where("gt.provider_id = ?", options.ProviderID)
	}
	if options.ModelID != "" {
		taskQuery = taskQuery.Where("gt.model_id = ?", options.ModelID)
	}
	if options.ImageType != "" {
		taskQuery = taskQuery.Where("gt.image_type = ?", options.ImageType)
	}
	if err := taskQuery.Select("gt.created_by AS user_id, gt.status, gt.created_at").Scan(&tasks).Error; err != nil {
		return nil, err
	}
	var history []struct {
		UserID        string
		FirstTaskUnix int64
		PreviousCount int64
	}
	previousFrom := options.PreviousRange.From
	previousTo := options.PreviousRange.To
	if previousFrom == nil || previousTo == nil {
		previousFrom = options.TimeRange.From
		previousTo = options.TimeRange.From
	}
	firstTaskExpression := "CAST(strftime('%s', MIN(gt.created_at)) AS INTEGER)"
	if strings.EqualFold(db.Dialector.Name(), "mysql") {
		firstTaskExpression = "UNIX_TIMESTAMP(MIN(gt.created_at))"
	}
	historyQuery := db.Table("generation_tasks AS gt").Where("gt.tenant_id = ? AND gt.created_by IN ?", scope.ID(), ids)
	historyQuery = applyAnalyticsUserTaskDimensions(historyQuery, options, "gt")
	if err := historyQuery.
		Select("gt.created_by AS user_id, "+firstTaskExpression+" AS first_task_unix, SUM(CASE WHEN gt.created_at >= ? AND gt.created_at < ? THEN 1 ELSE 0 END) AS previous_count", *previousFrom, *previousTo).
		Group("gt.created_by").Scan(&history).Error; err != nil {
		return nil, err
	}
	outputCounts := map[string]int64{}
	var outputs []struct {
		UserID string
		Count  int64
	}
	outputQuery := db.Table("task_outputs AS output").Joins("JOIN generation_tasks AS gt ON gt.tenant_id = output.tenant_id AND gt.id = output.task_id").
		Where("output.tenant_id = ? AND gt.created_by IN ?", scope.ID(), ids)
	outputQuery = applyTimeRange(outputQuery, "output.created_at", options.TimeRange)
	outputQuery = applyAnalyticsUserTaskDimensions(outputQuery, options, "gt")
	if err := outputQuery.Select("gt.created_by AS user_id, COUNT(*) AS count").Group("gt.created_by").Scan(&outputs).Error; err != nil {
		return nil, err
	}
	for _, row := range outputs {
		outputCounts[row.UserID] = row.Count
	}
	usageOptions := options
	usageOptions.UserID = ""
	usageOptions.Status = ""
	usageRows, err := analyticsUsageRows(db, scope, usageOptions)
	if err != nil {
		return nil, err
	}
	usageByUser := map[string][]analyticsUsageRow{}
	allowed := map[string]bool{}
	for _, id := range ids {
		allowed[id] = true
	}
	for _, row := range usageRows {
		if allowed[row.UserID] {
			usageByUser[row.UserID] = append(usageByUser[row.UserID], row)
		}
	}
	type userStats struct {
		taskCount  int64
		terminal   int64
		succeeded  int64
		activeDays map[string]bool
		lastTask   time.Time
	}
	stats := map[string]*userStats{}
	for _, task := range tasks {
		current := stats[task.UserID]
		if current == nil {
			current = &userStats{activeDays: map[string]bool{}}
			stats[task.UserID] = current
		}
		current.taskCount++
		current.activeDays[task.CreatedAt.In(analyticsLocation).Format("2006-01-02")] = true
		if task.CreatedAt.After(current.lastTask) {
			current.lastTask = task.CreatedAt
		}
		if analyticsTerminalStatus(task.Status) {
			current.terminal++
		}
		if task.Status == "SUCCEEDED" {
			current.succeeded++
		}
	}
	historyByUser := map[string]struct {
		first    time.Time
		previous int64
	}{}
	for _, row := range history {
		firstTaskAt := time.Time{}
		if row.FirstTaskUnix > 0 {
			firstTaskAt = time.Unix(row.FirstTaskUnix, 0).UTC()
		}
		historyByUser[row.UserID] = struct {
			first    time.Time
			previous int64
		}{first: firstTaskAt, previous: row.PreviousCount}
	}
	result := make([]AnalyticsUserRecord, 0, len(users))
	for _, user := range users {
		current := stats[user.ID]
		if current == nil {
			current = &userStats{activeDays: map[string]bool{}}
		}
		historyState := historyByUser[user.ID]
		lifecycle := analyticsLifecycle(current.taskCount > 0, historyState.previous > 0, historyState.first, options)
		lastLogin := ""
		if user.LastLoginAt != nil {
			lastLogin = formatTime(*user.LastLoginAt)
		}
		lastTask := ""
		if !current.lastTask.IsZero() {
			lastTask = formatTime(current.lastTask)
		}
		result = append(result, AnalyticsUserRecord{
			UserID:      user.ID,
			DisplayName: user.DisplayName,
			Email:       user.Email,
			Status:      user.Status,
			LastLoginAt: lastLogin,
			ActiveDays:  len(current.activeDays),
			TaskCount:   current.taskCount,
			OutputCount: outputCounts[user.ID],
			SuccessRate: rate(current.succeeded, current.terminal),
			Costs:       costMetricsFromRows(usageByUser[user.ID]),
			LastTaskAt:  lastTask,
			Lifecycle:   lifecycle,
		})
	}
	return result, nil
}

func (r Repository) AnalyticsTasks(ctx context.Context, scope tenant.Scope, options AnalyticsOptions, now time.Time) (AnalyticsTaskPage, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return AnalyticsTaskPage{}, err
	}
	query := analyticsTaskQuery(db, scope, options)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return AnalyticsTaskPage{}, err
	}
	var rows []struct {
		TaskID        string
		UserID        string
		UserName      string
		ProjectID     string
		ProjectName   string
		ProviderID    string
		ProviderName  string
		ModelID       string
		ModelName     string
		Type          string
		ImageType     string
		Status        string
		OutputCount   int64
		StartedAt     *time.Time
		FinishedAt    *time.Time
		EstimatedCost string
		Currency      string
		CostStatus    string
		ErrorCode     string
		ErrorMessage  string
		CreatedAt     time.Time
	}
	selectSQL := `
		gt.id AS task_id, gt.created_by AS user_id,
		COALESCE(NULLIF(u.display_name, ''), u.email, '') AS user_name,
		gt.project_id, COALESCE(p.name, '') AS project_name,
		gt.provider_id, COALESCE(ap.name, '') AS provider_name,
		gt.model_id, COALESCE(NULLIF(am.display_name, ''), am.model_name, '') AS model_name,
		gt.type, gt.image_type, gt.status, gt.started_at, gt.finished_at,
		(SELECT COUNT(*) FROM task_outputs o WHERE o.tenant_id = gt.tenant_id AND o.task_id = gt.id) AS output_count,
		COALESCE((SELECT ur.estimated_cost FROM usage_records ur WHERE ur.tenant_id = gt.tenant_id AND ur.task_id = gt.id ORDER BY ur.created_at DESC LIMIT 1), 0) AS estimated_cost,
		COALESCE((SELECT ur.currency FROM usage_records ur WHERE ur.tenant_id = gt.tenant_id AND ur.task_id = gt.id ORDER BY ur.created_at DESC LIMIT 1), 'USD') AS currency,
		COALESCE((SELECT ur.cost_status FROM usage_records ur WHERE ur.tenant_id = gt.tenant_id AND ur.task_id = gt.id ORDER BY ur.created_at DESC LIMIT 1), 'LEGACY_UNKNOWN') AS cost_status,
		gt.error_code, gt.error_message, gt.created_at
	`
	err = query.Joins("LEFT JOIN users AS u ON u.tenant_id = gt.tenant_id AND u.id = gt.created_by").
		Joins("LEFT JOIN projects AS p ON p.tenant_id = gt.tenant_id AND p.id = gt.project_id").
		Joins("LEFT JOIN ai_providers AS ap ON ap.tenant_id = gt.tenant_id AND ap.id = gt.provider_id").
		Joins("LEFT JOIN ai_models AS am ON am.tenant_id = gt.tenant_id AND am.id = gt.model_id").
		Select(selectSQL).Order("gt.created_at DESC, gt.id DESC").Limit(options.PageSize).Offset(pageOffset(options.PageNum, options.PageSize)).Scan(&rows).Error
	if err != nil {
		return AnalyticsTaskPage{}, err
	}
	records := make([]AnalyticsTaskRecord, 0, len(rows))
	for _, row := range rows {
		duration := int64(0)
		if row.StartedAt != nil && row.FinishedAt != nil && row.FinishedAt.After(*row.StartedAt) {
			duration = row.FinishedAt.Sub(*row.StartedAt).Milliseconds()
		}
		finished := ""
		if row.FinishedAt != nil {
			finished = formatTime(*row.FinishedAt)
		}
		records = append(records, AnalyticsTaskRecord{
			TaskID: row.TaskID, UserID: row.UserID, UserName: fallbackAnalyticsName(row.UserName, row.UserID),
			ProjectID: row.ProjectID, ProjectName: fallbackAnalyticsName(row.ProjectName, row.ProjectID),
			ProviderID: row.ProviderID, ProviderName: fallbackAnalyticsName(row.ProviderName, row.ProviderID),
			ModelID: row.ModelID, ModelName: fallbackAnalyticsName(row.ModelName, row.ModelID),
			Type: row.Type, ImageType: row.ImageType, Status: row.Status, OutputCount: row.OutputCount,
			DurationMs: duration, EstimatedCost: usagecost.FormatDecimal8(row.EstimatedCost), Currency: normalizeAnalyticsCurrency(row.Currency), CostStatus: row.CostStatus,
			ErrorCode: row.ErrorCode, ErrorMessage: row.ErrorMessage, CreatedAt: formatTime(row.CreatedAt), FinishedAt: finished,
		})
	}
	summary, err := analyticsMetricSet(db, scope, options, false)
	if err != nil {
		return AnalyticsTaskPage{}, err
	}
	return AnalyticsTaskPage{Meta: analyticsMeta(options, now), Summary: summary, Records: records, Total: total, PageNum: options.PageNum, PageSize: options.PageSize}, nil
}

func (r Repository) AnalyticsRequests(ctx context.Context, scope tenant.Scope, options AnalyticsOptions, now time.Time) (AnalyticsRequestsResponse, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return AnalyticsRequestsResponse{}, err
	}
	query := analyticsAPICallQuery(db, scope, options)
	var summary AnalyticsRequestSummary
	if err := query.Select(`
		COUNT(*) AS call_count,
		COALESCE(SUM(CASE WHEN api.status = 'SUCCESS' THEN 1 ELSE 0 END), 0) AS success_count,
		COALESCE(SUM(CASE WHEN api.status = 'FAILURE' THEN 1 ELSE 0 END), 0) AS failure_count
	`).Scan(&summary).Error; err != nil {
		return AnalyticsRequestsResponse{}, err
	}
	summary.SuccessRate = rate(summary.SuccessCount, summary.CallCount)
	summary.P50DurationMs, err = analyticsAPICallPercentile(db, scope, options, 0.50, "")
	if err != nil {
		return AnalyticsRequestsResponse{}, err
	}
	summary.P95DurationMs, err = analyticsAPICallPercentile(db, scope, options, 0.95, "")
	if err != nil {
		return AnalyticsRequestsResponse{}, err
	}
	bucket := analyticsBucketExpression(db, "api.created_at", options.Granularity)
	var trend []AnalyticsRequestTrendPoint
	if err := analyticsAPICallQuery(db, scope, options).Select(fmt.Sprintf(`
		%s AS bucket, COUNT(*) AS call_count,
		COALESCE(SUM(CASE WHEN api.status = 'SUCCESS' THEN 1 ELSE 0 END), 0) AS success_count,
		COALESCE(SUM(CASE WHEN api.status = 'FAILURE' THEN 1 ELSE 0 END), 0) AS failure_count
	`, bucket)).Group("bucket").Order("bucket ASC").Scan(&trend).Error; err != nil {
		return AnalyticsRequestsResponse{}, err
	}
	providers, err := analyticsProviderHealth(db, scope, options)
	if err != nil {
		return AnalyticsRequestsResponse{}, err
	}
	errors, err := analyticsErrorGroups(db, scope, options)
	if err != nil {
		return AnalyticsRequestsResponse{}, err
	}
	return AnalyticsRequestsResponse{Meta: analyticsMeta(options, now), Summary: summary, Trend: trend, Providers: providers, ErrorGroups: errors}, nil
}

func analyticsAPICallQuery(db *gorm.DB, scope tenant.Scope, options AnalyticsOptions) *gorm.DB {
	query := db.Table("api_call_logs AS api").Where("api.tenant_id = ?", scope.ID())
	query = applyTimeRange(query, "api.created_at", options.TimeRange)
	needsTask := options.UserID != "" || options.ProjectID != "" || options.ImageType != ""
	if needsTask {
		query = query.Joins("JOIN generation_tasks AS gt_api ON gt_api.tenant_id = api.tenant_id AND gt_api.id = api.task_id")
	}
	if options.UserID != "" {
		query = query.Where("gt_api.created_by = ?", options.UserID)
	}
	if options.ProjectID != "" {
		query = query.Where("gt_api.project_id = ?", options.ProjectID)
	}
	if options.ImageType != "" {
		query = query.Where("gt_api.image_type = ?", options.ImageType)
	}
	if options.ProviderID != "" {
		query = query.Where("api.provider_id = ?", options.ProviderID)
	}
	if options.ModelID != "" {
		query = query.Where("api.model_id = ?", options.ModelID)
	}
	if options.Status != "" {
		query = query.Where("api.status = ?", options.Status)
	}
	return query
}

func analyticsAPICallPercentile(db *gorm.DB, scope tenant.Scope, options AnalyticsOptions, percentile float64, providerID string) (int64, error) {
	query := analyticsAPICallQuery(db, scope, options)
	if providerID != "" {
		query = query.Where("api.provider_id = ?", providerID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil || count == 0 {
		return 0, err
	}
	offset := int(math.Ceil(float64(count)*percentile)) - 1
	var row struct{ DurationMs int64 }
	err := query.Select("api.duration_ms").Order("api.duration_ms ASC").Limit(1).Offset(offset).Scan(&row).Error
	return row.DurationMs, err
}

func analyticsProviderHealth(db *gorm.DB, scope tenant.Scope, options AnalyticsOptions) ([]AnalyticsProviderHealth, error) {
	var rows []struct {
		ProviderID   string
		ProviderName string
		CallCount    int64
		SuccessCount int64
	}
	err := analyticsAPICallQuery(db, scope, options).
		Joins("LEFT JOIN ai_providers AS ap_health ON ap_health.tenant_id = api.tenant_id AND ap_health.id = api.provider_id").
		Select(`
			api.provider_id, COALESCE(ap_health.name, '') AS provider_name,
			COUNT(*) AS call_count,
			COALESCE(SUM(CASE WHEN api.status = 'SUCCESS' THEN 1 ELSE 0 END), 0) AS success_count
		`).Group("api.provider_id, ap_health.name").Order("call_count DESC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	usageOptions := options
	usageOptions.Status = ""
	usageRows, err := analyticsUsageRows(db, scope, usageOptions)
	if err != nil {
		return nil, err
	}
	costs := analyticsCostsByDimension(usageRows, AnalyticsGroupProvider)
	result := make([]AnalyticsProviderHealth, 0, len(rows))
	for _, row := range rows {
		p95, percentileErr := analyticsAPICallPercentile(db, scope, options, 0.95, row.ProviderID)
		if percentileErr != nil {
			return nil, percentileErr
		}
		lastFailure := ""
		var lastFailureRow struct{ CreatedAt time.Time }
		lastFailureQuery := analyticsAPICallQuery(db, scope, options).
			Where("api.provider_id = ? AND api.status = 'FAILURE'", row.ProviderID).
			Select("api.created_at").Order("api.created_at DESC").Limit(1)
		if failureErr := lastFailureQuery.Scan(&lastFailureRow).Error; failureErr != nil {
			return nil, failureErr
		}
		if !lastFailureRow.CreatedAt.IsZero() {
			lastFailure = formatTime(lastFailureRow.CreatedAt)
		}
		result = append(result, AnalyticsProviderHealth{
			ProviderID: row.ProviderID, ProviderName: fallbackAnalyticsName(row.ProviderName, row.ProviderID), CallCount: row.CallCount,
			SuccessRate: rate(row.SuccessCount, row.CallCount), P95DurationMs: p95, LastFailureAt: lastFailure, Costs: costs[row.ProviderID],
		})
	}
	return result, nil
}

func applyAnalyticsUserTaskDimensions(query *gorm.DB, options AnalyticsOptions, alias string) *gorm.DB {
	column := func(name string) string { return alias + "." + name }
	if options.ProjectID != "" {
		query = query.Where(column("project_id")+" = ?", options.ProjectID)
	}
	if options.ProviderID != "" {
		query = query.Where(column("provider_id")+" = ?", options.ProviderID)
	}
	if options.ModelID != "" {
		query = query.Where(column("model_id")+" = ?", options.ModelID)
	}
	if options.ImageType != "" {
		query = query.Where(column("image_type")+" = ?", options.ImageType)
	}
	return query
}

func analyticsErrorGroups(db *gorm.DB, scope tenant.Scope, options AnalyticsOptions) ([]AnalyticsErrorGroup, error) {
	var rows []AnalyticsErrorGroup
	categoryExpression := `CASE
		WHEN UPPER(COALESCE(api.error_code, '')) LIKE '%TIMEOUT%' OR UPPER(COALESCE(api.error_code, '')) LIKE '%DEADLINE%' OR LOWER(COALESCE(api.error_message, '')) LIKE '%timeout%' OR LOWER(COALESCE(api.error_message, '')) LIKE '%deadline%' THEN 'PROVIDER_TIMEOUT'
		WHEN UPPER(COALESCE(api.error_code, '')) LIKE '%RATE%' OR UPPER(COALESCE(api.error_code, '')) LIKE '%LIMIT%' OR UPPER(COALESCE(api.error_code, '')) LIKE '%QUOTA%' OR LOWER(COALESCE(api.error_message, '')) LIKE '%429%' THEN 'RATE_LIMITED'
		WHEN UPPER(COALESCE(api.error_code, '')) LIKE '%AUTH%' OR UPPER(COALESCE(api.error_code, '')) LIKE '%CREDENTIAL%' OR UPPER(COALESCE(api.error_code, '')) LIKE '%UNAUTHORIZED%' OR UPPER(COALESCE(api.error_code, '')) LIKE '%FORBIDDEN%' THEN 'AUTHENTICATION_FAILED'
		WHEN UPPER(COALESCE(api.error_code, '')) LIKE '%PARAM%' OR UPPER(COALESCE(api.error_code, '')) LIKE '%INVALID%' OR UPPER(COALESCE(api.error_code, '')) LIKE '%UNSUPPORTED%' OR UPPER(COALESCE(api.error_code, '')) LIKE '%REQUEST%' THEN 'UNSUPPORTED_PARAMETER'
		WHEN UPPER(COALESCE(api.error_code, '')) LIKE '%TRANSPORT%' OR UPPER(COALESCE(api.error_code, '')) LIKE '%NETWORK%' OR UPPER(COALESCE(api.error_code, '')) LIKE '%CONNECTION%' THEN 'NETWORK_ERROR'
		WHEN UPPER(COALESCE(api.error_code, '')) LIKE '%HTTP%' OR UPPER(COALESCE(api.error_code, '')) LIKE '%PROVIDER%' OR UPPER(COALESCE(api.error_code, '')) LIKE '%SERVICE%' OR UPPER(COALESCE(api.error_code, '')) LIKE '%INTERNAL%' THEN 'PROVIDER_SERVICE_ERROR'
		ELSE 'OTHER'
	END`
	err := analyticsAPICallQuery(db, scope, options).Where("api.status = 'FAILURE'").
		Select(categoryExpression + " AS error_code, COUNT(DISTINCT api.task_id) AS count").
		Group(categoryExpression).Order("count DESC").Limit(10).Scan(&rows).Error
	return rows, err
}

func analyticsBucketExpression(db *gorm.DB, column string, granularity string) string {
	if strings.EqualFold(db.Dialector.Name(), "mysql") {
		switch granularity {
		case AnalyticsGranularityHour:
			return "DATE_FORMAT(DATE_ADD(" + column + ", INTERVAL 8 HOUR), '%Y-%m-%dT%H:00:00+08:00')"
		case AnalyticsGranularityWeek:
			local := "DATE_ADD(" + column + ", INTERVAL 8 HOUR)"
			return "DATE_FORMAT(DATE_SUB(" + local + ", INTERVAL WEEKDAY(" + local + ") DAY), '%Y-%m-%d')"
		default:
			return "DATE_FORMAT(DATE_ADD(" + column + ", INTERVAL 8 HOUR), '%Y-%m-%d')"
		}
	}
	switch granularity {
	case AnalyticsGranularityHour:
		return "strftime('%Y-%m-%dT%H:00:00+08:00', datetime(" + column + ", '+8 hours'))"
	case AnalyticsGranularityWeek:
		return "date(datetime(" + column + ", '+8 hours'), '-' || ((CAST(strftime('%w', datetime(" + column + ", '+8 hours')) AS INTEGER) + 6) % 7) || ' days')"
	default:
		return "strftime('%Y-%m-%d', datetime(" + column + ", '+8 hours'))"
	}
}

func analyticsBucket(value time.Time, granularity string) string {
	local := value.In(analyticsLocation)
	switch granularity {
	case AnalyticsGranularityHour:
		return local.Format("2006-01-02T15:00:00+08:00")
	case AnalyticsGranularityWeek:
		weekday := (int(local.Weekday()) + 6) % 7
		return local.AddDate(0, 0, -weekday).Format("2006-01-02")
	default:
		return local.Format("2006-01-02")
	}
}

func metricChanges(current AnalyticsMetricSet, previous AnalyticsMetricSet, compare bool) AnalyticsMetricChanges {
	if !compare {
		return AnalyticsMetricChanges{}
	}
	return AnalyticsMetricChanges{
		TaskCount:            numberChangePercent(float64(current.TaskCount), float64(previous.TaskCount)),
		OutputCount:          numberChangePercent(float64(current.OutputCount), float64(previous.OutputCount)),
		TaskSuccessRate:      numberChangePercent(current.TaskSuccessRate, previous.TaskSuccessRate),
		ActiveUserCount:      numberChangePercent(float64(current.ActiveUserCount), float64(previous.ActiveUserCount)),
		LoginActiveUserCount: numberChangePercent(float64(current.LoginActiveUserCount), float64(previous.LoginActiveUserCount)),
		P95DurationMs:        numberChangePercent(float64(current.P95DurationMs), float64(previous.P95DurationMs)),
	}
}

func numberChangePercent(current float64, previous float64) *float64 {
	if previous == 0 {
		return nil
	}
	value := (current - previous) / previous * 100
	return &value
}

func decimalChangePercent(current string, previous string) *float64 {
	currentValue, currentOK := new(bigFloat).parse(current)
	previousValue, previousOK := new(bigFloat).parse(previous)
	if !currentOK || !previousOK || previousValue == 0 {
		return nil
	}
	value := (currentValue - previousValue) / previousValue * 100
	return &value
}

type bigFloat struct{}

func (*bigFloat) parse(value string) (float64, bool) {
	var parsed float64
	_, err := fmt.Sscan(strings.TrimSpace(value), &parsed)
	return parsed, err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0)
}

func analyticsTerminalStatus(status string) bool {
	switch status {
	case "SUCCEEDED", "FAILED", "CANCELLED", "TIMED_OUT":
		return true
	default:
		return false
	}
}

func analyticsLifecycle(current bool, previous bool, firstTask time.Time, options AnalyticsOptions) string {
	if current && !firstTask.IsZero() && options.TimeRange.From != nil && !firstTask.Before(*options.TimeRange.From) {
		return "NEW"
	}
	if current && previous {
		return "RETURNING"
	}
	if current {
		return "RESURRECTED"
	}
	if previous {
		return "DORMANT"
	}
	return "INACTIVE"
}

func rate(numerator int64, denominator int64) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func normalizeAnalyticsCurrency(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if len(value) != 3 {
		return "USD"
	}
	return value
}

func fallbackAnalyticsName(name string, id string) string {
	if strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return "已删除或未知"
}
