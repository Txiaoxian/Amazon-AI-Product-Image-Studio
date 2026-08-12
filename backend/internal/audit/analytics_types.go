package audit

import "time"

const (
	AnalyticsGranularityHour = "hour"
	AnalyticsGranularityDay  = "day"
	AnalyticsGranularityWeek = "week"

	AnalyticsGroupUser      = "user"
	AnalyticsGroupProject   = "project"
	AnalyticsGroupProvider  = "provider"
	AnalyticsGroupModel     = "model"
	AnalyticsGroupImageType = "imageType"

	AnalyticsCostTypeEstimated = "ESTIMATED"
)

type AnalyticsOptions struct {
	TimeRange     TimeRange
	PreviousRange TimeRange
	Granularity   string
	Compare       bool
	UserID        string
	ProjectID     string
	ProviderID    string
	ModelID       string
	Status        string
	ImageType     string
	GroupBy       string
	Search        string
	PageNum       int
	PageSize      int
}

type AnalyticsMeta struct {
	From         string `json:"from"`
	To           string `json:"to"`
	Timezone     string `json:"timezone"`
	Granularity  string `json:"granularity"`
	ComparedFrom string `json:"comparedFrom,omitempty"`
	ComparedTo   string `json:"comparedTo,omitempty"`
	CostType     string `json:"costType"`
	GeneratedAt  string `json:"generatedAt"`
}

type AnalyticsMetricSet struct {
	TaskCount            int64   `json:"taskCount"`
	OutputCount          int64   `json:"outputCount"`
	TerminalTaskCount    int64   `json:"terminalTaskCount"`
	SucceededTaskCount   int64   `json:"succeededTaskCount"`
	TaskSuccessRate      float64 `json:"taskSuccessRate"`
	ActiveUserCount      int64   `json:"activeUserCount"`
	LoginActiveUserCount int64   `json:"loginActiveUserCount"`
	P95DurationMs        int64   `json:"p95DurationMs"`
}

type AnalyticsMetricChanges struct {
	TaskCount            *float64 `json:"taskCount"`
	OutputCount          *float64 `json:"outputCount"`
	TaskSuccessRate      *float64 `json:"taskSuccessRate"`
	ActiveUserCount      *float64 `json:"activeUserCount"`
	LoginActiveUserCount *float64 `json:"loginActiveUserCount"`
	P95DurationMs        *float64 `json:"p95DurationMs"`
}

type AnalyticsCostMetric struct {
	Currency          string   `json:"currency"`
	Amount            string   `json:"amount"`
	PreviousAmount    string   `json:"previousAmount,omitempty"`
	ChangePercent     *float64 `json:"changePercent"`
	RecordCount       int64    `json:"recordCount"`
	PricedRecordCount int64    `json:"pricedRecordCount"`
	PricingCoverage   float64  `json:"pricingCoverage"`
}

type AnalyticsTrendPoint struct {
	Bucket           string `json:"bucket"`
	TaskCount        int64  `json:"taskCount"`
	OutputCount      int64  `json:"outputCount"`
	SucceededCount   int64  `json:"succeededCount"`
	FailedCount      int64  `json:"failedCount"`
	TimedOutCount    int64  `json:"timedOutCount"`
	CancelledCount   int64  `json:"cancelledCount"`
	ActiveUserCount  int64  `json:"activeUserCount"`
	LoginActiveUsers int64  `json:"loginActiveUsers"`
}

type AnalyticsCostTrendPoint struct {
	Bucket        string `json:"bucket"`
	Currency      string `json:"currency"`
	EstimatedCost string `json:"estimatedCost"`
}

type AnalyticsRankingItem struct {
	Dimension     string                `json:"dimension"`
	DimensionID   string                `json:"dimensionId"`
	Name          string                `json:"name"`
	SecondaryName string                `json:"secondaryName,omitempty"`
	TaskCount     int64                 `json:"taskCount"`
	OutputCount   int64                 `json:"outputCount"`
	SuccessRate   float64               `json:"successRate"`
	Costs         []AnalyticsCostMetric `json:"costs"`
}

