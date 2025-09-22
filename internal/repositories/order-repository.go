// Файл: internal/repositories/order_repository.go
package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// --- Глобальные переменные ---

const (
	orderTable           = "orders"
	orderFieldsWithAlias = "o.id, o.name, o.address, o.department_id, o.otdel_id, o.branch_id, o.office_id, o.equipment_id, o.equipment_type_id, o.status_id, o.priority_id, o.user_id, o.executor_id, o.duration, o.created_at, o.updated_at"
	orderInsertFields    = "name, address, department_id, otdel_id, branch_id, office_id, equipment_id, equipment_type_id, status_id, priority_id, user_id, executor_id, duration"
)

var (
	orderAllowedFilterFields = map[string]bool{
		"department_id": true, "status_id": true, "priority_id": true,
		"user_id": true, "executor_id": true, "branch_id": true, "office_id": true,
	}
	orderAllowedSortFields = map[string]bool{
		"id": true, "created_at": true, "updated_at": true, "priority_id": true, "status_id": true, "duration": true,
	}
)

// --- Интерфейс и Структура (без изменений) ---

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

// --- CRUD Методы ---

// ИЗМЕНЕН scanOrder: Добавлено сканирование `EquipmentTypeID`
func (r *OrderRepository) scanOrder(row pgx.Row) (*entities.Order, error) {
	var order entities.Order
	err := row.Scan(
		&order.ID, &order.Name, &order.Address,
		&order.DepartmentID, &order.OtdelID, &order.BranchID, &order.OfficeID,
		&order.EquipmentID, &order.EquipmentTypeID, // <- Добавлено новое поле
		&order.StatusID, &order.PriorityID,
		&order.CreatorID, &order.ExecutorID, &order.Duration,
		&order.CreatedAt, &order.UpdatedAt,
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

// ИЗМЕНЕН FindByID: Использует правильную константу с алиасом
func (r *OrderRepository) FindByID(ctx context.Context, orderID uint64) (*entities.Order, error) {
	query := fmt.Sprintf("SELECT %s FROM %s o WHERE o.id = $1 AND o.deleted_at IS NULL", orderFieldsWithAlias, orderTable)
	row := r.storage.QueryRow(ctx, query, orderID)
	return r.scanOrder(row)
}

// ИЗМЕНЕН Create: Добавлено поле `equipment_type_id` в INSERT
func (r *OrderRepository) Create(ctx context.Context, tx pgx.Tx, order *entities.Order) (uint64, error) {
	// Теперь у нас 13 полей для вставки, и 13 плейсхолдеров ($1...$13)
	query := fmt.Sprintf(`INSERT INTO %s (%s) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) RETURNING id`,
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
		order.StatusID,
		order.PriorityID,
		order.CreatorID,
		order.ExecutorID,
		order.Duration,
	).Scan(&orderID)

	return orderID, err
}

// ИЗМЕНЕН Update: ПЕРЕПИСАН на динамическую сборку, чтобы не обновлять лишние поля.
// Это самый надежный способ.
func (r *OrderRepository) Update(ctx context.Context, tx pgx.Tx, order *entities.Order) error {
	builder := squirrel.Update(orderTable).
		PlaceholderFormat(squirrel.Dollar).
		Set("updated_at", squirrel.Expr("NOW()")).
		Where(squirrel.Eq{"id": order.ID, "deleted_at": nil})

	// Динамически добавляем поля, которые пришли из сервиса.
	// Сервис сам решает, что нужно обновлять, а репозиторий просто сохраняет.
	builder = builder.Set("name", order.Name)
	builder = builder.Set("address", order.Address)
	builder = builder.Set("department_id", order.DepartmentID)
	builder = builder.Set("otdel_id", order.OtdelID)
	builder = builder.Set("branch_id", order.BranchID)
	builder = builder.Set("office_id", order.OfficeID)
	builder = builder.Set("equipment_id", order.EquipmentID)
	builder = builder.Set("equipment_type_id", order.EquipmentTypeID) // <- Добавлено
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

// DeleteOrder и CountOrdersByOtdelID без изменений
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

// --- GetOrders (остается таким же, так как `orderFieldsWithAlias` мы обновили) ---

func (r *OrderRepository) GetOrders(ctx context.Context, filter types.Filter, securityFilter string, securityArgs []interface{}) ([]entities.Order, uint64, error) {
	allArgs := make([]interface{}, 0)
	conditions := []string{"o.deleted_at IS NULL"}
	placeholderNum := 1

	if securityFilter != "" {
		allArgs = append(allArgs, securityArgs...)
		tempFilter := securityFilter
		tempFilter = strings.ReplaceAll(tempFilter, "department_id", "o.department_id")
		tempFilter = strings.ReplaceAll(tempFilter, "user_id", "o.user_id")
		tempFilter = strings.ReplaceAll(tempFilter, "executor_id", "o.executor_id")
		for i := 0; i < len(securityArgs); i++ {
			tempFilter = strings.Replace(tempFilter, "?", fmt.Sprintf("$%d", placeholderNum), 1)
			placeholderNum++
		}
		conditions = append(conditions, tempFilter)
	}

	if filter.Search != "" {
		searchPattern := "%" + strings.ToLower(filter.Search) + "%"
		searchCondition := fmt.Sprintf("(o.name ILIKE $%d OR creator.fio ILIKE $%d OR executor.fio ILIKE $%d)", placeholderNum, placeholderNum+1, placeholderNum+2)
		conditions = append(conditions, searchCondition)
		allArgs = append(allArgs, searchPattern, searchPattern, searchPattern)
		placeholderNum += 3
	}

	for key, value := range filter.Filter {
		if !orderAllowedFilterFields[key] {
			continue
		}
		var values []string
		switch v := value.(type) {
		case []string:
			values = v
		case string:
			values = strings.Split(v, ",")
		default:
			continue
		}
		if len(values) == 0 {
			continue
		}
		dbField := "o." + key
		if len(values) == 1 {
			conditions = append(conditions, fmt.Sprintf("%s = $%d", dbField, placeholderNum))
			allArgs = append(allArgs, values[0])
			placeholderNum++
		} else {
			placeholders := make([]string, len(values))
			for i, item := range values {
				placeholders[i] = fmt.Sprintf("$%d", placeholderNum)
				allArgs = append(allArgs, item)
				placeholderNum++
			}
			conditions = append(conditions, fmt.Sprintf("%s IN (%s)", dbField, strings.Join(placeholders, ",")))
		}
	}

	whereClause := "WHERE " + strings.Join(conditions, " AND ")
	fromClause := `FROM orders o LEFT JOIN users creator ON o.user_id = creator.id LEFT JOIN users executor ON o.executor_id = executor.id`

	countQuery := fmt.Sprintf("SELECT COUNT(o.id) %s %s", fromClause, whereClause)
	var totalCount uint64
	if err := r.storage.QueryRow(ctx, countQuery, allArgs...).Scan(&totalCount); err != nil {
		r.logger.Error("ошибка подсчета заявок", zap.Error(err), zap.String("query", countQuery), zap.Any("args", allArgs))
		return nil, 0, err
	}
	if totalCount == 0 {
		return []entities.Order{}, 0, nil
	}

	orderByClause := "ORDER BY o.id DESC"
	if len(filter.Sort) > 0 {
		var sortParts []string
		for field, direction := range filter.Sort {
			if orderAllowedSortFields[field] {
				sortParts = append(sortParts, fmt.Sprintf("o.%s %s", field, direction))
			}
		}
		if len(sortParts) > 0 {
			orderByClause = "ORDER BY " + strings.Join(sortParts, ", ")
		}
	}

	limitClause := ""
	if filter.WithPagination {
		limitClause = fmt.Sprintf("LIMIT $%d OFFSET $%d", placeholderNum, placeholderNum+1)
		allArgs = append(allArgs, filter.Limit, filter.Offset)
	}

	mainQuery := fmt.Sprintf("SELECT %s %s %s %s %s", orderFieldsWithAlias, fromClause, whereClause, orderByClause, limitClause)
	rows, err := r.storage.Query(ctx, mainQuery, allArgs...)
	if err != nil {
		r.logger.Error("ошибка получения списка заявок", zap.Error(err), zap.String("query", mainQuery), zap.Any("args", allArgs))
		return nil, 0, err
	}
	defer rows.Close()

	orders := make([]entities.Order, 0, filter.Limit)
	for rows.Next() {
		order, err := r.scanOrder(rows)
		if err != nil {
			return nil, 0, err
		}
		orders = append(orders, *order)
	}
	return orders, totalCount, rows.Err()
}
