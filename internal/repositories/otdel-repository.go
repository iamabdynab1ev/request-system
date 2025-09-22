// repositories/otdel_repository.go
package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"

	"request-system/internal/dto"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const otdelTable = "otdels"

var (
	otdelAllowedFilterFields = map[string]string{"status_id": "status_id", "department_id": "department_id"}
	otdelAllowedSortFields   = map[string]string{"id": "id", "name": "name", "created_at": "created_at"}
)

type OtdelRepositoryInterface interface {
	GetOtdels(ctx context.Context, filter types.Filter) ([]entities.Otdel, uint64, error)
	FindOtdel(ctx context.Context, id uint64) (*entities.Otdel, error)
	CreateOtdel(ctx context.Context, otdel entities.Otdel) (*entities.Otdel, error)
	UpdateOtdel(ctx context.Context, id uint64, dto dto.UpdateOtdelDTO) (*entities.Otdel, error)
	DeleteOtdel(ctx context.Context, id uint64) error
}

type OtdelRepository struct {
	storage *pgxpool.Pool
	logger  *zap.Logger
}

func NewOtdelRepository(storage *pgxpool.Pool, logger *zap.Logger) OtdelRepositoryInterface {
	return &OtdelRepository{storage: storage, logger: logger}
}

func scanOtdel(row pgx.Row) (*entities.Otdel, error) {
	var o entities.Otdel
	err := row.Scan(&o.ID, &o.Name, &o.StatusID, &o.DepartmentsID, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ошибка сканирования otdel: %w", err)
	}
	return &o, nil
}

func (r *OtdelRepository) buildFilterQuery(filter types.Filter) (string, []interface{}) {
	conditions, args := []string{}, []interface{}{}
	argCounter := 1
	if filter.Search != "" {
		conditions, args = append(conditions, fmt.Sprintf("name ILIKE $%d", argCounter)), append(args, "%"+filter.Search+"%")
		argCounter++
	}
	for key, value := range filter.Filter {
		if dbColumn, ok := otdelAllowedFilterFields[key]; ok {
			items := strings.Split(fmt.Sprintf("%v", value), ",")
			if len(items) > 1 {
				placeholders := []string{}
				for _, item := range items {
					placeholders = append(placeholders, fmt.Sprintf("$%d", argCounter))
					args = append(args, item)
					argCounter++
				}
				conditions = append(conditions, fmt.Sprintf("%s IN (%s)", dbColumn, strings.Join(placeholders, ",")))
			} else {
				conditions = append(conditions, fmt.Sprintf("%s = $%d", dbColumn, argCounter))
				args = append(args, value)
				argCounter++
			}
		}
	}
	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func (r *OtdelRepository) GetOtdels(ctx context.Context, filter types.Filter) ([]entities.Otdel, uint64, error) {
	whereClause, args := r.buildFilterQuery(filter)
	countQuery := fmt.Sprintf("SELECT COUNT(id) FROM %s %s", otdelTable, whereClause)
	var total uint64
	if err := r.storage.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []entities.Otdel{}, 0, nil
	}

	orderByClause := "ORDER BY id DESC" // ... (Логика сортировки)

	limitClause := ""
	argCounter := len(args) + 1
	if filter.WithPagination {
		limitClause = fmt.Sprintf("LIMIT $%d OFFSET $%d", argCounter, argCounter+1)
		args = append(args, filter.Limit, filter.Offset)
	}

	query := fmt.Sprintf("SELECT id, name, status_id, department_id, created_at, updated_at FROM %s %s %s %s", otdelTable, whereClause, orderByClause, limitClause)

	rows, err := r.storage.Query(ctx, query, args...)
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

// --- ИСПРАВЛЕННЫЙ CREATE ---
func (r *OtdelRepository) CreateOtdel(ctx context.Context, otdel entities.Otdel) (*entities.Otdel, error) {
	// ID не передаем!
	query := `INSERT INTO otdels (name, status_id, department_id) VALUES ($1, $2, $3)
		RETURNING id, name, status_id, department_id, created_at, updated_at`

	return scanOtdel(r.storage.QueryRow(ctx, query, otdel.Name, otdel.StatusID, otdel.DepartmentsID))
}

// --- ИСПРАВЛЕННЫЙ ДИНАМИЧЕСКИЙ UPDATE ---
func (r *OtdelRepository) UpdateOtdel(ctx context.Context, id uint64, dto dto.UpdateOtdelDTO) (*entities.Otdel, error) {
	updateBuilder := sq.Update(otdelTable).
		PlaceholderFormat(sq.Dollar).
		Where(sq.Eq{"id": id}).
		Set("updated_at", sq.Expr("NOW()"))

	hasChanges := false
	if dto.Name != "" {
		updateBuilder = updateBuilder.Set("name", dto.Name)
		hasChanges = true
	}
	if dto.StatusID != 0 {
		updateBuilder = updateBuilder.Set("status_id", dto.StatusID)
		hasChanges = true
	}
	if dto.DepartmentsID != 0 {
		updateBuilder = updateBuilder.Set("department_id", dto.DepartmentsID)
		hasChanges = true
	}
	if !hasChanges {
		return r.FindOtdel(ctx, id)
	}
	query, args, err := updateBuilder.
		Suffix("RETURNING id, name, status_id, department_id, created_at, updated_at").
		ToSql()
	if err != nil {
		return nil, err
	}
	return scanOtdel(r.storage.QueryRow(ctx, query, args...))
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
