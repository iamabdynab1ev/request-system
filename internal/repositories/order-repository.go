package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"request-system/internal/dto"
	"request-system/pkg/contextkeys"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderRepositoryInterface interface {
	GetOrders(ctx context.Context, limit uint64, offset uint64) ([]dto.OrderDTO, uint64, error)
	FindOrder(ctx context.Context, id uint64) (*dto.OrderDTO, error)
	CreateOrder(ctx context.Context, creatorUserID int, orderDto dto.CreateOrderDTO) (int, error)
	UpdateOrder(ctx context.Context, id uint64, dto dto.UpdateOrderDTO) error
	DeleteOrder(ctx context.Context, id uint64) error
}

type OrderRepository struct {
	storage *pgxpool.Pool
}

func NewOrderRepository(storage *pgxpool.Pool) OrderRepositoryInterface {
	return &OrderRepository{
		storage: storage,
	}
}

func (r *OrderRepository) GetOrders(ctx context.Context, limit uint64, offset uint64) ([]dto.OrderDTO, uint64, error) {
	var total uint64
	if err := r.storage.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE deleted_at IS NULL`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("ошибка подсчета заявок: %w", err)
	}

	// ИСПРАВЛЕНИЕ: "priorities" заменено на "proreties" и "priority_id" на "prorety_id"
	query := `
		SELECT
			ord.id, ord.name, ord.department_id, ord.otdel_id, ord.branch_id, ord.office_id, 
			ord.equipment_id, ord.duration, ord.address, ord.created_at,
			s.id as status_id, s.name as status_name, 
			p.id as priority_id, p.name as priority_name,
			creator.id as creator_id, creator.fio as creator_fio, 
			executor.id as executor_id, executor.fio as executor_fio
		FROM orders ord
		LEFT JOIN statuses s ON ord.status_id = s.id
		LEFT JOIN proreties p ON ord.prorety_id = p.id 
		LEFT JOIN users creator ON ord.user_id = creator.id
		LEFT JOIN users executor ON ord.executor_id = executor.id
		WHERE ord.deleted_at IS NULL
		ORDER BY ord.created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := r.storage.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка получения списка заявок: %w", err)
	}
	defer rows.Close()

	orders := make([]dto.OrderDTO, 0)
	for rows.Next() {
		var order dto.OrderDTO
		var otdelID, branchID, officeID, equipmentID, priorityID, statusID, executorId sql.NullInt64
		var priorityName, statusName, executorFio, duration sql.NullString
		var createdAt time.Time

		err := rows.Scan(
			&order.ID, &order.Name, &order.DepartmentID,
			&otdelID, &branchID, &officeID, &equipmentID,
			&duration, &order.Address, &createdAt,
			&statusID, &statusName,
			&priorityID, &priorityName,
			&order.Creator.ID, &order.Creator.Fio,
			&executorId, &executorFio,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("ошибка сканирования заявки в списке: %w", err)
		}

		if statusID.Valid {
			order.Status.ID = int(statusID.Int64)
			order.Status.Name = statusName.String
		}
		// ИСПРАВЛЕНИЕ: Prorety вместо Priority
		if priorityID.Valid {
			order.Prorety.ID = int(priorityID.Int64)
			order.Prorety.Name = priorityName.String
		}
		if otdelID.Valid {
			order.OtdelID = int(otdelID.Int64)
		}
		if branchID.Valid {
			order.BranchID = int(branchID.Int64)
		}
		if officeID.Valid {
			order.OfficeID = int(officeID.Int64)
		}
		if equipmentID.Valid {
			order.EquipmentID = int(equipmentID.Int64)
		}
		if executorId.Valid {
			order.Executor = &dto.ShortUserDTO{ID: int(executorId.Int64), Fio: executorFio.String}
		}
		if duration.Valid {
			order.Duration = duration.String
		}

		order.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
		orders = append(orders, order)
	}
	return orders, total, nil
}

