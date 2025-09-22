package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"request-system/internal/dto"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// PriorityRepositoryInterface - обновленный интерфейс
type PriorityRepositoryInterface interface {
	GetPriorities(ctx context.Context, limit uint64, offset uint64, search string) ([]dto.PriorityDTO, uint64, error)
	FindPriority(ctx context.Context, id uint64) (*dto.PriorityDTO, error)
	CreatePriority(ctx context.Context, dto dto.CreatePriorityDTO) (*dto.PriorityDTO, error)
	UpdatePriority(ctx context.Context, id uint64, dto dto.UpdatePriorityDTO) (*dto.PriorityDTO, error)
	DeletePriority(ctx context.Context, id uint64) error
	FindByCode(ctx context.Context, code string) (*entities.Priority, error)
	FindByID(ctx context.Context, id uint64) (*entities.Priority, error)
}

// Глобальные константы без полей иконок
const (
	priorityTable  = "priorities"
	priorityFields = "id, name, rate, code, created_at, updated_at"
)

// dbPriority - структура для сканирования из БД, без иконок.
type dbPriority struct {
	ID        uint64
	Name      string
	Rate      sql.NullInt32
	Code      sql.NullString
	CreatedAt time.Time
	UpdatedAt sql.NullTime
}

// toDTO - конвертер без полей иконок.
func (db *dbPriority) toDTO() dto.PriorityDTO {
	return dto.PriorityDTO{
		ID:        db.ID,
		Name:      db.Name,
		Rate:      utils.NullInt32ToInt(db.Rate),
		Code:      utils.NullStringToString(db.Code),
		CreatedAt: db.CreatedAt.Local().Format("2006-01-02 15:04:05"),
		UpdatedAt: utils.NullTimeToEmptyString(db.UpdatedAt),
	}
}

type PriorityRepository struct {
	storage *pgxpool.Pool
	logger  *zap.Logger
}

func NewPriorityRepository(storage *pgxpool.Pool, logger *zap.Logger) PriorityRepositoryInterface {
	return &PriorityRepository{storage: storage, logger: logger}
}

// GetPriorities: изменен Scan
func (r *PriorityRepository) GetPriorities(ctx context.Context, limit, offset uint64, search string) ([]dto.PriorityDTO, uint64, error) {
	var total uint64
	var args []interface{}

	// ИСПРАВЛЕННАЯ ЛОГИКА
	whereClause := "" // Начинаем с пустой строки

	if search != "" {
		whereClause = "WHERE name ILIKE $1 OR code ILIKE $1" // Добавляем WHERE, только если он нужен
		args = append(args, "%"+search+"%")
	}
	// Условие "WHERE deleted_at IS NULL" ПОЛНОСТЬЮ УБРАНО.

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", priorityTable, whereClause)
	if err := r.storage.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		r.logger.Error("GetPriorities (repo): ошибка при подсчете количества", zap.Error(err), zap.String("query", countQuery))
		return nil, 0, err
	}

	if total == 0 {
		return []dto.PriorityDTO{}, 0, nil
	}

	// Код для постраничной навигации и сортировки остается без изменений
	queryArgs := append(args, limit, offset)
	query := fmt.Sprintf("SELECT %s FROM %s %s ORDER BY rate DESC, id LIMIT $%d OFFSET $%d",
		priorityFields, priorityTable, whereClause, len(args)+1, len(args)+2)

	rows, err := r.storage.Query(ctx, query, queryArgs...)
	if err != nil {
		r.logger.Error("GetPriorities (repo): ошибка при выполнении основного запроса", zap.Error(err), zap.String("query", query))
		return nil, 0, err
	}
	defer rows.Close()

	priorities := make([]dto.PriorityDTO, 0)
	for rows.Next() {
		var dbRow dbPriority
		if err := rows.Scan(&dbRow.ID, &dbRow.Name, &dbRow.Rate, &dbRow.Code, &dbRow.CreatedAt, &dbRow.UpdatedAt); err != nil {
			r.logger.Error("GetPriorities (repo): ошибка при сканировании строки", zap.Error(err))
			return nil, 0, err
		}
		priorities = append(priorities, dbRow.toDTO())
	}
	return priorities, total, rows.Err()
}

// FindPriority: изменен Scan
func (r *PriorityRepository) FindPriority(ctx context.Context, id uint64) (*dto.PriorityDTO, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", priorityFields, priorityTable)
	var dbRow dbPriority
	err := r.storage.QueryRow(ctx, query, id).Scan(&dbRow.ID, &dbRow.Name, &dbRow.Rate, &dbRow.Code, &dbRow.CreatedAt, &dbRow.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	priorityDTO := dbRow.toDTO()
	return &priorityDTO, nil
}

// CreatePriority: изменен INSERT и Scan
func (r *PriorityRepository) CreatePriority(ctx context.Context, dto dto.CreatePriorityDTO) (*dto.PriorityDTO, error) {
	query := fmt.Sprintf(`INSERT INTO %s (name, rate, code) VALUES ($1, $2, $3) RETURNING %s`, priorityTable, priorityFields)
	var dbRow dbPriority
	err := r.storage.QueryRow(ctx, query, dto.Name, dto.Rate, dto.Code).Scan(&dbRow.ID, &dbRow.Name, &dbRow.Rate, &dbRow.Code, &dbRow.CreatedAt, &dbRow.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // UNIQUE constraint
			return nil, apperrors.ErrConflict
		}
		r.logger.Error("Ошибка при создании приоритета в БД", zap.Error(err))
		return nil, err
	}
	createdDTO := dbRow.toDTO()
	return &createdDTO, nil
}

// UpdatePriority: изменен UPDATE и Scan, полностью убрана логика иконок
func (r *PriorityRepository) UpdatePriority(ctx context.Context, id uint64, dto dto.UpdatePriorityDTO) (*dto.PriorityDTO, error) {
	var setClauses []string
	var args []interface{}
	argId := 1

	if dto.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argId))
		args = append(args, *dto.Name)
		argId++
	}
	if dto.Rate != nil {
		setClauses = append(setClauses, fmt.Sprintf("rate = $%d", argId))
		args = append(args, *dto.Rate)
		argId++
	}
	if dto.Code != nil {
		setClauses = append(setClauses, fmt.Sprintf("code = $%d", argId))
		args = append(args, *dto.Code)
		argId++
	}

	if len(setClauses) == 0 {
		return r.FindPriority(ctx, id)
	}

	setClauses = append(setClauses, "updated_at = NOW()")
	setQuery := strings.Join(setClauses, ", ")

	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = $%d RETURNING %s", priorityTable, setQuery, argId, priorityFields)
	args = append(args, id)

	var dbRow dbPriority
	err := r.storage.QueryRow(ctx, query, args...).Scan(&dbRow.ID, &dbRow.Name, &dbRow.Rate, &dbRow.Code, &dbRow.CreatedAt, &dbRow.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, apperrors.ErrConflict
		}
		return nil, err
	}
	updatedDTO := dbRow.toDTO()
	return &updatedDTO, nil
}

// DeletePriority: теперь обрабатывает ошибку внешнего ключа
func (r *PriorityRepository) DeletePriority(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", priorityTable)
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
			return apperrors.ErrPriorityInUse
		}
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
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &priority, nil
}

func (r *PriorityRepository) FindByID(ctx context.Context, id uint64) (*entities.Priority, error) {
	query := `SELECT id, code, name FROM priorities WHERE id = $1 LIMIT 1`
	var priority entities.Priority
	err := r.storage.QueryRow(ctx, query, id).Scan(&priority.ID, &priority.Code, &priority.Name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &priority, nil
}
