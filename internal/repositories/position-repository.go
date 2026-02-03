package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"


	sq "github.com/Masterminds/squirrel"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"request-system/internal/entities"
	// ВАЖНО: Подключаем билдер
	"request-system/internal/infrastructure/bd" 
	
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
)

const (
	positionTable  = "positions"
	positionFields = `id, name, department_id, otdel_id, branch_id, office_id, type, status_id, created_at, updated_at, external_id, source_system`
)

var positionMap = map[string]string{
	"id":            "id",
	"name":          "name",
	"status_id":     "status_id",
	"type":          "type",
	"department_id": "department_id",
	"otdel_id":      "otdel_id",
	"branch_id":     "branch_id",
	"office_id":     "office_id",
	"created_at":    "created_at",
	"updated_at":    "updated_at",
}

type PositionRepositoryInterface interface {
	FindByID(ctx context.Context, tx pgx.Tx, id uint64) (*entities.Position, error)
	GetAll(ctx context.Context, filter types.Filter) ([]*entities.Position, uint64, error)
	Create(ctx context.Context, tx pgx.Tx, p entities.Position) (uint64, error)
	Update(ctx context.Context, tx pgx.Tx, id uint64, p entities.Position) error
	Delete(ctx context.Context, tx pgx.Tx, id int) error

	FindByExternalID(ctx context.Context, tx pgx.Tx, externalID string, sourceSystem string) (*entities.Position, error)
	FindByName(ctx context.Context, tx pgx.Tx, name string) (*entities.Position, error)
	FindByTypeAndOrg(ctx context.Context, tx pgx.Tx, posType string, depID *uint64, otdelID *uint64) ([]*entities.Position, error)
	FindIDByType(ctx context.Context, tx pgx.Tx, typeName string) (uint64, error)
}

type positionRepository struct {
	storage *pgxpool.Pool
	logger  *zap.Logger
}

func NewPositionRepository(storage *pgxpool.Pool, logger *zap.Logger) PositionRepositoryInterface {
	return &positionRepository{storage: storage, logger: logger}
}

// -------------------------------------------------------------
// Хелперы (Querier, Scan, FindOne)
// -------------------------------------------------------------
func (r *positionRepository) getQuerier(tx pgx.Tx) Querier {
	if tx != nil { return tx }
	return r.storage
}

func (r *positionRepository) scanRow(row pgx.Row) (*entities.Position, error) {
	var p entities.Position
	var externalID, sourceSystem sql.NullString

	err := row.Scan(
		&p.ID, &p.Name, &p.DepartmentID, &p.OtdelID, &p.BranchID, &p.OfficeID,
		&p.Type, &p.StatusID, &p.CreatedAt, &p.UpdatedAt,
		&externalID, &sourceSystem,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return nil, apperrors.ErrNotFound }
		return nil, fmt.Errorf("ошибка сканирования positions: %w", err)
	}
	if externalID.Valid { p.ExternalID = &externalID.String }
	if sourceSystem.Valid { p.SourceSystem = &sourceSystem.String }
	return &p, nil
}

func (r *positionRepository) findOnePosition(ctx context.Context, querier Querier, where sq.Eq) (*entities.Position, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	query, args, err := psql.Select(positionFields).From(positionTable).Where(where).ToSql()
	if err != nil {
		return nil, fmt.Errorf("ошибка сборки SQL для findOnePosition: %w", err)
	}
	return r.scanRow(querier.QueryRow(ctx, query, args...))
}

// -------------------------------------------------------------
// GetAll - ЧЕРЕЗ BD HELPER
// -------------------------------------------------------------
func (r *positionRepository) GetAll(ctx context.Context, filter types.Filter) ([]*entities.Position, uint64, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	
	// Ручной поиск (OR ILIKE)
	applySearch := func(b sq.SelectBuilder) sq.SelectBuilder {
		if filter.Search != "" {
			return b.Where(sq.ILike{"name": "%" + filter.Search + "%"})
		}
		return b
	}

	// 1. COUNT
	countBuilder := psql.Select("COUNT(id)").From(positionTable)
	countBuilder = applySearch(countBuilder)

	countFilter := filter
	countFilter.WithPagination = false
	countFilter.Sort = nil

	// Хелпер
	countBuilder = bd.ApplyListParams(countBuilder, countFilter, positionMap)

	countQuery, countArgs, err := countBuilder.ToSql()
	if err != nil { return nil, 0, fmt.Errorf("ошибка сборки SQL count: %w", err) }

	var total uint64
	if err := r.storage.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("ошибка выполнения count: %w", err)
	}
	if total == 0 { return []*entities.Position{}, 0, nil }

	// 2. SELECT
	selectBuilder := psql.Select(positionFields).From(positionTable)
	selectBuilder = applySearch(selectBuilder)

	if len(filter.Sort) == 0 {
		selectBuilder = selectBuilder.OrderBy("id DESC")
	}

	// Хелпер
	selectBuilder = bd.ApplyListParams(selectBuilder, filter, positionMap)

	query, args, err := selectBuilder.ToSql()
	if err != nil { return nil, 0, fmt.Errorf("ошибка сборки SQL select: %w", err) }

	rows, err := r.storage.Query(ctx, query, args...)
	if err != nil { return nil, 0, fmt.Errorf("ошибка выполнения select: %w", err) }
	defer rows.Close()

	positions := make([]*entities.Position, 0)
	for rows.Next() {
		pos, err := r.scanRow(rows)
		if err != nil { return nil, 0, err }
		positions = append(positions, pos)
	}

	if err := rows.Err(); err != nil { return nil, 0, fmt.Errorf("ошибка итерации rows: %w", err) }

	return positions, total, nil
}

