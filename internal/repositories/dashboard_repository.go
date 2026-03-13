package repositories

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"request-system/pkg/types"
)

type DashboardRepositoryInterface interface {
	GetAlerts(ctx context.Context, securityCondition sq.Sqlizer) (*types.DashboardAlerts, error)
	GetKPIsWithUser(ctx context.Context, securityCondition sq.Sqlizer, userID uint64) (*types.DashboardKPIs, error)
	GetSLAStats(ctx context.Context, securityCondition sq.Sqlizer) (*types.DashboardSLAStats, error)
	GetAvgTimeByPriority(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardTimeByGroup, error)
	GetAvgTimeByOrderType(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardTimeByGroup, error)
	GetCountByStatus(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardCountByGroup, error)
	GetCountByExecutor(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardCountByGroup, error)
	GetWeeklyVolume(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardChartData, error)
	GetTopCategories(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardCountByGroup, error)
	GetDepartmentStats(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardDepartmentStat, error)
	GetLastActivity(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardActivityItem, error)
	GetBranchStats(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardDepartmentStat, error)
}

type DashboardRepository struct {
	storage *pgxpool.Pool
	logger  *zap.Logger
}

func NewDashboardRepository(storage *pgxpool.Pool, logger *zap.Logger) DashboardRepositoryInterface {
	return &DashboardRepository{storage: storage, logger: logger}
}

func applySecurity(b sq.SelectBuilder, securityCondition sq.Sqlizer) sq.SelectBuilder {
	if securityCondition == nil {
		return b
	}

	switch v := securityCondition.(type) {
	case sq.And:
		if len(v) == 0 {
			return b
		}
	case sq.Or:
		if len(v) == 0 {
			return b
		}
	}
	return b.Where(securityCondition)
}
func startOfMonth() time.Time {
	loc, _ := time.LoadLocation("Asia/Dushanbe")
	if loc == nil {
		loc = time.Local
	}
	now := time.Now().In(loc)
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
}

const (
	sqlResolvedCheck = "s.code IN ('CLOSED')"
	sqlSuccessCheck  = "s.code IN ('CLOSED')"
	sqlOpenCheck     = "s.code NOT IN ('CLOSED')"
)

// 1. Alerts
func (r *DashboardRepository) GetAlerts(ctx context.Context, securityCondition sq.Sqlizer) (*types.DashboardAlerts, error) {
	// Critical = 1 (в твоих данных)
	base := sq.Select(
		"COUNT(CASE WHEN p.code = 'CRITICAL' AND "+sqlOpenCheck+" THEN 1 END)",
		"COUNT(CASE WHEN o.duration IS NOT NULL AND o.duration < NOW() AND "+sqlOpenCheck+" THEN 1 END)",
	).From("orders o").
		LeftJoin("statuses s ON o.status_id = s.id").
		LeftJoin("priorities p ON o.priority_id = p.id").
		Where(sq.Eq{"o.deleted_at": nil})

	base = applySecurity(base, securityCondition)
	query, args, err := base.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}

	stats := &types.DashboardAlerts{}
	err = r.storage.QueryRow(ctx, query, args...).Scan(&stats.CriticalCount, &stats.OverdueCount)
	return stats, err
}

