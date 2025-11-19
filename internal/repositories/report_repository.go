package repositories

import (
	"context"
	"fmt"

	"request-system/internal/authz"
	"request-system/internal/entities"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type ReportRepositoryInterface interface {
	GetReport(ctx context.Context, filter entities.ReportFilter) ([]entities.ReportItem, uint64, error)
}

type reportRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewReportRepository(db *pgxpool.Pool, logger *zap.Logger) ReportRepositoryInterface {
	return &reportRepository{db: db, logger: logger}
}

func (r *reportRepository) GetReport(ctx context.Context, filter entities.ReportFilter) ([]entities.ReportItem, uint64, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

	cte := `
		WITH first_delegation AS (
			SELECT DISTINCT ON (h.order_id)
				h.order_id, h.created_at AS delegated_at, u.fio AS responsible_fio
			FROM order_history h JOIN users u ON u.id = CAST(h.new_value AS bigint)
			WHERE h.event_type = 'DELEGATION' AND h.new_value ~ '^[0-9]+$'
			ORDER BY h.order_id, h.created_at ASC
		)
	`

	baseSelect := psql.Select().
		From("orders o").
		LeftJoin("users creator ON o.user_id = creator.id").
		LeftJoin("users executor ON o.executor_id = executor.id").
		LeftJoin("departments creator_dep ON creator.department_id = creator_dep.id").
		LeftJoin("order_types ot ON o.order_type_id = ot.id").
		LeftJoin("priorities p ON o.priority_id = p.id").
		LeftJoin("statuses s ON o.status_id = s.id").
		LeftJoin("first_delegation fd ON o.id = fd.order_id").
		Where(sq.Eq{"o.deleted_at": nil})

	// Обычные фильтры из UI
	if filter.DateFrom != nil {
		baseSelect = baseSelect.Where(sq.GtOrEq{"o.created_at": filter.DateFrom})
	}
	if filter.DateTo != nil {
		baseSelect = baseSelect.Where(sq.LtOrEq{"o.created_at": filter.DateTo})
	}
	if len(filter.ExecutorIDs) > 0 {
		baseSelect = baseSelect.Where(sq.Eq{"o.executor_id": filter.ExecutorIDs})
	}
	if len(filter.OrderTypeIDs) > 0 {
		baseSelect = baseSelect.Where(sq.Eq{"o.order_type_id": filter.OrderTypeIDs})
	}
	if len(filter.PriorityIDs) > 0 {
		baseSelect = baseSelect.Where(sq.Eq{"o.priority_id": filter.PriorityIDs})
	}

	// --- ДИНАМИЧЕСКАЯ ФИЛЬТРАЦИЯ ПО SCOPE ---

	if filter.PermissionsMap != nil {
		actor := filter.Actor
		_, hasScopeAll := filter.PermissionsMap[authz.ScopeAll]
		_, hasScopeAllView := filter.PermissionsMap[authz.ScopeAllView]

		if !hasScopeAll && !hasScopeAllView && actor != nil {

			scopeConditions := sq.Or{}
			appliedScope := false // Флаг, что мы применили хотя бы один "широкий" scope

			// Уровень "Управленцев"
			if _, ok := filter.PermissionsMap[authz.ScopeDepartment]; ok && actor.DepartmentID != nil {
				scopeConditions = append(scopeConditions, sq.Eq{"o.department_id": *actor.DepartmentID})
				appliedScope = true
			}
			if _, ok := filter.PermissionsMap[authz.ScopeBranch]; ok && actor.BranchID != nil {
				scopeConditions = append(scopeConditions, sq.Eq{"o.branch_id": *actor.BranchID})
				appliedScope = true
			}
			if _, ok := filter.PermissionsMap[authz.ScopeOtdel]; ok && actor.OtdelID != nil {
				scopeConditions = append(scopeConditions, sq.Eq{"o.otdel_id": *actor.OtdelID})
				appliedScope = true
			}
			if _, ok := filter.PermissionsMap[authz.ScopeOffice]; ok && actor.OfficeID != nil {
				scopeConditions = append(scopeConditions, sq.Eq{"o.office_id": *actor.OfficeID})
				appliedScope = true
			}

			// Уровень "Сотрудника" (применяется, только если нет широких прав)
			if !appliedScope {
				if _, ok := filter.PermissionsMap[authz.ScopeOwn]; ok {
					scopeConditions = append(scopeConditions, sq.Eq{"o.user_id": actor.ID})
					scopeConditions = append(scopeConditions, sq.Eq{"o.executor_id": actor.ID})
					scopeConditions = append(scopeConditions, sq.Expr("EXISTS (SELECT 1 FROM order_history h WHERE h.order_id = o.id AND h.user_id = ?)", actor.ID))
				}
			}

			if len(scopeConditions) > 0 {
				baseSelect = baseSelect.Where(scopeConditions)
			} else {
				return []entities.ReportItem{}, 0, nil // Если нет прав, ничего не показываем
			}
		}
	}

	// --- ОСТАЛЬНОЙ КОД БЕЗ ИЗМЕНЕНИЙ ---
	countBuilder := baseSelect.Columns("COUNT(o.id)")
	countQuery, countArgs, err := countBuilder.ToSql()
	if err != nil {
		return nil, 0, err
	}
	var totalCount uint64
	if err = r.db.QueryRow(ctx, cte+countQuery, countArgs...).Scan(&totalCount); err != nil {
		return nil, 0, err
	}
	if totalCount == 0 {
		return []entities.ReportItem{}, 0, nil
	}

	mainBuilder := baseSelect.Columns(
		"o.id AS order_id", "creator.fio AS creator_fio", "o.created_at", "ot.name AS order_type_name",
		"p.name AS priority_name", "s.name AS status_name", "o.name AS order_name",
		"fd.responsible_fio", "fd.delegated_at", "executor.fio AS executor_fio", "o.completed_at",
		"COALESCE(TO_CHAR(o.completed_at - o.created_at, 'HH24:MI:SS'), NULL) AS resolution_time_str",
		`CASE WHEN o.completed_at IS NOT NULL AND o.duration IS NOT NULL AND o.completed_at <= o.duration THEN 'Выполнен' WHEN o.completed_at IS NOT NULL AND o.duration IS NOT NULL AND o.completed_at > o.duration THEN 'Не выполнен' WHEN o.completed_at IS NULL AND o.duration IS NOT NULL AND NOW() > o.duration THEN 'Просрочен' ELSE 'В процессе' END AS sla_status`,
		"creator_dep.name AS source_department",
		"(SELECT comment FROM order_history WHERE order_id = o.id AND event_type = 'COMMENT' ORDER BY created_at DESC LIMIT 1) AS comment",
	).OrderBy("o.id DESC")

	if filter.PerPage > 0 {
		mainBuilder = mainBuilder.Limit(uint64(filter.PerPage)).Offset(uint64((filter.Page - 1) * filter.PerPage))
	}

	sql, args, err := mainBuilder.ToSql()
	if err != nil {
		return nil, 0, err
	}

	finalSQL := cte + sql

	rows, err := r.db.Query(ctx, finalSQL, args...)
	if err != nil {
		r.logger.Error("Ошибка выполнения основного запроса отчета", zap.Error(err), zap.String("sql", finalSQL))
		return nil, 0, err
	}
	defer rows.Close()

	reportItems, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.ReportItem])
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка сканирования отчета: %w", err)
	}

	return reportItems, totalCount, nil
}
