package api

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	auditlog "github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/audit"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"github.com/gin-gonic/gin"
)

const (
	analyticsPermissionUserRead = "user:read"
	analyticsMaxRangeDays       = 366
	analyticsExportMaxRows      = 10000
)

var analyticsAPILocation = time.FixedZone("Asia/Shanghai", 8*60*60)

type analyticsQueryKind string

const (
	analyticsQueryTasks    analyticsQueryKind = "tasks"
	analyticsQueryRequests analyticsQueryKind = "requests"
	analyticsQueryUsers    analyticsQueryKind = "users"
)

func (s *adminAuditUsageService) AnalyticsOverview(c *gin.Context) {
	principal, ok := s.requireAdminPermissions(c, auditlog.PermissionUsageRead)
	if !ok {
		return
	}
	options, err := parseAnalyticsQuery(c, analyticsQueryTasks)
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	response, err := s.repo.AnalyticsOverview(c.Request.Context(), scope, options, principal.HasPermission(auditlog.PermissionAuditRead), time.Now().UTC())
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *adminAuditUsageService) AnalyticsUsage(c *gin.Context) {
	principal, ok := s.requireAdminPermissions(c, auditlog.PermissionUsageRead)
	if !ok {
		return
	}
	options, err := parseAnalyticsQuery(c, analyticsQueryTasks)
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	response, err := s.repo.AnalyticsUsage(c.Request.Context(), scope, options, time.Now().UTC())
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *adminAuditUsageService) AnalyticsUsers(c *gin.Context) {
	principal, ok := s.requireAdminPermissions(c, analyticsPermissionUserRead)
	if !ok {
		return
	}
	options, err := parseAnalyticsQuery(c, analyticsQueryUsers)
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	response, err := s.repo.AnalyticsUsers(c.Request.Context(), scope, options, time.Now().UTC())
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	if !principal.HasPermission(auditlog.PermissionUsageRead) {
		response.Records = redactAnalyticsUserCosts(response.Records)
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *adminAuditUsageService) AnalyticsUserDetail(c *gin.Context) {
	principal, ok := s.requireAdminPermissions(c, analyticsPermissionUserRead)
	if !ok {
		return
	}
	userID, err := cleanAdminReadFilter(c.Param("id"))
	if err != nil || userID == "" {
		s.respondAnalyticsError(c, auditlog.ErrValidation)
		return
	}
	options, err := parseAnalyticsQuery(c, analyticsQueryUsers)
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	options.UserID = userID
	options.Status = ""
	options.PageNum = 1
	options.PageSize = 1
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	now := time.Now().UTC()
	includeCost := principal.HasPermission(auditlog.PermissionUsageRead)
	users, err := s.repo.AnalyticsUsers(c.Request.Context(), scope, options, now)
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	if len(users.Records) == 0 {
		s.respondAnalyticsError(c, auditlog.ErrNotFound)
		return
	}
	if !includeCost {
		users.Records = redactAnalyticsUserCosts(users.Records)
	}
	includeAudit := principal.HasPermission(auditlog.PermissionAuditRead)
	overview, err := s.repo.AnalyticsOverview(c.Request.Context(), scope, options, includeAudit, now)
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	if !includeCost {
		overview.CostTrend = []auditlog.AnalyticsCostTrendPoint{}
	}
	projectOptions := options
	projectOptions.GroupBy = auditlog.AnalyticsGroupProject
	projects, err := s.repo.AnalyticsUsage(c.Request.Context(), scope, projectOptions, now)
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	if !includeCost {
		projects.Breakdowns = redactAnalyticsBreakdownCosts(projects.Breakdowns)
	}
	modelOptions := options
	modelOptions.GroupBy = auditlog.AnalyticsGroupModel
	models, err := s.repo.AnalyticsUsage(c.Request.Context(), scope, modelOptions, now)
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	if !includeCost {
		models.Breakdowns = redactAnalyticsBreakdownCosts(models.Breakdowns)
	}
	failedTasks := []auditlog.AnalyticsTaskRecord{}
	if includeAudit {
		failedOptions := options
		failedOptions.Status = "FAILED"
		failedOptions.PageSize = 5
		failed, taskErr := s.repo.AnalyticsTasks(c.Request.Context(), scope, failedOptions, now)
		if taskErr != nil {
			s.respondAnalyticsError(c, taskErr)
			return
		}
		failedTasks = s.redactAnalyticsTaskRecords(failed.Records, includeCost)
	}
	httpx.JSON(c, http.StatusOK, auditlog.AnalyticsUserDetailResponse{
		Meta: users.Meta, User: users.Records[0], Trend: overview.Trend, CostTrend: overview.CostTrend,
		Projects: projects.Breakdowns, Models: models.Breakdowns, FailedTasks: failedTasks, AuditVisible: includeAudit,
	})
}

func (s *adminAuditUsageService) AnalyticsTasks(c *gin.Context) {
	principal, ok := s.requireAdminPermissions(c, auditlog.PermissionAuditRead)
	if !ok {
		return
	}
	options, err := parseAnalyticsQuery(c, analyticsQueryTasks)
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	response, err := s.repo.AnalyticsTasks(c.Request.Context(), scope, options, time.Now().UTC())
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	response.Records = s.redactAnalyticsTaskRecords(response.Records, principal.HasPermission(auditlog.PermissionUsageRead))
	httpx.JSON(c, http.StatusOK, response)
}

func (s *adminAuditUsageService) AnalyticsRequests(c *gin.Context) {
	principal, ok := s.requireAdminPermissions(c, auditlog.PermissionAuditRead)
	if !ok {
		return
	}
	options, err := parseAnalyticsQuery(c, analyticsQueryRequests)
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	response, err := s.repo.AnalyticsRequests(c.Request.Context(), scope, options, time.Now().UTC())
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	if !principal.HasPermission(auditlog.PermissionUsageRead) {
		for index := range response.Providers {
			response.Providers[index].Costs = []auditlog.AnalyticsCostMetric{}
		}
	}
	httpx.JSON(c, http.StatusOK, response)
}

func (s *adminAuditUsageService) ExportAnalytics(c *gin.Context) {
	dataset := strings.ToLower(strings.TrimSpace(c.Param("dataset")))
	permissions := []string{auditlog.PermissionUsageRead}
	kind := analyticsQueryTasks
	switch dataset {
	case "usage":
	case "users":
		permissions = []string{analyticsPermissionUserRead}
		kind = analyticsQueryUsers
	case "tasks":
		permissions = []string{auditlog.PermissionAuditRead}
	case "requests":
		permissions = []string{auditlog.PermissionAuditRead}
		kind = analyticsQueryRequests
	default:
		s.respondAnalyticsError(c, auditlog.ErrNotFound)
		return
	}
	principal, ok := s.requireAdminPermissions(c, permissions...)
	if !ok {
		return
	}
	options, err := parseAnalyticsQuery(c, kind)
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	options.PageNum = 1
	options.PageSize = analyticsExportMaxRows
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		s.respondAnalyticsError(c, err)
		return
	}
	filename := analyticsExportFilename(dataset, options)
	var content bytes.Buffer
	_, _ = content.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(&content)
	if err := s.writeAnalyticsCSV(c, writer, dataset, scope, options, principal); err != nil {
		s.log.Error("管理统计导出失败")
		s.respondAnalyticsError(c, err)
		return
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		s.log.Error("管理统计导出写入失败")
		s.respondAnalyticsError(c, err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(filename))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", content.Bytes())
}

func (s *adminAuditUsageService) writeAnalyticsCSV(c *gin.Context, writer *csv.Writer, dataset string, scope tenant.Scope, options auditlog.AnalyticsOptions, principal auth.Principal) error {
	now := time.Now().UTC()
	switch dataset {
	case "usage":
		response, err := s.repo.AnalyticsUsage(c.Request.Context(), scope, options, now)
		if err != nil {
			return err
		}
		if err := writer.Write([]string{"分组维度", "名称", "计费用量记录数", "输入计费用量（Token）", "输出计费用量（Token）", "计费图片数", "实际出图张数", "币种", "预计费用", "定价覆盖率"}); err != nil {
			return err
		}
		for _, row := range response.Breakdowns {
			if len(row.Costs) == 0 {
				if err := writer.Write(analyticsUsageCSVRow(row, auditlog.AnalyticsCostMetric{})); err != nil {
					return err
				}
				continue
			}
			for _, cost := range row.Costs {
				if err := writer.Write(analyticsUsageCSVRow(row, cost)); err != nil {
					return err
				}
			}
		}
	case "users":
		response, err := s.repo.AnalyticsUsers(c.Request.Context(), scope, options, now)
		if err != nil {
			return err
		}
		if err := writer.Write([]string{"用户名称", "邮箱", "账号状态", "生命周期", "活跃天数", "生图任务数", "实际出图张数", "任务成功率", "币种", "预计费用", "最近登录时间（北京时间）", "最近生图时间（北京时间）", "用户ID（技术标识）"}); err != nil {
			return err
		}
		for _, row := range response.Records {
			costs := row.Costs
			if !principal.HasPermission(auditlog.PermissionUsageRead) || len(costs) == 0 {
				costs = []auditlog.AnalyticsCostMetric{{}}
			}
			for _, cost := range costs {
				if err := writer.Write([]string{row.DisplayName, row.Email, analyticsEntityStatusLabel(row.Status), analyticsLifecycleLabel(row.Lifecycle), strconv.Itoa(row.ActiveDays), strconv.FormatInt(row.TaskCount, 10), strconv.FormatInt(row.OutputCount, 10), formatExportRate(row.SuccessRate), analyticsCurrencyLabel(cost.Currency), cost.Amount, formatAnalyticsCSVDateTime(row.LastLoginAt), formatAnalyticsCSVDateTime(row.LastTaskAt), row.UserID}); err != nil {
					return err
				}
			}
		}
	case "tasks":
		response, err := s.repo.AnalyticsTasks(c.Request.Context(), scope, options, now)
		if err != nil {
			return err
		}
		records := s.redactAnalyticsTaskRecords(response.Records, principal.HasPermission(auditlog.PermissionUsageRead))
		if err := writer.Write([]string{"创建时间（北京时间）", "用户", "项目", "中转站", "模型", "图片类型", "任务状态", "实际出图张数", "任务耗时", "币种", "预计费用", "错误码（技术详情）", "任务ID（技术标识）"}); err != nil {
			return err
		}
		for _, row := range records {
			if err := writer.Write([]string{formatAnalyticsCSVDateTime(row.CreatedAt), row.UserName, row.ProjectName, row.ProviderName, row.ModelName, analyticsImageTypeLabel(row.ImageType), analyticsTaskStatusLabel(row.Status), strconv.FormatInt(row.OutputCount, 10), formatAnalyticsDuration(row.DurationMs), analyticsCurrencyLabel(row.Currency), row.EstimatedCost, row.ErrorCode, row.TaskID}); err != nil {
				return err
			}
		}
	case "requests":
		response, err := s.repo.AnalyticsRequests(c.Request.Context(), scope, options, now)
		if err != nil {
			return err
		}
		if err := writer.Write([]string{"中转站", "模型调用次数", "调用成功率", "95%的调用耗时不超过", "最近异常时间（北京时间）", "币种", "预计费用", "中转站ID（技术标识）"}); err != nil {
			return err
		}
		for _, row := range response.Providers {
			costs := row.Costs
			if !principal.HasPermission(auditlog.PermissionUsageRead) || len(costs) == 0 {
				costs = []auditlog.AnalyticsCostMetric{{}}
			}
			for _, cost := range costs {
				if err := writer.Write([]string{row.ProviderName, strconv.FormatInt(row.CallCount, 10), formatExportRate(row.SuccessRate), formatAnalyticsDuration(row.P95DurationMs), formatAnalyticsCSVDateTime(row.LastFailureAt), analyticsCurrencyLabel(cost.Currency), cost.Amount, row.ProviderID}); err != nil {
					return err
				}
			}
		}
	}
	return writer.Error()
}

func analyticsUsageCSVRow(row auditlog.AnalyticsUsageBreakdown, cost auditlog.AnalyticsCostMetric) []string {
	name := row.Name
	if row.Dimension == auditlog.AnalyticsGroupImageType {
		name = analyticsImageTypeLabel(row.DimensionID)
	}
	return []string{analyticsDimensionLabel(row.Dimension), name, strconv.FormatInt(row.RecordCount, 10), strconv.FormatInt(row.InputTokens, 10), strconv.FormatInt(row.OutputTokens, 10), strconv.FormatInt(row.BilledImageCount, 10), strconv.FormatInt(row.OutputCount, 10), analyticsCurrencyLabel(cost.Currency), cost.Amount, formatExportRate(cost.PricingCoverage)}
}

func formatExportRate(value float64) string {
	return strconv.FormatFloat(value*100, 'f', 2, 64) + "%"
}

func analyticsDimensionLabel(value string) string {
	if label, ok := map[string]string{"user": "用户", "project": "项目", "provider": "中转站", "model": "模型", "imageType": "图片类型"}[value]; ok {
		return label
	}
	return "其他维度"
}

func analyticsEntityStatusLabel(value string) string {
	if label, ok := map[string]string{"ACTIVE": "正常", "ENABLED": "已启用", "DISABLED": "已停用", "ARCHIVED": "已归档"}[strings.ToUpper(strings.TrimSpace(value))]; ok {
		return label
	}
	return "未知状态"
}

func analyticsLifecycleLabel(value string) string {
	if label, ok := map[string]string{"NEW": "新活跃", "RETURNING": "持续活跃", "RESURRECTED": "回流", "DORMANT": "沉默", "INACTIVE": "未活跃"}[strings.ToUpper(strings.TrimSpace(value))]; ok {
		return label
	}
	return "状态待确认"
}

func analyticsTaskStatusLabel(value string) string {
	if label, ok := map[string]string{"QUEUED": "排队中", "RUNNING": "生成中", "RETRYING": "正在重试", "SUCCEEDED": "已完成", "FAILED": "失败", "TIMED_OUT": "已超时", "CANCELLED": "已取消"}[strings.ToUpper(strings.TrimSpace(value))]; ok {
		return label
	}
	return "未知任务状态"
}

func analyticsImageTypeLabel(value string) string {
	if label, ok := map[string]string{"MAIN": "商品主图", "A_PLUS": "A+ 图片", "SCENE": "场景图", "DETAIL": "细节图", "DIMENSION": "尺寸图", "SELLING_POINT": "卖点图", "PROMOTION": "宣传图", "COMPARISON": "对比图", "WHITE_BACKGROUND": "白底图", "LIFESTYLE": "生活方式图", "INFOGRAPHIC": "信息图", "PACKAGING": "包装图", "OTHER": "其他图片"}[strings.ToUpper(strings.TrimSpace(value))]; ok {
		return label
	}
	return "其他图片类型"
}

func analyticsCurrencyLabel(value string) string {
	if label, ok := map[string]string{"USD": "美元", "CNY": "人民币", "RMB": "人民币", "EUR": "欧元", "GBP": "英镑", "JPY": "日元", "HKD": "港币"}[strings.ToUpper(strings.TrimSpace(value))]; ok {
		return label
	}
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "其他币种"
}

func formatAnalyticsDuration(durationMs int64) string {
	if durationMs < 0 {
		return "暂无"
	}
	if durationMs < 1000 {
		return strconv.FormatInt(durationMs, 10) + "毫秒"
	}
	seconds := float64(durationMs) / 1000
	if seconds < 60 {
		return trimAnalyticsDecimal(seconds) + "秒"
	}
	minutes := seconds / 60
	if minutes < 60 {
		return trimAnalyticsDecimal(minutes) + "分钟"
	}
	return trimAnalyticsDecimal(minutes/60) + "小时"
}

func trimAnalyticsDecimal(value float64) string {
	return strings.TrimSuffix(strconv.FormatFloat(value, 'f', 1, 64), ".0")
}

func formatAnalyticsCSVDateTime(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return "时间待确认"
	}
	return parsed.In(analyticsAPILocation).Format("2006年1月2日 15:04")
}

