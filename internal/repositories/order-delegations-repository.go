package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"request-system/internal/dto"
	"request-system/pkg/contextkeys"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderDelegationRepositoryInterface interface {
	CreateOrderDelegation(ctx context.Context, payload dto.CreateOrderDelegationDTO) (int, error)
	CreateOrderDelegationInTx(ctx context.Context, tx pgx.Tx, delegatorID int, dto dto.CreateOrderDelegationDTO) error
	GetOrderDelegations(ctx context.Context, limit uint64, offset uint64) ([]dto.OrderDelegationDTO, uint64, error)
	FindOrderDelegation(ctx context.Context, id uint64) (*dto.OrderDelegationDTO, error)
	DeleteOrderDelegation(ctx context.Context, id uint64) error
}

type OrderDelegationRepository struct {
	storage *pgxpool.Pool
}

func NewOrderDelegationRepository(storage *pgxpool.Pool) OrderDelegationRepositoryInterface {
	return &OrderDelegationRepository{storage: storage}
}

func (r *OrderDelegationRepository) CreateOrderDelegation(ctx context.Context, payload dto.CreateOrderDelegationDTO) (int, error) {
	delegatorID, ok := ctx.Value(contextkeys.UserIDKey).(int)
	if !ok || delegatorID == 0 {
		return 0, apperrors.ErrInvalidUserID
	}

	query := `INSERT INTO order_delegations (order_id, delegation_user_id, delegated_user_id, status_id, created_at, updated_at) VALUES ($1, $2, $3, $4, NOW(), NOW()) RETURNING id`

	var newID int
	err := r.storage.QueryRow(ctx, query,
		payload.OrderID,
		delegatorID,
		payload.DelegatedUserID,
		payload.StatusID,
	).Scan(&newID)
	if err != nil {
		return 0, fmt.Errorf("ошибка создания делегирования: %w", err)
	}
	return newID, nil
}

func (r *OrderDelegationRepository) CreateOrderDelegationInTx(ctx context.Context, tx pgx.Tx, delegatorID int, dto dto.CreateOrderDelegationDTO) error {
	query := `INSERT INTO order_delegations (order_id, delegation_user_id, delegated_user_id, status_id, created_at, updated_at) 
	          VALUES ($1, $2, $3, $4, NOW(), NOW())`

	_, err := tx.Exec(ctx, query, dto.OrderID, delegatorID, dto.DelegatedUserID, dto.StatusID)
	if err != nil {
		return fmt.Errorf("ошибка создания записи в 'order_delegations': %w", err)
	}
	return nil
}
func (r *OrderDelegationRepository) GetOrderDelegations(ctx context.Context, limit uint64, offset uint64) ([]dto.OrderDelegationDTO, uint64, error) {
	var total uint64
	if err := r.storage.QueryRow(ctx, `SELECT COUNT(*) FROM order_delegations`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("ошибка подсчета делегирований: %w", err)
	}

	query := `
		SELECT 
			od.id, od.created_at, od.updated_at, s.id, s.name, o.id, o.name,
			delegator.id, delegator.fio, delegatee.id, delegatee.fio
		FROM order_delegations od
		LEFT JOIN statuses s ON od.status_id = s.id
		LEFT JOIN orders o ON od.order_id = o.id
		LEFT JOIN users delegator ON od.delegation_user_id = delegator.id
		LEFT JOIN users delegatee ON od.delegated_user_id = delegatee.id
		ORDER BY od.created_at DESC LIMIT $1 OFFSET $2`

	rows, err := r.storage.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var delegations []dto.OrderDelegationDTO
	for rows.Next() {
		var d dto.OrderDelegationDTO
		var createdAt, updatedAt time.Time
		var statusId, orderId, delegatorId, delegateeId sql.NullInt32
		var statusName, orderName, delegatorFio, delegateeFio sql.NullString

		err := rows.Scan(
			&d.ID, &createdAt, &updatedAt, &statusId, &statusName, &orderId, &orderName,
			&delegatorId, &delegatorFio, &delegateeId, &delegateeFio,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("ошибка сканирования делегирования: %w", err)
		}

		if statusId.Valid {
			d.Status.ID = int(statusId.Int32)
		}
		if statusName.Valid {
			d.Status.Name = statusName.String
		}
		if orderId.Valid {
			d.Order.ID = int(orderId.Int32)
		}
		if orderName.Valid {
			d.Order.Name = orderName.String
		}

		if delegatorId.Valid {

			d.Delegator = &dto.ShortUserDTO{ID: int(delegatorId.Int32), Fio: delegatorFio.String}
		}
		if delegateeId.Valid {

			d.Delegatee = &dto.ShortUserDTO{ID: int(delegateeId.Int32), Fio: delegateeFio.String}
		}

		d.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
		d.UpdatedAt = updatedAt.Local().Format("2006-01-02 15:04:05")
		delegations = append(delegations, d)
	}
	return delegations, total, nil
}

func (r *OrderDelegationRepository) FindOrderDelegation(ctx context.Context, id uint64) (*dto.OrderDelegationDTO, error) {
	query := `
		SELECT 
			od.id, od.created_at, od.updated_at, s.id, s.name, o.id, o.name,
			delegator.id, delegator.fio, delegatee.id, delegatee.fio
		FROM order_delegations od
		LEFT JOIN statuses s ON od.status_id = s.id
		LEFT JOIN orders o ON od.order_id = o.id
		LEFT JOIN users delegator ON od.delegation_user_id = delegator.id
		LEFT JOIN users delegatee ON od.delegated_user_id = delegatee.id
		WHERE od.id = $1`

	var d dto.OrderDelegationDTO
	var createdAt, updatedAt time.Time
	var statusId, orderId, delegatorId, delegateeId sql.NullInt32
	var statusName, orderName, delegatorFio, delegateeFio sql.NullString

	err := r.storage.QueryRow(ctx, query, id).Scan(
		&d.ID, &createdAt, &updatedAt,
		&statusId, &statusName, &orderId, &orderName,
		&delegatorId, &delegatorFio, &delegateeId, &delegateeFio,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, utils.ErrorNotFound
		}
		return nil, fmt.Errorf("ошибка при поиске делегирования %d: %w", id, err)
	}

	if statusId.Valid {
		d.Status.ID = int(statusId.Int32)
	}
	if statusName.Valid {
		d.Status.Name = statusName.String
	}
	if orderId.Valid {
		d.Order.ID = int(orderId.Int32)
	}
	if orderName.Valid {
		d.Order.Name = orderName.String
	}

	if delegatorId.Valid {

		d.Delegator = &dto.ShortUserDTO{ID: int(delegatorId.Int32), Fio: delegatorFio.String}
	}
	if delegateeId.Valid {

		d.Delegatee = &dto.ShortUserDTO{ID: int(delegateeId.Int32), Fio: delegateeFio.String}
	}

	d.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
	d.UpdatedAt = updatedAt.Local().Format("2006-01-02 15:04:05")
	return &d, nil
}

func (r *OrderDelegationRepository) DeleteOrderDelegation(ctx context.Context, id uint64) error {
	_, err := r.storage.Exec(ctx, `DELETE FROM order_delegations WHERE id = $1`, id)
	return err
}
