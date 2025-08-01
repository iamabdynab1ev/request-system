package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"request-system/internal/dto"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const priorityTable = "priorities"

// Убедитесь, что здесь перечислены все поля, которые вы сканируете.
const priorityFields = "id, icon_small, icon_big, name, rate, code, created_at, updated_at"

// dbPriority — это вспомогательная структура для сканирования из базы данных.
type dbPriority struct {
	ID        uint64
	IconSmall sql.NullString
	IconBig   sql.NullString
	Name      string
	Rate      int
	Code      sql.NullString
	CreatedAt time.Time
	UpdatedAt time.Time
}

// toDTO преобразует структуру базы данных в объект передачи данных (DTO).
func (db *dbPriority) toDTO() dto.PriorityDTO {
	return dto.PriorityDTO{
		ID:        db.ID,
		IconSmall: db.IconSmall.String,
		IconBig:   db.IconBig.String,
		Name:      db.Name,
		Rate:      db.Rate,
		Code:      db.Code.String,
		CreatedAt: db.CreatedAt.Local().Format("2006-01-02 15:04:05"),
		UpdatedAt: db.UpdatedAt.Local().Format("2006-01-02 15:04:05"),
	}
}

type PriorityRepositoryInterface interface {
	// Обновлено для возврата общего количества для пагинации.
	GetPriorities(ctx context.Context, limit uint64, offset uint64) ([]dto.PriorityDTO, uint64, error)
	FindPriority(ctx context.Context, id uint64) (*dto.PriorityDTO, error)
	// Обновлено для возврата созданного DTO.
	CreatePriority(ctx context.Context, dto dto.CreatePriorityDTO) (*dto.PriorityDTO, error)
	// Обновлено для возврата обновленного DTO.
	UpdatePriority(ctx context.Context, id uint64, dto dto.UpdatePriorityDTO) (*dto.PriorityDTO, error)
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

func (r *PriorityRepository) GetPriorities(ctx context.Context, limit uint64, offset uint64) ([]dto.PriorityDTO, uint64, error) {
	// Сначала получаем общее количество записей, соответствующих критериям.
	var total uint64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE deleted_at IS NULL", priorityTable)
	if err := r.storage.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return make([]dto.PriorityDTO, 0), 0, nil
	}

	// Затем получаем постраничный список записей.
	query := fmt.Sprintf(`
        SELECT %s FROM %s 
        WHERE deleted_at IS NULL
        ORDER BY rate DESC, id
        LIMIT $1 OFFSET $2
    `, priorityFields, priorityTable)

	rows, err := r.storage.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	priorities := make([]dto.PriorityDTO, 0)
	for rows.Next() {
		var dbRow dbPriority
		err := rows.Scan(
			&dbRow.ID, &dbRow.IconSmall, &dbRow.IconBig, &dbRow.Name,
			&dbRow.Rate, &dbRow.Code, &dbRow.CreatedAt, &dbRow.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		priorities = append(priorities, dbRow.toDTO())
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return priorities, total, nil
}

func (r *PriorityRepository) FindPriority(ctx context.Context, id uint64) (*dto.PriorityDTO, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1 AND deleted_at IS NULL", priorityFields, priorityTable)

	var dbRow dbPriority
	err := r.storage.QueryRow(ctx, query, id).Scan(
		&dbRow.ID, &dbRow.IconSmall, &dbRow.IconBig, &dbRow.Name,
		&dbRow.Rate, &dbRow.Code, &dbRow.CreatedAt, &dbRow.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}

	priorityDTO := dbRow.toDTO()
	return &priorityDTO, nil
}

func (r *PriorityRepository) CreatePriority(ctx context.Context, dto dto.CreatePriorityDTO) (*dto.PriorityDTO, error) {
	// Используем RETURNING, чтобы получить созданную строку обратно в одном запросе.
	query := fmt.Sprintf(`
        INSERT INTO %s (icon_small, icon_big, name, rate, code)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING %s
    `, priorityTable, priorityFields)

	var dbRow dbPriority
	err := r.storage.QueryRow(ctx, query,
		dto.IconSmall, dto.IconBig, dto.Name, dto.Rate, dto.Code,
	).Scan(
		&dbRow.ID, &dbRow.IconSmall, &dbRow.IconBig, &dbRow.Name,
		&dbRow.Rate, &dbRow.Code, &dbRow.CreatedAt, &dbRow.UpdatedAt,
	)
	if err != nil {
		// Здесь можно добавить проверку на специфические ошибки pgconn.PgError, например, на дубликат кода.
		return nil, err
	}

	createdDTO := dbRow.toDTO()
	return &createdDTO, nil
}

func (r *PriorityRepository) UpdatePriority(ctx context.Context, id uint64, dto dto.UpdatePriorityDTO) (*dto.PriorityDTO, error) {
	// Здесь также используем RETURNING, чтобы получить обновленные данные.
	query := fmt.Sprintf(`
        UPDATE %s
        SET icon_small = $1, icon_big = $2, name = $3, rate = $4, code = $5, updated_at = CURRENT_TIMESTAMP
        WHERE id = $6 AND deleted_at IS NULL
        RETURNING %s
    `, priorityTable, priorityFields)

	var dbRow dbPriority
	err := r.storage.QueryRow(ctx, query,
		dto.IconSmall, dto.IconBig, dto.Name, dto.Rate, dto.Code, id,
	).Scan(
		&dbRow.ID, &dbRow.IconSmall, &dbRow.IconBig, &dbRow.Name,
		&dbRow.Rate, &dbRow.Code, &dbRow.CreatedAt, &dbRow.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}

	updatedDTO := dbRow.toDTO()
	return &updatedDTO, nil
}

func (r *PriorityRepository) DeletePriority(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", priorityTable)

	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		// Здесь можно добавить проверку на ограничения внешнего ключа, если приоритеты где-то используются.
		return err
	}

	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}

	return nil
}

func (r *PriorityRepository) FindByCode(ctx context.Context, code string) (*entities.Priority, error) {
	query := `SELECT id, code, name FROM priorities WHERE code = $1 AND deleted_at IS NULL LIMIT 1`
	var priority entities.Priority
	err := r.storage.QueryRow(ctx, query, code).Scan(&priority.ID, &priority.Code, &priority.Name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &priority, nil
}