func analyticsExportFilename(dataset string, options auditlog.AnalyticsOptions) string {
	names := map[string]string{"usage": "用量与费用", "users": "用户与活跃", "tasks": "生图任务", "requests": "模型调用"}
	from := "开始"
	to := "结束"
	if options.TimeRange.From != nil {
		from = options.TimeRange.From.In(analyticsAPILocation).Format("20060102")
	}
	if options.TimeRange.To != nil {
		to = options.TimeRange.To.In(analyticsAPILocation).Add(-time.Nanosecond).Format("20060102")
	}
	return fmt.Sprintf("%s_%s-%s.csv", names[dataset], from, to)
}

func (s *adminAuditUsageService) redactAnalyticsTaskRecords(records []auditlog.AnalyticsTaskRecord, includeCost bool) []auditlog.AnalyticsTaskRecord {
	for index := range records {
		records[index].ErrorCode = s.redactor.RedactString(records[index].ErrorCode)
		records[index].ErrorMessage = s.redactor.SanitizeErrorMessage(records[index].ErrorMessage)
		if !includeCost {
			records[index].EstimatedCost = ""
			records[index].Currency = ""
			records[index].CostStatus = ""
		}
	}
	return records
}

func redactAnalyticsUserCosts(records []auditlog.AnalyticsUserRecord) []auditlog.AnalyticsUserRecord {
	for index := range records {
		records[index].Costs = []auditlog.AnalyticsCostMetric{}
	}
	return records
}