// 2. KPI
func (r *DashboardRepository) GetKPIsWithUser(ctx context.Context, securityCondition sq.Sqlizer, userID uint64) (*types.DashboardKPIs, error) {
	currStart := startOfMonth()
	prevStart := currStart.AddDate(0, -1, 0)

	base := sq.Select("o.id", "o.created_at", "o.completed_at", "o.status_id", "o.duration",
		"o.first_response_time_seconds", "o.resolution_time_seconds",
		"o.is_first_contact_resolution", "o.executor_id", "o.user_id").
		From("orders o").
		Where(sq.Eq{"o.deleted_at": nil})

	base = applySecurity(base, securityCondition)
	subSQL, subArgs, err := base.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}

	// --- ИСПРАВЛЕНИЕ: Используем плейсхолдеры вместо fmt.Sprintf ---
	// 1. Определяем индексы для плейсхолдеров, начиная после аргументов из Squirrel
	idx := len(subArgs)
	cDateIdx := idx + 1
	pDateIdx := idx + 2
	userIDIdx := idx + 3

	// 2. Формируем итоговый SQL
	// Я переписал его через простую структуру без хитрой интерполяции индексов %[n]
	sqlRaw := fmt.Sprintf(`
		WITH orders_filtered AS (%s)
		SELECT
			-- TOTAL
			COUNT(*) FILTER (WHERE created_at >= $%d) as ct,
			COUNT(*) FILTER (WHERE created_at >= $%d AND created_at < $%d) as pt,
			COUNT(*) FILTER (WHERE created_at >= $%d AND user_id = $%d) as mt,

			-- RESOLVED (Closed)
			COUNT(*) FILTER (WHERE status_id IN (SELECT id FROM statuses WHERE code = 'CLOSED') AND completed_at >= $%d) as cr,
			COUNT(*) FILTER (WHERE status_id IN (SELECT id FROM statuses WHERE code = 'CLOSED') AND completed_at >= $%d AND completed_at < $%d) as pr,
			COUNT(*) FILTER (WHERE status_id IN (SELECT id FROM statuses WHERE code = 'CLOSED') AND completed_at >= $%d AND executor_id = $%d) as mr,

			-- OPEN
			COUNT(*) FILTER (WHERE status_id NOT IN (SELECT id FROM statuses WHERE code IN ('CLOSED', 'REJECTED'))) as co,
			COUNT(*) FILTER (WHERE status_id NOT IN (SELECT id FROM statuses WHERE code IN ('CLOSED', 'REJECTED')) AND executor_id = $%d) as mo,

			-- SLA & METRICS (убрали фильтр > 0 для коротких заявок)
			COUNT(*) FILTER (WHERE status_id IN (SELECT id FROM statuses WHERE code = 'CLOSED') AND completed_at >= $%d AND (duration IS NULL OR completed_at <= duration)) as s1,
			COALESCE(AVG(first_response_time_seconds) FILTER (WHERE created_at >= $%d AND first_response_time_seconds >= 0), 0) as r1,
			COALESCE(AVG(resolution_time_seconds) FILTER (WHERE status_id IN (SELECT id FROM statuses WHERE code = 'CLOSED') AND completed_at >= $%d AND resolution_time_seconds >= 0), 0) as v1,
			
			COUNT(DISTINCT executor_id) FILTER (WHERE status_id NOT IN (SELECT id FROM statuses WHERE code IN ('CLOSED', 'REJECTED'))) as ag,

-- FCR (First Contact Resolution)
COUNT(*) FILTER (WHERE status_id IN (SELECT id FROM statuses WHERE code = 'CLOSED') AND completed_at >= $%d AND is_first_contact_resolution = true) as fcr_curr,
COUNT(*) FILTER (WHERE status_id IN (SELECT id FROM statuses WHERE code = 'CLOSED') AND completed_at >= $%d AND completed_at < $%d AND is_first_contact_resolution = true) as fcr_prev
FROM orders_filtered
	`,
		subSQL,
		cDateIdx,           // ct
		pDateIdx, cDateIdx, // pt
		cDateIdx, userIDIdx, // mt
		cDateIdx,           // cr
		pDateIdx, cDateIdx, // pr
		cDateIdx, userIDIdx, // mr
		userIDIdx,          // mo
		cDateIdx,           // s1
		cDateIdx,           // r1
		cDateIdx,           // v1
		cDateIdx,           // fcr_curr
		pDateIdx, cDateIdx, // fcr_prev
	)

	// 3. Сливаем все аргументы в правильном порядке
	fullArgs := append(subArgs, currStart, prevStart, userID)

	var (
		ct, pt, mt, cr, pr, mr, co, mo, s1, ag, fcr_curr, fcr_prev int64
		r1, v1                                                     float64
	)

	// Теперь ровно 13 полей в SELECT и 13 переменных в Scan
	err = r.storage.QueryRow(ctx, sqlRaw, fullArgs...).Scan(
		&ct, &pt, &mt, // 3
		&cr, &pr, &mr, // 6
		&co, &mo, // 8
		&s1,      // 9
		&r1, &v1, // 11
		&ag,                  // 12
		&fcr_curr, &fcr_prev, // 14
	)
	if err != nil {
		r.logger.Error("SQL Execution Error", zap.Error(err), zap.String("query", sqlRaw))
		return nil, err
	}

	// 3. Заполняем результат (res)
	res := &types.DashboardKPIs{}
	res.TotalTickets = types.DashboardKPIMetric{Current: float64(ct), Previous: float64(pt), Personal: float64(mt)}
	res.ResolvedTickets = types.DashboardKPIMetric{Current: float64(cr), Previous: float64(pr), Personal: float64(mr)}
	res.OpenTickets = types.DashboardKPIMetric{Current: float64(co), Personal: float64(mo)}
	res.AvgResponseTime = types.DashboardKPIMetric{Current: r1}
	res.AvgResolveTime = types.DashboardKPIMetric{Current: v1}
	res.ActiveAgents = ag

	if cr > 0 {
		res.SLACompliance.Current = (float64(s1) / float64(cr)) * 100
		res.FCRRate.Current = (float64(fcr_curr) / float64(cr)) * 100
	}
	if pr > 0 {
		res.FCRRate.Previous = (float64(fcr_prev) / float64(pr)) * 100
	}

	return res, nil
}

