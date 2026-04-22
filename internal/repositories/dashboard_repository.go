package repositories

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	pkgconstants "request-system/pkg/constants"
	"request-system/pkg/types"
)

var (
	dashboardClosedStatuses            = []string{pkgconstants.StatusClosed}
	dashboardResolvedStatuses          = []string{pkgconstants.StatusClosed}
	dashboardOpenExcludedStatuses      = []string{pkgconstants.StatusClosed}
	dashboardActiveAgentExcludedEvents = []string{
		"CREATE",
		"DELEGATION",
	}

	dashboardResolvedCheck = dashboardStatusInCheck("s.code", dashboardResolvedStatuses)
	dashboardOpenCheck     = dashboardStatusNotInCheck("s.code", dashboardOpenExcludedStatuses)
)

type DashboardRepositoryInterface interface {
	GetAlerts(ctx context.Context, securityCondition sq.Sqlizer, query types.DashboardQuery) (*types.DashboardAlerts, error)
	GetKPIsWithUser(ctx context.Context, securityCondition sq.Sqlizer, query types.DashboardQuery) (*types.DashboardKPIs, error)
	GetSLAStats(ctx context.Context, securityCondition sq.Sqlizer, query types.DashboardQuery) (*types.DashboardSLAStats, error)
	GetAvgTimeByPriority(ctx context.Context, securityCondition sq.Sqlizer, query types.DashboardQuery) ([]types.DashboardTimeByGroup, error)
	GetAvgTimeByOrderType(ctx context.Context, securityCondition sq.Sqlizer, query types.DashboardQuery) ([]types.DashboardTimeByGroup, error)
	GetCountByStatus(ctx context.Context, securityCondition sq.Sqlizer, query types.DashboardQuery) ([]types.DashboardCountByGroup, error)
	GetCountByExecutor(ctx context.Context, securityCondition sq.Sqlizer, query types.DashboardQuery) ([]types.DashboardExecutorCount, error)
	GetWeeklyVolume(ctx context.Context, securityCondition sq.Sqlizer, query types.DashboardQuery) ([]types.DashboardChartData, error)
	GetTopCategories(ctx context.Context, securityCondition sq.Sqlizer, query types.DashboardQuery) ([]types.DashboardCountByGroup, error)
	GetDepartmentStats(ctx context.Context, securityCondition sq.Sqlizer, query types.DashboardQuery) ([]types.DashboardDepartmentStat, error)
	GetLastActivity(ctx context.Context, securityCondition sq.Sqlizer, query types.DashboardQuery) ([]types.DashboardActivityItem, error)
	GetBranchStats(ctx context.Context, securityCondition sq.Sqlizer, query types.DashboardQuery) ([]types.DashboardDepartmentStat, error)
}

type DashboardRepository struct {
	storage *pgxpool.Pool
	logger  *zap.Logger
}

func NewDashboardRepository(storage *pgxpool.Pool, logger *zap.Logger) DashboardRepositoryInterface {
	return &DashboardRepository{storage: storage, logger: logger}
}

func (r *DashboardRepository) GetAlerts(ctx context.Context, securityCondition sq.Sqlizer, queryOptions types.DashboardQuery) (*types.DashboardAlerts, error) {
	builder := sq.Select(
		"COUNT(CASE WHEN p.code = 'CRITICAL' AND "+dashboardOpenCheck+" THEN 1 END)",
		"COUNT(CASE WHEN o.duration IS NOT NULL AND o.duration < NOW() AND "+dashboardOpenCheck+" THEN 1 END)",
	).
		From("orders o").
		LeftJoin("statuses s ON o.status_id = s.id").
		LeftJoin("priorities p ON o.priority_id = p.id").
		Where(sq.Eq{"o.deleted_at": nil})

	builder = applyDashboardSecurity(builder, securityCondition)
	query, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}

	result := &types.DashboardAlerts{}
	err = r.storage.QueryRow(ctx, query, args...).Scan(&result.CriticalCount, &result.OverdueCount)
	return result, err
}

