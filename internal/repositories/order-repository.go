package repositories

import (
	"context"
	"fmt"
	
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"request-system/internal/entities"

	"request-system/internal/infrastructure/bd"
	
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
)

const (
	orderTable = "orders"
)

var orderMap = map[string]string{
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
	"address":       "o.address",
	"duration":          "o.duration",
	"equipment_id":      "o.equipment_id",
	"equipment_type_id": "o.equipment_type_id",
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

func (r *OrderRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.storage.Begin(ctx)
}

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
	queryBuilder := r.buildOrderSelectQuery().Where(sq.Eq{"o.id": orderID, "o.deleted_at": nil})

	sqlStr, args, err := queryBuilder.ToSql()
	if err != nil { return nil, fmt.Errorf("FindByID SQL error: %w", err) }

	rows, err := r.storage.Query(ctx, sqlStr, args...)
	if err != nil { return nil, err }
	defer rows.Close()

	order, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.Order])
	if err != nil {
		if err == pgx.ErrNoRows { return nil, apperrors.ErrNotFound }
		return nil, err
	}
	return &order, nil
}

// -------------------------------------------------------------
// GetOrders - МАКСИМАЛЬНАЯ ОПТИМИЗАЦИЯ
// -------------------------------------------------------------
func (r *OrderRepository) GetOrders(ctx context.Context, filter types.Filter, securityCondition sq.Sqlizer) ([]entities.Order, uint64, error) {
	// 1. ХЕЛПЕРЫ ДЛЯ СЛОЖНЫХ УСЛОВИЙ
	
	// Поиск по тексту (ИЛИ по названию, ИЛИ по адресу)
	applySearch := func(b sq.SelectBuilder) sq.SelectBuilder {
		if filter.Search != "" {
			match := "%" + filter.Search + "%"
			return b.Where(sq.Or{
				sq.ILike{"o.name": match},
				sq.ILike{"o.address": match},
			})
		}
		return b
	}

	// Специфические фильтры (даты, просрочки)
	applySpecialFilters := func(b sq.SelectBuilder) sq.SelectBuilder {
		// Даты дедлайна
		if dFrom, ok := filter.Filter["duration_from"]; ok {
			b = b.Where(sq.GtOrEq{"o.duration": dFrom})
		}
		if dTo, ok := filter.Filter["duration_to"]; ok {
			b = b.Where(sq.LtOrEq{"o.duration": dTo})
		}

		// Просроченные заявки
		if val, ok := filter.Filter["overdue"]; ok {
			if valStr, _ := val.(string); valStr == "true" {
				// ВАЖНО: Джойн уже есть в countBuilder/selectBuilder или его надо добавить
				// Для надежности добавляем EXISTS (подзапрос), чтобы не дублировать джойны
				// или предполагаем, что если мы джойним статусы - это безопасно.
				// Тут лучше сделать явно.
				b = b.Join("statuses s_ovr ON o.status_id = s_ovr.id").
					Where("o.duration < NOW()").
					Where("s_ovr.code NOT IN ('CLOSED', 'COMPLETED', 'REJECTED')")
			}
		}
		
		// Очистка спец. фильтров из map, чтобы Helper не пытался применить их как простые равенства
		delete(filter.Filter, "duration_from")
		delete(filter.Filter, "duration_to")
		delete(filter.Filter, "overdue")

		return b
	}

	// --------------------------------------------------------
	// 2. ВЫПОЛНЕНИЕ COUNT (ОБЩЕЕ КОЛИЧЕСТВО)
	// --------------------------------------------------------
	countBuilder := sq.Select("count(o.id)").
		From(orderTable + " o").
		Where(sq.Eq{"o.deleted_at": nil}).
		PlaceholderFormat(sq.Dollar)

	// Security
	if securityCondition != nil {
		countBuilder = countBuilder.Where(securityCondition)
	}

	// Search & Specials
	countBuilder = applySearch(countBuilder)
	countBuilder = applySpecialFilters(countBuilder) 
	// ВАЖНО: После этого вызова filter.Filter УЖЕ ОЧИЩЕН от duration/overdue, 
	// так что можно смело передавать его дальше в хелпер.

	// Helper (branch_id=1,2, priority_id=...)
	countFilter := filter
	countFilter.WithPagination = false
	countFilter.Sort = nil
	countBuilder = bd.ApplyListParams(countBuilder, countFilter, orderMap)

	// Execute Count
	countSql, countArgs, err := countBuilder.ToSql()
	if err != nil { return nil, 0, err }

	var totalCount uint64
	if err := r.storage.QueryRow(ctx, countSql, countArgs...).Scan(&totalCount); err != nil {
		return nil, 0, err
	}
	if totalCount == 0 {
		return []entities.Order{}, 0, nil
	}

	// --------------------------------------------------------
	// 3. ВЫПОЛНЕНИЕ SELECT (СПИСОК)
	// --------------------------------------------------------
	selectBuilder := r.buildOrderSelectQuery(). // Внутри джойны users creator/executor
		Where(sq.Eq{"o.deleted_at": nil})

	// Security
	if securityCondition != nil {
		selectBuilder = selectBuilder.Where(securityCondition)
	}
	
	// Search
	selectBuilder = applySearch(selectBuilder)
	
	return r.getOrdersRefactored(ctx, filter, securityCondition)
}

