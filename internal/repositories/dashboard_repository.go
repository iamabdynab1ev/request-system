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
	GetKPIs(ctx context.Context, securityCondition sq.Sqlizer) (*types.DashboardKPIs, error)
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
	if securityCondition != nil {
		return b.Where(securityCondition)
	}
	return b
}

// 1. Alerts
// Критические и Просроченные (среди АКТИВНЫХ, то есть всех, кроме CLOSED)
func (r *DashboardRepository) GetAlerts(ctx context.Context, securityCondition sq.Sqlizer) (*types.DashboardAlerts, error) {
	// Логика: p.code=CRITICAL + Статус НЕ CLOSED
	base := sq.Select(
		"COUNT(CASE WHEN p.code = 'CRITICAL' AND s.code != 'CLOSED' THEN 1 END)",
		"COUNT(CASE WHEN o.duration IS NOT NULL AND o.duration < NOW() AND s.code != 'CLOSED' THEN 1 END)",
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

// 2. KPI (ГЛАВНЫЙ ЗАПРОС)
// Логика обновлена:
// Resolved/Res = Только CLOSED
// Active/Open  = Все кроме CLOSED
func (r *DashboardRepository) GetKPIs(ctx context.Context, securityCondition sq.Sqlizer) (*types.DashboardKPIs, error) {
	now := time.Now()
	currStart := now.AddDate(0, 0, -30)
	prevStart := now.AddDate(0, 0, -60)

	dummy := sq.Select("*").From("orders o").Where(sq.Eq{"o.deleted_at": nil})
	dummy = applySecurity(dummy, securityCondition)
	subSQL, subArgs, err := dummy.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}

	dateFmt := "2006-01-02 15:04:05"
	cDate := currStart.Format(dateFmt)
	pDate := prevStart.Format(dateFmt)

	sqlRaw := fmt.Sprintf(`
		SELECT
			-- 1. ВСЕГО ЗАЯВОК
			COUNT(*) FILTER (WHERE created_at >= '%[1]s') as curr_tot,
			COUNT(*) FILTER (WHERE created_at >= '%[2]s' AND created_at < '%[1]s') as prev_tot,
			
			-- 2. РЕШЕНО (Только статус CLOSED)
			COUNT(*) FILTER (WHERE status_id IN (SELECT id FROM statuses WHERE code = 'CLOSED') AND completed_at >= '%[1]s') as curr_res,
			COUNT(*) FILTER (WHERE status_id IN (SELECT id FROM statuses WHERE code = 'CLOSED') AND completed_at >= '%[2]s' AND completed_at < '%[1]s') as prev_res,

			-- 3. АКТИВНЫЕ/ОТКРЫТЫЕ (Все, что не CLOSED)
			COUNT(*) FILTER (WHERE status_id NOT IN (SELECT id FROM statuses WHERE code = 'CLOSED')) as curr_open,

			-- 4. SLA (Учитываем только реально Закрытые)
			COUNT(*) FILTER (WHERE status_id IN (SELECT id FROM statuses WHERE code = 'CLOSED') AND completed_at >= '%[1]s' AND completed_at <= duration) as curr_sla_ok,
			COUNT(*) FILTER (WHERE status_id IN (SELECT id FROM statuses WHERE code = 'CLOSED') AND completed_at >= '%[2]s' AND completed_at < '%[1]s' AND completed_at <= duration) as prev_sla_ok,

			-- 5. Время ответа
			COALESCE(AVG(first_response_time_seconds) FILTER (WHERE created_at >= '%[1]s'), 0) as curr_resp,
			COALESCE(AVG(first_response_time_seconds) FILTER (WHERE created_at >= '%[2]s' AND created_at < '%[1]s'), 0) as prev_resp,

			-- 6. Время решения (До финального закрытия)
			COALESCE(AVG(resolution_time_seconds) FILTER (WHERE completed_at >= '%[1]s' AND status_id IN (SELECT id FROM statuses WHERE code = 'CLOSED')), 0) as curr_slv,
			COALESCE(AVG(resolution_time_seconds) FILTER (WHERE completed_at >= '%[2]s' AND completed_at < '%[1]s' AND status_id IN (SELECT id FROM statuses WHERE code = 'CLOSED')), 0) as prev_slv,

			-- 7. FCR (Только для закрытых)
			COUNT(*) FILTER (WHERE is_first_contact_resolution = true AND completed_at >= '%[1]s' AND status_id IN (SELECT id FROM statuses WHERE code = 'CLOSED')) as curr_fcr,
			COUNT(*) FILTER (WHERE is_first_contact_resolution = true AND completed_at >= '%[2]s' AND completed_at < '%[1]s' AND status_id IN (SELECT id FROM statuses WHERE code = 'CLOSED')) as prev_fcr,

			-- 8. Агенты (с любой активной заявкой)
			COUNT(DISTINCT executor_id) FILTER (WHERE status_id NOT IN (SELECT id FROM statuses WHERE code = 'CLOSED')) as agents

		FROM (%[3]s) AS orders
	`, cDate, pDate, subSQL)

	var (
		currTot, prevTot, currRes, prevRes, currOpen, currSla, prevSla int64
		currFcr, prevFcr, agents                                       int64
		currResp, prevResp, currSlv, prevSlv                           float64
	)

	err = r.storage.QueryRow(ctx, sqlRaw, subArgs...).Scan(
		&currTot, &prevTot, &currRes, &prevRes, &currOpen,
		&currSla, &prevSla,
		&currResp, &prevResp, &currSlv, &prevSlv,
		&currFcr, &prevFcr, &agents,
	)
	if err != nil {
		return nil, err
	}

	res := &types.DashboardKPIs{}
	res.TotalTickets.Current = float64(currTot)
	res.TotalTickets.Previous = float64(prevTot)
	res.ResolvedTickets.Current = float64(currRes)
	res.ResolvedTickets.Previous = float64(prevRes)
	res.OpenTickets.Current = float64(currOpen)
	res.AvgResponseTime.Current = currResp
	res.AvgResponseTime.Previous = prevResp
	res.AvgResolveTime.Current = currSlv
	res.AvgResolveTime.Previous = prevSlv
	res.ActiveAgents = agents

	if currRes > 0 {
		res.SLACompliance.Current = (float64(currSla) / float64(currRes)) * 100
		res.FCRRate.Current = (float64(currFcr) / float64(currRes)) * 100
	}
	if prevRes > 0 {
		res.SLACompliance.Previous = (float64(prevSla) / float64(prevRes)) * 100
		res.FCRRate.Previous = (float64(prevFcr) / float64(prevRes)) * 100
	}

	return res, nil
}

// 3. SLA Pie
func (r *DashboardRepository) GetSLAStats(ctx context.Context, securityCondition sq.Sqlizer) (*types.DashboardSLAStats, error) {
	b := sq.Select("COUNT(*)", "COUNT(CASE WHEN completed_at <= duration THEN 1 END)").
		From("orders o").
		Join("statuses s ON o.status_id = s.id").
		Where(sq.Eq{"o.deleted_at": nil}).
		Where("o.completed_at IS NOT NULL AND o.duration IS NOT NULL").
		Where("s.code = 'CLOSED'") // Считаем SLA только по закрытым, иначе статистика будет врать

	b = applySecurity(b, securityCondition)
	sqlStr, args, err := b.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}

	stats := &types.DashboardSLAStats{}
	err = r.storage.QueryRow(ctx, sqlStr, args...).Scan(&stats.TotalCompleted, &stats.OnTime)
	return stats, err
}