func (r *DashboardRepository) GetKPIsWithUser(ctx context.Context, securityCondition sq.Sqlizer, queryOptions types.DashboardQuery) (*types.DashboardKPIs, error) {
	base := sq.Select(
		"o.id",
		"o.created_at",
		"o.completed_at",
		dashboardLatestStatusChangeTimestampExpr("o", pkgconstants.StatusClosed, "closed_at"),
		"o.duration",
		"o.first_response_time_seconds",
		"o.resolution_time_seconds",
		"o.is_first_contact_resolution",
		"o.executor_id",
		"o.user_id",
		"s.code AS status_code",
	).
		From("orders o").
		LeftJoin("statuses s ON o.status_id = s.id").
		Where(sq.Eq{"o.deleted_at": nil})
	base = applyDashboardSecurity(base, securityCondition)

	baseSQL, baseArgs, err := base.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}

	idx := len(baseArgs)
	currFromIdx := idx + 1
	currToIdx := idx + 2
	prevFromIdx := idx + 3
	prevToIdx := idx + 4
	userIDIdx := idx + 5

	closedStatusCheck := dashboardStatusInCheck("status_code", dashboardClosedStatuses)
	openStatusCheck := dashboardStatusNotInCheck("status_code", dashboardOpenExcludedStatuses)
	slaEligibleCheck := dashboardSLAEligibleCheck("duration")
	slaOnTimeCheck := dashboardSLAOnTimeCheck("duration", "closed_at")
	avgResolveExpr := dashboardResolutionSecondsExpr("closed_at", "created_at")
	activeAgentEventCheck := dashboardEventNotInCheck("h.event_type", dashboardActiveAgentExcludedEvents)
	activeAgentAssigneeCheck := dashboardLatestDelegationAssigneeCheck("h")

	sqlRaw := fmt.Sprintf(`
		WITH orders_filtered AS (%s),
		active_agents AS (
			SELECT COUNT(DISTINCT h.user_id) AS active_agent_count
			FROM order_history h
			JOIN orders_filtered fo ON fo.id = h.order_id
			WHERE h.user_id IS NOT NULL
			  AND h.created_at >= $%d
			  AND h.created_at < $%d
			  AND %s
			  AND %s
		)
		SELECT
			COUNT(*) FILTER (WHERE created_at >= $%d AND created_at < $%d) AS total_current,
			COUNT(*) FILTER (WHERE created_at >= $%d AND created_at < $%d) AS total_previous,
			COUNT(*) FILTER (WHERE created_at >= $%d AND created_at < $%d AND (user_id = $%d OR executor_id = $%d)) AS total_personal,

			COUNT(*) FILTER (WHERE %s AND closed_at >= $%d AND closed_at < $%d) AS resolved_current,
			COUNT(*) FILTER (WHERE %s AND closed_at >= $%d AND closed_at < $%d) AS resolved_previous,
			COUNT(*) FILTER (WHERE %s AND closed_at >= $%d AND closed_at < $%d AND (user_id = $%d OR executor_id = $%d)) AS resolved_personal,

			COUNT(*) FILTER (WHERE %s) AS open_current,
			COUNT(*) FILTER (WHERE %s AND (user_id = $%d OR executor_id = $%d)) AS open_personal,

			COUNT(*) FILTER (WHERE %s AND closed_at >= $%d AND closed_at < $%d AND %s) AS sla_current_total,
			COUNT(*) FILTER (WHERE %s AND closed_at >= $%d AND closed_at < $%d AND %s) AS sla_previous_total,
			COUNT(*) FILTER (WHERE %s AND closed_at >= $%d AND closed_at < $%d AND %s) AS sla_current_ontime,
			COUNT(*) FILTER (WHERE %s AND closed_at >= $%d AND closed_at < $%d AND %s) AS sla_previous_ontime,

			COALESCE(AVG(first_response_time_seconds) FILTER (WHERE created_at >= $%d AND created_at < $%d AND first_response_time_seconds >= 0), 0) AS avg_response_current,
			COALESCE(AVG(first_response_time_seconds) FILTER (WHERE created_at >= $%d AND created_at < $%d AND first_response_time_seconds >= 0), 0) AS avg_response_previous,

			COALESCE(AVG(%s) FILTER (WHERE %s AND closed_at >= $%d AND closed_at < $%d), 0) AS avg_resolve_current,
			COALESCE(AVG(%s) FILTER (WHERE %s AND closed_at >= $%d AND closed_at < $%d), 0) AS avg_resolve_previous,

			COALESCE((SELECT active_agent_count FROM active_agents), 0) AS active_agents,

			COUNT(*) FILTER (WHERE %s AND closed_at >= $%d AND closed_at < $%d AND is_first_contact_resolution = true) AS fcr_current,
			COUNT(*) FILTER (WHERE %s AND closed_at >= $%d AND closed_at < $%d AND is_first_contact_resolution = true) AS fcr_previous
		FROM orders_filtered
	`,
		baseSQL,
		currFromIdx, currToIdx, activeAgentEventCheck, activeAgentAssigneeCheck,
		currFromIdx, currToIdx,
		prevFromIdx, prevToIdx,
		currFromIdx, currToIdx, userIDIdx, userIDIdx,

		closedStatusCheck, currFromIdx, currToIdx,
		closedStatusCheck, prevFromIdx, prevToIdx,
		closedStatusCheck, currFromIdx, currToIdx, userIDIdx, userIDIdx,

		openStatusCheck,
		openStatusCheck, userIDIdx, userIDIdx,

		closedStatusCheck, currFromIdx, currToIdx, slaEligibleCheck,
		closedStatusCheck, prevFromIdx, prevToIdx, slaEligibleCheck,
		closedStatusCheck, currFromIdx, currToIdx, slaOnTimeCheck,
		closedStatusCheck, prevFromIdx, prevToIdx, slaOnTimeCheck,

		currFromIdx, currToIdx,
		prevFromIdx, prevToIdx,

		avgResolveExpr, closedStatusCheck, currFromIdx, currToIdx,
		avgResolveExpr, closedStatusCheck, prevFromIdx, prevToIdx,

		closedStatusCheck, currFromIdx, currToIdx,
		closedStatusCheck, prevFromIdx, prevToIdx,
	)

	args := append(
		baseArgs,
		queryOptions.Range.From,
		queryOptions.Range.To,
		queryOptions.PreviousRange.From,
		queryOptions.PreviousRange.To,
		queryOptions.UserID,
	)

	var (
		totalCurrent      int64
		totalPrevious     int64
		totalPersonal     int64
		resolvedCurrent   int64
		resolvedPrevious  int64
		resolvedPersonal  int64
		openCurrent       int64
		openPersonal      int64
		slaCurrentTotal   int64
		slaPreviousTotal  int64
		slaCurrentOnTime  int64
		slaPreviousOnTime int64
		avgRespCurrent    float64
		avgRespPrevious   float64
		avgResCurrent     float64
		avgResPrevious    float64
		activeAgents      int64
		fcrCurrent        int64
		fcrPrevious       int64
	)

	if err := r.storage.QueryRow(ctx, sqlRaw, args...).Scan(
		&totalCurrent,
		&totalPrevious,
		&totalPersonal,
		&resolvedCurrent,
		&resolvedPrevious,
		&resolvedPersonal,
		&openCurrent,
		&openPersonal,
		&slaCurrentTotal,
		&slaPreviousTotal,
		&slaCurrentOnTime,
		&slaPreviousOnTime,
		&avgRespCurrent,
		&avgRespPrevious,
		&avgResCurrent,
		&avgResPrevious,
		&activeAgents,
		&fcrCurrent,
		&fcrPrevious,
	); err != nil {
		return nil, err
	}

	result := &types.DashboardKPIs{
		TotalTickets: types.DashboardKPIMetric{
			Current:  float64(totalCurrent),
			Previous: float64(totalPrevious),
			Personal: float64(totalPersonal),
		},
		OpenTickets: types.DashboardKPIMetric{
			Current:  float64(openCurrent),
			Personal: float64(openPersonal),
		},
		ResolvedTickets: types.DashboardKPIMetric{
			Current:  float64(resolvedCurrent),
			Previous: float64(resolvedPrevious),
			Personal: float64(resolvedPersonal),
		},
		SLACompliance: types.DashboardKPIMetric{},
		AvgResponseTime: types.DashboardKPIMetric{
			Current:  avgRespCurrent,
			Previous: avgRespPrevious,
		},
		AvgResolveTime: types.DashboardKPIMetric{
			Current:  avgResCurrent,
			Previous: avgResPrevious,
		},
		FCRRate:      types.DashboardKPIMetric{},
		ActiveAgents: activeAgents,
	}

	if slaCurrentTotal > 0 {
		result.SLACompliance.Current = (float64(slaCurrentOnTime) / float64(slaCurrentTotal)) * 100
	}
	if slaPreviousTotal > 0 {
		result.SLACompliance.Previous = (float64(slaPreviousOnTime) / float64(slaPreviousTotal)) * 100
	}
	if resolvedCurrent > 0 {
		result.FCRRate.Current = (float64(fcrCurrent) / float64(resolvedCurrent)) * 100
	}
	if resolvedPrevious > 0 {
		result.FCRRate.Previous = (float64(fcrPrevious) / float64(resolvedPrevious)) * 100
	}

	return result, nil
}