// FindOrder находит одну заявку по ID.
func (r *OrderRepository) FindOrder(ctx context.Context, id uint64) (*dto.OrderDTO, error) {
	// ИСПРАВЛЕНИЕ: "priorities" заменено на "proreties" и "priority_id" на "prorety_id"
	query := `
		SELECT
			ord.id, ord.name, ord.department_id, ord.otdel_id, ord.branch_id, ord.office_id, 
			ord.equipment_id, ord.duration, ord.address, ord.created_at,
			s.id as status_id, s.name as status_name, 
			p.id as priority_id, p.name as priority_name,
			creator.id as creator_id, creator.fio as creator_fio, 
			executor.id as executor_id, executor.fio as executor_fio
		FROM orders ord
		LEFT JOIN statuses s ON ord.status_id = s.id
		LEFT JOIN proreties p ON ord.prorety_id = p.id
		LEFT JOIN users creator ON ord.user_id = creator.id
		LEFT JOIN users executor ON ord.executor_id = executor.id
		WHERE ord.id = $1 AND ord.deleted_at IS NULL`

	var order dto.OrderDTO
	var otdelID, branchID, officeID, equipmentID, priorityID, statusID, executorId sql.NullInt64
	var priorityName, statusName, executorFio, duration sql.NullString
	var createdAt time.Time

	err := r.storage.QueryRow(ctx, query, id).Scan(
		&order.ID, &order.Name, &order.DepartmentID,
		&otdelID, &branchID, &officeID, &equipmentID,
		&duration, &order.Address, &createdAt,
		&statusID, &statusName,
		&priorityID, &priorityName,
		&order.Creator.ID, &order.Creator.Fio,
		&executorId, &executorFio,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("ошибка сканирования заявки: %w", err)
	}

	if statusID.Valid {
		order.Status.ID = int(statusID.Int64)
		order.Status.Name = statusName.String
	}
	// ИСПРАВЛЕНИЕ: Prorety вместо Priority
	if priorityID.Valid {
		order.Prorety.ID = int(priorityID.Int64)
		order.Prorety.Name = priorityName.String
	}
	if otdelID.Valid {
		order.OtdelID = int(otdelID.Int64)
	}
	if branchID.Valid {
		order.BranchID = int(branchID.Int64)
	}
	if officeID.Valid {
		order.OfficeID = int(officeID.Int64)
	}
	if equipmentID.Valid {
		order.EquipmentID = int(equipmentID.Int64)
	}
	if executorId.Valid {
		order.Executor = &dto.ShortUserDTO{ID: int(executorId.Int64), Fio: executorFio.String}
	}
	if duration.Valid {
		order.Duration = duration.String
	}

	order.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
	return &order, nil
}
func (r *OrderRepository) CreateOrder(ctx context.Context, creatorUserID int, orderDto dto.CreateOrderDTO) (newOrderID int, err error) {
	tx, err := r.storage.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("ошибка при началении транзакции: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		} else if err != nil {
			_ = tx.Rollback(ctx)
		} else {
			if commitErr := tx.Commit(ctx); commitErr != nil {
				err = commitErr
			}
		}
	}()

	orderInsertQuery := `INSERT INTO orders (name, department_id, otdel_id, prorety_id, status_id, branch_id, office_id, equipment_id, user_id, duration, address, executor_id, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW()) RETURNING id`
	if err = tx.QueryRow(ctx, orderInsertQuery, orderDto.Name, orderDto.DepartmentID, orderDto.OtdelID, orderDto.ProretyID, orderDto.StatusID, orderDto.BranchID, orderDto.OfficeID, orderDto.EquipmentID, creatorUserID, orderDto.Duration, orderDto.Address, orderDto.ExecutorID).Scan(&newOrderID); err != nil {
		return 0, fmt.Errorf("'orders'га ёзишда хатолик: %w", err)
	}

	delegationInsertQuery := `INSERT INTO order_delegations (order_id, delegation_user_id, delegated_user_id, status_id, created_at, updated_at) VALUES ($1, $2, $3, $4, NOW(), NOW())`
	if _, err = tx.Exec(ctx, delegationInsertQuery, newOrderID, creatorUserID, orderDto.ExecutorID, 1); err != nil {
		return 0, fmt.Errorf("'order_delegations'га ёзишда хатолик: %w", err)
	}

	commentInsertQuery := `INSERT INTO order_comments (order_id, user_id, message, status_id, created_at, updated_at) VALUES ($1, $2, $3, $4, NOW(), NOW())`
	if _, err = tx.Exec(ctx, commentInsertQuery, newOrderID, creatorUserID, orderDto.Massage, orderDto.StatusID); err != nil {
		return 0, fmt.Errorf("'order_comments'га ёзишда хатолик: %w", err)
	}
	return newOrderID, err
}

