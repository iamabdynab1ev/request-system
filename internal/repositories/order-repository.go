package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils"
)

const (
	orderTable        = "orders"
	orderFields       = "id, name, department_id, otdel_id, priority_id, status_id, branch_id, office_id, equipment_id, user_id, executor_id, duration, address, created_at, updated_at, deleted_at, equipment_type_id, order_type_id, completed_at, resolution_time_seconds, first_response_time_seconds, is_first_contact_resolution"
	orderInsertFields = "name, address, department_id, otdel_id, branch_id, office_id, equipment_id, equipment_type_id, order_type_id, status_id, priority_id, user_id, executor_id, duration"
)

type OrderRepositoryInterface interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)
	FindByID(ctx context.Context, orderID uint64) (*entities.Order, error)
	Create(ctx context.Context, tx pgx.Tx, order *entities.Order) (uint64, error)
	GetOrders(ctx context.Context, filter types.Filter, securityCondition sq.Sqlizer) ([]entities.Order, uint64, error)
	Update(ctx context.Context, tx pgx.Tx, order *entities.Order) error
	DeleteOrder(ctx context.Context, orderID uint64) error
	CountOrdersByOtdelID(ctx context.Context, id uint64) (uint64, error)
	GetUserOrderStats(ctx context.Context, userID uint64, fromDate time.Time) (*types.UserOrderStats, error)
	applyBaseFilters(baseQuery sq.SelectBuilder, filter types.Filter, securityCondition sq.Sqlizer) sq.SelectBuilder
}

type OrderRepository struct {
	storage *pgxpool.Pool
	logger  *zap.Logger
}

func NewOrderRepository(storage *pgxpool.Pool, logger *zap.Logger) OrderRepositoryInterface {
	return &OrderRepository{storage: storage, logger: logger}
}

func (r *OrderRepository) scanOrder(row pgx.Row) (*entities.Order, error) {
	var order entities.Order
	err := row.Scan(
		&order.ID, &order.Name, &order.DepartmentID, &order.OtdelID, &order.PriorityID, &order.StatusID,
		&order.BranchID, &order.OfficeID, &order.EquipmentID, &order.CreatorID, &order.ExecutorID,
		&order.Duration, &order.Address, &order.CreatedAt, &order.UpdatedAt, &order.DeletedAt,
		&order.EquipmentTypeID, &order.OrderTypeID, &order.CompletedAt, &order.ResolutionTimeSeconds,
		&order.FirstResponseTimeSeconds, &order.IsFirstContactResolution,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &order, nil
}

func (r *OrderRepository) GetDashboardAlerts(ctx context.Context, filter types.Filter, securityCondition sq.Sqlizer) (*types.DashboardAlerts, error) {
	// Нам нужно найти кол-во просроченных (Overdue) и критических (Critical).
	// Предполагаем, что CRITICAL определяется по priority_id (надо будет проверить коды) или priorities.level.
	// Но для надежности джоиним priorities.

	baseQuery := sq.Select().
		From("orders AS o").
		LeftJoin("statuses s ON o.status_id = s.id").
		LeftJoin("priorities p ON o.priority_id = p.id").
		PlaceholderFormat(sq.Dollar)

	// Применяем твой базовый фильтр безопасности
	if securityCondition != nil {
		baseQuery = baseQuery.Where(securityCondition)
	}

	// Базовое условие: не удалено и не закрыто
	baseQuery = baseQuery.Where(sq.Eq{"o.deleted_at": nil}).
		Where(sq.NotEq{"s.code": []string{"COMPLETED", "CLOSED", "REJECTED"}})

	// Применяем фильтры дат если есть
	if filter.DateFrom != nil {
		baseQuery = baseQuery.Where(sq.GtOrEq{"o.created_at": filter.DateFrom})
	}

	queryBuilder := baseQuery.Columns(
		// Считаем критические (проверяем код приоритета или имя)
		"COUNT(CASE WHEN UPPER(p.code) = 'CRITICAL' OR UPPER(p.code) = 'HIGH' THEN 1 END) as critical_count",
		// Считаем просроченные (дедлайн прошел)
		"COUNT(CASE WHEN o.duration IS NOT NULL AND o.duration < NOW() THEN 1 END) as overdue_count",
	)

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build alert query: %w", err)
	}

	stats := &types.DashboardAlerts{}
	err = r.storage.QueryRow(ctx, query, args...).Scan(&stats.CriticalCount, &stats.OverdueCount)
	if err != nil {
		return nil, fmt.Errorf("failed to execute alert query: %w", err)
	}

	return stats, nil
}