func (r *DashboardRepository) GetSLAStats(ctx context.Context, securityCondition sq.Sqlizer, queryOptions types.DashboardQuery) (*types.DashboardSLAStats, error) {
	closedAtExpr := dashboardLatestStatusChangeTimestampScalarExpr("o", pkgconstants.StatusClosed)
	slaOnTimeCheck := dashboardSLAOnTimeCheck("o.duration", closedAtExpr)
	builder := sq.Select(
		"COUNT(CASE WHEN o.duration IS NOT NULL THEN 1 END)",
		"COUNT(CASE WHEN "+slaOnTimeCheck+" THEN 1 END)",
	).
		From("orders o").
		Join("statuses s ON o.status_id = s.id").
		Where(sq.Eq{"o.deleted_at": nil}).
		Where(dashboardResolvedCheck)
	builder = applyDashboardSecurity(builder, securityCondition)
	builder = applyDashboardExprRange(builder, closedAtExpr, queryOptions.Range)

	query, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}

	result := &types.DashboardSLAStats{}
	err = r.storage.QueryRow(ctx, query, args...).Scan(&result.TotalCompleted, &result.OnTime)
	return result, err
}

func (r *DashboardRepository) GetAvgTimeByPriority(ctx context.Context, securityCondition sq.Sqlizer, queryOptions types.DashboardQuery) ([]types.DashboardTimeByGroup, error) {
	closedAtExpr := dashboardLatestStatusChangeTimestampScalarExpr("o", pkgconstants.StatusClosed)
	avgResolveExpr := dashboardResolutionSecondsExpr(closedAtExpr, "o.created_at")
	builder := sq.Select(
		"p.name AS group_name",
		fmt.Sprintf("COALESCE(AVG(%s), 0) AS avg_seconds", avgResolveExpr),
	).
		From("orders o").
		Join("priorities p ON o.priority_id = p.id").
		Join("statuses s ON o.status_id = s.id").
		Where(dashboardResolvedCheck).
		Where(sq.Eq{"o.deleted_at": nil}).
		GroupBy("p.name")
	builder = applyDashboardSecurity(builder, securityCondition)
	builder = applyDashboardExprRange(builder, closedAtExpr, queryOptions.Range)
	return collectDashboardTimeGroups(ctx, r.storage, builder)
}

