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
)

// dbStatus - это временная структура для сканирования из БД
type dbStatus struct {
	ID        uint64
	IconSmall sql.NullString
	IconBig   sql.NullString
	Name      string
	Type      int
	Code      sql.NullString
	CreatedAt time.Time
	UpdatedAt sql.NullTime
}

// ToDTO конвертирует dbStatus в DTO для отправки клиенту
func (db *dbStatus) ToDTO() dto.StatusDTO {
	return dto.StatusDTO{
		ID:        db.ID,
		IconSmall: utils.NullStringToString(db.IconSmall),
		IconBig:   utils.NullStringToString(db.IconBig),
		Name:      db.Name,
		Type:      db.Type,
		Code:      utils.NullStringToString(db.Code),
		CreatedAt: db.CreatedAt.Local().Format("2006-01-02 15:04:05"),
		UpdatedAt: utils.NullTimeToEmptyString(db.UpdatedAt),
	}
}

// <<<--- НАЧАЛО: ИСПРАВЛЕННЫЙ КОНВЕРТЕР В СУЩНОСТЬ ---
// ToEntity конвертирует dbStatus в entities.Status для использования внутри сервисов
func (db *dbStatus) ToEntity() entities.Status {
	var codePtr *string
	if db.Code.Valid {
		code := db.Code.String // Создаем временную переменную
		codePtr = &code        // Берем адрес от нее
	}
	return entities.Status{
		ID:   int(db.ID), // FIX 1: Приводим uint64 к int
		Name: db.Name,
		Code: codePtr, // FIX 2: Правильно создаем *string
		Type: db.Type,
	}
}

// <<<--- КОНЕЦ ИСПРАВЛЕНИЙ ---

const (
	statusTable  = "statuses"
	statusFields = "id, icon_small, icon_big, name, type, code, created_at, updated_at"
)

// ИНТЕРФЕЙС - МИНИМАЛЬНЫЕ ИЗМЕНЕНИЯ, ЧТОБЫ ВСЕ РАБОТАЛО
type StatusRepositoryInterface interface {
	GetStatuses(ctx context.Context, limit, offset uint64, search string) ([]dto.StatusDTO, uint64, error)

	// Методы для сервиса OrderService (возвращают СУЩНОСТИ)
	FindStatus(ctx context.Context, id uint64) (*entities.Status, error)
	FindStatusInTx(ctx context.Context, tx pgx.Tx, id uint64) (*entities.Status, error)
	FindByCode(ctx context.Context, code string) (*entities.Status, error)
	FindByCodeInTx(ctx context.Context, tx pgx.Tx, code string) (*entities.Status, error)

	// Методы для контроллера статусов (возвращают DTO)
	CreateStatus(ctx context.Context, dto dto.CreateStatusDTO, iconSmallPath, iconBigPath string) (*dto.StatusDTO, error)
	UpdateStatus(ctx context.Context, id uint64, dto dto.UpdateStatusDTO, iconSmallPath, iconBigPath *string) (*dto.StatusDTO, error)
	DeleteStatus(ctx context.Context, id uint64) error

	// Публичный метод FindStatusAsDTO для контроллера
	FindStatusAsDTO(ctx context.Context, id uint64) (*dto.StatusDTO, error)
}

type statusRepository struct{ storage *pgxpool.Pool }

func NewStatusRepository(storage *pgxpool.Pool) StatusRepositoryInterface {
	return &statusRepository{storage: storage}
}

