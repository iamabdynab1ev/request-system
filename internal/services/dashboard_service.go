package services

import (
	"context"
	"fmt"
	"math"
	"sync"

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
	securityBuilder := sq.And{}

	if !authContext.HasPermission(authz.ScopeAll) && !authContext.HasPermission(authz.ScopeAllView) {
		scopeConditions := sq.Or{}
		if authContext.HasPermission(authz.ScopeDepartment) && actor.DepartmentID != nil {
			scopeConditions = append(scopeConditions, sq.Eq{"o.department_id": *actor.DepartmentID})
		}
		if authContext.HasPermission(authz.ScopeBranch) && actor.BranchID != nil {
			scopeConditions = append(scopeConditions, sq.Eq{"o.branch_id": *actor.BranchID})
		}
		if authContext.HasPermission(authz.ScopeOtdel) && actor.OtdelID != nil {
			scopeConditions = append(scopeConditions, sq.Eq{"o.otdel_id": *actor.OtdelID})
		}
		if authContext.HasPermission(authz.ScopeOwn) {
			scopeConditions = append(scopeConditions, sq.Eq{"o.user_id": actor.ID})
			scopeConditions = append(scopeConditions, sq.Eq{"o.executor_id": actor.ID})
		}
		if len(scopeConditions) > 0 {
			securityBuilder = append(securityBuilder, scopeConditions)
		} else {
			securityBuilder = append(securityBuilder, sq.Eq{"o.user_id": actor.ID})
		}
	}

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

	addTask(func() (err error) { alerts, err = s.repo.GetAlerts(ctx, securityBuilder); return })
	addTask(func() (err error) { kpis, err = s.repo.GetKPIs(ctx, securityBuilder); return })
	addTask(func() (err error) { sla, err = s.repo.GetSLAStats(ctx, securityBuilder); return })
	addTask(func() (err error) { timePrior, err = s.repo.GetAvgTimeByPriority(ctx, securityBuilder); return })
	addTask(func() (err error) { timeType, err = s.repo.GetAvgTimeByOrderType(ctx, securityBuilder); return })
	addTask(func() (err error) { cntStatus, err = s.repo.GetCountByStatus(ctx, securityBuilder); return })
	addTask(func() (err error) { cntExec, err = s.repo.GetCountByExecutor(ctx, securityBuilder); return })
	addTask(func() (err error) { weekly, err = s.repo.GetWeeklyVolume(ctx, securityBuilder); return })
	addTask(func() (err error) { topCat, err = s.repo.GetTopCategories(ctx, securityBuilder); return })
	addTask(func() (err error) { lastAct, err = s.repo.GetLastActivity(ctx, securityBuilder); return })
	addTask(func() (err error) { depts, err = s.repo.GetDepartmentStats(ctx, securityBuilder); return })

	wg.Wait()

	if len(errs) > 0 {
		s.logger.Error("Dashboard errors", zap.Error(errs[0]))
		return nil, apperrors.ErrInternalServer
	}

	// Форматирование (Бизнес логика текстов и трендов)
	if kpis != nil {
		process := func(m *types.DashboardKPIMetric, unit string, isTime bool) {
			// 1. Считаем % изменения
			if m.Previous > 0 {
				m.TrendPct = ((m.Current - m.Previous) / m.Previous) * 100
			} else if m.Current > 0 {
				m.TrendPct = 100 // Рост с нуля
			} else {
				m.TrendPct = 0
			}

			// Округляем процент до целого для красоты
			m.TrendPct = math.Round(m.TrendPct)

			// 2. Формируем текст тренда
			sign := ""
			suffix := "за месяц"

			if m.TrendPct > 0 {
				sign = "+"
			}

			if isTime {
				// ДЛЯ ВРЕМЕНИ (секунды):
				// Меньше - значит улучшение (быстрее работаем)
				// Больше - ухудшение
				if m.TrendPct < 0 {
					// Отрицательный процент (время уменьшилось)
					m.TrendText = fmt.Sprintf("%d%% улучшение", int(math.Abs(m.TrendPct)))
				} else if m.TrendPct > 0 {
					m.TrendText = fmt.Sprintf("%s%.0f%% за месяц", sign, m.TrendPct)
				} else {
					m.TrendText = "без изменений"
				}

				// Перевод секунд в часы для отображения
				hrs := m.Current / 3600
				m.Formatted = fmt.Sprintf("%.1fч", hrs)

			} else {
				// ДЛЯ ЧИСЕЛ И ПРОЦЕНТОВ (Количество):
				if m.TrendPct != 0 {
					m.TrendText = fmt.Sprintf("%s%.0f%% %s", sign, m.TrendPct, suffix)
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

		// Применяем ко всем
		process(&kpis.TotalTickets, "", false)
		process(&kpis.ResolvedTickets, "", false)

		process(&kpis.SLACompliance, "%", false)
		process(&kpis.FCRRate, "%", false)

		process(&kpis.AvgResponseTime, "time", true)
		process(&kpis.AvgResolveTime, "time", true)

		// Для открытых тренд не считается по прошлому периоду
		kpis.OpenTickets.Formatted = fmt.Sprintf("%.0f", kpis.OpenTickets.Current)
		kpis.OpenTickets.TrendText = "Актуальные"
		kpis.OpenTickets.TrendPct = 0
	}

	// Списки времени (форматируем секунды)
	for i := range timePrior {
		timePrior[i].AvgTimeFormatted = utils.FormatFloatSecondsToHumanReadable(timePrior[i].AvgSeconds)
	}
	for i := range timeType {
		timeType[i].AvgTimeFormatted = utils.FormatFloatSecondsToHumanReadable(timeType[i].AvgSeconds)
	}

	// Отделы (% выполнения)
	for i := range depts {
		if depts[i].TotalCount > 0 {
			depts[i].SolvedPercent = math.Round((float64(depts[i].ResolvedCount) / float64(depts[i].TotalCount)) * 100)
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
	}, nil
}