func (r *DashboardRepository) GetAvgTimeByOrderType(ctx context.Context, securityCondition sq.Sqlizer, queryOptions types.DashboardQuery) ([]types.DashboardTimeByGroup, error) {
	closedAtExpr := dashboardLatestStatusChangeTimestampScalarExpr("o", pkgconstants.StatusClosed)
	avgResolveExpr := dashboardResolutionSecondsExpr(closedAtExpr, "o.created_at")
	builder := sq.Select(
		"ot.name AS group_name",
		fmt.Sprintf("COALESCE(AVG(%s), 0) AS avg_seconds", avgResolveExpr),
	).
		From("orders o").
		Join("order_types ot ON o.order_type_id = ot.id").
		Join("statuses s ON o.status_id = s.id").
		Where(dashboardResolvedCheck).
		Where(sq.Eq{"o.deleted_at": nil}).
		GroupBy("ot.name")
	builder = applyDashboardSecurity(builder, securityCondition)
	builder = applyDashboardExprRange(builder, closedAtExpr, queryOptions.Range)
	return collectDashboardTimeGroups(ctx, r.storage, builder)
}

func (r *DashboardRepository) GetCountByStatus(ctx context.Context, securityCondition sq.Sqlizer, queryOptions types.DashboardQuery) ([]types.DashboardCountByGroup, error) {
	builder := sq.Select("s.name AS group_name", "COUNT(o.id) AS count").
		From("orders o").
		Join("statuses s ON o.status_id = s.id").
		Where(sq.Eq{"o.deleted_at": nil}).
		GroupBy("s.name").
		OrderBy("count DESC")
	builder = applyDashboardSecurity(builder, securityCondition)
	builder = applyDashboardRange(builder, "o.created_at", queryOptions.Range)
	return collectDashboardCountGroups(ctx, r.storage, builder)
}