// 4-5. Time (Priority, Type) - Здесь оставляем как есть, просто исключая NULL время

func (r *DashboardRepository) GetAvgTimeByPriority(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardTimeByGroup, error) {
	b := sq.Select("p.name as group_name", "COALESCE(AVG(o.resolution_time_seconds), 0) as avg_seconds").
		From("orders o").
		Join("priorities p ON o.priority_id = p.id").
		Join("statuses s ON o.status_id = s.id").
		Where("o.resolution_time_seconds IS NOT NULL").
		Where(sq.Eq{"o.deleted_at": nil}).
		Where("s.code = 'CLOSED'"). // Считаем среднее только по завершенным процессам
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

func (r *DashboardRepository) GetAvgTimeByOrderType(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardTimeByGroup, error) {
	b := sq.Select("ot.name as group_name", "COALESCE(AVG(o.resolution_time_seconds), 0) as avg_seconds").
		From("orders o").
		Join("order_types ot ON o.order_type_id = ot.id").
		Join("statuses s ON o.status_id = s.id").
		Where("o.resolution_time_seconds IS NOT NULL").
		Where(sq.Eq{"o.deleted_at": nil}).
		Where("s.code = 'CLOSED'").
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

// 6. Count by Status
func (r *DashboardRepository) GetCountByStatus(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardCountByGroup, error) {
	b := sq.Select("s.name as group_name", "COUNT(o.id) as count").
		From("orders o").
		Join("statuses s ON o.status_id = s.id").
		Where(sq.Eq{"o.deleted_at": nil}).
		GroupBy("s.name")
	b = applySecurity(b, securityCondition)
	sqlStr, args, _ := b.PlaceholderFormat(sq.Dollar).ToSql()
	rows, err := r.storage.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[types.DashboardCountByGroup])
}

// 7. Executor (Агенты с активными задачами)
func (r *DashboardRepository) GetCountByExecutor(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardCountByGroup, error) {
	// Активная задача = все, кроме CLOSED
	b := sq.Select("u.fio as group_name", "COUNT(o.id) as count").
		From("orders o").
		Join("users u ON o.executor_id = u.id").
		LeftJoin("statuses s ON o.status_id = s.id").
		Where(sq.Eq{"o.deleted_at": nil}).
		Where("s.code != 'CLOSED'").
		GroupBy("u.fio").
		OrderBy("count DESC")
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
	b := sq.Select(
		"to_char(date_trunc('day', created_at), 'DD.MM') as label",
		"COUNT(*) as value",
	).From("orders o").
		Where("created_at >= NOW() - INTERVAL '7 days'").
		Where(sq.Eq{"o.deleted_at": nil}).
		GroupBy("date_trunc('day', created_at)").
		OrderBy("date_trunc('day', created_at)")
	b = applySecurity(b, securityCondition)
	sqlStr, args, _ := b.PlaceholderFormat(sq.Dollar).ToSql()
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
		GroupBy("ot.name").
		OrderBy("count DESC").
		Limit(5)
	b = applySecurity(b, securityCondition)
	sqlStr, args, _ := b.PlaceholderFormat(sq.Dollar).ToSql()
	rows, err := r.storage.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[types.DashboardCountByGroup])
}

