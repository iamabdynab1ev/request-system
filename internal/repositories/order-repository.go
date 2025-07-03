package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"request-system/internal/dto"
	apperrors "request-system/pkg/errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderRepositoryInterface interface {
	GetOrders(ctx context.Context, limit uint64, offset uint64) ([]dto.OrderDTO, uint64, error)
	FindOrder(ctx context.Context, id uint64) (*dto.OrderDTO, error)
	SoftDeleteOrder(ctx context.Context, id uint64) error
	CreateOrderInTx(ctx context.Context, tx pgx.Tx, creatorUserID int, dto dto.CreateOrderDTO, executorID int) (int, error)
	FindOrderForUpdateInTx(ctx context.Context, tx pgx.Tx, id uint64) (*dto.OrderForUpdate, error)
	UpdateOrderInTx(ctx context.Context, tx pgx.Tx, id uint64, dto dto.UpdateOrderDTO) error
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

	query := `
		SELECT
			ord.id, ord.name, ord.department_id, ord.otdel_id, ord.branch_id, ord.office_id, 
			ord.equipment_id, ord.duration, ord.address, ord.created_at,
			s.id, s.name, p.id, p.name,
			creator.id, creator.fio, executor.id, executor.fio
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
		var otdelID, branchID, officeID, equipmentID, proretyID, statusID, executorId sql.NullInt64
		var executorFio, duration sql.NullString
		var createdAt time.Time

		err := rows.Scan(
			&order.ID, &order.Name,
			&order.DepartmentID,
			&otdelID, &branchID, &officeID, &equipmentID,
			&duration, &order.Address, &createdAt,
			&statusID,
			&order.Status.Name,
			&proretyID,
			&order.Prorety.Name,
			&order.Creator.ID, &order.Creator.Fio,
			&executorId, &executorFio,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("ошибка сканирования заявки в списке: %w", err)
		}
		if statusID.Valid {
			order.Status.ID = int(statusID.Int64)
		}
		if proretyID.Valid {
			order.Prorety.ID = int(proretyID.Int64)
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
		if duration.Valid && duration.String != "00:00:00" {
			order.Duration = duration.String
		}
		order.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
		orders = append(orders, order)
	}
	return orders, total, nil
}

func (r *OrderRepository) FindOrder(ctx context.Context, id uint64) (*dto.OrderDTO, error) {
	query := `
		SELECT
			ord.id, ord.name, ord.department_id, ord.otdel_id, ord.branch_id, ord.office_id, 
			ord.equipment_id, ord.duration, ord.address, ord.created_at,
			s.id, s.name, p.id, p.name,
			creator.id, creator.fio, executor.id, executor.fio
		FROM orders ord
		LEFT JOIN statuses s ON ord.status_id = s.id
		LEFT JOIN proreties p ON ord.prorety_id = p.id
		LEFT JOIN users creator ON ord.user_id = creator.id
		LEFT JOIN users executor ON ord.executor_id = executor.id
		WHERE ord.id = $1 AND ord.deleted_at IS NULL`

	var order dto.OrderDTO
	var otdelID, branchID, officeID, equipmentID, proretyID, statusID, executorId sql.NullInt64
	var executorFio, duration sql.NullString
	var createdAt time.Time

	err := r.storage.QueryRow(ctx, query, id).Scan(
		&order.ID, &order.Name,
		&order.DepartmentID,
		&otdelID, &branchID, &officeID, &equipmentID,
		&duration, &order.Address, &createdAt,
		&statusID, &order.Status.Name,
		&proretyID, &order.Prorety.Name,
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
	}
	if proretyID.Valid {
		order.Prorety.ID = int(proretyID.Int64)
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
	if duration.Valid && duration.String != "00:00:00" {
		order.Duration = duration.String
	}
	order.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
	return &order, nil
}
func (r *OrderRepository) SoftDeleteOrder(ctx context.Context, id uint64) error {
	query := `UPDATE orders SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("ошибка мягкого удаления заявки: %w", err)
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *OrderRepository) CreateOrderInTx(ctx context.Context, tx pgx.Tx, creatorUserID int, dto dto.CreateOrderDTO, executorID int) (newOrderID int, err error) {

	query := `INSERT INTO orders (
				name, department_id, otdel_id, prorety_id, status_id, 
				branch_id, office_id, equipment_id, user_id, 
				address, executor_id, created_at, updated_at
			  ) 
			  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW()) 
			  RETURNING id`
	otdelID := sql.NullInt32{Int32: int32(dto.OtdelID), Valid: dto.OtdelID > 0}
	branchID := sql.NullInt32{Int32: int32(dto.BranchID), Valid: dto.BranchID > 0}
	officeID := sql.NullInt32{Int32: int32(dto.OfficeID), Valid: dto.OfficeID > 0}
	equipmentID := sql.NullInt32{Int32: int32(dto.EquipmentID), Valid: dto.EquipmentID > 0}
	proretyID := sql.NullInt32{Int32: int32(dto.ProretyID), Valid: dto.ProretyID > 0}
	statusID := sql.NullInt32{Int32: int32(dto.StatusID), Valid: dto.StatusID > 0}

	err = tx.QueryRow(ctx, query,
		dto.Name,
		dto.DepartmentID,
		otdelID,
		proretyID,
		statusID,
		branchID,
		officeID,
		equipmentID,
		creatorUserID,
		dto.Address,
		executorID,
	).Scan(&newOrderID)

	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			return 0, fmt.Errorf("ошибка создания записи в 'orders', db_error: %s, detail: %s", pgErr.Message, pgErr.Detail)
		}
		return 0, fmt.Errorf("ошибка создания записи в 'orders': %w", err)
	}

	return newOrderID, nil
}
func (r *OrderRepository) FindOrderForUpdateInTx(ctx context.Context, tx pgx.Tx, id uint64) (*dto.OrderForUpdate, error) {
	var data dto.OrderForUpdate
	query := `SELECT executor_id, status_id FROM orders WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`

	err := tx.QueryRow(ctx, query, id).Scan(&data.CurrentExecutorID, &data.CurrentStatusID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("не удалось найти и заблокировать заявку для обновления: %w", err)
	}
	return &data, nil
}

func (r *OrderRepository) UpdateOrderInTx(ctx context.Context, tx pgx.Tx, id uint64, dto dto.UpdateOrderDTO) error {
	query := "UPDATE orders SET updated_at = NOW()"
	args := []interface{}{}
	argCounter := 1

	if dto.Name != "" {
		query += fmt.Sprintf(", name = $%d", argCounter)
		args = append(args, dto.Name)
		argCounter++
	}
	if dto.StatusID != 0 {
		query += fmt.Sprintf(", status_id = $%d", argCounter)
		args = append(args, dto.StatusID)
		argCounter++
	}
	if dto.ExecutorID != 0 {
		query += fmt.Sprintf(", executor_id = $%d", argCounter)
		args = append(args, dto.ExecutorID)
		argCounter++
	}
	if dto.DepartmentID != 0 {
		query += fmt.Sprintf(", department_id = $%d", argCounter)
		args = append(args, dto.DepartmentID)
		argCounter++
	}

	if len(args) == 0 {
		return nil
	}

	query += fmt.Sprintf(" WHERE id = $%d", argCounter)
	args = append(args, id)

	result, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("ошибка при обновлении заявки: %w", err)
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
