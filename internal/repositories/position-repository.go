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

const positionTable = "positions"
const positionFields = "id, name, created_at, updated_at"

type PositionRepositoryInterface interface {
	GetPositions(ctx context.Context, limit uint64, offset uint64) (interface{}, uint64, error)
	FindPosition(ctx context.Context, id uint64) (*dto.PositionDTO, error)
	CreatePosition(ctx context.Context, dto dto.CreatePositionDTO) error
	UpdatePosition(ctx context.Context, id uint64, dto dto.UpdatePositionDTO) error
	DeletePosition(ctx context.Context, id uint64) error
}

type PositionRepository struct {
	storage *pgxpool.Pool
}

func NewPositionRepository(storage *pgxpool.Pool) PositionRepositoryInterface {

	return &PositionRepository{
		storage: storage,
	}
}

func (r *PositionRepository) GetPositions(ctx context.Context, limit uint64, offset uint64) (interface{}, uint64, error) {
	data, total, err := FetchDataAndCount(ctx, r.storage, Params{
		Table:     "positions",
		Columns:   "positions.*",
		WithPg:    true,
		Limit:     limit,
		Offset:    offset,
		Relations: []Join{},
		Filter:    map[string]interface{}{},
	})

	return data, total, err
}

func (r *PositionRepository) FindPosition(ctx context.Context, id uint64) (*dto.PositionDTO, error) {
	query := fmt.Sprintf(`
		SELECT 
			%s	
		FROM %s	
		WHERE id = $1		
	`, positionFields, positionTable)

	var position dto.PositionDTO
	var createdAt *time.Time
	var updatedAt *time.Time

	err := r.storage.QueryRow(ctx, query, id).Scan(
		&position.ID,
		&position.Name,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}

	if createdAt != nil {
		position.CreatedAt = createdAt.Format("2006-01-02, 15:04:05")
	}

	if updatedAt != nil {
		position.UpdatedAt = updatedAt.Format("2006-01-02, 15:04:05")
	}
	return &position, nil
}

func (r *PositionRepository) CreatePosition(ctx context.Context, dto dto.CreatePositionDTO) error {

	query := fmt.Sprintf(`
        INSERT INTO %s (name)
		VALUES ($1)
    `, positionTable)

	_, err := r.storage.Exec(ctx, query,
		dto.Name,
	)

	if err != nil {
		return err
	}
	return nil
}

func (r *PositionRepository) UpdatePosition(ctx context.Context, id uint64, dto dto.UpdatePositionDTO) error {
	query := fmt.Sprintf(`
		UPDATE %s
		SET name = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`, positionTable)

	result, err := r.storage.Exec(ctx, query, dto.Name, id)
	if err != nil {
		return fmt.Errorf("update position: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return apperrors.ErrNotFound
	}

	return nil
}

func (r *PositionRepository) DeletePosition(ctx context.Context, id uint64) error {

	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", positionTable)

	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}

	return nil
}
