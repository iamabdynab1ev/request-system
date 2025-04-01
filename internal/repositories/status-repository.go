package repositories

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/entities"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StatusRepositoryInterface interface {
	GetStatuses(ctx context.Context, limit uint64, offset uint64) ([]dto.StatusDTO, error)
	FindStatus(ctx context.Context, id uint64) (entities.Status, error)
	CreateStatus(ctx context.Context, payload dto.CreateStatusDTO) error
	UpdateStatus(ctx context.Context, id uint64, payload dto.UpdateStatusDTO) error
	DeleteStatus(ctx context.Context, id uint64) error
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
	rows, err := r.storage.Query(ctx, "SELECT id, icon, name, type FROM statuses LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close() // Закрываем `rows`, чтобы избежать утечек

	// Инициализируем срез с оптимальной ёмкостью
	statuses := make([]dto.StatusDTO, 0, limit)

	for rows.Next() {
		var status dto.StatusDTO

		if err := rows.Scan(&status.ID, &status.Icon, &status.Name, &status.Type); err != nil {
			return nil, err
		}

		statuses = append(statuses, status)
	}

	// Проверяем возможные ошибки во время итерации
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return statuses, nil
}

func (r *StatusRepository) FindStatus(ctx context.Context, id uint64) (entities.Status, error) {
	return entities.Status{}, nil
}

func (r *StatusRepository) CreateStatus(ctx context.Context, payload dto.CreateStatusDTO) error {
	return nil
}

func (r *StatusRepository) UpdateStatus(ctx context.Context, id uint64, payload dto.UpdateStatusDTO) error {
	return nil
}

func (r *StatusRepository) DeleteStatus(ctx context.Context, id uint64) error {
	return nil
}
