package services

import (
	"testing"
	"time"

	"request-system/internal/dto"
	"request-system/pkg/types"
)

func TestSplitDashboardRequestForCache_SplitsSummaryAndActivity(t *testing.T) {
	req := dashboardRequest{
		filter: dto.DashboardFilterDTO{
			Widgets: []string{dashboardWidgetAlerts, dashboardWidgetLastActivity},
		},
		widgets: map[string]struct{}{
			dashboardWidgetAlerts:       {},
			dashboardWidgetLastActivity: {},
		},
	}

	parts := splitDashboardRequestForCache(req)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}

	if _, ok := parts[0].widgets[dashboardWidgetAlerts]; !ok {
		t.Fatalf("expected summary slice to contain alerts")
	}
	if _, ok := parts[0].widgets[dashboardWidgetLastActivity]; ok {
		t.Fatalf("did not expect summary slice to contain last_activity")
	}

	if _, ok := parts[1].widgets[dashboardWidgetLastActivity]; !ok {
		t.Fatalf("expected activity slice to contain last_activity")
	}
}

func TestDashboardCacheVersionKeysForWidgets(t *testing.T) {
	widgets := map[string]struct{}{
		dashboardWidgetAlerts:       {},
		dashboardWidgetLastActivity: {},
	}

	keys := dashboardCacheVersionKeysForWidgets(widgets)
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	if keys[0] == keys[1] {
		t.Fatalf("expected different cache keys, got %v", keys)
	}
}

func TestMergeDashboardStats(t *testing.T) {
	meta := &types.DashboardMeta{Period: types.DashboardPeriod30Days}
	parts := []dashboardSliceResult{
		{
			request: dashboardRequest{widgets: map[string]struct{}{dashboardWidgetAlerts: {}}},
			stats: &dto.DashboardStatsDTO{
				Meta:   meta,
				Alerts: &types.DashboardAlerts{CriticalCount: 5},
			},
		},
		{
			request: dashboardRequest{widgets: map[string]struct{}{dashboardWidgetLastActivity: {}}},
			stats: &dto.DashboardStatsDTO{
				LastActivity: []types.DashboardActivityItem{{ID: 7}},
			},
		},
	}

	result := mergeDashboardStats(parts)
	if result.Meta != meta {
		t.Fatalf("expected meta to be preserved")
	}
	if result.Alerts == nil || result.Alerts.CriticalCount != 5 {
		t.Fatalf("expected alerts to be merged, got %+v", result.Alerts)
	}
	if len(result.LastActivity) != 1 || result.LastActivity[0].ID != 7 {
		t.Fatalf("expected last activity to be merged, got %+v", result.LastActivity)
	}
}

func TestLoadDashboardWorkerLimit_DefaultsOnInvalidValue(t *testing.T) {
	t.Setenv("DASHBOARD_WORKER_LIMIT", "0")
	if got := loadDashboardWorkerLimit(); got != 6 {
		t.Fatalf("expected default worker limit 6, got %d", got)
	}
}

func TestFillMissingBuckets_DayGranularity(t *testing.T) {
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC)

	result := fillMissingBuckets(
		[]types.DashboardChartData{{Label: "2026-04-02", Value: 3}},
		types.DashboardDateRange{From: start, To: end},
		types.DashboardGranularityDay,
		time.UTC,
	)

	if len(result) != 3 {
		t.Fatalf("expected 3 buckets, got %d", len(result))
	}
	if result[0].Value != 0 || result[1].Value != 3 || result[2].Value != 0 {
		t.Fatalf("unexpected bucket values: %+v", result)
	}
}