func redactAnalyticsBreakdownCosts(records []auditlog.AnalyticsUsageBreakdown) []auditlog.AnalyticsUsageBreakdown {
	for index := range records {
		records[index].Costs = []auditlog.AnalyticsCostMetric{}
		records[index].InputTokens = 0
		records[index].OutputTokens = 0
		records[index].BilledImageCount = 0
	}
	return records
}

func (s *adminAuditUsageService) requireAdminPermissions(c *gin.Context, permissions ...string) (auth.Principal, bool) {
	principal, ok := auth.PrincipalFromGin(c)
	if !ok {
		httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "请先登录后再访问管理控制台。", nil)
		return auth.Principal{}, false
	}
	if !isAdminPrincipal(principal) {
		httpx.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "当前账号没有访问管理控制台的权限。", nil)
		return auth.Principal{}, false
	}
	for _, permission := range permissions {
		if !principal.HasPermission(permission) {
			httpx.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "当前账号缺少查看此模块所需的权限。", nil)
			return auth.Principal{}, false
		}
	}
	return principal, true
}

func (s *adminAuditUsageService) respondAnalyticsError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, auditlog.ErrValidation):
		httpx.AbortWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "筛选条件无效，请检查时间范围或筛选项。", nil)
	case errors.Is(err, auditlog.ErrForbidden):
		httpx.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "当前账号没有查看此数据的权限。", nil)
	case errors.Is(err, auditlog.ErrNotFound):
		httpx.AbortWithError(c, http.StatusNotFound, "NOT_FOUND", "没有找到对应的数据。", nil)
	default:
		s.log.Error("管理统计读取失败")
		httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "统计数据暂时无法加载，请稍后重试。", nil)
	}
}