type AnalyticsErrorGroup struct {
	ErrorCode string `json:"errorCode"`
	Count     int64  `json:"count"`
}

type AnalyticsOverviewResponse struct {
	Meta        AnalyticsMeta             `json:"meta"`
	Current     AnalyticsMetricSet        `json:"current"`
	Previous    AnalyticsMetricSet        `json:"previous"`
	Changes     AnalyticsMetricChanges    `json:"changes"`
	Costs       []AnalyticsCostMetric     `json:"costs"`
	Trend       []AnalyticsTrendPoint     `json:"trend"`
	CostTrend   []AnalyticsCostTrendPoint `json:"costTrend"`
	Rankings    []AnalyticsRankingItem    `json:"rankings"`
	ErrorGroups []AnalyticsErrorGroup     `json:"errorGroups"`
}

type AnalyticsUsageBreakdown struct {
	Dimension        string                `json:"dimension"`
	DimensionID      string                `json:"dimensionId"`
	Name             string                `json:"name"`
	RecordCount      int64                 `json:"recordCount"`
	InputTokens      int64                 `json:"inputTokens"`
	OutputTokens     int64                 `json:"outputTokens"`
	BilledImageCount int64                 `json:"billedImageCount"`
	OutputCount      int64                 `json:"outputCount"`
	Costs            []AnalyticsCostMetric `json:"costs"`
}

type AnalyticsUsageResponse struct {
	Meta        AnalyticsMeta             `json:"meta"`
	Costs       []AnalyticsCostMetric     `json:"costs"`
	OutputCount int64                     `json:"outputCount"`
	UnitCosts   []AnalyticsUnitCost       `json:"unitCosts"`
	CostTrend   []AnalyticsCostTrendPoint `json:"costTrend"`
	Breakdowns  []AnalyticsUsageBreakdown `json:"breakdowns"`
}

type AnalyticsUnitCost struct {
	Currency    string `json:"currency"`
	Amount      string `json:"amount"`
	OutputCount int64  `json:"outputCount"`
	Available   bool   `json:"available"`
}

type AnalyticsUserRecord struct {
	UserID      string                `json:"userId"`
	DisplayName string                `json:"displayName"`
	Email       string                `json:"email"`
	Status      string                `json:"status"`
	LastLoginAt string                `json:"lastLoginAt"`
	ActiveDays  int                   `json:"activeDays"`
	TaskCount   int64                 `json:"taskCount"`
	OutputCount int64                 `json:"outputCount"`
	SuccessRate float64               `json:"successRate"`
	Costs       []AnalyticsCostMetric `json:"costs"`
	LastTaskAt  string                `json:"lastTaskAt"`
	Lifecycle   string                `json:"lifecycle"`
}

type AnalyticsUserPage struct {
	Meta     AnalyticsMeta         `json:"meta"`
	Records  []AnalyticsUserRecord `json:"records"`
	Total    int64                 `json:"total"`
	PageNum  int                   `json:"pageNum"`
	PageSize int                   `json:"pageSize"`
}

type AnalyticsUserDetailResponse struct {
	Meta         AnalyticsMeta             `json:"meta"`
	User         AnalyticsUserRecord       `json:"user"`
	Trend        []AnalyticsTrendPoint     `json:"trend"`
	CostTrend    []AnalyticsCostTrendPoint `json:"costTrend"`
	Projects     []AnalyticsUsageBreakdown `json:"projects"`
	Models       []AnalyticsUsageBreakdown `json:"models"`
	FailedTasks  []AnalyticsTaskRecord     `json:"failedTasks"`
	AuditVisible bool                      `json:"auditVisible"`
}

