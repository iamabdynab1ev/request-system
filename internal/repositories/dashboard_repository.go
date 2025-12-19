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
	// 1. Если nil — сразу возврат (штатный случай для Админа)
	if securityCondition == nil {
		return b
	}

	// 2. Проверяем типы оберток Squirrel на пустоту.
	// Ошибка "syntax error at or near )" возникает, когда sq.And имеет len == 0
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

	// 3. Всё ок — добавляем WHERE
	return b.Where(securityCondition)
}

func startOfMonth() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
}

// Группы статусов для аналитики
// resolved: CLOSED, COMPLETED, REJECTED
// working: OPEN, IN_PROGRESS, SERVICE, REFINEMENT, CLARIFICATION, CONFIRMED
const (
	sqlResolvedCheck = "s.code IN ('CLOSED')"     // Считаем работу законченной (или отмененной)
	sqlSuccessCheck  = "s.code IN ('CLOSED')"     // Успешно выполнено
	sqlOpenCheck     = "s.code NOT IN ('CLOSED')" // Все что не завершено
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

	// Базовый SQL
	dummy := sq.Select("o.id", "o.created_at", "o.completed_at", "o.status_id", "o.duration", "o.first_response_time_seconds", "o.resolution_time_seconds", "o.is_first_contact_resolution", "o.executor_id", "o.user_id").
		From("orders o").
		Where(sq.Eq{"o.deleted_at": nil})

	dummy = applySecurity(dummy, securityCondition)
	subSQL, subArgs, err := dummy.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}

	// Добавляем ID юзера в аргументы ($N+1)
	userArgIndex := len(subArgs) + 1

	dateFmt := "2006-01-02 15:04:05"
	cDate := currStart.Format(dateFmt)
	pDate := prevStart.Format(dateFmt)

	sqlRaw := fmt.Sprintf(`
		WITH final_status AS (SELECT id FROM statuses WHERE code = 'CLOSED'),
		     filtered_orders AS (%[3]s)
		SELECT
			-- 1. TOTAL (Приход): Глобально + Мои (созданные мной)
			COUNT(*) FILTER (WHERE created_at >= '%[1]s') as curr_tot,
			COUNT(*) FILTER (WHERE created_at >= '%[2]s' AND created_at < '%[1]s') as prev_tot,
			COUNT(*) FILTER (WHERE created_at >= '%[1]s' AND user_id = $%[4]d) as my_tot, 

			-- 2. RESOLVED (Закрыто): Глобально + Мои (исполненные мной)
			COUNT(*) FILTER (WHERE status_id IN (SELECT id FROM final_status) AND completed_at >= '%[1]s') as curr_res,
			COUNT(*) FILTER (WHERE status_id IN (SELECT id FROM final_status) AND completed_at >= '%[2]s' AND completed_at < '%[1]s') as prev_res,
			COUNT(*) FILTER (WHERE status_id IN (SELECT id FROM final_status) AND completed_at >= '%[1]s' AND executor_id = $%[4]d) as my_res,

			-- 3. OPEN (Очередь): Глобально + Мои (висят на мне)
			COUNT(*) FILTER (WHERE status_id NOT IN (SELECT id FROM final_status)) as curr_open,
			COUNT(*) FILTER (WHERE status_id NOT IN (SELECT id FROM final_status) AND executor_id = $%[4]d) as my_open,

			-- SLA
			COUNT(*) FILTER (WHERE status_id IN (SELECT id FROM final_status) AND completed_at >= '%[1]s' AND completed_at <= duration) as curr_sla_ok,
			COUNT(*) FILTER (WHERE status_id IN (SELECT id FROM final_status) AND completed_at >= '%[2]s' AND completed_at < '%[1]s' AND completed_at <= duration) as prev_sla_ok,

			-- Metrics
			COALESCE(AVG(first_response_time_seconds) FILTER (WHERE created_at >= '%[1]s' AND first_response_time_seconds IS NOT NULL), 0),
			COALESCE(AVG(first_response_time_seconds) FILTER (WHERE created_at >= '%[2]s' AND created_at < '%[1]s' AND first_response_time_seconds IS NOT NULL), 0),
			COALESCE(AVG(resolution_time_seconds) FILTER (WHERE status_id IN (SELECT id FROM final_status) AND completed_at >= '%[1]s' AND resolution_time_seconds > 0), 0),
			COALESCE(AVG(resolution_time_seconds) FILTER (WHERE status_id IN (SELECT id FROM final_status) AND completed_at >= '%[2]s' AND completed_at < '%[1]s' AND resolution_time_seconds > 0), 0),
			COUNT(*) FILTER (WHERE is_first_contact_resolution = true AND status_id IN (SELECT id FROM final_status) AND completed_at >= '%[1]s'),
			COUNT(*) FILTER (WHERE is_first_contact_resolution = true AND status_id IN (SELECT id FROM final_status) AND completed_at >= '%[2]s' AND completed_at < '%[1]s'),
			COUNT(DISTINCT executor_id) FILTER (WHERE status_id NOT IN (SELECT id FROM final_status) AND executor_id IS NOT NULL)

		FROM filtered_orders
	`, cDate, pDate, subSQL, userArgIndex)

	var (
		ct, pt, myT, cr, pr, myR, co, myO, sla1, sla2, fcr1, fcr2, ag int64
		rt1, rt2, rvt1, rvt2                                          float64
	)

	// Добавляем userID к списку аргументов
	fullArgs := append(subArgs, userID)

	err = r.storage.QueryRow(ctx, sqlRaw, fullArgs...).Scan(
		&ct, &pt, &myT, // Total
		&cr, &pr, &myR, // Resolved
		&co, &myO, // Open (нет previous, т.к. backlog)
		&sla1, &sla2,
		&rt1, &rt2, &rvt1, &rvt2,
		&fcr1, &fcr2,
		&ag,
	)
	if err != nil {
		return nil, err
	}

	res := &types.DashboardKPIs{}

	// ЗАПОЛНЯЕМ TOTAL (с личными)
	res.TotalTickets.Current = float64(ct)
	res.TotalTickets.Previous = float64(pt)
	res.TotalTickets.Personal = float64(myT) // <--- ВОТ ТВОИ "2 ВАШИХ"

	// ЗАПОЛНЯЕМ RESOLVED (с личными)
	res.ResolvedTickets.Current = float64(cr)
	res.ResolvedTickets.Previous = float64(pr)
	res.ResolvedTickets.Personal = float64(myR) // Личные

	// ЗАПОЛНЯЕМ OPEN (с личными)
	res.OpenTickets.Current = float64(co)
	res.OpenTickets.Personal = float64(myO) // <--- ВОТ ТВОЙ "1 НАЗНАЧЕНО ВАМ"

	res.AvgResponseTime.Current = rt1
	res.AvgResponseTime.Previous = rt2
	res.AvgResolveTime.Current = rvt1
	res.AvgResolveTime.Previous = rvt2
	res.ActiveAgents = ag

	if cr > 0 {
		res.SLACompliance.Current = (float64(sla1) / float64(cr)) * 100
		res.FCRRate.Current = (float64(fcr1) / float64(cr)) * 100
	}
	if pr > 0 {
		res.SLACompliance.Previous = (float64(sla2) / float64(pr)) * 100
		res.FCRRate.Previous = (float64(fcr2) / float64(pr)) * 100
	}

	return res, nil
}

