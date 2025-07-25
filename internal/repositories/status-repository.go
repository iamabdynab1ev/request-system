package repositories

import (
	"context"
	"fmt"
	"request-system/internal/dto"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const statusTable = "statuses"
const statusFields = "id, icon_small, icon_big, name, type, code , created_at"

type StatusRepositoryInterface interface {
	GetStatuses(ctx context.Context, limit uint64, offset uint64) ([]dto.StatusDTO, error)
	FindStatus(ctx context.Context, id uint64) (*dto.StatusDTO, error)
	CreateStatus(ctx context.Context, dto dto.CreateStatusDTO) error
	UpdateStatus(ctx context.Context, id uint64, dto dto.UpdateStatusDTO) error
	DeleteStatus(ctx context.Context, id uint64) error
	FindByCode(ctx context.Context, code string) (*entities.Status, error)
}

type StatusRepository struct {
	storage *pgxpool.Pool
}

func NewStatusRepository(storage *pgxpool.Pool) StatusRepositoryInterface {
	return &StatusRepository{
		storage: storage,
	}
}

func (r *StatusRepository) GetStatuses(ctx context.Context, limit uint64, offset uint64) ([]dto.StatusDTO, error) {
	query := fmt.Sprintf("SELECT %s FROM %s", statusFields, statusTable)

	rows, err := r.storage.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	statuses := make([]dto.StatusDTO, 0)

	for rows.Next() {
		var status dto.StatusDTO
		var createdAt time.Time

		err := rows.Scan(&status.ID, &status.IconSmall, &status.IconBig, &status.Name, &status.Type, &status.Code, &createdAt)

		if err != nil {
			return nil, err
		}

		createdAtLocal := createdAt.Local()

		status.CreatedAt = createdAtLocal.Format("2006-01-02 15:04:05")

		statuses = append(statuses, status)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return statuses, nil
}

func (r *StatusRepository) FindStatus(ctx context.Context, id uint64) (*dto.StatusDTO, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", statusFields, statusTable)

	var status dto.StatusDTO
	var createdAt time.Time

	err := r.storage.QueryRow(ctx, query, id).Scan(
		&status.ID, &status.IconSmall, &status.IconBig, &status.Name, &status.Type, &status.Code, &createdAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.ErrNotFound
		}

		return nil, err
	}

	createdAtLocal := createdAt.Local()

	status.CreatedAt = createdAtLocal.Format("2006-01-02 15:04:05")

	return &status, nil
}

func (r *StatusRepository) CreateStatus(ctx context.Context, payload dto.CreateStatusDTO) error {
	query := fmt.Sprintf("INSERT INTO %s (icon_small, icon_big,  name, type, code) VALUES($1, $2, $3, $4, $5)", statusTable)

	_, err := r.storage.Exec(ctx, query, payload.IconSmall, payload.IconBig, payload.Name, payload.Type, payload.Code)
	if err != nil {
		return err
	}

	return nil
}

func (r *StatusRepository) UpdateStatus(ctx context.Context, id uint64, dto dto.UpdateStatusDTO) error {
	query := fmt.Sprintf("UPDATE %s SET icon_small = $1, icon_big = $2, name = $3, type = $4, code = $5 WHERE id = $6", statusTable)

	result, err := r.storage.Exec(ctx, query, dto.IconSmall, dto.IconBig, dto.Name, dto.Type, dto.Code, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}

	return nil
}

func (r *StatusRepository) DeleteStatus(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", statusTable)

	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}

	return nil
}
func (r *StatusRepository) FindByCode(ctx context.Context, code string) (*entities.Status, error) {
	query := `SELECT id, code, name FROM statuses WHERE code = $1 LIMIT 1`
	var status entities.Status
	err := r.storage.QueryRow(ctx, query, code).Scan(&status.ID, &status.Code, &status.Name)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &status, nil
}