func (r *DashboardRepository) GetCountByExecutor(ctx context.Context, securityCondition sq.Sqlizer, queryOptions types.DashboardQuery) ([]types.DashboardExecutorCount, error) {
	builder := sq.Select(
		"COALESCE(u.fio, 'Не назначен') AS group_name",
		"COUNT(o.id) AS count",
		"u.id AS user_id",
	).
		From("orders o").
		LeftJoin("users u ON o.executor_id = u.id").
		LeftJoin("statuses s ON o.status_id = s.id").
		Where(sq.Eq{"o.deleted_at": nil}).
		Where(dashboardOpenCheck).
		GroupBy("u.id", "u.fio").
		OrderBy("count DESC").
		Limit(15)
	builder = applyDashboardSecurity(builder, securityCondition)
	builder = applyDashboardRange(builder, "o.created_at", queryOptions.Range)
	return collectDashboardExecutorCounts(ctx, r.storage, builder)
}

func (r *DashboardRepository) GetWeeklyVolume(ctx context.Context, securityCondition sq.Sqlizer, queryOptions types.DashboardQuery) ([]types.DashboardChartData, error) {
	bucketExpr := dashboardBucketExpression(queryOptions.Granularity)
	builder := sq.Select(
		fmt.Sprintf("to_char(%s, 'YYYY-MM-DD') AS label", bucketExpr),
		"COUNT(*) AS value",
	).
		From("orders o").
		Where(sq.Eq{"o.deleted_at": nil}).
		GroupBy(bucketExpr).
		OrderBy(bucketExpr + " ASC")
	builder = applyDashboardSecurity(builder, securityCondition)
	builder = applyDashboardRange(builder, "o.created_at", queryOptions.Range)

	query, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := r.storage.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[types.DashboardChartData])
}

func (r *DashboardRepository) GetTopCategories(ctx context.Context, securityCondition sq.Sqlizer, queryOptions types.DashboardQuery) ([]types.DashboardCountByGroup, error) {
	builder := sq.Select(
		"ot.name AS group_name",
		"COUNT(o.id) AS count",
	).
		From("orders o").
		Join("order_types ot ON o.order_type_id = ot.id").
		Where(sq.Eq{"o.deleted_at": nil}).
		GroupBy("ot.name").
		OrderBy("count DESC").
		Limit(5)
	builder = applyDashboardSecurity(builder, securityCondition)
	builder = applyDashboardRange(builder, "o.created_at", queryOptions.Range)
	return collectDashboardCountGroups(ctx, r.storage, builder)
}

func (r *DashboardRepository) GetDepartmentStats(ctx context.Context, securityCondition sq.Sqlizer, queryOptions types.DashboardQuery) ([]types.DashboardDepartmentStat, error) {
	builder := buildDashboardOrgStatsBuilder("d.name", "departments d ON o.department_id = d.id", queryOptions)
	builder = applyDashboardSecurity(builder, securityCondition)
	return collectDashboardDepartmentStats(ctx, r.storage, builder)
}

