package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	pkgconstants "request-system/pkg/constants"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils"
)

const dashboardCacheTTL = 3 * time.Minute

const (
	dashboardWidgetAlerts          = "alerts"
	dashboardWidgetKPIs            = "kpis"
	dashboardWidgetSLA             = "sla"
	dashboardWidgetWeeklyVolume    = "weekly_volume"
	dashboardWidgetTimeByPriority  = "time_by_priority"
	dashboardWidgetTimeByOrderType = "time_by_order_type"
	dashboardWidgetCountByStatus   = "count_by_status"
	dashboardWidgetCountByExecutor = "count_by_executor"
	dashboardWidgetTopCategories   = "top_categories"
	dashboardWidgetDepartments     = "departments"
	dashboardWidgetBranches        = "branches"
	dashboardWidgetLastActivity    = "last_activity"
)

var dashboardWidgetSet = map[string]struct{}{
	dashboardWidgetAlerts:          {},
	dashboardWidgetKPIs:            {},
	dashboardWidgetSLA:             {},
	dashboardWidgetWeeklyVolume:    {},
	dashboardWidgetTimeByPriority:  {},
	dashboardWidgetTimeByOrderType: {},
	dashboardWidgetCountByStatus:   {},
	dashboardWidgetCountByExecutor: {},
	dashboardWidgetTopCategories:   {},
	dashboardWidgetDepartments:     {},
	dashboardWidgetBranches:        {},
	dashboardWidgetLastActivity:    {},
}

type DashboardService struct {
	repo     repositories.DashboardRepositoryInterface
	userRepo repositories.UserRepositoryInterface
	cache    repositories.CacheRepositoryInterface
	logger   *zap.Logger
	flight   singleflight.Group
	workers  int
}

type dashboardRequest struct {
	filter         dto.DashboardFilterDTO
	query          types.DashboardQuery
	effectiveScope string
	widgets        map[string]struct{}
}

func NewDashboardService(
	repo repositories.DashboardRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	cache repositories.CacheRepositoryInterface,
	logger *zap.Logger,
) *DashboardService {
	return &DashboardService{
		repo:     repo,
		userRepo: userRepo,
		cache:    cache,
		logger:   logger,
		workers:  loadDashboardWorkerLimit(),
	}
}

func (s *DashboardService) GetDashboardStats(ctx context.Context, filter dto.DashboardFilterDTO) (*dto.DashboardStatsDTO, error) {
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, apperrors.ErrUnauthorized
	}

	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		return nil, apperrors.ErrUnauthorized
	}

	actor, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	authContext := authz.Context{Actor: actor, Permissions: permissionsMap}
	if !authz.CanDo(authz.DashboardView, authContext) {
		return nil, apperrors.ErrForbidden
	}

	req, err := buildDashboardRequest(filter, userID)
	if err != nil {
		return nil, err
	}

	securityCondition := resolveDashboardSecurity(&authContext, actor, &req)
	return s.loadDashboardWithCache(ctx, userID, actor, permissionsMap, req, securityCondition)
}