func (r *OrderRepository) applyBaseFilters(baseQuery sq.SelectBuilder, filter types.Filter, securityCondition sq.Sqlizer) sq.SelectBuilder {
	if securityCondition != nil {
		baseQuery = baseQuery.Where(securityCondition)
	}
	if filter.DateFrom != nil {
		baseQuery = baseQuery.Where(sq.GtOrEq{"o.created_at": filter.DateFrom})
	}
	if filter.DateTo != nil {
		baseQuery = baseQuery.Where(sq.LtOrEq{"o.created_at": filter.DateTo})
	}
	if len(filter.ExecutorIDs) > 0 {
		baseQuery = baseQuery.Where(sq.Eq{"o.executor_id": filter.ExecutorIDs})
	}
	return baseQuery
}

func (r *OrderRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.storage.Begin(ctx)
}

func (r *OrderRepository) FindByID(ctx context.Context, orderID uint64) (*entities.Order, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1 AND deleted_at IS NULL", orderFields, orderTable)
	row := r.storage.QueryRow(ctx, query, orderID)
	return r.scanOrder(row)
}

func (r *OrderRepository) GetUserOrderStats(ctx context.Context, userID uint64, fromDate time.Time) (*types.UserOrderStats, error) {
	query := `
		SELECT 
			COUNT(CASE WHEN s.code IN ('IN_PROGRESS', 'CLARIFICATION', 'REFINEMENT') THEN 1 END) as in_progress_count,
			COUNT(CASE WHEN s.code = 'COMPLETED' THEN 1 END) as completed_count,
			COUNT(CASE WHEN s.code = 'CLOSED' THEN 1 END) as closed_count,
			COUNT(CASE WHEN o.duration IS NOT NULL AND o.duration < NOW() AND s.code NOT IN ('COMPLETED', 'CLOSED', 'REJECTED') THEN 1 END) as overdue_count,
			COALESCE(AVG(CASE WHEN s.code IN ('COMPLETED', 'CLOSED') AND o.resolution_time_seconds IS NOT NULL THEN o.resolution_time_seconds END), 0) as avg_resolution_seconds
		FROM orders o
		JOIN statuses s ON o.status_id = s.id
		WHERE (o.executor_id = $1 OR o.user_id = $1)
		  AND o.deleted_at IS NULL
		  AND o.created_at >= $2
	`

	var stats types.UserOrderStats
	err := r.storage.QueryRow(ctx, query, userID, fromDate).Scan(
		&stats.InProgressCount,
		&stats.CompletedCount,
		&stats.ClosedCount,
		&stats.OverdueCount,
		&stats.AvgResolutionSeconds,
	)
	if err != nil {
		r.logger.Error("Ошибка получения статистики пользователя", zap.Uint64("userID", userID), zap.Error(err))
		return nil, err
	}

	stats.TotalCount = stats.InProgressCount + stats.CompletedCount + stats.ClosedCount + stats.OverdueCount

	return &stats, nil
}

func (r *OrderRepository) Create(ctx context.Context, tx pgx.Tx, order *entities.Order) (uint64, error) {
	query := fmt.Sprintf(`INSERT INTO %s (%s) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14) RETURNING id`,
		orderTable, orderInsertFields)

	var orderID uint64
	r.logger.Info("РЕПОЗИТОРИЙ: Данные для записи в `orders`",
		zap.Uint64("creator_id", order.CreatorID),
		zap.Any("executor_id", order.ExecutorID),
		zap.String("name", order.Name),
	)

	err := tx.QueryRow(ctx, query,
		order.Name,
		order.Address,
		order.DepartmentID,
		order.OtdelID,
		order.BranchID,
		order.OfficeID,
		order.EquipmentID,
		order.EquipmentTypeID,
		order.OrderTypeID,
		order.StatusID,
		order.PriorityID,
		order.CreatorID,
		order.ExecutorID,
		order.Duration,
	).Scan(&orderID)
	if err != nil {
		r.logger.Error("ОШИБКА INSERT В orders", zap.Error(err))
		return 0, err
	}
	r.logger.Info("Заявка создана в БД", zap.Uint64("orderID", orderID))
	return orderID, nil
}