type AnalyticsTaskRecord struct {
	TaskID        string `json:"taskId"`
	UserID        string `json:"userId"`
	UserName      string `json:"userName"`
	ProjectID     string `json:"projectId"`
	ProjectName   string `json:"projectName"`
	ProviderID    string `json:"providerId"`
	ProviderName  string `json:"providerName"`
	ModelID       string `json:"modelId"`
	ModelName     string `json:"modelName"`
	Type          string `json:"type"`
	ImageType     string `json:"imageType"`
	Status        string `json:"status"`
	OutputCount   int64  `json:"outputCount"`
	DurationMs    int64  `json:"durationMs"`
	EstimatedCost string `json:"estimatedCost"`
	Currency      string `json:"currency"`
	CostStatus    string `json:"costStatus"`
	ErrorCode     string `json:"errorCode"`
	ErrorMessage  string `json:"errorMessage"`
	CreatedAt     string `json:"createdAt"`
	FinishedAt    string `json:"finishedAt"`
}

type AnalyticsTaskPage struct {
	Meta     AnalyticsMeta         `json:"meta"`
	Summary  AnalyticsMetricSet    `json:"summary"`
	Records  []AnalyticsTaskRecord `json:"records"`
	Total    int64                 `json:"total"`
	PageNum  int                   `json:"pageNum"`
	PageSize int                   `json:"pageSize"`
}

type AnalyticsRequestSummary struct {
	CallCount     int64   `json:"callCount"`
	SuccessCount  int64   `json:"successCount"`
	FailureCount  int64   `json:"failureCount"`
	SuccessRate   float64 `json:"successRate"`
	P50DurationMs int64   `json:"p50DurationMs"`
	P95DurationMs int64   `json:"p95DurationMs"`
}

type AnalyticsRequestTrendPoint struct {
	Bucket       string `json:"bucket"`
	CallCount    int64  `json:"callCount"`
	SuccessCount int64  `json:"successCount"`
	FailureCount int64  `json:"failureCount"`
}

type AnalyticsProviderHealth struct {
	ProviderID    string                `json:"providerId"`
	ProviderName  string                `json:"providerName"`
	CallCount     int64                 `json:"callCount"`
	SuccessRate   float64               `json:"successRate"`
	P95DurationMs int64                 `json:"p95DurationMs"`
	LastFailureAt string                `json:"lastFailureAt"`
	Costs         []AnalyticsCostMetric `json:"costs"`
}

type AnalyticsRequestsResponse struct {
	Meta        AnalyticsMeta                `json:"meta"`
	Summary     AnalyticsRequestSummary      `json:"summary"`
	Trend       []AnalyticsRequestTrendPoint `json:"trend"`
	Providers   []AnalyticsProviderHealth    `json:"providers"`
	ErrorGroups []AnalyticsErrorGroup        `json:"errorGroups"`
}

func (options AnalyticsOptions) Previous() AnalyticsOptions {
	previous := options
	previous.TimeRange = options.PreviousRange
	previous.Compare = false
	return previous
}

func (options AnalyticsOptions) ValidRange() bool {
	return options.TimeRange.From != nil && options.TimeRange.To != nil && options.TimeRange.From.Before(*options.TimeRange.To)
}

func analyticsMeta(options AnalyticsOptions, now time.Time) AnalyticsMeta {
	meta := AnalyticsMeta{
		Timezone:    "Asia/Shanghai",
		Granularity: options.Granularity,
		CostType:    AnalyticsCostTypeEstimated,
		GeneratedAt: formatTime(now),
	}
	if options.TimeRange.From != nil {
		meta.From = formatTime(*options.TimeRange.From)
	}
	if options.TimeRange.To != nil {
		meta.To = formatTime(*options.TimeRange.To)
	}
	if options.Compare && options.PreviousRange.From != nil && options.PreviousRange.To != nil {
		meta.ComparedFrom = formatTime(*options.PreviousRange.From)
		meta.ComparedTo = formatTime(*options.PreviousRange.To)
	}
	return meta
}
