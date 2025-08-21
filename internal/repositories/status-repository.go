package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"request-system/internal/dto"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"

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
		UpdatedAt: utils.NullTimeToEmptyString(db.UpdatedAt),
	}
}

const (
	statusTable  = "statuses"
	statusFields = "id, icon_small, icon_big, name, type, code, created_at, updated_at"
)

type StatusRepositoryInterface interface {
	GetStatuses(ctx context.Context, limit, offset uint64, search string) ([]dto.StatusDTO, uint64, error)
	FindStatus(ctx context.Context, id uint64) (*dto.StatusDTO, error)
	FindByCode(ctx context.Context, code string) (*dto.StatusDTO, error)
	CreateStatus(ctx context.Context, dto dto.CreateStatusDTO, iconSmallPath, iconBigPath string) (*dto.StatusDTO, error)
	UpdateStatus(ctx context.Context, id uint64, dto dto.UpdateStatusDTO, iconSmallPath, iconBigPath *string) (*dto.StatusDTO, error)
	DeleteStatus(ctx context.Context, id uint64) error
}

type statusRepository struct{ storage *pgxpool.Pool }

func NewStatusRepository(storage *pgxpool.Pool) StatusRepositoryInterface {
	return &statusRepository{storage: storage}
}

func (r *statusRepository) GetStatuses(ctx context.Context, limit, offset uint64, search string) ([]dto.StatusDTO, uint64, error) {
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

func (r *statusRepository) CreateStatus(ctx context.Context, payload dto.CreateStatusDTO, iconSmallPath, iconBigPath string) (*dto.StatusDTO, error) {
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
	if len(setClauses) == 0 {
		return r.FindStatus(ctx, id)
	}

	setClauses = append(setClauses, "updated_at = NOW()")
	setQuery := strings.Join(setClauses, ", ")

	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = $%d RETURNING %s", statusTable, setQuery, argId, statusFields)
	args = append(args, id)

	var dbRow dbStatus
	err := r.storage.QueryRow(ctx, query, args...).Scan(&dbRow.ID, &dbRow.IconSmall, &dbRow.IconBig, &dbRow.Name, &dbRow.Type, &dbRow.Code, &dbRow.CreatedAt, &dbRow.UpdatedAt)
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