func (r *OrderRepository) UpdateOrder(ctx context.Context, id uint64, dto dto.UpdateOrderDTO) error {
	updatorUserID, ok := ctx.Value(contextkeys.UserIDKey).(int)
	if !ok {
		return fmt.Errorf("не удалось определить пользователя, выполняющего обновление")
	}

	tx, err := r.storage.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer tx.Rollback(ctx)

	var currentExecutorID sql.NullInt32
	var currentStatusID int
	findQuery := `SELECT executor_id, status_id FROM orders WHERE id = $1 FOR UPDATE`
	if err := tx.QueryRow(ctx, findQuery, id).Scan(&currentExecutorID, &currentStatusID); err != nil {
		if err == pgx.ErrNoRows {
			return utils.ErrorNotFound
		}
		return fmt.Errorf("не удалось найти заявку для обновления: %w", err)
	}

	updateQuery := "UPDATE orders SET updated_at = NOW()"
	args := []interface{}{}
	argCounter := 1

	if dto.Name != "" {
		updateQuery += fmt.Sprintf(", name = $%d", argCounter)
		args = append(args, dto.Name)
		argCounter++
	}
	if dto.StatusID != 0 {
		updateQuery += fmt.Sprintf(", status_id = $%d", argCounter)
		args = append(args, dto.StatusID)
		argCounter++
	}
	if dto.ExecutorID != 0 {
		updateQuery += fmt.Sprintf(", executor_id = $%d", argCounter)
		args = append(args, dto.ExecutorID)
		argCounter++
	}
	if dto.DepartmentID != 0 {
		updateQuery += fmt.Sprintf(", department_id = $%d", argCounter)
		args = append(args, dto.DepartmentID)
		argCounter++
	}
	// ... (добавь другие поля при необходимости) ...

	updateQuery += fmt.Sprintf(" WHERE id = $%d", argCounter)
	args = append(args, id)

	if _, err := tx.Exec(ctx, updateQuery, args...); err != nil {
		return fmt.Errorf("ошибка при обновлении заявки: %w", err)
	}

	if dto.ExecutorID != 0 && int(currentExecutorID.Int32) != dto.ExecutorID {
		delegationQuery := `INSERT INTO order_delegations (order_id, delegation_user_id, delegated_user_id, status_id, created_at, updated_at) VALUES ($1, $2, $3, $4, NOW(), NOW())`
		statusForAction := dto.StatusID
		if statusForAction == 0 {
			statusForAction = currentStatusID
		}

		if _, err := tx.Exec(ctx, delegationQuery, id, updatorUserID, dto.ExecutorID, statusForAction); err != nil {
			return fmt.Errorf("ошибка создания записи о делегировании: %w", err)
		}
	}

	if dto.StatusID != 0 && currentStatusID != dto.StatusID {
		commentMessage := fmt.Sprintf("Статус заявки изменен (старый ID: %d, новый ID: %d)", currentStatusID, dto.StatusID)
		commentQuery := `INSERT INTO order_comments (order_id, user_id, message, status_id, created_at, updated_at) VALUES ($1, $2, $3, $4, NOW(), NOW())`
		if _, err := tx.Exec(ctx, commentQuery, id, updatorUserID, commentMessage, dto.StatusID); err != nil {
			return fmt.Errorf("ошибка создания комментария о смене статуса: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (r *OrderRepository) DeleteOrder(ctx context.Context, id uint64) error {
	_, err := r.storage.Exec(ctx, `DELETE FROM orders WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("ошибка удаления заявки: %w", err)
	}
	return nil
}