func (s *DashboardService) loadDashboardStats(ctx context.Context, req dashboardRequest, securityCondition sq.Sqlizer) (*dto.DashboardStatsDTO, error) {
	var (
		alerts    *types.DashboardAlerts
		kpis      *types.DashboardKPIs
		sla       *types.DashboardSLAStats
		timePrior = make([]types.DashboardTimeByGroup, 0)
		timeType  = make([]types.DashboardTimeByGroup, 0)
		cntStatus = make([]types.DashboardCountByGroup, 0)
		cntExec   = make([]types.DashboardExecutorCount, 0)
		weekly    = make([]types.DashboardChartData, 0)
		topCat    = make([]types.DashboardCountByGroup, 0)
		lastAct   = make([]types.DashboardActivityItem, 0)
		depts     = make([]types.DashboardDepartmentStat, 0)
		branches  = make([]types.DashboardDepartmentStat, 0)
	)

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(s.workers)

	if req.wants(dashboardWidgetAlerts) {
		group.Go(func() error {
			var err error
			alerts, err = s.repo.GetAlerts(groupCtx, securityCondition, req.query)
			return err
		})
	}
	if req.wants(dashboardWidgetKPIs) {
		group.Go(func() error {
			var err error
			kpis, err = s.repo.GetKPIsWithUser(groupCtx, securityCondition, req.query)
			return err
		})
	}
	if req.wants(dashboardWidgetSLA) {
		group.Go(func() error {
			var err error
			sla, err = s.repo.GetSLAStats(groupCtx, securityCondition, req.query)
			return err
		})
	}
	if req.wants(dashboardWidgetTimeByPriority) {
		group.Go(func() error {
			var err error
			timePrior, err = s.repo.GetAvgTimeByPriority(groupCtx, securityCondition, req.query)
			return err
		})
	}
	if req.wants(dashboardWidgetTimeByOrderType) {
		group.Go(func() error {
			var err error
			timeType, err = s.repo.GetAvgTimeByOrderType(groupCtx, securityCondition, req.query)
			return err
		})
	}
	if req.wants(dashboardWidgetCountByStatus) {
		group.Go(func() error {
			var err error
			cntStatus, err = s.repo.GetCountByStatus(groupCtx, securityCondition, req.query)
			return err
		})
	}
	if req.wants(dashboardWidgetCountByExecutor) {
		group.Go(func() error {
			var err error
			cntExec, err = s.repo.GetCountByExecutor(groupCtx, securityCondition, req.query)
			return err
		})
	}
	if req.wants(dashboardWidgetWeeklyVolume) {
		group.Go(func() error {
			var err error
			weekly, err = s.repo.GetWeeklyVolume(groupCtx, securityCondition, req.query)
			return err
		})
	}
	if req.wants(dashboardWidgetTopCategories) {
		group.Go(func() error {
			var err error
			topCat, err = s.repo.GetTopCategories(groupCtx, securityCondition, req.query)
			return err
		})
	}
	if req.wants(dashboardWidgetLastActivity) {
		group.Go(func() error {
			var err error
			lastAct, err = s.repo.GetLastActivity(groupCtx, securityCondition, req.query)
			return err
		})
	}
	if req.wants(dashboardWidgetDepartments) {
		group.Go(func() error {
			var err error
			depts, err = s.repo.GetDepartmentStats(groupCtx, securityCondition, req.query)
			return err
		})
	}
	if req.wants(dashboardWidgetBranches) {
		group.Go(func() error {
			var err error
			branches, err = s.repo.GetBranchStats(groupCtx, securityCondition, req.query)
			return err
		})
	}

	if err := group.Wait(); err != nil {
		return nil, apperrors.NewInternalError("Ошибка загрузки дашборда")
	}

	if kpis != nil {
		decorateDashboardKPIs(kpis)
	}
	formatDashboardTimeGroups(timePrior)
	formatDashboardTimeGroups(timeType)
	decorateDashboardDepartmentStats(depts)
	decorateDashboardDepartmentStats(branches)
	weekly = fillMissingBuckets(weekly, req.query.Range, req.query.Granularity, time.Local)

	return &dto.DashboardStatsDTO{
		Meta:            buildDashboardMeta(req),
		Alerts:          alerts,
		KPIs:            kpis,
		SLA:             sla,
		WeeklyVolume:    weekly,
		TimeByPriority:  timePrior,
		TimeByOrderType: timeType,
		CountByStatus:   cntStatus,
		CountByExecutor: cntExec,
		TopCategories:   topCat,
		Departments:     depts,
		Branches:        branches,
		LastActivity:    lastAct,
	}, nil
}