func parseAnalyticsQuery(c *gin.Context, kind analyticsQueryKind) (auditlog.AnalyticsOptions, error) {
	now := time.Now().UTC()
	timeRange, previous, err := parseAnalyticsTimeRanges(c, now)
	if err != nil {
		return auditlog.AnalyticsOptions{}, err
	}
	pageNum, err := parseBoundedPositiveInt(c.Query("pageNum"), adminReadDefaultPageNum, 0)
	if err != nil {
		return auditlog.AnalyticsOptions{}, err
	}
	pageSize, err := parseBoundedPositiveInt(c.Query("pageSize"), adminReadDefaultPageSize, adminReadMaxPageSize)
	if err != nil {
		return auditlog.AnalyticsOptions{}, err
	}
	granularity := strings.ToLower(strings.TrimSpace(c.Query("granularity")))
	if granularity == "" {
		granularity = auditlog.AnalyticsGranularityDay
	}
	if granularity != auditlog.AnalyticsGranularityHour && granularity != auditlog.AnalyticsGranularityDay && granularity != auditlog.AnalyticsGranularityWeek {
		return auditlog.AnalyticsOptions{}, auditlog.ErrValidation
	}
	compare, err := parseAnalyticsBool(c.Query("compare"), true)
	if err != nil {
		return auditlog.AnalyticsOptions{}, err
	}
	groupBy := strings.TrimSpace(c.Query("groupBy"))
	if groupBy == "" {
		groupBy = auditlog.AnalyticsGroupProvider
	}
	if !validAnalyticsGroup(groupBy) {
		return auditlog.AnalyticsOptions{}, auditlog.ErrValidation
	}
	status, err := cleanAnalyticsStatus(c.Query("status"), kind)
	if err != nil {
		return auditlog.AnalyticsOptions{}, err
	}
	filters := make([]string, 0, 7)
	for _, raw := range []string{c.Query("userId"), c.Query("projectId"), c.Query("providerId"), c.Query("modelId"), c.Query("imageType"), c.Query("search")} {
		value, cleanErr := cleanAdminReadFilter(raw)
		if cleanErr != nil {
			return auditlog.AnalyticsOptions{}, cleanErr
		}
		filters = append(filters, value)
	}
	return auditlog.AnalyticsOptions{
		TimeRange: timeRange, PreviousRange: previous, Granularity: granularity, Compare: compare,
		UserID: filters[0], ProjectID: filters[1], ProviderID: filters[2], ModelID: filters[3], ImageType: filters[4], Search: filters[5],
		Status: status, GroupBy: groupBy, PageNum: pageNum, PageSize: pageSize,
	}, nil
}