// 3. SLA Stats
func (r *DashboardRepository) GetSLAStats(ctx context.Context, securityCondition sq.Sqlizer) (*types.DashboardSLAStats, error) {
	// SLA считаем для выполненных заявок за этот месяц
	b := sq.Select(
		"COUNT(*)",
		"COUNT(CASE WHEN completed_at <= duration THEN 1 END)").
		From("orders o").Join("statuses s ON o.status_id = s.id").
		Where(sq.Eq{"o.deleted_at": nil}).
		Where(sqlSuccessCheck). // CLOSED or COMPLETED
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

// 4. AvgTime Priority (Сюда данные попадают только если есть >0 в базе)
func (r *DashboardRepository) GetAvgTimeByPriority(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardTimeByGroup, error) {
	b := sq.Select("p.name as group_name", "COALESCE(AVG(o.resolution_time_seconds), 0) as avg_seconds").
		From("orders o").
		Join("priorities p ON o.priority_id = p.id").
		Join("statuses s ON o.status_id = s.id").
		Where(sqlSuccessCheck). // Считаем среднее только по решенным
		Where(sq.GtOrEq{"o.completed_at": startOfMonth()}).
		Where(sq.Eq{"o.deleted_at": nil}).
		Where("o.resolution_time_seconds > 0"). // Исключаем нули
		GroupBy("p.name")

	b = applySecurity(b, securityCondition)
	sqlStr, args, _ := b.PlaceholderFormat(sq.Dollar).ToSql()
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
		Where("o.resolution_time_seconds > 0").
		GroupBy("ot.name")
	b = applySecurity(b, securityCondition)
	sqlStr, args, _ := b.PlaceholderFormat(sq.Dollar).ToSql()
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
	sqlStr, args, _ := b.PlaceholderFormat(sq.Dollar).ToSql()
	rows, err := r.storage.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[types.DashboardCountByGroup])
}

// 7. Executor
func (r *DashboardRepository) GetCountByExecutor(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardCountByGroup, error) {
	// COALESCE(u.fio, 'Не назначен') - чтобы не терять 5 не назначенных заявок
	b := sq.Select("COALESCE(u.fio, 'Не назначен') as group_name", "COUNT(o.id) as count").
		From("orders o").
		LeftJoin("users u ON o.executor_id = u.id"). // LeftJoin важен!
		LeftJoin("statuses s ON o.status_id = s.id").
		Where(sq.Eq{"o.deleted_at": nil}).
		Where(sqlOpenCheck). // Показываем нагрузку только по открытым заявкам
		GroupBy("u.fio").
		OrderBy("count DESC").Limit(15)
	b = applySecurity(b, securityCondition)
	sqlStr, args, _ := b.PlaceholderFormat(sq.Dollar).ToSql()
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
		Where(sq.GtOrEq{"o.created_at": startOfMonth()}). // Популярные в этом месяце
		GroupBy("ot.name").
		OrderBy("count DESC").Limit(5)
	b = applySecurity(b, securityCondition)
	sqlStr, args, _ := b.PlaceholderFormat(sq.Dollar).ToSql()
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
	sqlStr, args, _ := b.PlaceholderFormat(sq.Dollar).ToSql()
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
	sqlStr, args, _ := b.PlaceholderFormat(sq.Dollar).ToSql()
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
		rows.Scan(&i.ID, &ts, &ev, &cm, &nv, &i.AuthorName, &i.OrderName)
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

// 12. Branches (Исправленная версия, теперь отобразит Филиалы!)
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
		Where(sq.Eq{"o.department_id": nil}). // Фильтр по филиалам (без департаментов)
		GroupBy("b.name")

	b = applySecurity(b, securityCondition)

	sqlStr, args, _ := b.PlaceholderFormat(sq.Dollar).ToSql()
	rows, err := r.storage.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[types.DashboardDepartmentStat])
}
