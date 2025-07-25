package repositories

import (
	"context"
	"fmt"

	"request-system/internal/dto"
	apperrors "request-system/pkg/errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const otdelTable = "otdels"
const otdelTableFields = "id, name, status_id, departments_id, created_at, updated_at"

type OtdelRepositoryInterface interface {
	GetOtdels(ctx context.Context, limit uint64, offset uint64) ([]dto.OtdelDTO, error)
	FindOtdel(ctx context.Context, id uint64) (*dto.OtdelDTO, error)
	CreateOtdel(ctx context.Context, dto dto.CreateOtdelDTO) error
	UpdateOtdel(ctx context.Context, id uint64, dto dto.UpdateOtdelDTO) error
	DeleteOtdel(ctx context.Context, id uint64) error
}

type OtdelRepository struct {
	storage *pgxpool.Pool
}

func NewOtdelRepository(storage *pgxpool.Pool) OtdelRepositoryInterface {

	return &OtdelRepository{
		storage: storage,
	}
}

func (r *OtdelRepository) GetOtdels(ctx context.Context, limit uint64, offset uint64) ([]dto.OtdelDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			%s
		FROM %s r
		`, otdelTableFields, otdelTable)

	rows, err := r.storage.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	otdels := make([]dto.OtdelDTO, 0)

	for rows.Next() {
		var otdel dto.OtdelDTO
		var createdAt time.Time
		var updatedAt time.Time

		err := rows.Scan(
			&otdel.ID,
			&otdel.Name,
			&otdel.StatusID,
			&otdel.DepartmentsID,
			&createdAt,
			&updatedAt,
		)

		if err != nil {
			return nil, err
		}

		createdAtLocal := createdAt.Local()
		updatedAtLocal := updatedAt.Local()

		otdel.CreatedAt = createdAtLocal.Format("2006-01-02 15:04:05")
		otdel.UpdatedAt = updatedAtLocal.Format("2006-01-02 15:04:05")

		otdels = append(otdels, otdel)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return otdels, nil
}

func (r *OtdelRepository) FindOtdel(ctx context.Context, id uint64) (*dto.OtdelDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			%s
		FROM %s r
		WHERE r.id = $1
	`, otdelTableFields, otdelTable)

	var otdel dto.OtdelDTO
	var createdAt time.Time
	var updatedAt time.Time

	err := r.storage.QueryRow(ctx, query, id).Scan(
		&otdel.ID,
		&otdel.Name,
		&otdel.StatusID,
		&otdel.DepartmentsID,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}

	createdAtLocal := createdAt.Local()
	updatedAtLocal := updatedAt.Local()

	otdel.CreatedAt = createdAtLocal.Format("2006-01-02 15:04:05")
	otdel.UpdatedAt = updatedAtLocal.Format("2006-01-02 15:04:05")

	return &otdel, nil
}

func (r *OtdelRepository) CreateOtdel(ctx context.Context, dto dto.CreateOtdelDTO) error {
	query := fmt.Sprintf(`
        INSERT INTO %s (name, status_id, departments_id)
        VALUES ($1, $2, $3)
    `, otdelTable)

	_, err := r.storage.Exec(ctx, query,
		dto.Name,
		dto.StatusID,
		dto.DepartmentsID,
	)

	if err != nil {
		return err
	}
	return nil
}

func (r *OtdelRepository) UpdateOtdel(ctx context.Context, id uint64, dto dto.UpdateOtdelDTO) error {
	query := fmt.Sprintf(`
        UPDATE %s
        SET name = $1, status_id = $2, departments_id = $3, updated_at = CURRENT_TIMESTAMP
        WHERE id = $4
    `, otdelTable)

	result, err := r.storage.Exec(ctx, query,
		dto.Name,
		dto.StatusID,
		dto.DepartmentsID,
		id,
	)

	if err != nil {

		return err
	}

	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *OtdelRepository) DeleteOtdel(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", otdelTable)

	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}

	return nil
}