// 3. SLA Stats
func (r *DashboardRepository) GetSLAStats(ctx context.Context, securityCondition sq.Sqlizer) (*types.DashboardSLAStats, error) {
	b := sq.Select(
		"COUNT(*)",
		"COUNT(CASE WHEN completed_at <= duration THEN 1 END)").
		From("orders o").Join("statuses s ON o.status_id = s.id").
		Where(sq.Eq{"o.deleted_at": nil}).
		Where(sqlSuccessCheck).
		Where(sq.GtOrEq{"o.completed_at": startOfMonth()})

	b = applySecurity(b, securityCondition)
	sqlStr, args, err := b.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}
	stats := &types.DashboardSLAStats{}
	err = r.storage.QueryRow(ctx, sqlStr, args...).Scan(&stats.TotalCompleted, &stats.OnTime)
	return stats, err
}

func (r *DashboardRepository) GetAvgTimeByPriority(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardTimeByGroup, error) {
	b := sq.Select("p.name as group_name", "COALESCE(AVG(o.resolution_time_seconds), 0) as avg_seconds").
		From("orders o").
		Join("priorities p ON o.priority_id = p.id").
		Join("statuses s ON o.status_id = s.id").
		Where(sqlSuccessCheck).
		Where(sq.GtOrEq{"o.completed_at": startOfMonth()}).
		Where(sq.Eq{"o.deleted_at": nil}).
		GroupBy("p.name")

	b = applySecurity(b, securityCondition)
	sqlStr, args, err := b.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, fmt.Errorf("ошибка построения запроса: %w", err)
	}
	rows, err := r.storage.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[types.DashboardTimeByGroup])
}

// 5. AvgTime Type
func (r *DashboardRepository) GetAvgTimeByOrderType(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardTimeByGroup, error) {
	b := sq.Select("ot.name as group_name", "COALESCE(AVG(o.resolution_time_seconds), 0) as avg_seconds").
		From("orders o").
		Join("order_types ot ON o.order_type_id = ot.id").
		Join("statuses s ON o.status_id = s.id").
		Where(sqlSuccessCheck).
		Where(sq.GtOrEq{"o.completed_at": startOfMonth()}).
		Where(sq.Eq{"o.deleted_at": nil}).
		GroupBy("ot.name")
	b = applySecurity(b, securityCondition)
	sqlStr, args, err := b.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, fmt.Errorf("ошибка построения запроса: %w", err)
	}
	rows, err := r.storage.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[types.DashboardTimeByGroup])
}

// 6. Count Status (Всего в системе)
func (r *DashboardRepository) GetCountByStatus(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardCountByGroup, error) {
	b := sq.Select("s.name as group_name", "COUNT(o.id) as count").
		From("orders o").
		Join("statuses s ON o.status_id = s.id").
		Where(sq.Eq{"o.deleted_at": nil}).
		GroupBy("s.name").OrderBy("count DESC") // добавил сортировку
	b = applySecurity(b, securityCondition)
	sqlStr, args, err := b.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, fmt.Errorf("ошибка построения запроса: %w", err)
	}
	rows, err := r.storage.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[types.DashboardCountByGroup])
}

// 7. Executor
func (r *DashboardRepository) GetCountByExecutor(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardCountByGroup, error) {
	b := sq.Select("COALESCE(u.fio, 'Не назначен') as group_name", "COUNT(o.id) as count").
		From("orders o").
		LeftJoin("users u ON o.executor_id = u.id").
		LeftJoin("statuses s ON o.status_id = s.id").
		Where(sq.Eq{"o.deleted_at": nil}).
		Where(sqlOpenCheck).
		GroupBy("u.fio").
		OrderBy("count DESC").Limit(15)
	b = applySecurity(b, securityCondition)
	sqlStr, args, err := b.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, fmt.Errorf("ошибка построения запроса: %w", err)
	}
	rows, err := r.storage.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[types.DashboardCountByGroup])
}

// 8. Weekly
func (r *DashboardRepository) GetWeeklyVolume(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardChartData, error) {
	b := sq.Select("to_char(created_at, 'DD.MM') as label", "COUNT(*) as value").
		From("orders o").
		Where("created_at >= (CURRENT_DATE - INTERVAL '14 days')").
		Where(sq.Eq{"o.deleted_at": nil})
	b = applySecurity(b, securityCondition)
	sqlStr, args, _ := b.GroupBy("to_char(created_at, 'DD.MM')", "date_trunc('day', created_at)").
		OrderBy("date_trunc('day', created_at) ASC").PlaceholderFormat(sq.Dollar).ToSql()
	rows, err := r.storage.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[types.DashboardChartData])
}

