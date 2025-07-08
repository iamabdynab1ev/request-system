package repositories

import (
	"context"
	"fmt"
	"request-system/internal/dto"
	"request-system/pkg/utils"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const proretyTable = "proreties"
const proretyFields = "id, icon, name, rate, created_at, updated_at"

type ProretyRepositoryInterface interface {
	GetProreties(ctx context.Context, limit uint64, offset uint64) ([]dto.ProretyDTO, error)
	FindProrety(ctx context.Context, id uint64) (*dto.ProretyDTO, error)
	CreateProrety(ctx context.Context, dto dto.CreateProretyDTO) error
	UpdateProrety(ctx context.Context, id uint64, dto dto.UpdateProretyDTO) error
	DeleteProrety(ctx context.Context, id uint64) error
}

type ProretyRepository struct {
	storage *pgxpool.Pool
}

func NewProretyRepository(storage *pgxpool.Pool) ProretyRepositoryInterface {

	return &ProretyRepository{
		storage: storage,
	}
}

func (r *ProretyRepository) GetProreties(ctx context.Context, limit uint64, offset uint64) ([]dto.ProretyDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			%s
		FROM %s r
		`, proretyFields, proretyTable)

	rows, err := r.storage.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	proreties := make([]dto.ProretyDTO, 0)

	for rows.Next() {
		var prorety dto.ProretyDTO
		var createdAt time.Time
		var updatedAt time.Time

		err := rows.Scan(
			&prorety.ID,
			&prorety.Icon,
			&prorety.Name,
			&prorety.Rate,
			&createdAt,
			&updatedAt,
		)

		if err != nil {
			return nil, err
		}

		createdAtLocal := createdAt.Local()
		updatedAtLocal := updatedAt.Local()

		prorety.CreatedAt = createdAtLocal.Format("2006-01-02 15:04:05")
		prorety.UpdatedAt = updatedAtLocal.Format("2006-01-02 15:04:05")

		proreties = append(proreties, prorety)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return proreties, nil
}

func (r *ProretyRepository) FindProrety(ctx context.Context, id uint64) (*dto.ProretyDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			%s
		FROM %s r
		WHERE r.id = $1
	`, proretyFields, proretyTable)

	var prorety dto.ProretyDTO
	var createdAt time.Time
	var updatedAt time.Time

	err := r.storage.QueryRow(ctx, query, id).Scan(
		&prorety.ID,
		&prorety.Icon,
		&prorety.Name,
		&prorety.Rate,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, utils.ErrorNotFound
		}
		return nil, err
	}

	createdAtLocal := createdAt.Local()
	updatedAtLocal := updatedAt.Local()

	prorety.CreatedAt = createdAtLocal.Format("2006-01-02 15:04:05")
	prorety.UpdatedAt = updatedAtLocal.Format("2006-01-02 15:04:05")

	return &prorety, nil
}

func (r *ProretyRepository) CreateProrety(ctx context.Context, dto dto.CreateProretyDTO) error {
	query := fmt.Sprintf(`
        INSERT INTO %s (icon, name, rate)
        VALUES ($1, $2, $3)
    `, proretyTable)

	_, err := r.storage.Exec(ctx, query,
		dto.Icon,
		dto.Name,
		dto.Rate,
	)

	if err != nil {
		return err
	}
	return nil
}

func (r *ProretyRepository) UpdateProrety(ctx context.Context, id uint64, dto dto.UpdateProretyDTO) error {
	query := fmt.Sprintf(`
        UPDATE %s
        SET icon = $1, name = $2, rate = $3, updated_at = CURRENT_TIMESTAMP
        WHERE id = $4
    `, proretyTable)

	result, err := r.storage.Exec(ctx, query,
		dto.Icon,
		dto.Name,
		dto.Rate,
		id,
	)

	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return utils.ErrorNotFound
	}
	return nil
}

func (r *ProretyRepository) DeleteProrety(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", proretyTable)

	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return utils.ErrorNotFound
	}

	return nil
}