// -------------------------------------------------------------
// CRUD
// -------------------------------------------------------------
func (r *positionRepository) FindByID(ctx context.Context, tx pgx.Tx, id uint64) (*entities.Position, error) {
	return r.findOnePosition(ctx, r.getQuerier(tx), sq.Eq{"id": id})
}
func (r *positionRepository) FindByName(ctx context.Context, tx pgx.Tx, name string) (*entities.Position, error) {
	return r.findOnePosition(ctx, r.getQuerier(tx), sq.Eq{"name": name})
}
func (r *positionRepository) FindByExternalID(ctx context.Context, tx pgx.Tx, externalID string, sourceSystem string) (*entities.Position, error) {
	return r.findOnePosition(ctx, r.getQuerier(tx), sq.Eq{"external_id": externalID, "source_system": sourceSystem})
}
func (r *positionRepository) FindIDByType(ctx context.Context, tx pgx.Tx, typeName string) (uint64, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	query, args, err := psql.Select("id").From(positionTable).Where(sq.Eq{"type": typeName}).Limit(1).ToSql()
	if err != nil { return 0, fmt.Errorf("ошибка сборки FindIDByType: %w", err) }

	var id uint64
	if err := r.getQuerier(tx).QueryRow(ctx, query, args...).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return 0, apperrors.ErrNotFound }
		return 0, err
	}
	return id, nil
}

func (r *positionRepository) Create(ctx context.Context, tx pgx.Tx, p entities.Position) (uint64, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	query, args, err := psql.Insert(positionTable).
		Columns("name", "department_id", "otdel_id", "branch_id", "office_id", "type", "status_id", "external_id", "source_system", "created_at", "updated_at").
		Values(p.Name, p.DepartmentID, p.OtdelID, p.BranchID, p.OfficeID, p.Type, p.StatusID, p.ExternalID, p.SourceSystem, sq.Expr("NOW()"), sq.Expr("NOW()")).
		Suffix("RETURNING id").ToSql()
	if err != nil { return 0, fmt.Errorf("ошибка сборки Create: %w", err) }

	var newID uint64
	if err := tx.QueryRow(ctx, query, args...).Scan(&newID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return 0, fmt.Errorf("должность уже существует: %w", apperrors.ErrConflict)
		}
		return 0, fmt.Errorf("ошибка создания positions: %w", err)
	}
	return newID, nil
}

func (r *positionRepository) Update(ctx context.Context, tx pgx.Tx, id uint64, p entities.Position) error {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	query, args, err := psql.Update(positionTable).
		Set("name", p.Name).
		Set("department_id", p.DepartmentID).
		Set("otdel_id", p.OtdelID).
		Set("branch_id", p.BranchID).
		Set("office_id", p.OfficeID).
		Set("type", p.Type).
		Set("status_id", p.StatusID).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": id}).ToSql()
	if err != nil { return fmt.Errorf("ошибка сборки Update: %w", err) }

	result, err := tx.Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("должность уже существует: %w", apperrors.ErrConflict)
		}
		return fmt.Errorf("ошибка обновления positions: %w", err)
	}
	if result.RowsAffected() == 0 { return apperrors.ErrNotFound }
	return nil
}

func (r *positionRepository) Delete(ctx context.Context, tx pgx.Tx, id int) error {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	query, args, err := psql.Delete(positionTable).Where(sq.Eq{"id": id}).ToSql()
	if err != nil { return fmt.Errorf("ошибка сборки Delete: %w", err) }

	result, err := tx.Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return apperrors.NewHttpError(http.StatusBadRequest, "Запись используется", err, nil)
		}
		return fmt.Errorf("ошибка удаления: %w", err)
	}
	if result.RowsAffected() == 0 { return apperrors.ErrNotFound }
	return nil
}

func (r *positionRepository) FindByTypeAndOrg(ctx context.Context, tx pgx.Tx, posType string, depID *uint64, otdelID *uint64) ([]*entities.Position, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	builder := psql.Select(positionFields).From(positionTable).Where(sq.Eq{"type": posType}).OrderBy("id")

	if depID != nil { builder = builder.Where(sq.Eq{"department_id": *depID}) }
	if otdelID != nil { builder = builder.Where(sq.Eq{"otdel_id": *otdelID}) }

	query, args, err := builder.ToSql()
	if err != nil { return nil, fmt.Errorf("ошибка сборки FindByTypeAndOrg: %w", err) }

	rows, err := r.getQuerier(tx).Query(ctx, query, args...)
	if err != nil { return nil, fmt.Errorf("ошибка выполнения: %w", err) }
	defer rows.Close()

	var positions []*entities.Position
	for rows.Next() {
		pos, err := r.scanRow(rows)
		if err != nil { return nil, err }
		positions = append(positions, pos)
	}
	return positions, nil
}
