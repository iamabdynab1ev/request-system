package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"request-system/internal/dto"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

func (db *dbStatus) ToDTO() dto.StatusDTO {
	return dto.StatusDTO{
		ID:        db.ID,
		IconSmall: utils.NullStringToString(db.IconSmall),
		IconBig:   utils.NullStringToString(db.IconBig),
		Name:      db.Name,
		Type:      db.Type,
		Code:      utils.NullStringToString(db.Code),
		CreatedAt: db.CreatedAt.Local().Format("2006-01-02 15:04:05"),
		UpdatedAt: utils.NullTimeToString(db.UpdatedAt),
	}
}

const statusTable = "statuses"
const statusFields = "id, icon_small, icon_big, name, type, code, created_at, updated_at"

type StatusRepositoryInterface interface {
	GetStatuses(ctx context.Context, limit uint64, offset uint64) ([]dto.StatusDTO, uint64, error)
	FindStatus(ctx context.Context, id uint64) (*dto.StatusDTO, error)
	FindByCode(ctx context.Context, code string) (*dto.StatusDTO, error)
	CreateStatus(ctx context.Context, dto dto.CreateStatusDTO) (*dto.StatusDTO, error)
	UpdateStatus(ctx context.Context, id uint64, dto dto.UpdateStatusDTO) (*dto.StatusDTO, error)
	DeleteStatus(ctx context.Context, id uint64) error
}

type statusRepository struct {
	storage *pgxpool.Pool
}

func NewStatusRepository(storage *pgxpool.Pool) StatusRepositoryInterface {
	return &statusRepository{storage: storage}
}

func (r *statusRepository) GetStatuses(ctx context.Context, limit, offset uint64) ([]dto.StatusDTO, uint64, error) {
	var total uint64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", statusTable)
	if err := r.storage.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return make([]dto.StatusDTO, 0), 0, nil
	}

	query := fmt.Sprintf("SELECT %s FROM %s ORDER BY id LIMIT $1 OFFSET $2", statusFields, statusTable)
	rows, err := r.storage.Query(ctx, query, limit, offset)
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

func (r *statusRepository) FindStatus(ctx context.Context, id uint64) (*dto.StatusDTO, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", statusFields, statusTable)
	var dbRow dbStatus
	err := r.storage.QueryRow(ctx, query, id).Scan(&dbRow.ID, &dbRow.IconSmall, &dbRow.IconBig, &dbRow.Name, &dbRow.Type, &dbRow.Code, &dbRow.CreatedAt, &dbRow.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	statusDTO := dbRow.ToDTO()
	return &statusDTO, nil
}

func (r *statusRepository) FindByCode(ctx context.Context, code string) (*dto.StatusDTO, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE code = $1 LIMIT 1", statusFields, statusTable)
	var dbRow dbStatus
	err := r.storage.QueryRow(ctx, query, code).Scan(&dbRow.ID, &dbRow.IconSmall, &dbRow.IconBig, &dbRow.Name, &dbRow.Type, &dbRow.Code, &dbRow.CreatedAt, &dbRow.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	statusDTO := dbRow.ToDTO()
	return &statusDTO, nil
}

func (r *statusRepository) DeleteStatus(ctx context.Context, id uint64) error {
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

func (r *statusRepository) CreateStatus(ctx context.Context, payload dto.CreateStatusDTO) (*dto.StatusDTO, error) {
	query := fmt.Sprintf("INSERT INTO %s (icon_small, icon_big, name, type, code) VALUES($1, $2, $3, $4, $5) RETURNING %s", statusTable, statusFields)
	var dbRow dbStatus
	err := r.storage.QueryRow(ctx, query, payload.IconSmall, payload.IconBig, payload.Name, payload.Type, payload.Code).Scan(&dbRow.ID, &dbRow.IconSmall, &dbRow.IconBig, &dbRow.Name, &dbRow.Type, &dbRow.Code, &dbRow.CreatedAt, &dbRow.UpdatedAt)
	if err != nil {
		// Here you might want to check for specific pg errors, like unique constraint violations
		return nil, err
	}
	statusDTO := dbRow.ToDTO()
	return &statusDTO, nil
}

func (r *statusRepository) UpdateStatus(ctx context.Context, id uint64, dto dto.UpdateStatusDTO) (*dto.StatusDTO, error) {
	query := fmt.Sprintf("UPDATE %s SET icon_small = $1, icon_big = $2, name = $3, type = $4, code = $5, updated_at = NOW() WHERE id = $6 RETURNING %s", statusTable, statusFields)
	var dbRow dbStatus
	err := r.storage.QueryRow(ctx, query, dto.IconSmall, dto.IconBig, dto.Name, dto.Type, dto.Code, id).Scan(&dbRow.ID, &dbRow.IconSmall, &dbRow.IconBig, &dbRow.Name, &dbRow.Type, &dbRow.Code, &dbRow.CreatedAt, &dbRow.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	statusDTO := dbRow.ToDTO()
	return &statusDTO, nil
}
