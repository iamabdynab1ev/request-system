package dto

import "request-system/pkg/types"

type DashboardStatsDTO struct {
	Alerts          *types.DashboardAlerts          `json:"alerts"`
	KPIs            *types.DashboardKPIs            `json:"kpis"`
	SLA             *types.DashboardSLAStats        `json:"sla"`
	WeeklyVolume    []types.DashboardChartData      `json:"weekly_volume"`
	TimeByPriority  []types.DashboardTimeByGroup    `json:"time_by_priority"`
	TimeByOrderType []types.DashboardTimeByGroup    `json:"time_by_order_type"`
	CountByStatus   []types.DashboardCountByGroup   `json:"count_by_status"`
	CountByExecutor []types.DashboardCountByGroup   `json:"count_by_executor"`
	TopCategories   []types.DashboardCountByGroup   `json:"top_categories"`
	Departments     []types.DashboardDepartmentStat `json:"departments"`
	Branches        []types.DashboardDepartmentStat `json:"branches"`
	LastActivity    []types.DashboardActivityItem   `json:"last_activity"`
}
