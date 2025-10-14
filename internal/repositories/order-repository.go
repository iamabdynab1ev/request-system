// Файл: internal/repositories/order_repository.go
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils"

	"github.com/Masterminds/squirrel"
	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const (
	orderTable           = "orders"
	orderFieldsWithAlias = "o.id, o.name, o.address, o.department_id, o.otdel_id, o.branch_id, o.office_id, o.equipment_id, o.equipment_type_id, o.order_type_id, o.status_id, o.priority_id, o.user_id, o.executor_id, o.duration, o.created_at, o.updated_at"
	orderInsertFields    = "name, address, department_id, otdel_id, branch_id, office_id, equipment_id, equipment_type_id, order_type_id, status_id, priority_id, user_id, executor_id, duration"
)

type OrderRepositoryInterface interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)
	FindByID(ctx context.Context, orderID uint64) (*entities.Order, error)
	GetOrders(ctx context.Context, filter types.Filter, securityFilter string, securityArgs []interface{}) ([]entities.Order, uint64, error)
	Create(ctx context.Context, tx pgx.Tx, order *entities.Order) (uint64, error)
	Update(ctx context.Context, tx pgx.Tx, order *entities.Order) error
	DeleteOrder(ctx context.Context, orderID uint64) error
	CountOrdersByOtdelID(ctx context.Context, id uint64) (int, error)
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
	var (
		otdelID, branchID, officeID         sql.NullInt64
		equipmentID, equipmentTypeID        sql.NullInt64
		executorID, priorityID, orderTypeID sql.NullInt64
		address                             sql.NullString
		duration                            sql.NullTime
	)

	err := row.Scan(
		&order.ID, &order.Name, &address, &order.DepartmentID, &otdelID, &branchID, &officeID,
		&equipmentID, &equipmentTypeID, &orderTypeID, &order.StatusID, &priorityID,
		&order.CreatorID, &executorID, &duration,
		&order.CreatedAt, &order.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	order.Address = utils.NullStringToStrPtr(address)
	order.OtdelID = utils.NullInt64ToUint64Ptr(otdelID)
	order.BranchID = utils.NullInt64ToUint64Ptr(branchID)
	order.OfficeID = utils.NullInt64ToUint64Ptr(officeID)
	order.EquipmentID = utils.NullInt64ToUint64Ptr(equipmentID)
	order.EquipmentTypeID = utils.NullInt64ToUint64Ptr(equipmentTypeID)
	order.ExecutorID = utils.NullInt64ToUint64Ptr(executorID)
	order.PriorityID = utils.NullInt64ToUint64Ptr(priorityID)

	order.OrderTypeID = utils.NullInt64ToUint64(orderTypeID)
	if duration.Valid {
		order.Duration = &duration.Time
	}

	return &order, nil
}

func (r *OrderRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.storage.Begin(ctx)
}

func (r *OrderRepository) FindByID(ctx context.Context, orderID uint64) (*entities.Order, error) {
	query := fmt.Sprintf("SELECT %s FROM %s o WHERE o.id = $1 AND o.deleted_at IS NULL", orderFieldsWithAlias, orderTable)
	row := r.storage.QueryRow(ctx, query, orderID)
	return r.scanOrder(row)
}

func (r *OrderRepository) Create(ctx context.Context, tx pgx.Tx, order *entities.Order) (uint64, error) {
	query := fmt.Sprintf(`INSERT INTO %s (%s) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14) RETURNING id`,
		orderTable, orderInsertFields)

	var orderID uint64
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

	return orderID, err
}

func (r *OrderRepository) Update(ctx context.Context, tx pgx.Tx, order *entities.Order) error {
	builder := squirrel.Update(orderTable).
		PlaceholderFormat(squirrel.Dollar).
		Set("updated_at", squirrel.Expr("NOW()")).
		Where(squirrel.Eq{"id": order.ID, "deleted_at": nil})

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
		return fmt.Errorf("ошибка сборки UPDATE запроса для заявки: %w", err)
	}

	result, err := tx.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}

	return nil
}

