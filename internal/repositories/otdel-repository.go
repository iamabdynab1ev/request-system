// Файл: internal/repositories/otdel-repository.go
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

"request-system/internal/infrastructure/bd"
	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
)

const otdelTable = "otdels"

var (
	otdelAllowedFilterFields = map[string]string{
		"status_id":      "status_id",
		"department_id": "department_id",
		"branch_id":      "branch_id",
		"parent_id":      "parent_id",
	}
	otdelAllowedSortFields = map[string]bool{"id": true, "name": true, "created_at": true, "updated_at": true}
)
var otdelMap = map[string]string{
	"id":             "id",
	"name":           "name",
	"created_at":     "created_at",
	"updated_at":     "updated_at",
	"status_id":      "status_id",
	"branch_id":      "branch_id",
	"department_id":  "departments_id",
	"departments_id": "departments_id",
	"parent_id":      "parent_id",
}

// OtdelRepositoryInterface - полный и актуальный интерфейс.
type OtdelRepositoryInterface interface {
	GetOtdels(ctx context.Context, filter types.Filter) ([]entities.Otdel, uint64, error)
	FindOtdel(ctx context.Context, id uint64) (*entities.Otdel, error)
	DeleteOtdel(ctx context.Context, id uint64) error
	CreateOtdel(ctx context.Context, tx pgx.Tx, otdel entities.Otdel) (uint64, error)
	UpdateOtdel(ctx context.Context, tx pgx.Tx, id uint64, otdel entities.Otdel) error
	FindByExternalID(ctx context.Context, tx pgx.Tx, externalID string, sourceSystem string) (*entities.Otdel, error)
	ValidateOtdelsInDepartment(ctx context.Context, deptID uint64, otdelIDs []uint64) (bool, error)
}

type OtdelRepository struct {
	storage *pgxpool.Pool
	logger  *zap.Logger
}

func NewOtdelRepository(storage *pgxpool.Pool, logger *zap.Logger) OtdelRepositoryInterface {
	return &OtdelRepository{storage: storage, logger: logger}
}

// scanOtdel - обновлена для сканирования всех полей.
func scanOtdel(row pgx.Row) (*entities.Otdel, error) {
	var o entities.Otdel
	var externalID, sourceSystem sql.NullString

	// ИЗМЕНЕНИЕ: Добавлены o.BranchID и o.ParentID
	err := row.Scan(
		&o.ID,
		&o.Name,
		&o.StatusID,
		&o.DepartmentsID,
		&o.BranchID,
		&o.ParentID,
		&o.CreatedAt,
		&o.UpdatedAt,
		&externalID,
		&sourceSystem,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ошибка сканирования otdel: %w", err)
	}

	if externalID.Valid {
		o.ExternalID = &externalID.String
	}
	if sourceSystem.Valid {
		o.SourceSystem = &sourceSystem.String
	}

	return &o, nil
}

func (r *OtdelRepository) CreateOtdel(ctx context.Context, tx pgx.Tx, otdel entities.Otdel) (uint64, error) {
	query := `
		INSERT INTO otdels (name, status_id, departments_id, branch_id, parent_id, external_id, source_system, created_at, updated_at)
		VALUES($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		RETURNING id
	`
	var newID uint64
	err := tx.QueryRow(ctx, query,
		otdel.Name, otdel.StatusID, otdel.DepartmentsID, otdel.BranchID, otdel.ParentID,
		otdel.ExternalID, otdel.SourceSystem,
	).Scan(&newID)
	return newID, err
}