func (r *DashboardRepository) GetLastActivity(ctx context.Context, securityCondition sq.Sqlizer, queryOptions types.DashboardQuery) ([]types.DashboardActivityItem, error) {
	resolver := newDashboardActivityReferenceResolver(ctx, r)
	builder := sq.Select(
		"h.id",
		"h.created_at",
		"h.event_type",
		"h.comment",
		"h.new_value",
		"COALESCE(u.fio, 'Система')",
		"o.name",
	).
		From("order_history h").
		Join("orders o ON h.order_id = o.id").
		LeftJoin("users u ON h.user_id = u.id").
		Where(sq.Eq{"o.deleted_at": nil}).
		OrderBy("h.created_at DESC").
		Limit(10)
	builder = applyDashboardSecurity(builder, securityCondition)
	builder = applyDashboardRange(builder, "h.created_at", queryOptions.Range)

	query, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := r.storage.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]types.DashboardActivityItem, 0, 10)
	for rows.Next() {
		var item types.DashboardActivityItem
		var createdAt time.Time
		var eventType, comment, newValue *string

		if err := rows.Scan(&item.ID, &createdAt, &eventType, &comment, &newValue, &item.AuthorName, &item.OrderName); err != nil {
			return nil, err
		}

		item.Date = createdAt.Format("02.01 15:04")
		switch {
		case comment != nil && strings.TrimSpace(*comment) != "":
			if eventType != nil {
				item.Text = resolver.renderComment(*eventType, *comment)
			} else {
				item.Text = *comment
			}
		case eventType != nil:
			item.Text = mapDashboardEventText(*eventType)
		default:
			item.Text = "Обновил заявку"
		}

		items = append(items, item)
	}

	return items, rows.Err()
}

func (r *DashboardRepository) GetBranchStats(ctx context.Context, securityCondition sq.Sqlizer, queryOptions types.DashboardQuery) ([]types.DashboardDepartmentStat, error) {
	builder := buildDashboardOrgStatsBuilder("b.name", "branches b ON o.branch_id = b.id", queryOptions).
		Where(sq.Eq{"o.department_id": nil})
	builder = applyDashboardSecurity(builder, securityCondition)
	return collectDashboardDepartmentStats(ctx, r.storage, builder)
}

func applyDashboardSecurity(builder sq.SelectBuilder, securityCondition sq.Sqlizer) sq.SelectBuilder {
	if securityCondition == nil {
		return builder
	}
	return builder.Where(securityCondition)
}

func applyDashboardRange(builder sq.SelectBuilder, column string, dateRange types.DashboardDateRange) sq.SelectBuilder {
	return builder.Where(sq.GtOrEq{column: dateRange.From}).Where(sq.Lt{column: dateRange.To})
}

func applyDashboardExprRange(builder sq.SelectBuilder, expr string, dateRange types.DashboardDateRange) sq.SelectBuilder {
	return builder.Where(sq.Expr(expr+" >= ?", dateRange.From)).Where(sq.Expr(expr+" < ?", dateRange.To))
}

func dashboardStatusInCheck(column string, codes []string) string {
	return fmt.Sprintf("%s IN (%s)", column, dashboardQuotedStatusList(codes))
}

func dashboardStatusNotInCheck(column string, codes []string) string {
	return fmt.Sprintf("%s NOT IN (%s)", column, dashboardQuotedStatusList(codes))
}

func dashboardEventNotInCheck(column string, eventTypes []string) string {
	return fmt.Sprintf("%s NOT IN (%s)", column, dashboardQuotedStatusList(eventTypes))
}

func dashboardQuotedStatusList(codes []string) string {
	quoted := make([]string, 0, len(codes))
	for _, code := range codes {
		quoted = append(quoted, fmt.Sprintf("'%s'", strings.ReplaceAll(code, "'", "''")))
	}

	return strings.Join(quoted, ", ")
}

func dashboardSLAEligibleCheck(durationColumn string) string {
	return durationColumn + " IS NOT NULL"
}

func dashboardSLAOnTimeCheck(durationColumn, completedColumn string) string {
	return fmt.Sprintf("%s AND %s <= %s", dashboardSLAEligibleCheck(durationColumn), completedColumn, durationColumn)
}

func dashboardResolutionSecondsExpr(closedAtExpr, createdColumn string) string {
	return fmt.Sprintf("GREATEST(EXTRACT(EPOCH FROM (%s - %s)), 0)", closedAtExpr, createdColumn)
}

func dashboardLatestStatusChangeTimestampExpr(orderAlias, statusCode, alias string) string {
	return dashboardLatestStatusChangeTimestampScalarExpr(orderAlias, statusCode) + " AS " + alias
}

func dashboardLatestStatusChangeTimestampScalarExpr(orderAlias, statusCode string) string {
	return fmt.Sprintf(`(
		SELECT h.created_at
		FROM order_history h
		JOIN statuses target_status ON target_status.code = '%s'
		WHERE h.order_id = %s.id
		  AND h.event_type = 'STATUS_CHANGE'
		  AND h.new_value = target_status.id::text
		ORDER BY h.created_at DESC
		LIMIT 1
	)`, strings.ReplaceAll(statusCode, "'", "''"), orderAlias)
}