func (r *OrderRepository) Update(ctx context.Context, tx pgx.Tx, order *entities.Order) error {
	builder := sq.Update(orderTable).
		PlaceholderFormat(sq.Dollar).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": order.ID, "deleted_at": nil})

	builder = builder.Set("name", order.Name)
	builder = builder.Set("address", order.Address)
	builder = builder.Set("department_id", order.DepartmentID)
	builder = builder.Set("otdel_id", order.OtdelID)
	builder = builder.Set("branch_id", order.BranchID)
	builder = builder.Set("office_id", order.OfficeID)
	builder = builder.Set("equipment_id", order.EquipmentID)
	builder = builder.Set("equipment_type_id", order.EquipmentTypeID)
	builder = builder.Set("order_type_id", order.OrderTypeID)
	builder = builder.Set("status_id", order.StatusID)
	builder = builder.Set("priority_id", order.PriorityID)
	builder = builder.Set("executor_id", order.ExecutorID)
	builder = builder.Set("duration", order.Duration)

	builder = builder.Set("completed_at", order.CompletedAt)
	builder = builder.Set("resolution_time_seconds",
		sq.Expr("CASE WHEN ?::timestamp IS NOT NULL THEN EXTRACT(EPOCH FROM (?::timestamp - created_at)) ELSE NULL END", order.CompletedAt, order.CompletedAt),
	)
	// <<-- ДОБАВЛЕНЫ НОВЫЕ ПОЛЯ -->>
	builder = builder.Set("first_response_time_seconds", order.FirstResponseTimeSeconds)
	builder = builder.Set("is_first_contact_resolution", order.IsFirstContactResolution)

	sql, args, err := builder.ToSql()
	if err != nil {
		return fmt.Errorf("ошибка сборки UPDATE запроса для заявки: %w", err)
	}

	result, err := tx.Exec(ctx, sql, args...)
	if err != nil {
		r.logger.Error("Ошибка выполнения UPDATE запроса", zap.Error(err), zap.String("sql", sql), zap.Any("args", args))
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}

	r.logger.Info("Заявка обновлена в БД", zap.Uint64("orderID", order.ID))
	return nil
}

