package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
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
	GetOrders(ctx context.Context, filter types.Filter, securityFilter string, securityArgs []interface{}) ([]entities.Order, uint64, error)
	Update(ctx context.Context, tx pgx.Tx, order *entities.Order) error
	DeleteOrder(ctx context.Context, orderID uint64) error
	CountOrdersByOtdelID(ctx context.Context, id uint64) (uint64, error)
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

func (r *OrderRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.storage.Begin(ctx)
}

func (r *OrderRepository) FindByID(ctx context.Context, orderID uint64) (*entities.Order, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1 AND deleted_at IS NULL", orderFields, orderTable)
	row := r.storage.QueryRow(ctx, query, orderID)
	return r.scanOrder(row)
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

	sql, args, err := builder.ToSql()
	if err != nil {
		r.logger.Error("Ошибка сборки UPDATE запроса для заявки", zap.Error(err))
		return fmt.Errorf("ошибка сборки UPDATE запроса для заявки: %w", err)
	}
	result, err := tx.Exec(ctx, sql, args...)
	if err != nil {
		r.logger.Error("Ошибка выполнения UPDATE запроса", zap.Error(err), zap.String("sql", sql), zap.Any("args", args))
		return err
	}
	if result.RowsAffected() == 0 {
		r.logger.Warn("Заявка не найдена для обновления", zap.Uint64("orderID", order.ID))
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

func (r *OrderRepository) GetOrders(ctx context.Context, filter types.Filter, securityFilter string, securityArgs []interface{}) ([]entities.Order, uint64, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

	selectBuilder := psql.Select(orderFields).From(orderTable + " AS o").Where(sq.Eq{"o.deleted_at": nil})
	countBuilder := psql.Select("COUNT(o.id)").From(orderTable + " AS o").Where(sq.Eq{"o.deleted_at": nil})

	// Применяем securityFilter с правильными placeholders
	if securityFilter != "" {
		// Заменяем ? на $1, $2 и т.д.
		placeholderCount := len(securityArgs)
		newSecurityFilter := securityFilter
		for i := 0; i < placeholderCount; i++ {
			newSecurityFilter = strings.Replace(newSecurityFilter, "?", fmt.Sprintf("$%d", i+1), 1)
		}
		selectBuilder = selectBuilder.Where(newSecurityFilter, securityArgs...)
		countBuilder = countBuilder.Where(newSecurityFilter, securityArgs...)
	}

	for key, value := range filter.Filter {
		clause := sq.Eq{"o." + key: value}
		selectBuilder = selectBuilder.Where(clause)
		countBuilder = countBuilder.Where(clause)
	}

	if filter.Search != "" {
		clause := sq.ILike{"o.name": "%" + filter.Search + "%"}
		selectBuilder = selectBuilder.Where(clause)
		countBuilder = countBuilder.Where(clause)
	}

	countQuery, countArgs, err := countBuilder.ToSql()
	if err != nil {
		r.logger.Error("GetOrders: ошибка сборки запроса подсчета", zap.Error(err))
		return nil, 0, err
	}

	var totalCount uint64
	if err := r.storage.QueryRow(ctx, countQuery, countArgs...).Scan(&totalCount); err != nil {
		r.logger.Error("Ошибка подсчета заявок", zap.Error(err), zap.String("query", countQuery), zap.Any("args", countArgs))
		return nil, 0, err
	}
	if totalCount == 0 {
		r.logger.Info("Заявки не найдены", zap.Any("filter", filter))
		return []entities.Order{}, 0, nil
	}

	selectBuilder = selectBuilder.OrderBy("o.created_at DESC")
	if filter.WithPagination {
		selectBuilder = selectBuilder.Limit(uint64(filter.Limit)).Offset(uint64(filter.Offset))
	}

	mainQuery, mainArgs, err := selectBuilder.ToSql()
	if err != nil {
		r.logger.Error("GetOrders: ошибка сборки основного запроса", zap.Error(err))
		return nil, 0, err
	}

	r.logger.Debug("Выполнение запроса GetOrders", zap.String("query", mainQuery), zap.Any("args", mainArgs))
	rows, err := r.storage.Query(ctx, mainQuery, mainArgs...)
	if err != nil {
		r.logger.Error("Ошибка получения списка заявок", zap.Error(err), zap.String("query", mainQuery), zap.Any("args", mainArgs))
		return nil, 0, err
	}
	defer rows.Close()

	orders := make([]entities.Order, 0, filter.Limit)
	for rows.Next() {
		order, err := r.scanOrder(rows)
		if err != nil {
			r.logger.Error("Ошибка сканирования строки", zap.Error(err))
			return nil, 0, err
		}
		orders = append(orders, *order)
	}
	if err := rows.Err(); err != nil {
		r.logger.Error("Ошибка итерации строк", zap.Error(err))
		return nil, 0, err
	}

	r.logger.Info("Заявки получены", zap.Int("count", len(orders)), zap.Uint64("total_count", totalCount))
	return orders, totalCount, nil
}
