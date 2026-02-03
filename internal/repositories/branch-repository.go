package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"request-system/internal/entities"
	// ВАЖНО: Подключаем твой билдер
	"request-system/internal/infrastructure/bd"
	
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
)

const branchTable = "branches"

// ЕДИНАЯ КАРТА ПОЛЕЙ (Фильтр + Сортировка)
var branchMap = map[string]string{
	"id":           "b.id",
	"name":         "b.name",
	"short_name":   "b.short_name",
	"status_id":    "b.status_id",
	"address":      "b.address",
	"email":        "b.email",
	"open_date":    "b.open_date",
	"created_at":   "b.created_at",
	"updated_at":   "b.updated_at",
}

// Интерфейс
type BranchRepositoryInterface interface {
	GetBranches(ctx context.Context, filter types.Filter) ([]entities.Branch, uint64, error)
	FindBranch(ctx context.Context, id uint64) (*entities.Branch, error)
	DeleteBranch(ctx context.Context, id uint64) error
	CreateBranch(ctx context.Context, tx pgx.Tx, branch entities.Branch) (uint64, error)
	UpdateBranch(ctx context.Context, tx pgx.Tx, id uint64, branch entities.Branch) error
	FindByExternalID(ctx context.Context, tx pgx.Tx, externalID string, sourceSystem string) (*entities.Branch, error)
}

type BranchRepository struct {
	storage *pgxpool.Pool
	logger  *zap.Logger
}

func NewBranchRepository(storage *pgxpool.Pool, logger *zap.Logger) BranchRepositoryInterface {
	return &BranchRepository{storage: storage, logger: logger}
}

// -----------------------------------------------------------
// SCAN
// -----------------------------------------------------------

func scanBranch(row pgx.Row) (*entities.Branch, error) {
	var b entities.Branch
	var s entities.Status
	// Для nullable полей
	var externalId, sourceSystem, address, phone, email, emailIndex sql.NullString
	var openDate sql.NullTime

	err := row.Scan(
		&b.ID, &b.Name, &b.ShortName, &address, &phone, &email, &emailIndex,
		&openDate, &b.StatusID, &externalId, &sourceSystem,
		&b.CreatedAt, &b.UpdatedAt,
		&s.ID, &s.Name,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ошибка сканирования branch: %w", err)
	}

	if address.Valid {
		b.Address = &address.String
	}
	if phone.Valid {
		b.PhoneNumber = &phone.String
	}
	if email.Valid {
		b.Email = &email.String
	}
	if emailIndex.Valid {
		b.EmailIndex = &emailIndex.String
	}
	if openDate.Valid {
		b.OpenDate = &openDate.Time
	}
	if externalId.Valid {
		b.ExternalID = &externalId.String
	}
	if sourceSystem.Valid {
		b.SourceSystem = &sourceSystem.String
	}
	// Если Join на Status отработал
	if s.ID > 0 {
		b.Status = &s
	}

	return &b, nil
}