func (r *OrderRepository) DeleteOrder(ctx context.Context, orderID uint64) error {
	query := `UPDATE orders SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	result, err := r.storage.Exec(ctx, query, orderID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *OrderRepository) CountOrdersByOtdelID(ctx context.Context, id uint64) (int, error) {
	var count int
	query := "SELECT COUNT(id) FROM orders WHERE otdel_id = $1 AND deleted_at IS NULL"
	err := r.storage.QueryRow(ctx, query, id).Scan(&count)
	if err != nil {
		r.logger.Error("ошибка подсчета заявок в отделе", zap.Uint64("otdelID", id), zap.Error(err))
		return 0, err
	}
	return count, nil
}

// ЗАМЕНИ СТАРЫЙ МЕТОД ЭТИМ УЛУЧШЕННЫМ ВАРИАНТОМ
func (r *OrderRepository) GetOrders(ctx context.Context, filter types.Filter, securityFilter string, securityArgs []interface{}) ([]entities.Order, uint64, error) {
	baseQuery := sq.Select(
		"o.id, o.name, o.address, o.department_id, o.otdel_id, o.branch_id, o.office_id, o.equipment_id, " +
			"o.equipment_type_id, o.order_type_id, o.status_id, o.priority_id, o.user_id, o.executor_id, " +
			"o.duration, o.created_at, o.updated_at, " +
			"s.name AS status_name, p.name AS priority_name, " +
			"creator.fio AS creator_fio, executor.fio AS executor_fio",
	).
		From("orders o").
		LeftJoin("users creator ON o.user_id = creator.id").
		LeftJoin("users executor ON o.executor_id = executor.id").
		LeftJoin("statuses s ON o.status_id = s.id").
		LeftJoin("priorities p ON o.priority_id = p.id").
		Where(sq.Eq{"o.deleted_at": nil}).
		PlaceholderFormat(sq.Dollar)

	if securityFilter != "" {
		baseQuery = baseQuery.Where(securityFilter, securityArgs...)
	}

	// --- Фильтры ---
	for key, value := range filter.Filter {
		switch key {
		case "department_id", "status_id", "priority_id", "user_id", "executor_id", "branch_id", "office_id":
			baseQuery = baseQuery.Where(sq.Eq{"o." + key: value})
		}
	}

	// --- Поиск ---
	if filter.Search != "" {
		searchPattern := "%" + strings.ToLower(filter.Search) + "%"
		baseQuery = baseQuery.Where(sq.Or{
			sq.ILike{"o.name": searchPattern},
			sq.ILike{"creator.fio": searchPattern},
			sq.ILike{"executor.fio": searchPattern},
		})
	}

	// --- Подсчёт totalCount ---
	countBuilder := sq.Select("COUNT(o.id)").From("orders o").
		LeftJoin("users creator ON o.user_id = creator.id").
		LeftJoin("users executor ON o.executor_id = executor.id").
		LeftJoin("statuses s ON o.status_id = s.id").
		LeftJoin("priorities p ON o.priority_id = p.id").
		Where(sq.Eq{"o.deleted_at": nil}).
		PlaceholderFormat(sq.Dollar)

	if securityFilter != "" {
		countBuilder = countBuilder.Where(securityFilter, securityArgs...)
	}
	for key, value := range filter.Filter {
		switch key {
		case "department_id", "status_id", "priority_id", "user_id", "executor_id", "branch_id", "office_id":
			countBuilder = countBuilder.Where(sq.Eq{"o." + key: value})
		}
	}
	if filter.Search != "" {
		searchPattern := "%" + strings.ToLower(filter.Search) + "%"
		countBuilder = countBuilder.Where(sq.Or{
			sq.ILike{"o.name": searchPattern},
			sq.ILike{"creator.fio": searchPattern},
			sq.ILike{"executor.fio": searchPattern},
		})
	}

	countQuery, countArgs, err := countBuilder.ToSql()
	if err != nil {
		r.logger.Error("GetOrders: ошибка сборки запроса подсчета", zap.Error(err))
		return nil, 0, err
	}

	var totalCount uint64
	if err := r.storage.QueryRow(ctx, countQuery, countArgs...).Scan(&totalCount); err != nil {
		r.logger.Error("ошибка подсчета заявок", zap.Error(err), zap.String("query", countQuery), zap.Any("args", countArgs))
		return nil, 0, err
	}
	if totalCount == 0 {
		return []entities.Order{}, 0, nil
	}

	// --- Сортировка ---
	orderByClause := "o.id DESC"
	if len(filter.Sort) > 0 {
		var sortParts []string
		for field, direction := range filter.Sort {
			dir := "ASC"
			if strings.ToUpper(direction) == "DESC" {
				dir = "DESC"
			}
			switch field {
			case "status_name":
				sortParts = append(sortParts, fmt.Sprintf("s.name %s", dir))
			case "priority_name":
				sortParts = append(sortParts, fmt.Sprintf("p.name %s", dir))
			case "created_at", "updated_at", "duration", "id":
				sortParts = append(sortParts, fmt.Sprintf("o.%s %s", field, dir))
			}
		}
		if len(sortParts) > 0 {
			orderByClause = strings.Join(sortParts, ", ")
		}
	}
	baseQuery = baseQuery.OrderBy(orderByClause)

	// --- Пагинация ---
	if filter.WithPagination {
		baseQuery = baseQuery.Limit(uint64(filter.Limit)).Offset(uint64(filter.Offset))
	}

	mainQuery, mainArgs, err := baseQuery.ToSql()
	if err != nil {
		r.logger.Error("GetOrders: ошибка сборки основного запроса", zap.Error(err))
		return nil, 0, err
	}

	rows, err := r.storage.Query(ctx, mainQuery, mainArgs...)
	if err != nil {
		r.logger.Error("ошибка получения списка заявок", zap.Error(err), zap.String("query", mainQuery), zap.Any("args", mainArgs))
		return nil, 0, err
	}
	fields := rows.FieldDescriptions()
	cols := make([]string, len(fields))
	for i, fd := range fields {
		cols[i] = string(fd.Name)
	}

	r.logger.Info("Колонки из SELECT:", zap.Strings("cols", cols))
	r.logger.Info("Count колонок:", zap.Int("count", len(cols)))
	defer rows.Close()

	orders := make([]entities.Order, 0, filter.Limit)
	for rows.Next() {
		order, err := r.scanOrderWithRelations(rows)
		if err != nil {
			return nil, 0, err
		}
		orders = append(orders, *order)
	}
	return orders, totalCount, rows.Err()
}

func (r *OrderRepository) scanOrderWithRelations(row pgx.Row) (*entities.Order, error) {
	var order entities.Order
	var (
		otdelID, branchID, officeID         sql.NullInt64
		equipmentID, equipmentTypeID        sql.NullInt64
		executorID, priorityID, orderTypeID sql.NullInt64
		address                             sql.NullString
		duration                            sql.NullTime
	)

	err := row.Scan(
		// Старые 17 полей
		&order.ID, &order.Name, &address, &order.DepartmentID, &otdelID, &branchID, &officeID,
		&equipmentID, &equipmentTypeID, &orderTypeID, &order.StatusID, &priorityID,
		&order.CreatorID, &executorID, &duration,
		&order.CreatedAt, &order.UpdatedAt,
		// Новые 4 поля из JOIN'ов
		&order.StatusName, &order.PriorityName, &order.CreatorFIO, &order.ExecutorFIO,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}

	// Код конвертации NULL-типов
	order.Address = utils.NullStringToStrPtr(address)
	order.OtdelID = utils.NullInt64ToUint64Ptr(otdelID)
	order.BranchID = utils.NullInt64ToUint64Ptr(branchID)
	order.OfficeID = utils.NullInt64ToUint64Ptr(officeID)
	order.EquipmentID = utils.NullInt64ToUint64Ptr(equipmentID)
	order.EquipmentTypeID = utils.NullInt64ToUint64Ptr(equipmentTypeID)
	order.ExecutorID = utils.NullInt64ToUint64Ptr(executorID)
	order.PriorityID = utils.NullInt64ToUint64Ptr(priorityID)
	order.OrderTypeID = utils.NullInt64ToUint64(orderTypeID)
	if duration.Valid {
		order.Duration = &duration.Time
	}

	return &order, nil
}