// 9. Top Categories
func (r *DashboardRepository) GetTopCategories(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardCountByGroup, error) {
	b := sq.Select("ot.name as group_name", "COUNT(o.id) as count").
		From("orders o").
		Join("order_types ot ON o.order_type_id = ot.id").
		Where(sq.Eq{"o.deleted_at": nil}).
		Where(sq.GtOrEq{"o.created_at": startOfMonth()}).
		GroupBy("ot.name").
		OrderBy("count DESC").Limit(5)
	b = applySecurity(b, securityCondition)
	sqlStr, args, err := b.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, fmt.Errorf("ошибка построения запроса: %w", err)
	}
	rows, err := r.storage.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[types.DashboardCountByGroup])
}

// 10. Department
func (r *DashboardRepository) GetDepartmentStats(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardDepartmentStat, error) {
	b := sq.Select("d.name",
		// Open: все что не Закрыто, Выполнено или Отменено
		"COUNT(CASE WHEN "+sqlOpenCheck+" THEN 1 END) as open_count",
		// Resolved: Успешно выполнено
		"COUNT(CASE WHEN "+sqlSuccessCheck+" THEN 1 END) as resolved_count",
		"COUNT(CASE WHEN p.code='CRITICAL' AND "+sqlOpenCheck+" THEN 1 END) as critical_count",
		"COUNT(*) as total_count").
		From("orders o").
		Join("departments d ON o.department_id = d.id").
		LeftJoin("statuses s ON o.status_id = s.id").
		LeftJoin("priorities p ON o.priority_id = p.id").
		Where(sq.Eq{"o.deleted_at": nil}).
		GroupBy("d.name")
	b = applySecurity(b, securityCondition)
	sqlStr, args, err := b.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, fmt.Errorf("ошибка построения запроса: %w", err)
	}
	rows, err := r.storage.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[types.DashboardDepartmentStat])
}

// 11. Activity (тут норм)
func (r *DashboardRepository) GetLastActivity(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardActivityItem, error) {
	// ... Код Activity оставить без изменений, он хорош ...
	// Просто скопируй то, что у тебя уже было выше в GetLastActivity
	b := sq.Select("h.id", "h.created_at", "h.event_type", "h.comment", "h.new_value", "COALESCE(u.fio, 'Система')", "o.name").
		From("order_history h").Join("orders o ON h.order_id = o.id").LeftJoin("users u ON h.user_id = u.id").
		Where(sq.Eq{"o.deleted_at": nil}).OrderBy("h.created_at DESC").Limit(10)
	b = applySecurity(b, securityCondition)
	sqlStr, args, err := b.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, fmt.Errorf("ошибка построения запроса: %w", err)
	}
	rows, err := r.storage.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []types.DashboardActivityItem
	for rows.Next() {
		var i types.DashboardActivityItem
		var ts time.Time
		var ev, cm, nv *string
		if err := rows.Scan(&i.ID, &ts, &ev, &cm, &nv, &i.AuthorName, &i.OrderName); err != nil {
			r.logger.Error("Ошибка чтения строки активности", zap.Error(err))
			continue
		}
		i.Date = ts.Format("02.01 15:04")
		if cm != nil && *cm != "" {
			i.Text = *cm
		} else {
			if ev == nil {
				continue
			}
			switch *ev {
			case "CREATE":
				i.Text = "Создал новую заявку"
			case "STATUS_CHANGE":
				i.Text = "Изменил статус заявки"
			default:
				i.Text = "Обновил заявку"
			}
		}
		items = append(items, i)
	}
	return items, nil
}

func (r *DashboardRepository) GetBranchStats(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardDepartmentStat, error) {
	b := sq.Select("b.name",
		"COUNT(CASE WHEN s.code != 'CLOSED' THEN 1 END) as open_count",
		"COUNT(CASE WHEN s.code = 'CLOSED' THEN 1 END) as resolved_count",
		"COUNT(CASE WHEN p.code='CRITICAL' AND s.code != 'CLOSED' THEN 1 END) as critical_count",
		"COUNT(*) as total_count",
	).
		From("orders o").
		Join("branches b ON o.branch_id = b.id").
		LeftJoin("statuses s ON o.status_id = s.id").
		LeftJoin("priorities p ON o.priority_id = p.id").
		Where(sq.Eq{"o.deleted_at": nil}).
		Where(sq.Eq{"o.department_id": nil}).
		GroupBy("b.name")

	b = applySecurity(b, securityCondition)

	sqlStr, args, err := b.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, fmt.Errorf("ошибка построения запроса: %w", err)
	}
	rows, err := r.storage.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[types.DashboardDepartmentStat])
}
