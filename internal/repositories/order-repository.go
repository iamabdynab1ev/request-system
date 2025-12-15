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

	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
)

const (
	orderTable = "orders"
)

var allowedOrderFilters = map[string]string{
	"id":            "o.id",
	"name":          "o.name",
	"status_id":     "o.status_id",
	"priority_id":   "o.priority_id",
	"department_id": "o.department_id",
	"branch_id":     "o.branch_id",
	"otdel_id":      "o.otdel_id",
	"office_id":     "o.office_id",
	"executor_id":   "o.executor_id",
	"creator_id":    "o.user_id",
	"user_id":       "o.user_id",
	"created_at":    "o.created_at",
	"updated_at":    "o.updated_at",
	"order_type_id": "o.order_type_id",
}

type OrderRepositoryInterface interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)

	FindByID(ctx context.Context, orderID uint64) (*entities.Order, error)
	Create(ctx context.Context, tx pgx.Tx, order *entities.Order) (uint64, error)
	Update(ctx context.Context, tx pgx.Tx, order *entities.Order) error
	DeleteOrder(ctx context.Context, orderID uint64) error

	GetOrders(ctx context.Context, filter types.Filter, securityCondition sq.Sqlizer) ([]entities.Order, uint64, error)

	GetUserOrderStats(ctx context.Context, userID uint64, fromDate time.Time) (*types.UserOrderStats, error)
}

type OrderRepository struct {
	storage *pgxpool.Pool
	logger  *zap.Logger
}

func NewOrderRepository(storage *pgxpool.Pool, logger *zap.Logger) OrderRepositoryInterface {
	return &OrderRepository{storage: storage, logger: logger}
}

// BeginTx - начало транзакции
func (r *OrderRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.storage.Begin(ctx)
}

// buildOrderSelectQuery - базовый SELECT с JOIN для получения FIO
func (r *OrderRepository) buildOrderSelectQuery() sq.SelectBuilder {
	return sq.Select(
		"o.id",
		"o.name",
		"o.address",
		"o.department_id",
		"o.otdel_id",
		"o.branch_id",
		"o.office_id",
		"o.equipment_id",
		"o.equipment_type_id",
		"o.order_type_id",
		"o.status_id",
		"o.priority_id",
		"o.user_id",
		"o.executor_id",
		"o.duration",
		"o.created_at",
		"o.updated_at",
		"o.deleted_at",
		"o.completed_at",
		"o.first_response_time_seconds",
		"o.resolution_time_seconds",
		"o.is_first_contact_resolution",
		// JOIN для FIO
		"creator.fio as creator_name",
		"executor.fio as executor_name",
	).
		From(orderTable + " o").
		LeftJoin("users creator ON o.user_id = creator.id").
		LeftJoin("users executor ON o.executor_id = executor.id").
		PlaceholderFormat(sq.Dollar)
}

func (r *OrderRepository) FindByID(ctx context.Context, orderID uint64) (*entities.Order, error) {
	queryBuilder := r.buildOrderSelectQuery().
		Where(sq.Eq{"o.id": orderID, "o.deleted_at": nil})

	sqlStr, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("ошибка сборки SQL для FindByID: %w", err)
	}

	rows, err := r.storage.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	order, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.Order])
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.ErrNotFound
		}
		r.logger.Error("Ошибка маппинга заявки", zap.Error(err))
		return nil, err
	}

	return &order, nil
}

