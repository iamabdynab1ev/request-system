package types

// Alerts
type DashboardAlerts struct {
	CriticalCount int64 `json:"critical_count"`
	OverdueCount  int64 `json:"overdue_count"`
}

// KPI Metric
type DashboardKPIMetric struct {
	Current   float64 `json:"current"`
	Previous  float64 `json:"-"`
	Formatted string  `json:"formatted"`
	TrendPct  float64 `json:"trend_pct"`
	TrendText string  `json:"trend_text"`
	Personal  float64 `json:"personal"`
}

// KPI Groups
type DashboardKPIs struct {
	TotalTickets    DashboardKPIMetric `json:"total_orders"`
	OpenTickets     DashboardKPIMetric `json:"open_orders"`
	ResolvedTickets DashboardKPIMetric `json:"resolved_orders"`
	SLACompliance   DashboardKPIMetric `json:"sla_compliance"`
	AvgResponseTime DashboardKPIMetric `json:"avg_response_time"`
	AvgResolveTime  DashboardKPIMetric `json:"avg_resolve_time"`
	FCRRate         DashboardKPIMetric `json:"fcr_rate"`
	ActiveAgents    int64              `json:"active_agents"`
}

type DashboardSLAStats struct {
	TotalCompleted int64 `json:"total_completed"`
	OnTime         int64 `json:"on_time"`
}

type DashboardTimeByGroup struct {
	GroupName        string  `json:"group_name"`
	AvgSeconds       float64 `json:"avg_seconds" db:"avg_seconds"`
	AvgTimeFormatted string  `json:"avg_time_formatted" db:"-"`
}

type DashboardCountByGroup struct {
	GroupName string `json:"group_name" db:"group_name"`
	Count     int64  `json:"count" db:"count"`
}

type DashboardChartData struct {
	Label string `json:"label" db:"label"`
	Value int64  `json:"value" db:"value"`
}

type DashboardActivityItem struct {
	ID         int64  `json:"id"`
	Text       string `json:"text"`
	Date       string `json:"date"`
	AuthorName string `json:"author_name"`
	OrderName  string `json:"order_name"`
}

type DashboardDepartmentStat struct {
	Name          string  `json:"name" db:"name"`
	OpenCount     int64   `json:"open_count" db:"open_count"`
	ResolvedCount int64   `json:"resolved_count" db:"resolved_count"`
	CriticalCount int64   `json:"critical_count" db:"critical_count"`
	TotalCount    int64   `json:"total_count" db:"total_count"`
	SolvedPercent float64 `json:"solved_percent" db:"-"`
}