func parseAnalyticsTimeRanges(c *gin.Context, now time.Time) (auditlog.TimeRange, auditlog.TimeRange, error) {
	fromRaw := firstNonEmpty(c.Query("from"), c.Query("createdAtFrom"))
	toRaw := firstNonEmpty(c.Query("to"), c.Query("createdAtTo"))
	var from time.Time
	var to time.Time
	if fromRaw == "" && toRaw == "" {
		localNow := now.In(analyticsAPILocation)
		to = time.Date(localNow.Year(), localNow.Month(), localNow.Day()+1, 0, 0, 0, 0, analyticsAPILocation).UTC()
		from = to.In(analyticsAPILocation).AddDate(0, 0, -30).UTC()
	} else {
		if fromRaw == "" || toRaw == "" {
			return auditlog.TimeRange{}, auditlog.TimeRange{}, auditlog.ErrValidation
		}
		parsedFrom, err := parseAnalyticsTime(fromRaw, false)
		if err != nil {
			return auditlog.TimeRange{}, auditlog.TimeRange{}, err
		}
		parsedTo, err := parseAnalyticsTime(toRaw, true)
		if err != nil {
			return auditlog.TimeRange{}, auditlog.TimeRange{}, err
		}
		from, to = parsedFrom, parsedTo
	}
	if !from.Before(to) || to.Sub(from) > analyticsMaxRangeDays*24*time.Hour {
		return auditlog.TimeRange{}, auditlog.TimeRange{}, auditlog.ErrValidation
	}
	duration := to.Sub(from)
	previousFrom := from.Add(-duration)
	previousTo := from
	return auditlog.TimeRange{From: &from, To: &to, ToExclusive: true}, auditlog.TimeRange{From: &previousFrom, To: &previousTo, ToExclusive: true}, nil
}

