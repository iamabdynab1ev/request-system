package repositories

import (
	"context"
	"fmt"

	"request-system/internal/entities"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ReportRepositoryInterface interface {
	GetReport(ctx context.Context, filter entities.ReportFilter) ([]entities.ReportItem, uint64, error)
}

type reportRepository struct {
	db *pgxpool.Pool
}

func NewReportRepository(db *pgxpool.Pool) ReportRepositoryInterface {
	return &reportRepository{db: db}
}

func (r *reportRepository) GetReport(ctx context.Context, filter entities.ReportFilter) ([]entities.ReportItem, uint64, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

	// 1. Создаем ОБЩУЮ БАЗУ для обоих запросов (FROM, JOIN, WHERE)
	baseSelect := psql.Select().
		From("orders o").
		LeftJoin("users creator ON o.user_id = creator.id").
		LeftJoin("users executor ON o.executor_id = executor.id").
		LeftJoin("order_types ot ON o.order_type_id = ot.id").
		LeftJoin("priorities p ON o.priority_id = p.id").
		LeftJoin("statuses s ON o.status_id = s.id").
		Where(sq.Eq{"o.deleted_at": nil})

	// 2. Применяем к этой базе все фильтры
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

	// 3. Строим и выполняем COUNT-запрос
	countBuilder := baseSelect.Columns("COUNT(o.id)")
	countQuery, countArgs, err := countBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка сборки COUNT-запроса: %w", err)
	}
	var totalCount uint64
	if err = r.db.QueryRow(ctx, countQuery, countArgs...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("ошибка выполнения COUNT-запроса: %w", err)
	}
	if totalCount == 0 {
		return []entities.ReportItem{}, 0, nil
	}

	// 4. Достраиваем основной запрос, добавляя КОЛОНКИ и СОРТИРОВКУ
	mainBuilder := baseSelect.Columns(
		"o.id", "creator.fio", "o.created_at", "ot.name", "p.name", "s.name", "o.name", "executor.fio",
		"(SELECT MIN(h.created_at) FROM order_history h WHERE h.order_id = o.id AND h.event_type = 'DELEGATION')",
		"o.completed_at",
		"CASE WHEN o.resolution_time_seconds IS NOT NULL THEN ROUND(o.resolution_time_seconds::numeric / 3600, 2) ELSE NULL END",
		`CASE 
			WHEN o.completed_at IS NOT NULL AND o.duration IS NOT NULL AND o.completed_at <= o.duration THEN 'Выполнен'
			WHEN o.completed_at IS NOT NULL AND o.duration IS NOT NULL AND o.completed_at > o.duration THEN 'Не выполнен'
			WHEN o.duration IS NOT NULL AND NOW() > o.duration AND s.code != 'CLOSED' THEN 'Просрочен'
			ELSE '-' 
		 END`,
		"(SELECT h.comment FROM order_history h WHERE h.order_id = o.id AND h.event_type = 'COMMENT' ORDER BY h.created_at DESC LIMIT 1)",
	).OrderBy("o.id DESC")

	// 5. Добавляем ПАГИНАЦИЮ
	if filter.PerPage > 0 {
		mainBuilder = mainBuilder.Limit(uint64(filter.PerPage)).Offset(uint64((filter.Page - 1) * filter.PerPage))
	}

	// 6. Выполняем основной запрос
	sql, args, err := mainBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка сборки основного запроса: %w", err)
	}
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка выполнения основного запроса: %w", err)
	}
	defer rows.Close()

	// 7. Сканируем результаты
	var reportItems []entities.ReportItem
	for rows.Next() {
		var item entities.ReportItem
		err := rows.Scan(
			&item.OrderID, &item.CreatorFio, &item.CreatedAt, &item.OrderTypeName,
			&item.PriorityName, &item.StatusName, &item.OrderName, &item.ExecutorFio,
			&item.DelegatedAt, &item.CompletedAt, &item.ResolutionHours,
			&item.SLAStatus, &item.Comment,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("ошибка сканирования строки: %w", err)
		}
		reportItems = append(reportItems, item)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	return reportItems, totalCount, nil
}