// -----------------------------------------------------------
// GET (Список) - ИСПОЛЬЗУЕМ HELPER BD
// -----------------------------------------------------------
func (r *BranchRepository) GetBranches(ctx context.Context, filter types.Filter) ([]entities.Branch, uint64, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	
	// Ручной поиск
	applySearch := func(b sq.SelectBuilder) sq.SelectBuilder {
		if filter.Search != "" {
			pat := "%" + filter.Search + "%"
			return b.Where(sq.Or{
				sq.ILike{"b.name": pat},
				sq.ILike{"b.short_name": pat},
			})
		}
		return b
	}

	// 1. COUNT
	countBuilder := psql.Select("COUNT(b.id)").From("branches AS b")
	
	countBuilder = applySearch(countBuilder)

	countFilter := filter
	countFilter.WithPagination = false
	countFilter.Sort = nil

	// Helper для count
	countBuilder = bd.ApplyListParams(countBuilder, countFilter, branchMap)

	var total uint64
	sqlCount, argsCount, _ := countBuilder.ToSql()
	if err := r.storage.QueryRow(ctx, sqlCount, argsCount...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []entities.Branch{}, 0, nil
	}

	// 2. SELECT
	baseBuilder := psql.Select(
		"b.id", "b.name", "b.short_name", "b.address", "b.phone_number", "b.email", "b.email_index",
		"b.open_date", "b.status_id", "b.external_id", "b.source_system",
		"b.created_at", "b.updated_at",
		"COALESCE(s.id, 0)", "COALESCE(s.name, '')",
	).From("branches AS b").LeftJoin("statuses s ON b.status_id = s.id")

	baseBuilder = applySearch(baseBuilder)

	if len(filter.Sort) == 0 {
		baseBuilder = baseBuilder.OrderBy("b.id DESC")
	}

	// Helper для select
	baseBuilder = bd.ApplyListParams(baseBuilder, filter, branchMap)

	query, args, err := baseBuilder.ToSql()
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.storage.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	branches := make([]entities.Branch, 0, filter.Limit)
	for rows.Next() {
		branch, err := scanBranch(rows)
		if err != nil {
			return nil, 0, err
		}
		branches = append(branches, *branch)
	}

	return branches, total, nil
}

// -----------------------------------------------------------
// FIND ONE
// -----------------------------------------------------------
func (r *BranchRepository) findOne(ctx context.Context, querier Querier, where sq.Eq) (*entities.Branch, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	queryBuilder := psql.Select(
		"b.id", "b.name", "b.short_name", "b.address", "b.phone_number", "b.email", "b.email_index",
		"b.open_date", "b.status_id", "b.external_id", "b.source_system",
		"b.created_at", "b.updated_at",
		"COALESCE(s.id, 0)", "COALESCE(s.name, '')",
	).From("branches b").LeftJoin("statuses s ON b.status_id = s.id").Where(where)

	sql, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, err
	}
	return scanBranch(querier.QueryRow(ctx, sql, args...))
}

func (r *BranchRepository) FindBranch(ctx context.Context, id uint64) (*entities.Branch, error) {
	return r.findOne(ctx, r.storage, sq.Eq{"b.id": id})
}

func (r *BranchRepository) FindByExternalID(ctx context.Context, tx pgx.Tx, externalID string, sourceSystem string) (*entities.Branch, error) {
	var querier Querier = r.storage
	if tx != nil {
		querier = tx
	}
	return r.findOne(ctx, querier, sq.Eq{"b.external_id": externalID, "b.source_system": sourceSystem})
}

// -----------------------------------------------------------
// CRUD
// -----------------------------------------------------------

func (r *BranchRepository) CreateBranch(ctx context.Context, tx pgx.Tx, branch entities.Branch) (uint64, error) {
	query := `
		INSERT INTO branches (name, short_name, address, phone_number, email, email_index, open_date, status_id, external_id, source_system, created_at, updated_at)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		RETURNING id
	`
	var newID uint64
	err := tx.QueryRow(ctx, query,
		branch.Name, branch.ShortName, branch.Address, branch.PhoneNumber,
		branch.Email, branch.EmailIndex, branch.OpenDate, branch.StatusID,
		branch.ExternalID, branch.SourceSystem,
	).Scan(&newID)

	return newID, err
}

func (r *BranchRepository) UpdateBranch(ctx context.Context, tx pgx.Tx, id uint64, branch entities.Branch) error {
	query := `
		UPDATE branches
		SET name = $1, short_name = $2, address = $3, phone_number = $4, email = $5,
		    email_index = $6, open_date = $7, status_id = $8, updated_at = NOW()
		WHERE id = $9
	`
	result, err := tx.Exec(ctx, query,
		branch.Name, branch.ShortName, branch.Address, branch.PhoneNumber,
		branch.Email, branch.EmailIndex, branch.OpenDate, branch.StatusID, id,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *BranchRepository) DeleteBranch(ctx context.Context, id uint64) error {
	query := `DELETE FROM branches WHERE id = $1`
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