func parseAnalyticsTime(raw string, endOfDate bool) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, auditlog.ErrValidation
	}
	if value, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return value.UTC(), nil
	}
	value, err := time.ParseInLocation("2006-01-02", raw, analyticsAPILocation)
	if err != nil {
		return time.Time{}, auditlog.ErrValidation
	}
	if endOfDate {
		value = value.AddDate(0, 0, 1)
	}
	return value.UTC(), nil
}

func parseAnalyticsBool(raw string, fallback bool) (bool, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return fallback, nil
	}
	switch raw {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, auditlog.ErrValidation
	}
}

func cleanAnalyticsStatus(raw string, kind analyticsQueryKind) (string, error) {
	value := strings.ToUpper(strings.TrimSpace(raw))
	if value == "" {
		return "", nil
	}
	allowed := map[analyticsQueryKind]map[string]bool{
		analyticsQueryTasks:    {"QUEUED": true, "RUNNING": true, "RETRYING": true, "SUCCEEDED": true, "FAILED": true, "TIMED_OUT": true, "CANCELLED": true},
		analyticsQueryRequests: {"SUCCESS": true, "FAILURE": true},
		analyticsQueryUsers:    {"ACTIVE": true, "DISABLED": true},
	}
	if !allowed[kind][value] {
		return "", auditlog.ErrValidation
	}
	return value, nil
}

func validAnalyticsGroup(value string) bool {
	switch value {
	case auditlog.AnalyticsGroupUser, auditlog.AnalyticsGroupProject, auditlog.AnalyticsGroupProvider, auditlog.AnalyticsGroupModel, auditlog.AnalyticsGroupImageType:
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