func (r *OrderRepository) DeleteOrder(ctx context.Context, orderID uint64) error {
	query := `UPDATE orders SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	result, err := r.storage.Exec(ctx, query, orderID)
	if err != nil {
		r.logger.Error("Ошибка при удалении заявки", zap.Uint64("orderID", orderID), zap.Error(err))
		return err
	}
	if result.RowsAffected() == 0 {
		r.logger.Warn("Заявка не найдена для удаления", zap.Uint64("orderID", orderID))
		return apperrors.ErrNotFound
	}
	r.logger.Info("Заявка удалена", zap.Uint64("orderID", orderID))
	return nil
}

func (r *OrderRepository) CountOrdersByOtdelID(ctx context.Context, id uint64) (uint64, error) {
	var count uint64
	query := "SELECT COUNT(id) FROM orders WHERE otdel_id = $1 AND deleted_at IS NULL"
	err := r.storage.QueryRow(ctx, query, id).Scan(&count)
	if err != nil {
		r.logger.Error("Ошибка подсчета заявок по отделу", zap.Uint64("otdelID", id), zap.Error(err))
		return 0, err
	}
	r.logger.Debug("Подсчет заявок по отделу", zap.Uint64("otdelID", id), zap.Uint64("count", count))
	return count, nil
}

func (r *OrderRepository) GetOrders(ctx context.Context, filter types.Filter, securityCondition sq.Sqlizer) ([]entities.Order, uint64, error) {
	// Use squirrel for building all parts of the query.
	// The base builder will be used for both counting and selecting.
	baseBuilder := sq.Select().From(orderTable + " AS o").
		PlaceholderFormat(sq.Dollar).
		Where(sq.Eq{"o.deleted_at": nil})

	// Apply the mandatory security condition.
	if securityCondition != nil {
		baseBuilder = baseBuilder.Where(securityCondition)
	}

	// Apply filters from the request.
	for key, value := range filter.Filter {
		switch key {
		case "duration_from":
			baseBuilder = baseBuilder.Where(sq.GtOrEq{"o.duration": value})
		case "duration_to":
			baseBuilder = baseBuilder.Where(sq.LtOrEq{"o.duration": value})
		case "overdue":
			// Join with statuses is needed for this filter.
			// It's safe to add it again; squirrel is smart enough.
			baseBuilder = baseBuilder.Join("statuses s ON o.status_id = s.id").
				Where(sq.Expr("o.duration < NOW() AND s.code NOT IN ('CLOSED', 'COMPLETED', 'REJECTED')"))
		default:
			// Handle both slice of IDs and comma-separated string of IDs.
			var idsToFilter []uint64
			if ids, ok := value.([]uint64); ok {
				idsToFilter = ids
			} else if valStr, ok := value.(string); ok && strings.Contains(valStr, ",") {
				parsedIDs, _ := utils.ParseUint64Slice(strings.Split(valStr, ","))
				idsToFilter = parsedIDs
			}

			if len(idsToFilter) > 0 {
				baseBuilder = baseBuilder.Where(sq.Eq{"o." + key: idsToFilter})
			} else {
				baseBuilder = baseBuilder.Where(sq.Eq{"o." + key: value})
			}
		}
	}

	// Apply search term.
	if filter.Search != "" {
		baseBuilder = baseBuilder.Where(sq.ILike{"o.name": "%" + filter.Search + "%"})
	}

	// Build and execute the count query.
	countBuilder := baseBuilder.Columns("COUNT(o.id)")
	countQuery, countArgs, err := countBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build count query for orders: %w", err)
	}

	var totalCount uint64
	if err := r.storage.QueryRow(ctx, countQuery, countArgs...).Scan(&totalCount); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			r.logger.Error("Ошибка подсчета заявок", zap.Error(err), zap.String("query", countQuery), zap.Any("args", countArgs))
			return nil, 0, fmt.Errorf("failed to execute count query for orders: %w", err)
		}
		// If no rows, totalCount is 0, which is correct.
	}

	if totalCount == 0 {
		return []entities.Order{}, 0, nil
	}

	// Build the main select query from the base.
	selectBuilder := baseBuilder.Columns("o.*")
	if len(filter.Sort) > 0 {
		for field, direction := range filter.Sort {
			selectBuilder = selectBuilder.OrderBy(fmt.Sprintf("o.%s %s", field, direction))
		}
	} else {
		selectBuilder = selectBuilder.OrderBy("o.created_at DESC")
	}

	if filter.WithPagination {
		selectBuilder = selectBuilder.Limit(uint64(filter.Limit)).Offset(uint64(filter.Offset))
	}

	mainQuery, mainArgs, err := selectBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build select query for orders: %w", err)
	}

	r.logger.Debug("Выполнение запроса GetOrders", zap.String("query", mainQuery), zap.Any("args", mainArgs))
	rows, err := r.storage.Query(ctx, mainQuery, mainArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	orders := make([]entities.Order, 0, filter.Limit)
	for rows.Next() {
		order, err := r.scanOrder(rows)
		if err != nil {
			r.logger.Error("Ошибка сканирования строки", zap.Error(err))
			return nil, 0, fmt.Errorf("failed to scan order row: %w", err)
		}
		orders = append(orders, *order)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating over order rows: %w", err)
	}

	r.logger.Info("Заявки получены", zap.Int("count", len(orders)), zap.Uint64("total_count", totalCount))
	return orders, totalCount, nil
}
