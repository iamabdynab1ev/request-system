package repositories

import (
	"context"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OrderRepositoryInterface отвечает ИСКЛЮЧИТЕЛЬНО за операции с сущностью Order.
type OrderRepositoryInterface interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)
	FindByID(ctx context.Context, orderID uint64) (*entities.Order, error)
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

func (r *OrderRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.storage.Begin(ctx)
}

func (r *OrderRepository) FindByID(ctx context.Context, orderID uint64) (*entities.Order, error) {
	query := `SELECT id, name, address, department_id, status_id, priority_id, user_id, executor_id, created_at, updated_at FROM orders WHERE id = $1 AND deleted_at IS NULL`
	var order entities.Order
	err := r.storage.QueryRow(ctx, query, orderID).Scan(
		&order.ID, &order.Name, &order.Address, &order.DepartmentID,
		&order.StatusID, &order.PriorityID, &order.CreatorID, &order.ExecutorID,
		&order.CreatedAt, &order.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &order, nil
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
		SET name = $1, address = $2, department_id = $3, otdel_id = $4, branch_id = $5, office_id = $6, equipment_id = $7, status_id = $8, priority_id = $9, user_id = $10, executor_id = $11
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
