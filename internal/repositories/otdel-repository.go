package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const otdelTable = "otdels"

var otdelAllowedFilterFields = map[string]string{
	"status_id":     "status_id",
	"department_id": "department_id",
}

var otdelAllowedSortFields = map[string]string{
	"id":         "id",
	"name":       "name",
	"created_at": "created_at",
}

type OtdelRepositoryInterface interface {
	GetOtdels(ctx context.Context, filter types.Filter) ([]entities.Otdel, uint64, error)
	FindOtdel(ctx context.Context, id uint64) (*entities.Otdel, error)
	CreateOtdel(ctx context.Context, otdel entities.Otdel) (*entities.Otdel, error)
	UpdateOtdel(ctx context.Context, id uint64, otdel entities.Otdel) (*entities.Otdel, error)
	DeleteOtdel(ctx context.Context, id uint64) error
}

type OtdelRepository struct {
	storage *pgxpool.Pool
	logger  *zap.Logger
}

func NewOtdelRepository(storage *pgxpool.Pool, logger *zap.Logger) OtdelRepositoryInterface {
	return &OtdelRepository{
		storage: storage,
		logger:  logger,
	}
}

func scanOtdel(row pgx.Row) (*entities.Otdel, error) {
	var o entities.Otdel
	var createdAt, updatedAt time.Time
	err := row.Scan(&o.ID, &o.Name, &o.StatusID, &o.DepartmentsID, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ошибка сканирования otdel: %w", err)
	}
	o.CreatedAt, o.UpdatedAt = &createdAt, &updatedAt
	return &o, nil
}

func (r *OtdelRepository) GetOtdels(ctx context.Context, filter types.Filter) ([]entities.Otdel, uint64, error) {
	args := make([]interface{}, 0)
	conditions := []string{}
	argCounter := 1

	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", argCounter))
		args = append(args, searchPattern)
		argCounter++
	}

	for key, value := range filter.Filter {
		if dbColumn, ok := otdelAllowedFilterFields[key]; ok {
			if strVal, ok := value.(string); ok && strings.Contains(strVal, ",") {
				items, placeholders := splitMultiValue(strVal, argCounter)
				conditions = append(conditions, fmt.Sprintf("%s IN (%s)", dbColumn, strings.Join(placeholders, ",")))
				args = append(args, items...)
				argCounter += len(items)
			} else {
				conditions = append(conditions, fmt.Sprintf("%s = $%d", dbColumn, argCounter))
				args = append(args, value)
				argCounter++
			}
		}
	}

	var whereClause string
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(id) FROM %s %s", otdelTable, whereClause)
	var total uint64
	if err := r.storage.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []entities.Otdel{}, 0, nil
	}

	orderByClause := "ORDER BY id DESC"
	// (Логика сортировки)

	limitClause, paginationArgs := buildPagination(filter, argCounter)
	finalArgs := append(args, paginationArgs...)

	selectFields := "id, name, status_id, department_id, created_at, updated_at"
	query := fmt.Sprintf("SELECT %s FROM %s %s %s %s", selectFields, otdelTable, whereClause, orderByClause, limitClause)

	rows, err := r.storage.Query(ctx, query, finalArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	otdels := make([]entities.Otdel, 0)
	for rows.Next() {
		o, err := scanOtdel(rows)
		if err != nil {
			return nil, 0, err
		}
		otdels = append(otdels, *o)
	}
	return otdels, total, rows.Err()
}

func (r *OtdelRepository) FindOtdel(ctx context.Context, id uint64) (*entities.Otdel, error) {
	query := "SELECT id, name, status_id, department_id, created_at, updated_at FROM otdels WHERE id = $1"
	return scanOtdel(r.storage.QueryRow(ctx, query, id))
}

func (r *OtdelRepository) CreateOtdel(ctx context.Context, otdel entities.Otdel) (*entities.Otdel, error) {
	query := `INSERT INTO otdels (name, status_id, department_id) VALUES ($1, $2, $3)
		RETURNING id, name, status_id, department_id, created_at, updated_at`
	return scanOtdel(r.storage.QueryRow(ctx, query, otdel.Name, otdel.StatusID, otdel.DepartmentsID))
}

func (r *OtdelRepository) UpdateOtdel(ctx context.Context, id uint64, otdel entities.Otdel) (*entities.Otdel, error) {
	query := `UPDATE otdels SET name=$1, status_id=$2, department_id=$3, updated_at=NOW()
		WHERE id=$4 RETURNING id, name, status_id, department_id, created_at, updated_at`
	return scanOtdel(r.storage.QueryRow(ctx, query, otdel.Name, otdel.StatusID, otdel.DepartmentsID, id))
}

func (r *OtdelRepository) DeleteOtdel(ctx context.Context, id uint64) error {
	query := "DELETE FROM otdels WHERE id = $1"
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// Хелперы для чистоты кода
func splitMultiValue(val string, startIdx int) ([]interface{}, []string) {
	items := strings.Split(val, ",")
	placeholders := make([]string, len(items))
	args := make([]interface{}, len(items))
	for i, item := range items {
		placeholders[i] = fmt.Sprintf("$%d", startIdx+i)
		args[i] = item
	}
	return args, placeholders
}

func buildPagination(filter types.Filter, startIdx int) (string, []interface{}) {
	if !filter.WithPagination {
		return "", nil
	}
	return fmt.Sprintf("LIMIT $%d OFFSET $%d", startIdx, startIdx+1), []interface{}{filter.Limit, filter.Offset}
}