// 10. Department Stats (Отделы)
func (r *DashboardRepository) GetDepartmentStats(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardDepartmentStat, error) {
	b := sq.Select(
		"d.name",
		"COUNT(CASE WHEN s.code != 'CLOSED' THEN 1 END) as open_count",
		"COUNT(CASE WHEN s.code = 'CLOSED' THEN 1 END) as resolved_count",
		"COUNT(CASE WHEN p.code='CRITICAL' AND s.code != 'CLOSED' THEN 1 END) as critical_count",
		"COUNT(*) as total_count",
	).
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

// 11. Activity
// 11. Last Activity (Умная лента с комментариями)
func (r *DashboardRepository) GetLastActivity(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardActivityItem, error) {
	b := sq.Select(
		"h.id",
		"h.created_at",
		"h.event_type",
		"h.comment",   // <-- Добавили: берем текст комментария
		"h.new_value", // <-- Добавили: берем новое значение (для файлов или имен)
		"COALESCE(u.fio, 'Система') as author_name",
		"o.name as order_name",
	).
		From("order_history h").
		Join("orders o ON h.order_id = o.id").
		LeftJoin("users u ON h.user_id = u.id").
		Where(sq.Eq{"o.deleted_at": nil}).
		OrderBy("h.created_at DESC").
		Limit(6)

	b = applySecurity(b, securityCondition)
	sqlStr, args, err := b.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.storage.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []types.DashboardActivityItem
	for rows.Next() {
		var item types.DashboardActivityItem
		var ts time.Time
		var eventType string
		var comment, newValue *string // Используем указатели для NULL значений

		if err := rows.Scan(&item.ID, &ts, &eventType, &comment, &newValue, &item.AuthorName, &item.OrderName); err != nil {
			return nil, err
		}

		item.Date = ts.Format("02.01 15:04")

		// --- ЛОГИКА ОТОБРАЖЕНИЯ ТЕКСТА ---

		// 1. Если есть живой комментарий - это самое важное (это то, что пишет человек или авто-делегирование)
		if comment != nil && *comment != "" {
			item.Text = *comment
		} else {
			// 2. Если комментария нет, формируем текст по типу события
			switch eventType {
			case "CREATE":
				item.Text = "Создал новую заявку"
			case "STATUS_CHANGE":
				// Здесь newValue содержит ID статуса, но без JOIN получить имя сложно.
				// Оставим пока просто "Изменил статус", так как статус виден внутри самой заявки.
				item.Text = "Изменил статус заявки"
			case "PRIORITY_CHANGE":
				item.Text = "Изменил приоритет"
			case "ATTACHMENT_ADD":
				fileName := "файл"
				if newValue != nil {
					fileName = *newValue
				}
				item.Text = fmt.Sprintf("Загрузил: %s", fileName)
			case "DURATION_CHANGE":
				item.Text = "Изменил срок выполнения"
			default:
				item.Text = "Обновил заявку"
			}
		}

		items = append(items, item)
	}
	return items, nil
}

// 12. Branch Stats (Филиалы + Офисы)
func (r *DashboardRepository) GetBranchStats(ctx context.Context, securityCondition sq.Sqlizer) ([]types.DashboardDepartmentStat, error) {
	// Агрегируем по Branch.
	// Даже если указан office_id, заявка всё равно привязана к branch_id (обычно).
	// Если у вас в БД office_id заполнен, а branch_id NULL (что странно), то нужно джоинить office -> branch.
	// Но предположим, что branch_id всегда есть, если выбран филиал.

	b := sq.Select(
		"b.name",
		"COUNT(CASE WHEN s.code != 'CLOSED' THEN 1 END) as open_count",
		"COUNT(CASE WHEN s.code = 'CLOSED' THEN 1 END) as resolved_count",
		"COUNT(CASE WHEN p.code='CRITICAL' AND s.code != 'CLOSED' THEN 1 END) as critical_count",
		"COUNT(*) as total_count",
	).
		From("orders o").
		Join("branches b ON o.branch_id = b.id"). // <-- Основное отличие: Джоиним Branches
		LeftJoin("statuses s ON o.status_id = s.id").
		LeftJoin("priorities p ON o.priority_id = p.id").
		Where(sq.Eq{"o.deleted_at": nil}).
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
