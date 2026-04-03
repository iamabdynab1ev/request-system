package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	pkgconstants "request-system/pkg/constants"
	"request-system/pkg/types"
)

const (
	dashboardResolvedCheck = "s.code = 'COMPLETED'"
	dashboardOpenCheck     = "s.code <> 'CLOSED'"
)

type DashboardRepositoryInterface interface {
	GetAlerts(ctx context.Context, securityCondition sq.Sqlizer) (*types.DashboardAlerts, error)
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

func (r *DashboardRepository) GetAlerts(ctx context.Context, securityCondition sq.Sqlizer) (*types.DashboardAlerts, error) {
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

	sqlRaw := fmt.Sprintf(`
		WITH orders_filtered AS (%s)
		SELECT
			COUNT(*) FILTER (WHERE created_at >= $%d AND created_at < $%d) AS total_current,
			COUNT(*) FILTER (WHERE created_at >= $%d AND created_at < $%d) AS total_previous,
			COUNT(*) FILTER (WHERE created_at >= $%d AND created_at < $%d AND (user_id = $%d OR executor_id = $%d)) AS total_personal,

			COUNT(*) FILTER (WHERE status_code = '%s' AND completed_at >= $%d AND completed_at < $%d) AS resolved_current,
			COUNT(*) FILTER (WHERE status_code = '%s' AND completed_at >= $%d AND completed_at < $%d) AS resolved_previous,
			COUNT(*) FILTER (WHERE status_code = '%s' AND completed_at >= $%d AND completed_at < $%d AND (user_id = $%d OR executor_id = $%d)) AS resolved_personal,

			COUNT(*) FILTER (WHERE status_code <> '%s') AS open_current,
			COUNT(*) FILTER (WHERE status_code <> '%s' AND (user_id = $%d OR executor_id = $%d)) AS open_personal,

			COUNT(*) FILTER (WHERE status_code = '%s' AND completed_at >= $%d AND completed_at < $%d AND (duration IS NULL OR completed_at <= duration)) AS sla_current_ontime,
			COUNT(*) FILTER (WHERE status_code = '%s' AND completed_at >= $%d AND completed_at < $%d AND (duration IS NULL OR completed_at <= duration)) AS sla_previous_ontime,

			COALESCE(AVG(first_response_time_seconds) FILTER (WHERE created_at >= $%d AND created_at < $%d AND first_response_time_seconds >= 0), 0) AS avg_response_current,
			COALESCE(AVG(first_response_time_seconds) FILTER (WHERE created_at >= $%d AND created_at < $%d AND first_response_time_seconds >= 0), 0) AS avg_response_previous,

			COALESCE(AVG(resolution_time_seconds) FILTER (WHERE status_code = '%s' AND completed_at >= $%d AND completed_at < $%d AND resolution_time_seconds >= 0), 0) AS avg_resolve_current,
			COALESCE(AVG(resolution_time_seconds) FILTER (WHERE status_code = '%s' AND completed_at >= $%d AND completed_at < $%d AND resolution_time_seconds >= 0), 0) AS avg_resolve_previous,

			COUNT(DISTINCT executor_id) FILTER (WHERE status_code <> '%s' AND executor_id IS NOT NULL) AS active_agents,

			COUNT(*) FILTER (WHERE status_code = '%s' AND completed_at >= $%d AND completed_at < $%d AND is_first_contact_resolution = true) AS fcr_current,
			COUNT(*) FILTER (WHERE status_code = '%s' AND completed_at >= $%d AND completed_at < $%d AND is_first_contact_resolution = true) AS fcr_previous
		FROM orders_filtered
	`,
		baseSQL,
		currFromIdx, currToIdx,
		prevFromIdx, prevToIdx,
		currFromIdx, currToIdx, userIDIdx, userIDIdx,

		pkgconstants.StatusCompleted, currFromIdx, currToIdx,
		pkgconstants.StatusCompleted, prevFromIdx, prevToIdx,
		pkgconstants.StatusCompleted, currFromIdx, currToIdx, userIDIdx, userIDIdx,

		pkgconstants.StatusClosed,
		pkgconstants.StatusClosed, userIDIdx, userIDIdx,

		pkgconstants.StatusCompleted, currFromIdx, currToIdx,
		pkgconstants.StatusCompleted, prevFromIdx, prevToIdx,

		currFromIdx, currToIdx,
		prevFromIdx, prevToIdx,

		pkgconstants.StatusCompleted, currFromIdx, currToIdx,
		pkgconstants.StatusCompleted, prevFromIdx, prevToIdx,

		pkgconstants.StatusClosed,

		pkgconstants.StatusCompleted, currFromIdx, currToIdx,
		pkgconstants.StatusCompleted, prevFromIdx, prevToIdx,
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

	if resolvedCurrent > 0 {
		result.SLACompliance.Current = (float64(slaCurrentOnTime) / float64(resolvedCurrent)) * 100
		result.FCRRate.Current = (float64(fcrCurrent) / float64(resolvedCurrent)) * 100
	}
	if resolvedPrevious > 0 {
		result.SLACompliance.Previous = (float64(slaPreviousOnTime) / float64(resolvedPrevious)) * 100
		result.FCRRate.Previous = (float64(fcrPrevious) / float64(resolvedPrevious)) * 100
	}

	return result, nil
}

func (r *DashboardRepository) GetSLAStats(ctx context.Context, securityCondition sq.Sqlizer, queryOptions types.DashboardQuery) (*types.DashboardSLAStats, error) {
	builder := sq.Select(
		"COUNT(*)",
		"COUNT(CASE WHEN o.duration IS NULL OR o.completed_at <= o.duration THEN 1 END)",
	).
		From("orders o").
		Join("statuses s ON o.status_id = s.id").
		Where(sq.Eq{"o.deleted_at": nil}).
		Where(dashboardResolvedCheck)
	builder = applyDashboardSecurity(builder, securityCondition)
	builder = applyDashboardRange(builder, "o.completed_at", queryOptions.Range)

	query, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}

	result := &types.DashboardSLAStats{}
	err = r.storage.QueryRow(ctx, query, args...).Scan(&result.TotalCompleted, &result.OnTime)
	return result, err
}

func (r *DashboardRepository) GetAvgTimeByPriority(ctx context.Context, securityCondition sq.Sqlizer, queryOptions types.DashboardQuery) ([]types.DashboardTimeByGroup, error) {
	builder := sq.Select(
		"p.name AS group_name",
		"COALESCE(AVG(o.resolution_time_seconds), 0) AS avg_seconds",
	).
		From("orders o").
		Join("priorities p ON o.priority_id = p.id").
		Join("statuses s ON o.status_id = s.id").
		Where(dashboardResolvedCheck).
		Where(sq.Eq{"o.deleted_at": nil}).
		GroupBy("p.name")
	builder = applyDashboardSecurity(builder, securityCondition)
	builder = applyDashboardRange(builder, "o.completed_at", queryOptions.Range)
	return collectDashboardTimeGroups(ctx, r.storage, builder)
}

func (r *DashboardRepository) GetAvgTimeByOrderType(ctx context.Context, securityCondition sq.Sqlizer, queryOptions types.DashboardQuery) ([]types.DashboardTimeByGroup, error) {
	builder := sq.Select(
		"ot.name AS group_name",
		"COALESCE(AVG(o.resolution_time_seconds), 0) AS avg_seconds",
	).
		From("orders o").
		Join("order_types ot ON o.order_type_id = ot.id").
		Join("statuses s ON o.status_id = s.id").
		Where(dashboardResolvedCheck).
		Where(sq.Eq{"o.deleted_at": nil}).
		GroupBy("ot.name")
	builder = applyDashboardSecurity(builder, securityCondition)
	builder = applyDashboardRange(builder, "o.completed_at", queryOptions.Range)
	return collectDashboardTimeGroups(ctx, r.storage, builder)
}

func (r *DashboardRepository) GetCountByStatus(ctx context.Context, securityCondition sq.Sqlizer, _ types.DashboardQuery) ([]types.DashboardCountByGroup, error) {
	builder := sq.Select("s.name AS group_name", "COUNT(o.id) AS count").
		From("orders o").
		Join("statuses s ON o.status_id = s.id").
		Where(sq.Eq{"o.deleted_at": nil}).
		GroupBy("s.name").
		OrderBy("count DESC")
	builder = applyDashboardSecurity(builder, securityCondition)
	return collectDashboardCountGroups(ctx, r.storage, builder)
}

func (r *DashboardRepository) GetCountByExecutor(ctx context.Context, securityCondition sq.Sqlizer, _ types.DashboardQuery) ([]types.DashboardExecutorCount, error) {
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
			item.Text = *comment
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
