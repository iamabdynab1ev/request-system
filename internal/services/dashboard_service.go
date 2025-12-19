package services

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	sq "github.com/Masterminds/squirrel"
	"go.uber.org/zap"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils"
)

type DashboardService struct {
	repo     repositories.DashboardRepositoryInterface
	userRepo repositories.UserRepositoryInterface
	logger   *zap.Logger
}

func NewDashboardService(repo repositories.DashboardRepositoryInterface, userRepo repositories.UserRepositoryInterface, logger *zap.Logger) *DashboardService {
	return &DashboardService{repo: repo, userRepo: userRepo, logger: logger}
}

func (s *DashboardService) GetDashboardStats(ctx context.Context, filter types.Filter) (*dto.DashboardStatsDTO, error) {
	userID, _ := utils.GetUserIDFromCtx(ctx)
	permissionsMap, _ := utils.GetPermissionsMapFromCtx(ctx)
	actor, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	authContext := authz.Context{Actor: actor, Permissions: permissionsMap}

	// === ЖЕЛЕЗОБЕТОННАЯ СБОРКА УСЛОВИЙ ===
	// Используем nil интерфейс по умолчанию.
	var securityBuilder sq.Sqlizer = nil

	// Временный список условий (не sq.And, а просто массив)
	var preds []sq.Sqlizer

	// Если пользователь НЕ админ и НЕ аудитор — включаем фильтры
	if !authContext.HasPermission(authz.ScopeAll) && !authContext.HasPermission(authz.ScopeAllView) {

		// Собираем условия "ИЛИ" (ScopeDepartment OR ScopeBranch ...)
		var orPreds []sq.Sqlizer

		if authContext.HasPermission(authz.ScopeDepartment) && actor.DepartmentID != nil {
			orPreds = append(orPreds, sq.Eq{"o.department_id": *actor.DepartmentID})
		}
		if authContext.HasPermission(authz.ScopeBranch) && actor.BranchID != nil {
			orPreds = append(orPreds, sq.Eq{"o.branch_id": *actor.BranchID})
		}
		if authContext.HasPermission(authz.ScopeOtdel) && actor.OtdelID != nil {
			orPreds = append(orPreds, sq.Eq{"o.otdel_id": *actor.OtdelID})
		}
		if authContext.HasPermission(authz.ScopeOwn) {
			orPreds = append(orPreds, sq.Eq{"o.user_id": actor.ID})
			orPreds = append(orPreds, sq.Eq{"o.executor_id": actor.ID})
		}

		if len(orPreds) > 0 {
			// Если набрали скоупы — добавляем их как (cond1 OR cond2 OR ...)
			preds = append(preds, sq.Or(orPreds))
		} else {
			// Если прав нет никаких вообще — показываем только своё (защита от пустой выборки)
			preds = append(preds, sq.Eq{"o.user_id": actor.ID})
		}
	}

	// ФИНАЛ: Если мы насобирали условия — упаковываем их в AND
	if len(preds) > 0 {
		securityBuilder = sq.And(preds)
	} else {
		// Если массив пуст (Админ), оставляем nil
		securityBuilder = nil
	}
	// ======================================

	var (
		wg        sync.WaitGroup
		alerts    *types.DashboardAlerts
		kpis      *types.DashboardKPIs
		sla       *types.DashboardSLAStats
		timePrior []types.DashboardTimeByGroup
		timeType  []types.DashboardTimeByGroup
		cntStatus []types.DashboardCountByGroup
		cntExec   []types.DashboardCountByGroup
		weekly    []types.DashboardChartData
		topCat    []types.DashboardCountByGroup
		lastAct   []types.DashboardActivityItem
		depts     []types.DashboardDepartmentStat
		branches  []types.DashboardDepartmentStat

		errs []error
		mu   sync.Mutex
	)

	addTask := func(fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}()
	}

	// Запускаем параллельные запросы
	addTask(func() (err error) { alerts, err = s.repo.GetAlerts(ctx, securityBuilder); return })
	// Передаем actor.ID для расчета "Моих" показателей
	addTask(func() (err error) { kpis, err = s.repo.GetKPIsWithUser(ctx, securityBuilder, actor.ID); return })
	addTask(func() (err error) { sla, err = s.repo.GetSLAStats(ctx, securityBuilder); return })
	addTask(func() (err error) { timePrior, err = s.repo.GetAvgTimeByPriority(ctx, securityBuilder); return })
	addTask(func() (err error) { timeType, err = s.repo.GetAvgTimeByOrderType(ctx, securityBuilder); return })
	addTask(func() (err error) { cntStatus, err = s.repo.GetCountByStatus(ctx, securityBuilder); return })
	addTask(func() (err error) { cntExec, err = s.repo.GetCountByExecutor(ctx, securityBuilder); return })
	addTask(func() (err error) { weekly, err = s.repo.GetWeeklyVolume(ctx, securityBuilder); return })
	addTask(func() (err error) { topCat, err = s.repo.GetTopCategories(ctx, securityBuilder); return })
	addTask(func() (err error) { lastAct, err = s.repo.GetLastActivity(ctx, securityBuilder); return })
	addTask(func() (err error) { depts, err = s.repo.GetDepartmentStats(ctx, securityBuilder); return })
	addTask(func() (err error) { branches, err = s.repo.GetBranchStats(ctx, securityBuilder); return })

	wg.Wait()

	if len(errs) > 0 {
		// Логируем ошибку, чтобы понять какой именно запрос упал
		s.logger.Error("Dashboard fetching error", zap.Error(errs[0]))
		return nil, apperrors.NewInternalError("Ошибка загрузки дашборда")
	}

	// 1. Заполняем пропуски в графике нулями
	weekly = fillMissingDays(weekly)

	// 2. Рассчитываем тренды для KPI
	if kpis != nil {
		process := func(m *types.DashboardKPIMetric, unit string, isTime bool) {
			if m.Previous > 0 {
				m.TrendPct = ((m.Current - m.Previous) / m.Previous) * 100
			} else if m.Current > 0 {
				m.TrendPct = 100
			} else {
				m.TrendPct = 0
			}
			m.TrendPct = math.Round(m.TrendPct)

			sign := ""
			if m.TrendPct > 0 {
				sign = "+"
			}

			if isTime {
				if m.TrendPct < 0 {
					m.TrendText = fmt.Sprintf("%d%% улучшение", int(math.Abs(m.TrendPct)))
				} else if m.TrendPct > 0 {
					m.TrendText = fmt.Sprintf("%s%.0f%% за месяц", sign, m.TrendPct)
				} else {
					m.TrendText = "без изменений"
				}
				hrs := m.Current / 3600
				m.Formatted = fmt.Sprintf("%.1fч", hrs)
			} else {
				if m.TrendPct != 0 {
					m.TrendText = fmt.Sprintf("%s%.0f%% за месяц", sign, m.TrendPct)
				} else {
					m.TrendText = "0% за месяц"
				}
				if unit == "%" {
					m.Formatted = fmt.Sprintf("%.0f%%", m.Current)
				} else {
					m.Formatted = fmt.Sprintf("%.0f", m.Current)
				}
			}
		}

		process(&kpis.TotalTickets, "", false)
		process(&kpis.ResolvedTickets, "", false)
		process(&kpis.SLACompliance, "%", false)
		process(&kpis.FCRRate, "%", false)
		process(&kpis.AvgResponseTime, "time", true)
		process(&kpis.AvgResolveTime, "time", true)

		kpis.OpenTickets.Formatted = fmt.Sprintf("%.0f", kpis.OpenTickets.Current)
		kpis.OpenTickets.TrendText = "Актуальные"
		kpis.OpenTickets.TrendPct = 0
	}

	// 3. Форматируем время в таблицах
	for i := range timePrior {
		timePrior[i].AvgTimeFormatted = utils.FormatFloatSecondsToHumanReadable(timePrior[i].AvgSeconds)
	}
	for i := range timeType {
		timeType[i].AvgTimeFormatted = utils.FormatFloatSecondsToHumanReadable(timeType[i].AvgSeconds)
	}

	// 4. Расчет процентов (Solved %)
	for i := range depts {
		if depts[i].TotalCount > 0 {
			depts[i].SolvedPercent = math.Round((float64(depts[i].ResolvedCount) / float64(depts[i].TotalCount)) * 100)
		}
	}
	for i := range branches {
		if branches[i].TotalCount > 0 {
			branches[i].SolvedPercent = math.Round((float64(branches[i].ResolvedCount) / float64(branches[i].TotalCount)) * 100)
		}
	}

	return &dto.DashboardStatsDTO{
		Alerts:          alerts,
		KPIs:            kpis,
		SLA:             sla,
		WeeklyVolume:    weekly,
		TimeByPriority:  timePrior,
		TimeByOrderType: timeType,
		CountByStatus:   cntStatus,
		CountByExecutor: cntExec,
		TopCategories:   topCat,
		LastActivity:    lastAct,
		Departments:     depts,
		Branches:        branches,
	}, nil
}

// fillMissingDays генерирует 14 точек графика (от "13 дней назад" до "сегодня")
func fillMissingDays(data []types.DashboardChartData) []types.DashboardChartData {
	dataMap := make(map[string]int64)
	for _, item := range data {
		dataMap[item.Label] = item.Value
	}

	var result []types.DashboardChartData
	// Используем время сервера, выровненное по часам, или просто Day
	now := time.Now()
	// Важно: если БД хранит UTC, а тут Local, метки могут не совпасть на границе дня.
	// Но для dashboard "приблизительно сегодня" обычно достаточно.
	start := now.AddDate(0, 0, -13)

	for i := 0; i < 14; i++ {
		day := start.AddDate(0, 0, i)
		label := day.Format("02.01")

		val := int64(0)
		if v, exists := dataMap[label]; exists {
			val = v
		}

		result = append(result, types.DashboardChartData{
			Label: label,
			Value: val,
		})
	}
	return result
}
