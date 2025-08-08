package repositories

import (
	"context"
	"errors"
	"fmt"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	orderTable           = "orders"
	orderSelectFields    = "id, name, address, department_id, status_id, priority_id, user_id, executor_id, duration::TEXT, created_at, updated_at"
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
	GetOrders(ctx context.Context, filter types.Filter, actor *entities.User) ([]entities.Order, uint64, error)
	Create(ctx context.Context, tx pgx.Tx, order *entities.Order) (uint64, error)
	Update(ctx context.Context, tx pgx.Tx, order *entities.Order) error
	DeleteOrder(ctx context.Context, orderID uint64) error
}

type OrderRepository struct {
	storage *pgxpool.Pool
}

func NewOrderRepository(storage *pgxpool.Pool) OrderRepositoryInterface {
	return &OrderRepository{
		storage: storage,
	}
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

func (r *OrderRepository) GetOrders(ctx context.Context, filter types.Filter, actor *entities.User) ([]entities.Order, uint64, error) {
	var args []interface{}
	conditions := []string{"deleted_at IS NULL"}
	placeholderID := 1

	switch actor.RoleName {
	case "Super Admin", "Admin", "Viewing audit":
	case "User":
		conditions = append(conditions, fmt.Sprintf("department_id = $%d", placeholderID))
		args = append(args, actor.DepartmentID)
		placeholderID++
	default:
		conditions = append(conditions, fmt.Sprintf("executor_id = $%d", placeholderID))
		args = append(args, actor.ID)
		placeholderID++
	}

	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", placeholderID))
		args = append(args, "%"+filter.Search+"%")
		placeholderID++
	}

	for key, value := range filter.Filter {
		if orderAllowedFilterFields[key] {
			conditions = append(conditions, fmt.Sprintf("%s = $%d", key, placeholderID))
			args = append(args, value)
			placeholderID++
		}
	}

	whereClause := "WHERE " + strings.Join(conditions, " AND ")

	countQuery := fmt.Sprintf("SELECT COUNT(id) FROM %s %s", orderTable, whereClause)
	var totalCount uint64
	if err := r.storage.QueryRow(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("ошибка подсчета заявок: %w", err)
	}

	if totalCount == 0 {
		return []entities.Order{}, 0, nil
	}

	orderByClause := "ORDER BY id DESC"

	limitClause := ""
	if filter.WithPagination {
		limitClause = fmt.Sprintf("LIMIT $%d OFFSET $%d", placeholderID, placeholderID+1)
		args = append(args, filter.Limit, filter.Offset)
	}

	mainQuery := fmt.Sprintf("SELECT %s FROM %s %s %s %s",
		orderSelectFields, orderTable, whereClause, orderByClause, limitClause)

	rows, err := r.storage.Query(ctx, mainQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка получения заявок: %w", err)
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
		order.CreatorID, order.ExecutorID, utils.StringPtrToNullString(order.Duration), order.ID,
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