func (r *OrderRepository) getOrdersRefactored(ctx context.Context, filter types.Filter, securityCondition sq.Sqlizer) ([]entities.Order, uint64, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

	// 🔥 ИЗВЛЕКАЕМ ВСЕ СПЕЦИАЛЬНЫЕ ФИЛЬТРЫ
	durationFrom, _ := filter.Filter["duration_from"]
	durationTo, _ := filter.Filter["duration_to"]
	createdFrom, _ := filter.Filter["created_from"]   // 🔥 ДОБАВЛЕНО
	createdTo, _ := filter.Filter["created_to"]       // 🔥 ДОБАВЛЕНО
	overdueVal, _ := filter.Filter["overdue"]
	
	// 🔥 УДАЛЯЕМ ИХ ИЗ MAP
	delete(filter.Filter, "duration_from")
	delete(filter.Filter, "duration_to")
	delete(filter.Filter, "created_from")   // 🔥 ДОБАВЛЕНО
	delete(filter.Filter, "created_to")     // 🔥 ДОБАВЛЕНО
	delete(filter.Filter, "overdue")

	// 🔥 ФУНКЦИЯ ПРИМЕНЕНИЯ СПЕЦИАЛЬНЫХ ФИЛЬТРОВ
	applySpecials := func(b sq.SelectBuilder) sq.SelectBuilder {
		// Duration фильтры (срок выполнения)
		if durationFrom != nil {
			b = b.Where(sq.GtOrEq{"o.duration": durationFrom})
		}
		if durationTo != nil {
			b = b.Where(sq.LtOrEq{"o.duration": durationTo})
		}
		
		// 🔥 НОВОЕ: Created фильтры (дата создания)
		if createdFrom != nil {
			b = b.Where(sq.GtOrEq{"o.created_at": createdFrom})
		}
		if createdTo != nil {
			b = b.Where(sq.LtOrEq{"o.created_at": createdTo})
		}
		
		// Просроченные
		if overdueVal != nil {
			if s, ok := overdueVal.(string); ok && s == "true" {
				b = b.Join("statuses s_ovr ON o.status_id = s_ovr.id").
					Where("o.duration < NOW()").
					Where("s_ovr.code NOT IN ('CLOSED', 'COMPLETED', 'REJECTED')")
			}
		}
		return b
	}

	applySearch := func(b sq.SelectBuilder) sq.SelectBuilder {
		if filter.Search != "" {
			match := "%" + filter.Search + "%"
			return b.Where(sq.Or{
				sq.ILike{"o.name": match},
				sq.ILike{"o.address": match},
			})
		}
		return b
	}

	// COUNT
	countBuilder := psql.Select("count(o.id)").From(orderTable + " o").Where(sq.Eq{"o.deleted_at": nil})
	
	if securityCondition != nil { countBuilder = countBuilder.Where(securityCondition) }
	
	countBuilder = applySearch(countBuilder)
	countBuilder = applySpecials(countBuilder)
	
	countFilter := filter
	countFilter.WithPagination = false
	countFilter.Sort = nil

	countBuilder = bd.ApplyListParams(countBuilder, countFilter, orderMap)
	
	var totalCount uint64
	sqlCount, argsCount, _ := countBuilder.ToSql()
	if err := r.storage.QueryRow(ctx, sqlCount, argsCount...).Scan(&totalCount); err != nil {
		return nil, 0, err
	}
	if totalCount == 0 {
		return []entities.Order{}, 0, nil
	}

	// SELECT
	selectBuilder := r.buildOrderSelectQuery().Where(sq.Eq{"o.deleted_at": nil})

	if securityCondition != nil { selectBuilder = selectBuilder.Where(securityCondition) }

	selectBuilder = applySearch(selectBuilder)
	selectBuilder = applySpecials(selectBuilder)
	
	if len(filter.Sort) == 0 {
		selectBuilder = selectBuilder.OrderBy("o.created_at DESC")
	}

	selectBuilder = bd.ApplyListParams(selectBuilder, filter, orderMap)

	sqlSelect, argsSelect, err := selectBuilder.ToSql()
	if err != nil { return nil, 0, err }

	rows, err := r.storage.Query(ctx, sqlSelect, argsSelect...)
	if err != nil { return nil, 0, err }
	defer rows.Close()

	orders, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.Order])
	if err != nil { return nil, 0, err }
	
	return orders, totalCount, nil
}

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
	b := sq.Update(orderTable).PlaceholderFormat(sq.Dollar).
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
	if err != nil { return err }

	_, err = tx.Exec(ctx, sqlStr, args...)
	return err
}

func (r *OrderRepository) DeleteOrder(ctx context.Context, orderID uint64) error {
	query := `UPDATE orders SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	cmd, err := r.storage.Exec(ctx, query, orderID)
	if err != nil { return err }
	if cmd.RowsAffected() == 0 { return apperrors.ErrNotFound }
	return nil
}

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
	if err != nil { return nil, err }
	stats.TotalCount = stats.InProgressCount + stats.CompletedCount + stats.ClosedCount + stats.OverdueCount
	return &stats, nil
}
