package repositories

import (
	"context"
	"fmt"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
		&order.ID, &order.Name, &order.Address, &order.DepartmentID,
		&order.StatusID, &order.PriorityID, &order.CreatorID, &order.ExecutorID,
		&order.CreatedAt, &order.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *OrderRepository) GetOrders(ctx context.Context, filter types.Filter, actor *entities.User) ([]entities.Order, uint64, error) {
	var args []interface{}
	conditions := []string{"deleted_at IS NULL"}
	placeholderID := 1

	const SuperAdminRoleName string = "Super Admin"
	const AdminRoleName string = "Admin"
	const UserRoleName string = "User"
	const ViewingAuditRoleName string = "Viewing audit"

	switch actor.RoleName {
	case SuperAdminRoleName, AdminRoleName, ViewingAuditRoleName:

	case UserRoleName:
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
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(id) FROM orders %s", whereClause)
	var totalCount uint64
	if err := r.storage.QueryRow(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("ошибка подсчета заявок: %w", err)
	}

	if totalCount == 0 {
		return []entities.Order{}, 0, nil
	}

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
		limitClause = fmt.Sprintf("LIMIT $%d OFFSET $%d", placeholderID, placeholderID+1)
		args = append(args, filter.Limit, filter.Offset)
	}
	mainQuery := fmt.Sprintf("SELECT id, name, address, department_id, status_id, priority_id, user_id, executor_id, created_at, updated_at FROM orders %s %s %s",
		whereClause, orderByClause, limitClause)
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

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return orders, totalCount, nil
}

func (r *OrderRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.storage.Begin(ctx)
}

func (r *OrderRepository) FindByID(ctx context.Context, orderID uint64) (*entities.Order, error) {
	query := `SELECT id, name, address, department_id, status_id, priority_id, user_id, executor_id, created_at, updated_at FROM orders WHERE id = $1 AND deleted_at IS NULL`
	row := r.storage.QueryRow(ctx, query, orderID)
	order, err := r.scanOrder(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return order, nil
}

func (r *OrderRepository) Create(ctx context.Context, tx pgx.Tx, order *entities.Order) (uint64, error) {
	query := `
		INSERT INTO orders 
		(name, address, department_id, otdel_id, branch_id, office_id, equipment_id, status_id, priority_id, user_id, executor_id) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) 
		RETURNING id`
	var orderID uint64
	err := tx.QueryRow(ctx, query,
		order.Name, order.Address, order.DepartmentID, order.OtdelID, order.BranchID,
		order.OfficeID, order.EquipmentID, order.StatusID, order.PriorityID, order.CreatorID, order.ExecutorID,
	).Scan(&orderID)
	if err != nil {
		return 0, err
	}
	return orderID, nil
}

func (r *OrderRepository) Update(ctx context.Context, tx pgx.Tx, order *entities.Order) error {
	query := `
		UPDATE orders 
		SET name = $1, address = $2, department_id = $3, otdel_id = $4, branch_id = $5, office_id = $6, equipment_id = $7, status_id = $8, priority_id = $9, user_id = $10, executor_id = $11, updated_at = NOW()
		WHERE id = $12`
	_, err := tx.Exec(ctx, query,
		order.Name, order.Address, order.DepartmentID, order.OtdelID, order.BranchID,
		order.OfficeID, order.EquipmentID, order.StatusID, order.PriorityID, order.CreatorID, order.ExecutorID, order.ID,
	)
	if err != nil {
		return err
	}
	return nil
}