func dashboardLatestDelegationAssigneeCheck(historyAlias string) string {
	return fmt.Sprintf(`COALESCE((
		SELECT assign.new_value
		FROM order_history assign
		WHERE assign.order_id = %s.order_id
		  AND assign.event_type = 'DELEGATION'
		  AND assign.created_at <= %s.created_at
		ORDER BY assign.created_at DESC
		LIMIT 1
	), '') = %s.user_id::text`, historyAlias, historyAlias, historyAlias)
}

func dashboardBucketExpression(granularity string) string {
	switch granularity {
	case types.DashboardGranularityMonth:
		return "date_trunc('month', o.created_at)"
	case types.DashboardGranularityWeek:
		return "date_trunc('week', o.created_at)"
	default:
		return "date_trunc('day', o.created_at)"
	}
}

func buildDashboardOrgStatsBuilder(groupColumn, joinClause string, queryOptions types.DashboardQuery) sq.SelectBuilder {
	builder := sq.Select(
		groupColumn,
		"COUNT(CASE WHEN "+dashboardOpenCheck+" THEN 1 END) AS open_count",
		"COUNT(CASE WHEN "+dashboardResolvedCheck+" THEN 1 END) AS resolved_count",
		"COUNT(CASE WHEN p.code = 'CRITICAL' AND "+dashboardOpenCheck+" THEN 1 END) AS critical_count",
		"COUNT(*) AS total_count",
	).
		From("orders o").
		Join(joinClause).
		LeftJoin("statuses s ON o.status_id = s.id").
		LeftJoin("priorities p ON o.priority_id = p.id").
		Where(sq.Eq{"o.deleted_at": nil}).
		GroupBy(groupColumn)

	return applyDashboardRange(builder, "o.created_at", queryOptions.Range)
}

func collectDashboardTimeGroups(ctx context.Context, storage *pgxpool.Pool, builder sq.SelectBuilder) ([]types.DashboardTimeByGroup, error) {
	query, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := storage.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[types.DashboardTimeByGroup])
}

func collectDashboardCountGroups(ctx context.Context, storage *pgxpool.Pool, builder sq.SelectBuilder) ([]types.DashboardCountByGroup, error) {
	query, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := storage.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[types.DashboardCountByGroup])
}

func collectDashboardExecutorCounts(ctx context.Context, storage *pgxpool.Pool, builder sq.SelectBuilder) ([]types.DashboardExecutorCount, error) {
	query, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := storage.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[types.DashboardExecutorCount])
}

func collectDashboardDepartmentStats(ctx context.Context, storage *pgxpool.Pool, builder sq.SelectBuilder) ([]types.DashboardDepartmentStat, error) {
	query, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := storage.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[types.DashboardDepartmentStat])
}

type dashboardActivityReferenceResolver struct {
	ctx  context.Context
	repo *DashboardRepository

	departmentNames map[uint64]string
	departmentSeen  map[uint64]bool
	otdelNames      map[uint64]string
	otdelSeen       map[uint64]bool
	branchNames     map[uint64]string
	branchSeen      map[uint64]bool
	officeNames     map[uint64]string
	officeSeen      map[uint64]bool
}

func newDashboardActivityReferenceResolver(ctx context.Context, repo *DashboardRepository) *dashboardActivityReferenceResolver {
	return &dashboardActivityReferenceResolver{
		ctx:             ctx,
		repo:            repo,
		departmentNames: make(map[uint64]string),
		departmentSeen:  make(map[uint64]bool),
		otdelNames:      make(map[uint64]string),
		otdelSeen:       make(map[uint64]bool),
		branchNames:     make(map[uint64]string),
		branchSeen:      make(map[uint64]bool),
		officeNames:     make(map[uint64]string),
		officeSeen:      make(map[uint64]bool),
	}
}

func (r *dashboardActivityReferenceResolver) renderComment(eventType, comment string) string {
	comment = strings.TrimSpace(comment)
	if eventType != "STRUCTURE_CHANGE" || comment == "" {
		return comment
	}

	return humanizeDashboardStructureComment(comment, r.nameByField)
}

