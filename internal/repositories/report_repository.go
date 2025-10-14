package repositories

import (
	"context"
	"fmt"
	"strings"

	entity "request-system/internal/entities" // <-- Используй псевдоним

	// Удобный SQL-билдер, если есть в проекте
	"github.com/jackc/pgx/v5/pgxpool"
)

// ReportRepository определяет интерфейс для репозитория отчетов.
type ReportRepository interface {
	GetHistoryReport(ctx context.Context, filter entity.ReportFilter) ([]entity.HistoryReportItem, uint64, error)
}

// reportRepository реализует ReportRepository.
type reportRepository struct {
	db *pgxpool.Pool
}

// NewReportRepository создает новый экземпляр репозитория.
func NewReportRepository(db *pgxpool.Pool) ReportRepository {
	return &reportRepository{db: db}
}

// queryBuilder - утилита для построения запроса.
type queryBuilder struct {
	conditions []string
	args       []interface{}
	paramIndex int
}

func (b *queryBuilder) Add(condition string, arg interface{}) {
	b.paramIndex++
	b.conditions = append(b.conditions, fmt.Sprintf(condition, b.paramIndex))
	b.args = append(b.args, arg)
}

// filterFunc - тип функции-фильтра.
type filterFunc func(b *queryBuilder, filter entity.ReportFilter)

var filterRegistry = []filterFunc{
	func(b *queryBuilder, f entity.ReportFilter) {
		if len(f.OrderIDs) > 0 {
			b.Add("h.order_id = ANY($%d)", f.OrderIDs)
		}
	},
	func(b *queryBuilder, f entity.ReportFilter) {
		if len(f.UserIDs) > 0 {
			b.Add("h.user_id = ANY($%d)", f.UserIDs)
		}
	},
	func(b *queryBuilder, f entity.ReportFilter) {
		if len(f.EventTypes) > 0 {
			b.Add("h.event_type = ANY($%d)", f.EventTypes)
		}
	},
	func(b *queryBuilder, f entity.ReportFilter) {
		if f.DateFrom != nil {
			b.Add("h.created_at >= $%d", f.DateFrom)
		}
	},
	func(b *queryBuilder, f entity.ReportFilter) {
		if f.DateTo != nil {
			b.Add("h.created_at <= $%d", f.DateTo)
		}
	},
	func(b *queryBuilder, f entity.ReportFilter) {
		if f.MetadataJSON != "" {
			b.Add("h.metadata @> $%d::jsonb", f.MetadataJSON)
		}
	},
}

// GetHistoryReport - основной метод, который выполняет два запроса: COUNT и SELECT.
func (r *reportRepository) GetHistoryReport(ctx context.Context, filter entity.ReportFilter) ([]entity.HistoryReportItem, uint64, error) {
	// 1. Применяем фильтры
	filterQb := queryBuilder{}
	for _, applyFilter := range filterRegistry {
		applyFilter(&filterQb, filter)
	}

	// 2. Делаем запрос для подсчета totalCount (быстрый, без JOIN'ов)
	countQuery := "SELECT COUNT(h.id) FROM order_history h"
	if len(filterQb.conditions) > 0 {
		countQuery += " WHERE " + strings.Join(filterQb.conditions, " AND ")
	}

	var totalCount uint64
	err := r.db.QueryRow(ctx, countQuery, filterQb.args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка подсчета записей в отчете: %w", err)
	}
	if totalCount == 0 {
		return []entity.HistoryReportItem{}, 0, nil
	}

	// 3. Строим основной, "богатый" запрос с JOIN'ами
	baseQuery := `
		SELECT 
			h.id, h.order_id, o.name AS order_name, h.user_id, u.fio AS user_name,
			h.event_type, h.old_value, h.new_value, h.comment, h.metadata, h.created_at
		FROM 
			order_history h
		LEFT JOIN 
			orders o ON h.order_id = o.id
		LEFT JOIN 
			users u ON h.user_id = u.id
	`
	finalQuery := baseQuery
	if len(filterQb.conditions) > 0 {
		finalQuery += " WHERE " + strings.Join(filterQb.conditions, " AND ")
	}

	sortOrder := "DESC"
	if strings.ToUpper(filter.SortOrder) == "ASC" {
		sortOrder = "ASC"
	}
	finalQuery += fmt.Sprintf(" ORDER BY h.created_at %s, h.id %s", sortOrder, sortOrder)

	limit := 100
	if filter.PerPage > 0 {
		limit = filter.PerPage
	}
	offset := 0
	if filter.Page > 1 {
		offset = (filter.Page - 1) * limit
	}

	argsForQuery := append(filterQb.args, limit, offset)
	finalQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(filterQb.args)+1, len(filterQb.args)+2)

	// 4. Выполняем основной запрос
	rows, err := r.db.Query(ctx, finalQuery, argsForQuery...)
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка выполнения основного запроса отчета: %w", err)
	}
	defer rows.Close()

	// 5. Сканируем результаты
	var results []entity.HistoryReportItem
	for rows.Next() {
		var item entity.HistoryReportItem
		err := rows.Scan(
			&item.ID, &item.OrderID, &item.OrderName, &item.UserID, &item.UserName,
			&item.EventType, &item.OldValue, &item.NewValue, &item.Comment,
			&item.Metadata, &item.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("ошибка сканирования строки отчета: %w", err)
		}
		results = append(results, item)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	return results, totalCount, nil
}