func (r *OtdelRepository) UpdateOtdel(ctx context.Context, tx pgx.Tx, id uint64, otdel entities.Otdel) error {
	query := `
		UPDATE otdels
		SET name = $1, status_id = $2, departments_id = $3, branch_id = $4, parent_id = $5, updated_at = NOW()
		WHERE id = $6
	`
	result, err := tx.Exec(ctx, query,
		otdel.Name, otdel.StatusID, otdel.DepartmentsID, otdel.BranchID, otdel.ParentID, id,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *OtdelRepository) findOneOtdel(ctx context.Context, querier Querier, where sq.Eq) (*entities.Otdel, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

	queryBuilder := psql.Select("id, name, status_id, departments_id, branch_id, parent_id, created_at, updated_at, external_id, source_system").
		From(otdelTable).
		Where(where)

	sql, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, err
	}

	return scanOtdel(querier.QueryRow(ctx, sql, args...))
}

func (r *OtdelRepository) FindByExternalID(ctx context.Context, tx pgx.Tx, externalID string, sourceSystem string) (*entities.Otdel, error) {
	var querier Querier = r.storage
	if tx != nil {
		querier = tx
	}
	return r.findOneOtdel(ctx, querier, sq.Eq{"external_id": externalID, "source_system": sourceSystem})
}

func (r *OtdelRepository) FindOtdel(ctx context.Context, id uint64) (*entities.Otdel, error) {
	return r.findOneOtdel(ctx, r.storage, sq.Eq{"id": id})
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

func (r *OtdelRepository) GetOtdels(ctx context.Context, filter types.Filter) ([]entities.Otdel, uint64, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

	// --- 1. COUNT ---
	countBuilder := psql.Select("COUNT(id)").From(otdelTable)

	// Ручной поиск по тексту оставляем здесь (это не точное совпадение)
	if filter.Search != "" {
		countBuilder = countBuilder.Where(sq.ILike{"name": "%" + filter.Search + "%"})
	}

	// Копируем фильтр для Count (отключаем лишнее)
	countFilter := filter
	countFilter.WithPagination = false
	countFilter.Sort = nil

	// Используем BD Helper
	countBuilder = bd.ApplyListParams(countBuilder, countFilter, otdelMap)

	var total uint64
	sqlCount, argsCount, _ := countBuilder.ToSql()
	if err := r.storage.QueryRow(ctx, sqlCount, argsCount...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []entities.Otdel{}, 0, nil
	}

	// --- 2. SELECT ---
	baseBuilder := psql.Select("id, name, status_id, departments_id, branch_id, parent_id, created_at, updated_at, external_id, source_system").From(otdelTable)

	if filter.Search != "" {
		baseBuilder = baseBuilder.Where(sq.ILike{"name": "%" + filter.Search + "%"})
	}

	// Сортировка по умолчанию
	if len(filter.Sort) == 0 {
		baseBuilder = baseBuilder.OrderBy("id DESC")
	}

	// Используем BD Helper (здесь применится фильтр, сортировка и пагинация)
	baseBuilder = bd.ApplyListParams(baseBuilder, filter, otdelMap)

	sqlSelect, argsSelect, err := baseBuilder.ToSql()
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.storage.Query(ctx, sqlSelect, argsSelect...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	otdels := make([]entities.Otdel, 0)
	for rows.Next() {
		o, err := scanOtdel(rows) // Эта функция у тебя ниже в файле
		if err != nil {
			return nil, 0, err
		}
		otdels = append(otdels, *o)
	}
	return otdels, total, nil
}

func (r *OtdelRepository) ValidateOtdelsInDepartment(ctx context.Context, deptID uint64, otdelIDs []uint64) (bool, error) {
	if len(otdelIDs) == 0 {
		return true, nil
	}
	query := `SELECT COUNT(DISTINCT id) FROM otdels WHERE departments_id = $1 AND id = ANY($2)`
	
	var count int
	err := r.storage.QueryRow(ctx, query, deptID, otdelIDs).Scan(&count)
	if err != nil {
		return false, err
	}

	uniqueInput := make(map[uint64]bool)
	for _, id := range otdelIDs {
		if id > 0 { uniqueInput[id] = true }
	}

	return count == len(uniqueInput), nil
}