func humanizeDashboardStructureComment(comment string, lookup func(field string, id uint64) string) string {
	comment = strings.TrimSpace(comment)
	const prefix = "Смена структуры:"
	if comment == "" || !strings.HasPrefix(comment, prefix) {
		return comment
	}

	body := strings.TrimSpace(strings.TrimPrefix(comment, prefix))
	if body == "" {
		return comment
	}

	rawParts := strings.Split(body, ";")
	parts := make([]string, 0, len(rawParts))
	for _, rawPart := range rawParts {
		if part := humanizeDashboardStructurePart(rawPart, lookup); part != "" {
			parts = append(parts, part)
		}
	}

	if len(parts) == 0 {
		return comment
	}

	return prefix + " " + strings.Join(parts, "; ")
}

func humanizeDashboardStructurePart(part string, lookup func(field string, id uint64) string) string {
	part = strings.TrimSpace(part)
	if part == "" {
		return ""
	}

	fieldAndValue := strings.SplitN(part, ":", 2)
	if len(fieldAndValue) != 2 {
		return part
	}

	field := strings.TrimSpace(fieldAndValue[0])
	label, ok := dashboardStructureFieldLabel(field)
	if !ok {
		return part
	}

	transition := strings.TrimSpace(fieldAndValue[1])
	values := strings.SplitN(transition, "→", 2)
	if len(values) != 2 {
		values = strings.SplitN(transition, "->", 2)
	}
	if len(values) != 2 {
		value := humanizeDashboardStructureValue(field, transition, lookup)
		if value == "" {
			return label
		}
		return fmt.Sprintf("%s: %s", label, value)
	}

	oldValue := humanizeDashboardStructureValue(field, values[0], lookup)
	newValue := humanizeDashboardStructureValue(field, values[1], lookup)

	switch {
	case oldValue != "" && newValue != "":
		return fmt.Sprintf("%s: %s → %s", label, oldValue, newValue)
	case newValue != "":
		return fmt.Sprintf("%s: %s", label, newValue)
	case oldValue != "":
		return fmt.Sprintf("%s: %s → —", label, oldValue)
	default:
		return label
	}
}

func humanizeDashboardStructureValue(field, raw string, lookup func(field string, id uint64) string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "—" {
		return ""
	}

	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return raw
	}

	if lookup != nil {
		if name := strings.TrimSpace(lookup(field, id)); name != "" {
			return fmt.Sprintf("«%s»", name)
		}
	}

	return fmt.Sprintf("ID %d", id)
}

func dashboardStructureFieldLabel(field string) (string, bool) {
	switch field {
	case "department_id":
		return "департамент", true
	case "otdel_id":
		return "отдел", true
	case "branch_id":
		return "филиал", true
	case "office_id":
		return "офис", true
	default:
		return "", false
	}
}

func (r *dashboardActivityReferenceResolver) nameByField(field string, id uint64) string {
	switch field {
	case "department_id":
		return r.lookupName(id, r.departmentNames, r.departmentSeen, "SELECT name FROM departments WHERE id = $1")
	case "otdel_id":
		return r.lookupName(id, r.otdelNames, r.otdelSeen, "SELECT name FROM otdels WHERE id = $1")
	case "branch_id":
		return r.lookupName(id, r.branchNames, r.branchSeen, "SELECT name FROM branches WHERE id = $1")
	case "office_id":
		return r.lookupName(id, r.officeNames, r.officeSeen, "SELECT name FROM offices WHERE id = $1")
	default:
		return ""
	}
}

func (r *dashboardActivityReferenceResolver) lookupName(id uint64, cache map[uint64]string, seen map[uint64]bool, query string) string {
	if seen[id] {
		return cache[id]
	}

	seen[id] = true

	var name string
	if err := r.repo.storage.QueryRow(r.ctx, query, id).Scan(&name); err != nil {
		if r.repo.logger != nil {
			r.repo.logger.Debug("dashboard activity reference lookup failed", zap.Uint64("id", id), zap.Error(err))
		}
		return ""
	}

	cache[id] = name
	return name
}

func mapDashboardEventText(eventType string) string {
	switch eventType {
	case "CREATE":
		return "Создал новую заявку"
	case "STATUS_CHANGE":
		return "Изменил статус заявки"
	case "DELEGATION":
		return "Изменил исполнителя заявки"
	default:
		return "Обновил заявку"
	}
}