// GetOrders - УНИВЕРСАЛЬНАЯ ФИЛЬТРАЦИЯ как у Users
func (r *OrderRepository) GetOrders(ctx context.Context, filter types.Filter, securityCondition sq.Sqlizer) ([]entities.Order, uint64, error) {
	countBuilder := sq.Select("count(o.id)").
		From(orderTable + " o").
		Where(sq.Eq{"o.deleted_at": nil}).
		PlaceholderFormat(sq.Dollar)

	// Security условия
	if securityCondition != nil {
		countBuilder = countBuilder.Where(securityCondition)
	}

	// ПОИСК (по name и address)
	if filter.Search != "" {
		match := "%" + filter.Search + "%"
		countBuilder = countBuilder.Where(sq.Or{
			sq.ILike{"o.name": match},
			sq.ILike{"o.address": match},
		})
	}

	// ФИЛЬТРЫ (динамические)
	// Специальные фильтры
	if dFrom, ok := filter.Filter["duration_from"]; ok {
		countBuilder = countBuilder.Where(sq.GtOrEq{"o.duration": dFrom})
		delete(filter.Filter, "duration_from")
	}
	if dTo, ok := filter.Filter["duration_to"]; ok {
		countBuilder = countBuilder.Where(sq.LtOrEq{"o.duration": dTo})
		delete(filter.Filter, "duration_to")
	}

	// Просроченные заявки
	isOverdue := false
	if val, ok := filter.Filter["overdue"]; ok {
		if valStr, _ := val.(string); valStr == "true" {
			isOverdue = true
		}
		delete(filter.Filter, "overdue")
	}

	// Универсальные фильтры через белый список
	for jsonField, val := range filter.Filter {
		if dbCol, ok := allowedOrderFilters[jsonField]; ok {
			// Поддержка множественных значений через запятую
			if s, ok := val.(string); ok && strings.Contains(s, ",") {
				countBuilder = countBuilder.Where(sq.Eq{dbCol: strings.Split(s, ",")})
			} else {
				countBuilder = countBuilder.Where(sq.Eq{dbCol: val})
			}
		}
	}

	// JOIN для просроченных
	if isOverdue {
		countBuilder = countBuilder.Join("statuses s ON o.status_id = s.id").
			Where("o.duration < NOW()").
			Where("s.code NOT IN ('CLOSED', 'COMPLETED', 'REJECTED')")
	}

	// Выполняем COUNT
	countSql, countArgs, err := countBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка сборки SQL count: %w", err)
	}

	var totalCount uint64
	if err := r.storage.QueryRow(ctx, countSql, countArgs...).Scan(&totalCount); err != nil {
		return nil, 0, err
	}
	if totalCount == 0 {
		return []entities.Order{}, 0, nil
	}

	selectBuilder := r.buildOrderSelectQuery().
		Where(sq.Eq{"o.deleted_at": nil})

	// Применяем ТЕ ЖЕ условия что и в COUNT
	if securityCondition != nil {
		selectBuilder = selectBuilder.Where(securityCondition)
	}

	if filter.Search != "" {
		match := "%" + filter.Search + "%"
		selectBuilder = selectBuilder.Where(sq.Or{
			sq.ILike{"o.name": match},
			sq.ILike{"o.address": match},
		})
	}

	// Восстанавливаем фильтры (они были удалены из map)
	if dFrom, ok := filter.Filter["duration_from"]; ok {
		selectBuilder = selectBuilder.Where(sq.GtOrEq{"o.duration": dFrom})
	}
	if dTo, ok := filter.Filter["duration_to"]; ok {
		selectBuilder = selectBuilder.Where(sq.LtOrEq{"o.duration": dTo})
	}

	for jsonField, val := range filter.Filter {
		if dbCol, ok := allowedOrderFilters[jsonField]; ok {
			if s, ok := val.(string); ok && strings.Contains(s, ",") {
				selectBuilder = selectBuilder.Where(sq.Eq{dbCol: strings.Split(s, ",")})
			} else {
				selectBuilder = selectBuilder.Where(sq.Eq{dbCol: val})
			}
		}
	}

	if isOverdue {
		selectBuilder = selectBuilder.Join("statuses s ON o.status_id = s.id").
			Where("o.duration < NOW()").
			Where("s.code NOT IN ('CLOSED', 'COMPLETED', 'REJECTED')")
	}

	// СОРТИРОВКА (динамическая через белый список)
	if len(filter.Sort) > 0 {
		for jsonField, dir := range filter.Sort {
			if dbCol, ok := allowedOrderFilters[jsonField]; ok {
				direction := "DESC"
				if strings.ToLower(dir) == "asc" {
					direction = "ASC"
				}
				selectBuilder = selectBuilder.OrderBy(fmt.Sprintf("%s %s", dbCol, direction))
			}
		}
	} else {
		// Сортировка по умолчанию
		selectBuilder = selectBuilder.OrderBy("o.created_at DESC")
	}

	// ПАГИНАЦИЯ
	if filter.WithPagination {
		if filter.Limit > 0 {
			selectBuilder = selectBuilder.Limit(uint64(filter.Limit))
		}
		if filter.Offset >= 0 {
			selectBuilder = selectBuilder.Offset(uint64(filter.Offset))
		}
	}

	// Выполняем SELECT
	finalSql, finalArgs, err := selectBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка сборки SQL select: %w", err)
	}
	r.logger.Warn("DEBUGGING GetOrders SQL", zap.String("sql", finalSql), zap.Any("args", finalArgs))
	rows, err := r.storage.Query(ctx, finalSql, finalArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	orders, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.Order])
	if err != nil {
		r.logger.Error("Ошибка сканирования списка заявок", zap.Error(err))
		return nil, 0, err
	}

	return orders, totalCount, nil
}

