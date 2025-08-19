package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"request-system/internal/dto"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const priorityTable = "priorities"
const priorityFields = "id, icon_small, icon_big, name, rate, code, created_at, updated_at"

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
	GetPriorities(ctx context.Context, limit uint64, offset uint64, search string) ([]dto.PriorityDTO, uint64, error)
	FindPriority(ctx context.Context, id uint64) (*dto.PriorityDTO, error)
	CreatePriority(ctx context.Context, dto dto.CreatePriorityDTO) (*dto.PriorityDTO, error)
	UpdatePriority(ctx context.Context, id uint64, dto dto.UpdatePriorityDTO) (*dto.PriorityDTO, error)
	DeletePriority(ctx context.Context, id uint64) error
	FindByCode(ctx context.Context, code string) (*entities.Priority, error)
	FindByID(ctx context.Context, id uint64) (*entities.Priority, error) // Added for convenience
}

type PriorityRepository struct{ storage *pgxpool.Pool }

func NewPriorityRepository(storage *pgxpool.Pool) PriorityRepositoryInterface {
	return &PriorityRepository{storage: storage}
}

func (r *PriorityRepository) GetPriorities(ctx context.Context, limit, offset uint64, search string) ([]dto.PriorityDTO, uint64, error) {
	var total uint64
	args := make([]interface{}, 0)
	whereClause := "WHERE deleted_at IS NULL"

	if search != "" {
		whereClause += " AND (name ILIKE $1 OR code ILIKE $1)"
		args = append(args, "%"+search+"%")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", priorityTable, whereClause)
	if err := r.storage.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []dto.PriorityDTO{}, 0, nil
	}

	queryArgs := append(args, limit, offset)
	query := fmt.Sprintf(`SELECT %s FROM %s %s ORDER BY rate DESC, id LIMIT $%d OFFSET $%d`,
		priorityFields, priorityTable, whereClause, len(args)+1, len(args)+2)

	rows, err := r.storage.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	priorities := make([]dto.PriorityDTO, 0)
	for rows.Next() {
		var dbRow dbPriority
		if err := rows.Scan(&dbRow.ID, &dbRow.IconSmall, &dbRow.IconBig, &dbRow.Name, &dbRow.Rate, &dbRow.Code, &dbRow.CreatedAt, &dbRow.UpdatedAt); err != nil {
			return nil, 0, err
		}
		priorities = append(priorities, dbRow.toDTO())
	}
	return priorities, total, rows.Err()
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
	query := fmt.Sprintf(`INSERT INTO %s (icon_small, icon_big, name, rate, code) VALUES ($1, $2, $3, $4, $5) RETURNING %s`,
		priorityTable, priorityFields)

	var dbRow dbPriority
	err := r.storage.QueryRow(ctx, query, dto.IconSmall, dto.IconBig, dto.Name, dto.Rate, dto.Code).Scan(
		&dbRow.ID, &dbRow.IconSmall, &dbRow.IconBig, &dbRow.Name,
		&dbRow.Rate, &dbRow.Code, &dbRow.CreatedAt, &dbRow.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	createdDTO := dbRow.toDTO()
	return &createdDTO, nil
}

func (r *PriorityRepository) UpdatePriority(ctx context.Context, id uint64, dto dto.UpdatePriorityDTO) (*dto.PriorityDTO, error) {
	var setClauses []string
	args := pgx.NamedArgs{"id": id}

	if dto.Name != nil {
		setClauses = append(setClauses, "name = @name")
		args["name"] = *dto.Name
	}
	if dto.Code != nil {
		setClauses = append(setClauses, "code = @code")
		args["code"] = *dto.Code
	}
	if dto.IconSmall != nil {
		setClauses = append(setClauses, "icon_small = @icon_small")
		args["icon_small"] = *dto.IconSmall
	}
	if dto.IconBig != nil {
		setClauses = append(setClauses, "icon_big = @icon_big")
		args["icon_big"] = *dto.IconBig
	}
	if dto.Rate != nil {
		setClauses = append(setClauses, "rate = @rate")
		args["rate"] = *dto.Rate
	}

	if len(setClauses) == 0 {
		return r.FindPriority(ctx, id)
	}

	query := fmt.Sprintf(`UPDATE %s SET updated_at = NOW(), %s WHERE id = @id AND deleted_at IS NULL RETURNING %s`,
		priorityTable, strings.Join(setClauses, ", "), priorityFields)

	var dbRow dbPriority
	err := r.storage.QueryRow(ctx, query, args).Scan(
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
func (r *PriorityRepository) FindByID(ctx context.Context, id uint64) (*entities.Priority, error) {
	query := `SELECT id, code, name FROM priorities WHERE id = $1 AND deleted_at IS NULL LIMIT 1`
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