func (s *DashboardService) readDashboardFromCache(ctx context.Context, cacheKey string) (*dto.DashboardStatsDTO, error) {
	if s.cache == nil {
		return nil, fmt.Errorf("cache is disabled")
	}
	cached, err := s.cache.Get(ctx, cacheKey)
	if err != nil {
		return nil, err
	}

	var result dto.DashboardStatsDTO
	if err := json.Unmarshal([]byte(cached), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *DashboardService) writeDashboardToCache(ctx context.Context, cacheKey string, result *dto.DashboardStatsDTO) {
	if s.cache == nil {
		return
	}
	payload, err := json.Marshal(result)
	if err != nil {
		s.logger.Warn("dashboard cache marshal failed", zap.Error(err))
		return
	}
	if err := s.cache.Set(ctx, cacheKey, payload, dashboardCacheTTL); err != nil {
		s.logger.Warn("dashboard cache set failed", zap.Error(err))
	}
}

func (s *DashboardService) loadDashboardWithCache(
	ctx context.Context,
	userID uint64,
	actor *entities.User,
	permissionsMap map[string]bool,
	req dashboardRequest,
	securityCondition sq.Sqlizer,
) (*dto.DashboardStatsDTO, error) {
	parts := splitDashboardRequestForCache(req)
	if len(parts) == 1 {
		return s.loadDashboardSliceWithCache(ctx, userID, actor, permissionsMap, parts[0], securityCondition)
	}

	results := make([]dashboardSliceResult, len(parts))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(len(parts))

	for i, part := range parts {
		i, part := i, part
		group.Go(func() error {
			stats, err := s.loadDashboardSliceWithCache(groupCtx, userID, actor, permissionsMap, part, securityCondition)
			if err != nil {
				return err
			}

			results[i] = dashboardSliceResult{
				request: part,
				stats:   stats,
			}
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}

	return mergeDashboardStats(results), nil
}

func (s *DashboardService) loadDashboardSliceWithCache(
	ctx context.Context,
	userID uint64,
	actor *entities.User,
	permissionsMap map[string]bool,
	req dashboardRequest,
	securityCondition sq.Sqlizer,
) (*dto.DashboardStatsDTO, error) {
	cacheVersion := s.loadDashboardCacheVersion(ctx, req.widgets)
	cacheKey, err := buildDashboardCacheKey(userID, actor, permissionsMap, req, cacheVersion)
	if err != nil {
		s.logger.Warn("dashboard cache key build failed", zap.Uint64("user_id", userID), zap.Error(err))
		cacheKey = ""
	}

	if cacheKey != "" {
		if cached, cacheErr := s.readDashboardFromCache(ctx, cacheKey); cacheErr == nil {
			return cached, nil
		}
	}

	loadFn := func() (*dto.DashboardStatsDTO, error) {
		result, loadErr := s.loadDashboardStats(ctx, req, securityCondition)
		if loadErr != nil {
			return nil, loadErr
		}
		if cacheKey != "" {
			s.writeDashboardToCache(ctx, cacheKey, result)
		}
		return result, nil
	}

	if cacheKey == "" {
		return loadFn()
	}

	value, err, _ := s.flight.Do(cacheKey, func() (interface{}, error) {
		if cached, cacheErr := s.readDashboardFromCache(ctx, cacheKey); cacheErr == nil {
			return cached, nil
		}
		return loadFn()
	})
	if err != nil {
		return nil, err
	}

	return value.(*dto.DashboardStatsDTO), nil
}

func (s *DashboardService) loadDashboardCacheVersion(ctx context.Context, widgets map[string]struct{}) string {
	if s.cache == nil {
		return "0"
	}

	parts := make([]string, 0, 2)
	for _, key := range dashboardCacheVersionKeysForWidgets(widgets) {
		version, err := s.cache.Get(ctx, key)
		if err != nil || strings.TrimSpace(version) == "" {
			parts = append(parts, key+"=0")
			continue
		}
		parts = append(parts, key+"="+version)
	}

	if len(parts) == 0 {
		return "0"
	}

	return strings.Join(parts, "|")
}

func buildDashboardRequest(filter dto.DashboardFilterDTO, userID uint64) (dashboardRequest, error) {
	normalized := normalizeDashboardFilter(filter)
	dateRange, previousRange, err := resolveDashboardRanges(normalized, time.Now().In(time.Local))
	if err != nil {
		return dashboardRequest{}, err
	}

	widgets := make(map[string]struct{}, len(dashboardWidgetSet))
	if len(normalized.Widgets) == 0 {
		for widget := range dashboardWidgetSet {
			widgets[widget] = struct{}{}
		}
	} else {
		for _, widget := range normalized.Widgets {
			widgets[widget] = struct{}{}
		}
	}

	return dashboardRequest{
		filter: normalized,
		query: types.DashboardQuery{
			Range:         dateRange,
			PreviousRange: previousRange,
			Granularity:   normalized.Granularity,
			UserID:        userID,
		},
		widgets: widgets,
	}, nil
}

func resolveDashboardSecurity(authContext *authz.Context, actor *entities.User, req *dashboardRequest) sq.Sqlizer {
	switch {
	case authContext.HasPermission(authz.ScopeAll) || authContext.HasPermission(authz.ScopeAllView):
		req.effectiveScope = types.DashboardScopeAll
		return nil
	case authContext.HasPermission(authz.ScopeDepartment) && actor.DepartmentID != nil:
		req.effectiveScope = types.DashboardScopeDepartment
		return sq.Eq{"o.department_id": *actor.DepartmentID}
	case authContext.HasPermission(authz.ScopeBranch) && actor.BranchID != nil:
		req.effectiveScope = types.DashboardScopeBranch
		return sq.Eq{"o.branch_id": *actor.BranchID}
	case authContext.HasPermission(authz.ScopeOtdel) && actor.OtdelID != nil:
		req.effectiveScope = types.DashboardScopeOtdel
		return sq.Eq{"o.otdel_id": *actor.OtdelID}
	case authContext.HasPermission(authz.ScopeOffice) && actor.OfficeID != nil:
		req.effectiveScope = types.DashboardScopeOffice
		return sq.Eq{"o.office_id": *actor.OfficeID}
	default:
		req.effectiveScope = types.DashboardScopeOwn
		return sq.Or{
			sq.Eq{"o.user_id": actor.ID},
			sq.Eq{"o.executor_id": actor.ID},
		}
	}
}

func normalizeDashboardFilter(filter dto.DashboardFilterDTO) dto.DashboardFilterDTO {
	filter.Period = strings.TrimSpace(strings.ToLower(filter.Period))
	if filter.Period == "" {
		filter.Period = types.DashboardPeriodMonth
	}

	filter.Granularity = strings.TrimSpace(strings.ToLower(filter.Granularity))
	if filter.Granularity == "" {
		filter.Granularity = types.DashboardGranularityDay
	}

	normalizedWidgets := make([]string, 0, len(filter.Widgets))
	seen := make(map[string]struct{}, len(filter.Widgets))
	for _, widget := range filter.Widgets {
		widget = strings.TrimSpace(strings.ToLower(widget))
		if widget == "" {
			continue
		}
		if _, ok := dashboardWidgetSet[widget]; !ok {
			continue
		}
		if _, ok := seen[widget]; ok {
			continue
		}
		seen[widget] = struct{}{}
		normalizedWidgets = append(normalizedWidgets, widget)
	}
	filter.Widgets = normalizedWidgets
	return filter
}

func resolveDashboardRanges(filter dto.DashboardFilterDTO, now time.Time) (types.DashboardDateRange, types.DashboardDateRange, error) {
	loc := now.Location()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	var from time.Time
	var to time.Time

	switch filter.Period {
	case types.DashboardPeriodToday:
		from = startOfToday
		to = now
	case types.DashboardPeriod7Days:
		from = startOfToday.AddDate(0, 0, -6)
		to = now
	case types.DashboardPeriod14Days:
		from = startOfToday.AddDate(0, 0, -13)
		to = now
	case types.DashboardPeriod30Days:
		from = startOfToday.AddDate(0, 0, -29)
		to = now
	case types.DashboardPeriodCustom:
		if filter.DateFrom == nil || filter.DateTo == nil {
			return types.DashboardDateRange{}, types.DashboardDateRange{}, apperrors.NewBadRequestError("Для custom периода нужны обе даты")
		}
		from = time.Date(filter.DateFrom.In(loc).Year(), filter.DateFrom.In(loc).Month(), filter.DateFrom.In(loc).Day(), 0, 0, 0, 0, loc)
		dateTo := filter.DateTo.In(loc)
		to = time.Date(dateTo.Year(), dateTo.Month(), dateTo.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), loc)
	case "", types.DashboardPeriodMonth:
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		to = now
	default:
		return types.DashboardDateRange{}, types.DashboardDateRange{}, apperrors.NewBadRequestError("Некорректный период дашборда")
	}

	if filter.Period != types.DashboardPeriodCustom {
		if filter.DateFrom != nil {
			dateFrom := filter.DateFrom.In(loc)
			from = time.Date(dateFrom.Year(), dateFrom.Month(), dateFrom.Day(), 0, 0, 0, 0, loc)
		}
		if filter.DateTo != nil {
			dateTo := filter.DateTo.In(loc)
			to = time.Date(dateTo.Year(), dateTo.Month(), dateTo.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), loc)
		}
	}

	if to.Before(from) {
		return types.DashboardDateRange{}, types.DashboardDateRange{}, apperrors.NewBadRequestError("date_to не может быть раньше date_from")
	}

	currentRange := types.DashboardDateRange{
		From: from,
		To:   to.Add(time.Nanosecond),
	}
	duration := currentRange.To.Sub(currentRange.From)
	previousRange := types.DashboardDateRange{
		From: currentRange.From.Add(-duration),
		To:   currentRange.From,
	}
	return currentRange, previousRange, nil
}

func buildDashboardMeta(req dashboardRequest) *types.DashboardMeta {
	return &types.DashboardMeta{
		GeneratedAt:    time.Now().In(time.Local).Format(time.RFC3339),
		Timezone:       time.Local.String(),
		EffectiveScope: req.effectiveScope,
		Period:         req.filter.Period,
		DateFrom:       req.query.Range.From.Format(time.RFC3339),
		DateTo:         req.query.Range.To.Format(time.RFC3339),
		Granularity:    req.query.Granularity,
	}
}

func buildDashboardCacheKey(userID uint64, actor *entities.User, permissionsMap map[string]bool, req dashboardRequest, version string) (string, error) {
	type actorScope struct {
		DepartmentID *uint64 `json:"department_id,omitempty"`
		BranchID     *uint64 `json:"branch_id,omitempty"`
		OtdelID      *uint64 `json:"otdel_id,omitempty"`
		OfficeID     *uint64 `json:"office_id,omitempty"`
	}

	perms := make([]string, 0, len(permissionsMap))
	for perm, allowed := range permissionsMap {
		if allowed {
			perms = append(perms, perm)
		}
	}
	sort.Strings(perms)

	payload := map[string]interface{}{
		"version":     version,
		"user_id":     userID,
		"permissions": perms,
		"scope": actorScope{
			DepartmentID: actor.DepartmentID,
			BranchID:     actor.BranchID,
			OtdelID:      actor.OtdelID,
			OfficeID:     actor.OfficeID,
		},
		"effective_scope": req.effectiveScope,
		"period":          req.filter.Period,
		"date_from":       req.query.Range.From.Format(time.RFC3339),
		"date_to":         req.query.Range.To.Format(time.RFC3339),
		"granularity":     req.query.Granularity,
		"widgets":         sortedDashboardWidgets(req.widgets),
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(raw)
	return "dashboard:stats:" + version + ":" + hex.EncodeToString(sum[:]), nil
}

func sortedDashboardWidgets(widgets map[string]struct{}) []string {
	result := make([]string, 0, len(widgets))
	for widget := range widgets {
		result = append(result, widget)
	}
	sort.Strings(result)
	return result
}

type dashboardSliceResult struct {
	request dashboardRequest
	stats   *dto.DashboardStatsDTO
}

func splitDashboardRequestForCache(req dashboardRequest) []dashboardRequest {
	summaryWidgets := make(map[string]struct{})
	activityWidgets := make(map[string]struct{})

	for widget := range req.widgets {
		if widget == dashboardWidgetLastActivity {
			activityWidgets[widget] = struct{}{}
			continue
		}
		summaryWidgets[widget] = struct{}{}
	}

	parts := make([]dashboardRequest, 0, 2)
	if len(summaryWidgets) > 0 {
		parts = append(parts, cloneDashboardRequestWithWidgets(req, summaryWidgets))
	}
	if len(activityWidgets) > 0 {
		parts = append(parts, cloneDashboardRequestWithWidgets(req, activityWidgets))
	}
	if len(parts) == 0 {
		parts = append(parts, req)
	}

	return parts
}

func cloneDashboardRequestWithWidgets(req dashboardRequest, widgets map[string]struct{}) dashboardRequest {
	clonedWidgets := make(map[string]struct{}, len(widgets))
	for widget := range widgets {
		clonedWidgets[widget] = struct{}{}
	}

	filterWidgets := make([]string, 0, len(clonedWidgets))
	for _, widget := range sortedDashboardWidgets(clonedWidgets) {
		filterWidgets = append(filterWidgets, widget)
	}

	clonedFilter := req.filter
	clonedFilter.Widgets = filterWidgets

	return dashboardRequest{
		filter:         clonedFilter,
		query:          req.query,
		effectiveScope: req.effectiveScope,
		widgets:        clonedWidgets,
	}
}

func mergeDashboardStats(parts []dashboardSliceResult) *dto.DashboardStatsDTO {
	result := &dto.DashboardStatsDTO{}

	for _, part := range parts {
		if part.stats == nil {
			continue
		}

		if result.Meta == nil && part.stats.Meta != nil {
			result.Meta = part.stats.Meta
		}

		applyDashboardSlice(result, part.stats, part.request)
	}

	return result
}

func applyDashboardSlice(dst *dto.DashboardStatsDTO, src *dto.DashboardStatsDTO, req dashboardRequest) {
	if req.wants(dashboardWidgetAlerts) {
		dst.Alerts = src.Alerts
	}
	if req.wants(dashboardWidgetKPIs) {
		dst.KPIs = src.KPIs
	}
	if req.wants(dashboardWidgetSLA) {
		dst.SLA = src.SLA
	}
	if req.wants(dashboardWidgetWeeklyVolume) {
		dst.WeeklyVolume = src.WeeklyVolume
	}
	if req.wants(dashboardWidgetTimeByPriority) {
		dst.TimeByPriority = src.TimeByPriority
	}
	if req.wants(dashboardWidgetTimeByOrderType) {
		dst.TimeByOrderType = src.TimeByOrderType
	}
	if req.wants(dashboardWidgetCountByStatus) {
		dst.CountByStatus = src.CountByStatus
	}
	if req.wants(dashboardWidgetCountByExecutor) {
		dst.CountByExecutor = src.CountByExecutor
	}
	if req.wants(dashboardWidgetTopCategories) {
		dst.TopCategories = src.TopCategories
	}
	if req.wants(dashboardWidgetDepartments) {
		dst.Departments = src.Departments
	}
	if req.wants(dashboardWidgetBranches) {
		dst.Branches = src.Branches
	}
	if req.wants(dashboardWidgetLastActivity) {
		dst.LastActivity = src.LastActivity
	}
}

func dashboardCacheVersionKeysForWidgets(widgets map[string]struct{}) []string {
	keys := make([]string, 0, 2)

	if dashboardUsesSummaryCache(widgets) {
		keys = append(keys, pkgconstants.DashboardCacheVersionSummaryKey)
	}
	if dashboardUsesActivityCache(widgets) {
		keys = append(keys, pkgconstants.DashboardCacheVersionActivityKey)
	}

	if len(keys) == 0 {
		keys = append(keys, pkgconstants.DashboardCacheVersionSummaryKey)
	}

	return keys
}

func dashboardUsesSummaryCache(widgets map[string]struct{}) bool {
	for widget := range widgets {
		if widget != dashboardWidgetLastActivity {
			return true
		}
	}
	return false
}

func dashboardUsesActivityCache(widgets map[string]struct{}) bool {
	_, ok := widgets[dashboardWidgetLastActivity]
	return ok
}

func loadDashboardWorkerLimit() int {
	const defaultWorkers = 6

	raw := strings.TrimSpace(os.Getenv("DASHBOARD_WORKER_LIMIT"))
	if raw == "" {
		return defaultWorkers
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return defaultWorkers
	}

	return value
}

func fillMissingBuckets(input []types.DashboardChartData, dateRange types.DashboardDateRange, granularity string, loc *time.Location) []types.DashboardChartData {
	if loc == nil {
		loc = time.Local
	}

	existing := make(map[string]int64, len(input))
	for _, item := range input {
		existing[item.Label] = item.Value
	}

	start := truncateDashboardBucket(dateRange.From.In(loc), granularity)
	end := truncateDashboardBucket(dateRange.To.Add(-time.Nanosecond).In(loc), granularity)

	result := make([]types.DashboardChartData, 0)
	for current := start; !current.After(end); current = nextDashboardBucket(current, granularity) {
		key := current.Format("2006-01-02")
		result = append(result, types.DashboardChartData{
			Label: formatDashboardBucketLabel(current, granularity),
			Value: existing[key],
		})
	}
	return result
}

func truncateDashboardBucket(t time.Time, granularity string) time.Time {
	switch granularity {
	case types.DashboardGranularityMonth:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	case types.DashboardGranularityWeek:
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		dayStart := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		return dayStart.AddDate(0, 0, -(weekday - 1))
	default:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	}
}

func nextDashboardBucket(t time.Time, granularity string) time.Time {
	switch granularity {
	case types.DashboardGranularityMonth:
		return t.AddDate(0, 1, 0)
	case types.DashboardGranularityWeek:
		return t.AddDate(0, 0, 7)
	default:
		return t.AddDate(0, 0, 1)
	}
}

func formatDashboardBucketLabel(t time.Time, granularity string) string {
	switch granularity {
	case types.DashboardGranularityMonth:
		return t.Format("01.2006")
	default:
		return t.Format("02.01")
	}
}

func decorateDashboardKPIs(kpis *types.DashboardKPIs) {
	decorateDashboardTrend(&kpis.TotalTickets)
	decorateDashboardTrend(&kpis.ResolvedTickets)
	decorateDashboardTrend(&kpis.SLACompliance)
	decorateDashboardTrend(&kpis.FCRRate)
	decorateDashboardTrend(&kpis.AvgResponseTime)
	decorateDashboardTrend(&kpis.AvgResolveTime)

	kpis.TotalTickets.Formatted = fmt.Sprintf("%.0f", kpis.TotalTickets.Current)
	kpis.ResolvedTickets.Formatted = fmt.Sprintf("%.0f", kpis.ResolvedTickets.Current)
	kpis.SLACompliance.Formatted = fmt.Sprintf("%.0f%%", kpis.SLACompliance.Current)
	kpis.FCRRate.Formatted = fmt.Sprintf("%.0f%%", kpis.FCRRate.Current)
	kpis.AvgResponseTime.Formatted = humanizeSeconds(kpis.AvgResponseTime.Current)
	kpis.AvgResolveTime.Formatted = humanizeSeconds(kpis.AvgResolveTime.Current)
	kpis.OpenTickets.Formatted = fmt.Sprintf("%.0f", kpis.OpenTickets.Current)
	kpis.OpenTickets.TrendText = "на текущий момент"
}

func decorateDashboardTrend(metric *types.DashboardKPIMetric) {
	switch {
	case metric.Previous > 0:
		metric.TrendPct = math.Round(((metric.Current - metric.Previous) / metric.Previous) * 100)
	case metric.Current > 0:
		metric.TrendPct = 100
	default:
		metric.TrendPct = 0
	}

	sign := ""
	if metric.TrendPct > 0 {
		sign = "+"
	}
	if metric.TrendPct == 0 {
		metric.TrendText = "без изменений"
		return
	}
	metric.TrendText = fmt.Sprintf("%s%.0f%% к предыдущему периоду", sign, metric.TrendPct)
}

func formatDashboardTimeGroups(groups []types.DashboardTimeByGroup) {
	for i := range groups {
		groups[i].AvgTimeFormatted = humanizeSeconds(groups[i].AvgSeconds)
	}
}

func decorateDashboardDepartmentStats(stats []types.DashboardDepartmentStat) {
	for i := range stats {
		if stats[i].TotalCount == 0 {
			continue
		}
		stats[i].SolvedPercent = (float64(stats[i].ResolvedCount) / float64(stats[i].TotalCount)) * 100
	}
}

func humanizeSeconds(v float64) string {
	totalSeconds := int(math.Round(v))
	if totalSeconds <= 0 {
		return "0 сек"
	}
	if totalSeconds < 60 {
		return fmt.Sprintf("%d сек", totalSeconds)
	}
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	if hours > 0 {
		return fmt.Sprintf("%dч %dм", hours, minutes)
	}
	return fmt.Sprintf("%dм", minutes)
}

func (r dashboardRequest) wants(widget string) bool {
	_, ok := r.widgets[widget]
	return ok
}