// Create - создание заявки
func (r *OrderRepository) Create(ctx context.Context, tx pgx.Tx, order *entities.Order) (uint64, error) {
	query := `INSERT INTO orders 
		(name, address, department_id, otdel_id, branch_id, office_id, 
		 equipment_id, equipment_type_id, order_type_id, status_id, priority_id, 
		 user_id, executor_id, duration, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW(), NOW())
		RETURNING id`

	err := tx.QueryRow(ctx, query,
		order.Name, order.Address, order.DepartmentID, order.OtdelID,
		order.BranchID, order.OfficeID, order.EquipmentID, order.EquipmentTypeID,
		order.OrderTypeID, order.StatusID, order.PriorityID, order.CreatorID,
		order.ExecutorID, order.Duration,
	).Scan(&order.ID)

	return order.ID, err
}

func (r *OrderRepository) Update(ctx context.Context, tx pgx.Tx, order *entities.Order) error {
	b := sq.Update(orderTable).
		PlaceholderFormat(sq.Dollar).
		Set("updated_at", sq.Expr("NOW()")).
		Set("name", order.Name).
		Set("address", order.Address).
		Set("duration", order.Duration).
		Set("status_id", order.StatusID).
		Set("priority_id", order.PriorityID).
		Set("executor_id", order.ExecutorID).
		Set("department_id", order.DepartmentID).
		Set("otdel_id", order.OtdelID).
		Set("branch_id", order.BranchID).
		Set("office_id", order.OfficeID).
		Set("order_type_id", order.OrderTypeID).
		Set("equipment_id", order.EquipmentID).
		Set("equipment_type_id", order.EquipmentTypeID).
		Set("completed_at", order.CompletedAt).
		Set("resolution_time_seconds", order.ResolutionTimeSeconds).
		Set("first_response_time_seconds", order.FirstResponseTimeSeconds).
		Set("is_first_contact_resolution", order.IsFirstContactResolution).
		Where(sq.Eq{"id": order.ID, "deleted_at": nil})

	sqlStr, args, err := b.ToSql()
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, sqlStr, args...)
	return err
}

func (r *OrderRepository) DeleteOrder(ctx context.Context, orderID uint64) error {
	query := `UPDATE orders SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	cmd, err := r.storage.Exec(ctx, query, orderID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// GetUserOrderStats - аналитика
func (r *OrderRepository) GetUserOrderStats(ctx context.Context, userID uint64, fromDate time.Time) (*types.UserOrderStats, error) {
	query := `
		SELECT 
			COUNT(CASE WHEN s.code IN ('IN_PROGRESS', 'CLARIFICATION', 'REFINEMENT') THEN 1 END),
			COUNT(CASE WHEN s.code = 'COMPLETED' THEN 1 END),
			COUNT(CASE WHEN s.code = 'CLOSED' THEN 1 END),
			COUNT(CASE WHEN o.duration IS NOT NULL AND o.duration < NOW() AND s.code NOT IN ('COMPLETED', 'CLOSED', 'REJECTED') THEN 1 END),
			COALESCE(AVG(CASE WHEN s.code IN ('COMPLETED', 'CLOSED') AND o.resolution_time_seconds > 0 THEN o.resolution_time_seconds END), 0)
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
		return nil, err
	}
	stats.TotalCount = stats.InProgressCount + stats.CompletedCount + stats.ClosedCount + stats.OverdueCount
	return &stats, nil
}
