package repositories

import (
	"context"
	"fmt"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/pkg/utils"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	STATUS_TABLE  = "statuses"
	STATUS_FIELDS = "id, icon, name, type, code, created_at"
)

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
	query := fmt.Sprintf("SELECT %s FROM %s", STATUS_FIELDS, STATUS_TABLE)

	rows, err := r.storage.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	statuses := make([]dto.StatusDTO, 0)

	for rows.Next() {
		var status dto.StatusDTO
		var createdAt time.Time

		err := rows.Scan(&status.ID, &status.Icon, &status.Name, &status.Type, &status.Code, &createdAt)

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
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", STATUS_FIELDS, STATUS_TABLE)

	var status dto.StatusDTO
	var createdAt time.Time

	err := r.storage.QueryRow(ctx, query, id).Scan(
		&status.ID, &status.Icon, &status.Name, &status.Type, &status.Code, &createdAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, utils.ErrorNotFound
		}

		return nil, err
	}

	createdAtLocal := createdAt.Local()

	status.CreatedAt = createdAtLocal.Format("2006-01-02 15:04:05")

	return &status, nil
}

func (r *StatusRepository) CreateStatus(ctx context.Context, payload dto.CreateStatusDTO) error {
	query := fmt.Sprintf("INSERT INTO %s (icon, name, type, code) VALUES($1, $2, $3, $4)", STATUS_TABLE)

	_, err := r.storage.Exec(ctx, query, payload.Icon, payload.Name, payload.Type, payload.Code)
	if err != nil {
		return err
	}

	return nil
}

func (r *StatusRepository) UpdateStatus(ctx context.Context, id uint64, dto dto.UpdateStatusDTO) error {
	query := fmt.Sprintf("UPDATE %s SET icon = $1, name = $2, type = $3, code = $4 WHERE id = $5", STATUS_TABLE)

	result, err := r.storage.Exec(ctx, query, dto.Icon, dto.Name, dto.Type, dto.Code, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return utils.ErrorNotFound
	}

	return nil
}

func (r *StatusRepository) DeleteStatus(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", STATUS_TABLE)

	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return utils.ErrorNotFound
	}

	return nil
}
func (r *StatusRepository) FindByCode(ctx context.Context, code string) (*entities.Status, error) {
	query := `SELECT id, name, icon, type, code FROM statuses WHERE code = $1`

	var status entities.Status
	err := r.storage.QueryRow(ctx, query, code).Scan(&status.ID, &status.Name, &status.Icon, &status.Type, &status.Code)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, utils.ErrorNotFound
		}
		return nil, fmt.Errorf("ошибка поиска статуса по коду '%s': %w", code, err)
	}

	return &status, nil
}
