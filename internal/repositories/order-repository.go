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

// allowedOrderFilters — карта соответствия "Параметр API -> Колонка БД".
// Это защищает нас от SQL Injection и позволяет фронтенду использовать красивые имена.
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

func (r *OrderRepository) FindByID(ctx context.Context, orderID uint64) (*entities.Order, error) {
	queryBuilder := sq.Select("o.*").
		From(orderTable + " o").
		Where(sq.Eq{"o.id": orderID, "o.deleted_at": nil}).
		PlaceholderFormat(sq.Dollar)

	sqlStr, args, _ := queryBuilder.ToSql()

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

func (r *OrderRepository) GetOrders(ctx context.Context, filter types.Filter, securityCondition sq.Sqlizer) ([]entities.Order, uint64, error) {
	// 1. Строим Базовый Запрос (ТОЛЬКО условия, без сортировки и лимитов пока)
	baseBuilder := sq.Select().From(orderTable + " o").
		PlaceholderFormat(sq.Dollar).
		Where(sq.Eq{"o.deleted_at": nil})

	if securityCondition != nil {
		baseBuilder = baseBuilder.Where(securityCondition)
	}

	if filter.Search != "" {
		match := "%" + filter.Search + "%"
		baseBuilder = baseBuilder.Where(sq.Or{
			sq.ILike{"o.name": match},
			sq.ILike{"o.address": match},
		})
	}

	// Применяем фильтры дат и спец. фильтры
	if val, ok := filter.Filter["overdue"]; ok {
		if valStr, _ := val.(string); valStr == "true" {
			baseBuilder = baseBuilder.Join("statuses s ON o.status_id = s.id").
				Where("o.duration < NOW()").
				Where("s.code NOT IN ('CLOSED', 'COMPLETED', 'REJECTED')")
		}
		delete(filter.Filter, "overdue")
	}
	if dFrom, ok := filter.Filter["duration_from"]; ok {
		baseBuilder = baseBuilder.Where(sq.GtOrEq{"o.duration": dFrom})
		delete(filter.Filter, "duration_from")
	}
	if dTo, ok := filter.Filter["duration_to"]; ok {
		baseBuilder = baseBuilder.Where(sq.LtOrEq{"o.duration": dTo})
		delete(filter.Filter, "duration_to")
	}

	// 2. Накладываем автоматические WHERE фильтры (это наш билдер)
	// infrastructure.ApplyListParams вернет уже сортированный билдер с пагинацией,
	// но нам сначала нужен чистый запрос для COUNT.

	// ХИТРОСТЬ: Применяем фильтры вручную тут же или делаем это до вызова билдера
	// Чтобы не усложнять, мы просто разделим билдер.

	// Наложим ТОЛЬКО WHERE из фильтра (без сортировки и лимита)
	countBuilder := baseBuilder
	for jsonField, val := range filter.Filter {
		if dbCol, ok := allowedOrderFilters[jsonField]; ok {
			if s, ok := val.(string); ok && strings.Contains(s, ",") {
				countBuilder = countBuilder.Where(sq.Eq{dbCol: strings.Split(s, ",")})
			} else {
				countBuilder = countBuilder.Where(sq.Eq{dbCol: val})
			}
		}
	}

	// --- 3. Считаем COUNT ---
	countSql, countArgs, err := countBuilder.Columns("count(o.id)").ToSql()
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

	// --- 4. Получаем данные (Select) ---
	// Теперь берем тот же `countBuilder` (в нем все WHERE есть) и добавляем сортировку/пагинацию
	selectBuilder := countBuilder.Columns("o.*")

	// Применяем параметры сортировки и пагинации вручную (так надежнее без remove методов)
	// Сортировка
	if len(filter.Sort) > 0 {
		for jsonField, dir := range filter.Sort {
			if dbCol, ok := allowedOrderFilters[jsonField]; ok {
				sqlDir := "ASC"
				if strings.ToLower(dir) == "desc" {
					sqlDir = "DESC"
				}
				selectBuilder = selectBuilder.OrderBy(fmt.Sprintf("%s %s", dbCol, sqlDir))
			}
		}
	} else {
		selectBuilder = selectBuilder.OrderBy("o.created_at DESC")
	}

	// Пагинация
	if filter.WithPagination {
		if filter.Limit > 0 {
			selectBuilder = selectBuilder.Limit(uint64(filter.Limit))
		}
		if filter.Offset >= 0 {
			selectBuilder = selectBuilder.Offset(uint64(filter.Offset))
		}
	}

	finalSql, finalArgs, err := selectBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка сборки SQL select: %w", err)
	}

	// Выполняем
	rows, err := r.storage.Query(ctx, finalSql, finalArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// CollectRows сам маппит в структуру
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

		// Основные поля
		Set("name", order.Name).
		Set("address", order.Address).
		Set("duration", order.Duration).
		Set("status_id", order.StatusID).
		Set("priority_id", order.PriorityID).
		Set("executor_id", order.ExecutorID).

		// Орг структура
		Set("department_id", order.DepartmentID).
		Set("otdel_id", order.OtdelID).
		Set("branch_id", order.BranchID).
		Set("office_id", order.OfficeID).

		// Типы и оборудование
		Set("order_type_id", order.OrderTypeID).
		Set("equipment_id", order.EquipmentID).
		Set("equipment_type_id", order.EquipmentTypeID).

		// Статистические метрики (они тоже меняются)
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

// GetUserOrderStats - сложный аналитический запрос, оставляем "как есть" (твой код был хороший)
func (r *OrderRepository) GetUserOrderStats(ctx context.Context, userID uint64, fromDate time.Time) (*types.UserOrderStats, error) {
	// Оптимизация: один проход по базе (scan index scan) вместо 5 запросов
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
