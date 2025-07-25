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

const priorityTable = "priorities"
const priorityFields = "id, icon_small, icon_big, name, rate, code, created_at, updated_at"

type PriorityRepositoryInterface interface {
	GetPriorities(ctx context.Context, limit uint64, offset uint64) ([]dto.PriorityDTO, error)
	FindPriority(ctx context.Context, id uint64) (*dto.PriorityDTO, error)
	CreatePriority(ctx context.Context, dto dto.CreatePriorityDTO) error
	UpdatePriority(ctx context.Context, id uint64, dto dto.UpdatePriorityDTO) error
	DeletePriority(ctx context.Context, id uint64) error
	FindByCode(ctx context.Context, code string) (*entities.Priority, error)
}

type PriorityRepository struct {
	storage *pgxpool.Pool
}

func NewPriorityRepository(storage *pgxpool.Pool) PriorityRepositoryInterface {

	return &PriorityRepository{
		storage: storage,
	}
}

func (r *PriorityRepository) GetPriorities(ctx context.Context, limit uint64, offset uint64) ([]dto.PriorityDTO, error) {
	query := fmt.Sprintf(`
		SELECT %s FROM %s WHERE deleted_at IS NULL
		`, priorityFields, priorityTable)

	rows, err := r.storage.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	priorities := make([]dto.PriorityDTO, 0)
	for rows.Next() {
		var priority dto.PriorityDTO
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&priority.ID, &priority.IconSmall, &priority.IconBig, &priority.Name,
			&priority.Rate, &priority.Code, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		priority.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
		priority.UpdatedAt = updatedAt.Local().Format("2006-01-02 15:04:05")
		priorities = append(priorities, priority)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return priorities, nil
}

func (r *PriorityRepository) FindPriority(ctx context.Context, id uint64) (*dto.PriorityDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			%s
		FROM %s r
		WHERE r.id = $1
	`, priorityFields, priorityTable)

	var prorety dto.PriorityDTO
	var createdAt time.Time
	var updatedAt time.Time

	err := r.storage.QueryRow(ctx, query, id).Scan(
		&prorety.ID,
		&prorety.IconSmall,
		&prorety.IconBig,
		&prorety.Name,
		&prorety.Rate,
		&prorety.Code,
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

	prorety.CreatedAt = createdAtLocal.Format("2006-01-02 15:04:05")
	prorety.UpdatedAt = updatedAtLocal.Format("2006-01-02 15:04:05")

	return &prorety, nil
}

func (r *PriorityRepository) CreatePriority(ctx context.Context, dto dto.CreatePriorityDTO) error {
	query := fmt.Sprintf(`
        INSERT INTO %s (icon_small, icon_big, name, rate, code)
        VALUES ($1, $2, $3, $4, $5)
    `, priorityTable)

	_, err := r.storage.Exec(ctx, query,
		dto.IconSmall,
		dto.IconBig,
		dto.Name,
		dto.Rate,
		dto.Code,
	)

	if err != nil {
		return err
	}
	return nil
}

func (r *PriorityRepository) UpdatePriority(ctx context.Context, id uint64, dto dto.UpdatePriorityDTO) error {
	query := fmt.Sprintf(`
        UPDATE %s
        SET icon_small = $1, icon_big = $2, name = $3, rate = $4, code = $5, updated_at = CURRENT_TIMESTAMP
        WHERE id = $6
    `, priorityTable)

	result, err := r.storage.Exec(ctx, query,
		dto.IconSmall,
		dto.IconBig,
		dto.Name,
		dto.Rate,
		dto.Code,
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

func (r *PriorityRepository) DeletePriority(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", priorityTable)

	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}

	return nil
}
func (r *PriorityRepository) FindByCode(ctx context.Context, code string) (*entities.Priority, error) {
	query := `SELECT id, code, name FROM priorities WHERE code = $1 LIMIT 1`
	var priority entities.Priority
	err := r.storage.QueryRow(ctx, query, code).Scan(&priority.ID, &priority.Code, &priority.Name)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &priority, nil
}
