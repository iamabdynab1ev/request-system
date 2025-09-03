package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const (
	orderTable           = "orders"
	orderSelectFields    = "id, name, address, department_id, status_id, priority_id, user_id, executor_id, duration, created_at, updated_at"
	orderInsertFields    = "name, address, department_id, otdel_id, branch_id, office_id, equipment_id, status_id, priority_id, user_id, executor_id"
	orderUpdateSetClause = `name = $1, address = $2, department_id = $3, otdel_id = $4, branch_id = $5, office_id = $6, equipment_id = $7, status_id = $8, priority_id = $9, user_id = $10, executor_id = $11, duration = $12, updated_at = NOW()`
)

var orderAllowedFilterFields = map[string]bool{
	"department_id": true, "status_id": true, "priority_id": true,
	"user_id": true, "executor_id": true, "branch_id": true, "office_id": true,
}

var orderAllowedSortFields = map[string]bool{
	"id": true, "created_at": true, "updated_at": true, "priority_id": true,
}

type OrderRepositoryInterface interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)
	FindByID(ctx context.Context, orderID uint64) (*entities.Order, error)
	GetOrders(ctx context.Context, filter types.Filter, securityFilter string, securityArgs []interface{}) ([]entities.Order, uint64, error)
	Create(ctx context.Context, tx pgx.Tx, order *entities.Order) (uint64, error)
	Update(ctx context.Context, tx pgx.Tx, order *entities.Order) error
	DeleteOrder(ctx context.Context, orderID uint64) error
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
		&order.ID, &order.Name, &order.Address,
		&order.DepartmentID, &order.StatusID, &order.PriorityID,
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

func (r *OrderRepository) GetOrders(ctx context.Context, filter types.Filter, securityFilter string, securityArgs []interface{}) ([]entities.Order, uint64, error) {
	allArgs := make([]interface{}, 0)
	conditions := []string{"deleted_at IS NULL"}
	placeholderNum := 1

	// Шаг 1: Применяем фильтр безопасности (этот код мы уже исправили)
	if securityFilter != "" {
		for i := 0; i < strings.Count(securityFilter, "?"); i++ {
			securityFilter = strings.Replace(securityFilter, "?", fmt.Sprintf("$%d", placeholderNum), 1)
			placeholderNum++
		}
		conditions = append(conditions, securityFilter)
		allArgs = append(allArgs, securityArgs...)
	}

	// Шаг 2: Применяем текстовый поиск
	if filter.Search != "" {
		searchPattern := "%" + strings.ToLower(filter.Search) + "%" // поиск без учета регистра
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", placeholderNum))
		allArgs = append(allArgs, searchPattern)
		placeholderNum++
	}

	// >>> НАЧАЛО ГЛАВНЫХ ИЗМЕНЕНИЙ <<<
	// Шаг 3: Применяем фильтры из URL (status_id, branch_id и т.д.)
	for key, value := range filter.Filter {
		// Пропускаем поля, которые не разрешены для фильтрации
		if !orderAllowedFilterFields[key] {
			continue
		}

		// Используем switch-case для проверки типа значения
		switch v := value.(type) {
		case []string:
			// Этот блок работает для ?filter[branch_id]=1,2,4
			// Если пришел срез строк, генерируем SQL-оператор IN (...)
			placeholders := make([]string, len(v))
			for i, item := range v {
				placeholders[i] = fmt.Sprintf("$%d", placeholderNum)
				allArgs = append(allArgs, item) // Добавляем каждый элемент среза как отдельный аргумент
				placeholderNum++
			}
			conditions = append(conditions, fmt.Sprintf("%s IN (%s)", key, strings.Join(placeholders, ",")))

		case string:
			// Этот блок работает для ?filter[status_id]=1
			// Если пришла обычная строка, генерируем оператор =
			conditions = append(conditions, fmt.Sprintf("%s = $%d", key, placeholderNum))
			allArgs = append(allArgs, v)
			placeholderNum++

			// Можно добавить default для обработки других типов, если понадобится
		}
	}
	// >>> КОНЕЦ ГЛАВНЫХ ИЗМЕНЕНИЙ <<<

	whereClause := "WHERE " + strings.Join(conditions, " AND ")

	countQuery := fmt.Sprintf("SELECT COUNT(id) FROM %s %s", orderTable, whereClause)
	var totalCount uint64
	if err := r.storage.QueryRow(ctx, countQuery, allArgs...).Scan(&totalCount); err != nil {
		// Ошибка происходит именно здесь, поэтому логируем аргументы
		r.logger.Error("ошибка подсчета заявок", zap.Error(err), zap.String("query", countQuery), zap.Any("args", allArgs))
		return nil, 0, fmt.Errorf("ошибка подсчета заявок: %w", err)
	}

	if totalCount == 0 {
		return []entities.Order{}, 0, nil
	}

	// ... (весь остальной код функции - orderByClause, limitClause, mainQuery - остается БЕЗ ИЗМЕНЕНИЙ) ...
	orderByClause := "ORDER BY id DESC"
	if len(filter.Sort) > 0 {
		var sortParts []string
		for field, direction := range filter.Sort {
			if orderAllowedSortFields[field] {
				sortParts = append(sortParts, fmt.Sprintf("%s %s", field, direction))
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

	mainQuery := fmt.Sprintf("SELECT %s FROM %s %s %s %s", orderSelectFields, orderTable, whereClause, orderByClause, limitClause)

	rows, err := r.storage.Query(ctx, mainQuery, allArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	orders := make([]entities.Order, 0)
	for rows.Next() {
		order, err := r.scanOrder(rows)
		if err != nil {
			return nil, 0, err
		}
		orders = append(orders, *order)
	}
	return orders, totalCount, rows.Err()
}

func (r *OrderRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.storage.Begin(ctx)
}

func (r *OrderRepository) FindByID(ctx context.Context, orderID uint64) (*entities.Order, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1 AND deleted_at IS NULL", orderSelectFields, orderTable)
	row := r.storage.QueryRow(ctx, query, orderID)
	return r.scanOrder(row)
}

func (r *OrderRepository) Create(ctx context.Context, tx pgx.Tx, order *entities.Order) (uint64, error) {
	query := fmt.Sprintf(`INSERT INTO %s (%s) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id`,
		orderTable, orderInsertFields)
	var orderID uint64
	err := tx.QueryRow(ctx, query,
		order.Name, order.Address, order.DepartmentID, order.OtdelID, order.BranchID,
		order.OfficeID, order.EquipmentID, order.StatusID, order.PriorityID,
		order.CreatorID, order.ExecutorID,
	).Scan(&orderID)
	return orderID, err
}

func (r *OrderRepository) Update(ctx context.Context, tx pgx.Tx, order *entities.Order) error {
	query := fmt.Sprintf(`UPDATE %s SET %s WHERE id = $13 AND deleted_at IS NULL`, orderTable, orderUpdateSetClause)
	_, err := tx.Exec(ctx, query,
		order.Name, order.Address, order.DepartmentID, order.OtdelID, order.BranchID,
		order.OfficeID, order.EquipmentID, order.StatusID, order.PriorityID,
		order.CreatorID, order.ExecutorID, order.Duration, order.ID,
	)
	return err
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