func (r *statusRepository) scanRow(row pgx.Row) (*dbStatus, error) {
	var dbRow dbStatus
	err := row.Scan(&dbRow.ID, &dbRow.IconSmall, &dbRow.IconBig, &dbRow.Name, &dbRow.Type, &dbRow.Code, &dbRow.CreatedAt, &dbRow.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &dbRow, nil
}

func (r *statusRepository) GetStatuses(ctx context.Context, limit, offset uint64, search string) ([]dto.StatusDTO, uint64, error) {
	// Этот метод без изменений, он корректен
	var total uint64
	var args []interface{}
	whereClause := ""

	if search != "" {
		whereClause = "WHERE name ILIKE $1 OR code ILIKE $1"
		args = append(args, "%"+search+"%")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", statusTable, whereClause)
	if err := r.storage.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []dto.StatusDTO{}, 0, nil
	}

	queryArgs := append(args, limit, offset)
	query := fmt.Sprintf("SELECT %s FROM %s %s ORDER BY id LIMIT $%d OFFSET $%d",
		statusFields, statusTable, whereClause, len(args)+1, len(args)+2)

	rows, err := r.storage.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	statuses := make([]dto.StatusDTO, 0)
	for rows.Next() {
		var dbRow dbStatus
		if err := rows.Scan(&dbRow.ID, &dbRow.IconSmall, &dbRow.IconBig, &dbRow.Name, &dbRow.Type, &dbRow.Code, &dbRow.CreatedAt, &dbRow.UpdatedAt); err != nil {
			return nil, 0, err
		}
		statuses = append(statuses, dbRow.ToDTO())
	}
	return statuses, total, rows.Err()
}

// --- МЕТОДЫ, ВОЗВРАЩАЮЩИЕ СУЩНОСТИ ДЛЯ ORDER SERVICE ---
func (r *statusRepository) FindStatus(ctx context.Context, id uint64) (*entities.Status, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", statusFields, statusTable)
	row := r.storage.QueryRow(ctx, query, id)
	dbRow, err := r.scanRow(row)
	if err != nil {
		return nil, err
	}
	entity := dbRow.ToEntity()
	return &entity, nil
}

func (r *statusRepository) FindByCode(ctx context.Context, code string) (*entities.Status, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE code = $1 LIMIT 1", statusFields, statusTable)
	row := r.storage.QueryRow(ctx, query, code)
	dbRow, err := r.scanRow(row)
	if err != nil {
		return nil, err
	}
	entity := dbRow.ToEntity()
	return &entity, nil
}

func (r *statusRepository) FindStatusInTx(ctx context.Context, tx pgx.Tx, id uint64) (*entities.Status, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", statusFields, statusTable)
	row := tx.QueryRow(ctx, query, id)
	dbRow, err := r.scanRow(row)
	if err != nil {
		return nil, err
	}
	entity := dbRow.ToEntity()
	return &entity, nil
}

func (r *statusRepository) FindByCodeInTx(ctx context.Context, tx pgx.Tx, code string) (*entities.Status, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE code = $1 LIMIT 1", statusFields, statusTable)
	row := tx.QueryRow(ctx, query, code)
	dbRow, err := r.scanRow(row)
	if err != nil {
		return nil, err
	}
	entity := dbRow.ToEntity()
	return &entity, nil
}

// --- МЕТОДЫ, ВОЗВРАЩАЮЩИЕ DTO ДЛЯ ДРУГИХ ЧАСТЕЙ ПРИЛОЖЕНИЯ ---

// FindStatusAsDTO - это старый FindStatus, возвращающий DTO. Используется в контроллере.
func (r *statusRepository) FindStatusAsDTO(ctx context.Context, id uint64) (*dto.StatusDTO, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", statusFields, statusTable)
	row := r.storage.QueryRow(ctx, query, id)
	dbRow, err := r.scanRow(row)
	if err != nil {
		return nil, err
	}
	dto := dbRow.ToDTO()
	return &dto, nil
}

func (r *statusRepository) CreateStatus(ctx context.Context, payload dto.CreateStatusDTO, iconSmallPath, iconBigPath string) (*dto.StatusDTO, error) {
	// Этот метод без изменений, он корректен
	query := fmt.Sprintf("INSERT INTO %s (name, type, code, icon_small, icon_big) VALUES($1, $2, $3, $4, $5) RETURNING %s", statusTable, statusFields)
	var dbRow dbStatus
	err := r.storage.QueryRow(ctx, query, payload.Name, payload.Type, payload.Code, iconSmallPath, iconBigPath).Scan(&dbRow.ID, &dbRow.IconSmall, &dbRow.IconBig, &dbRow.Name, &dbRow.Type, &dbRow.Code, &dbRow.CreatedAt, &dbRow.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, apperrors.ErrConflict
		}
		return nil, err
	}
	statusDTO := dbRow.ToDTO()
	return &statusDTO, nil
}

// <<<--- НАЧАЛО: ИСПРАВЛЕННЫЙ UpdateStatus ---
func (r *statusRepository) UpdateStatus(ctx context.Context, id uint64, dto dto.UpdateStatusDTO, iconSmallPath, iconBigPath *string) (*dto.StatusDTO, error) {
	var setClauses []string
	var args []interface{}
	argId := 1

	if dto.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argId))
		args = append(args, *dto.Name)
		argId++
	}
	if dto.Type != nil {
		setClauses = append(setClauses, fmt.Sprintf("type = $%d", argId))
		args = append(args, *dto.Type)
		argId++
	}
	if dto.Code != nil {
		setClauses = append(setClauses, fmt.Sprintf("code = $%d", argId))
		args = append(args, *dto.Code)
		argId++
	}
	if iconSmallPath != nil {
		setClauses = append(setClauses, fmt.Sprintf("icon_small = $%d", argId))
		args = append(args, *iconSmallPath)
		argId++
	}
	if iconBigPath != nil {
		setClauses = append(setClauses, fmt.Sprintf("icon_big = $%d", argId))
		args = append(args, *iconBigPath)
		argId++
	}

	// FIX 3: Если нет изменений, используем FindStatusAsDTO
	if len(setClauses) == 0 {
		return r.FindStatusAsDTO(ctx, id)
	}

	setClauses = append(setClauses, "updated_at = NOW()")
	setQuery := strings.Join(setClauses, ", ")

	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = $%d RETURNING %s", statusTable, setQuery, argId, statusFields)
	args = append(args, id)

	row := r.storage.QueryRow(ctx, query, args...)
	dbRow, err := r.scanRow(row)
	if err != nil {
		return nil, err
	}

	statusDTO := dbRow.ToDTO()
	return &statusDTO, nil
}

// <<<--- КОНЕЦ ИСПРАВЛЕНИЙ ---

func (r *statusRepository) DeleteStatus(ctx context.Context, id uint64) error {
	// Этот метод без изменений, он корректен
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", statusTable)
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return apperrors.ErrStatusInUse
		}
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
