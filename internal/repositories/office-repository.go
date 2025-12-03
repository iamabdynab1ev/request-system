// Файл: internal/repositories/office-repository.go
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"request-system/internal/dto"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
)

const officeTable = "offices"

var (
	officeAllowedFilterFields = map[string]string{
		"status_id": "o.status_id",
		"branch_id": "o.branch_id",
		"parent_id": "o.parent_id",
	}
	officeAllowedSortFields = map[string]string{
		"id": "o.id", "name": "o.name", "open_date": "o.open_date", "created_at": "o.created_at",
	}
)

type OfficeRepositoryInterface interface {
	GetOffices(ctx context.Context, filter types.Filter) ([]entities.Office, uint64, error)
	FindOffice(ctx context.Context, id uint64) (*entities.Office, error)
	DeleteOffice(ctx context.Context, id uint64) error
	FindByName(ctx context.Context, tx pgx.Tx, name string) (*entities.Office, error)
	UpdateOfficeWithDTO(ctx context.Context, id uint64, dto dto.UpdateOfficeDTO) error

	CreateOffice(ctx context.Context, tx pgx.Tx, office entities.Office) (uint64, error)
	UpdateOffice(ctx context.Context, tx pgx.Tx, id uint64, office entities.Office) error
	FindByExternalID(ctx context.Context, tx pgx.Tx, externalID string, sourceSystem string) (*entities.Office, error)
}

type OfficeRepository struct {
	storage *pgxpool.Pool
	logger  *zap.Logger
}

func NewOfficeRepository(storage *pgxpool.Pool, logger *zap.Logger) OfficeRepositoryInterface {
	return &OfficeRepository{storage: storage, logger: logger}
}

func (r *OfficeRepository) scanOffice(row pgx.Row) (*entities.Office, error) {
	var o entities.Office
	var externalID, sourceSystem sql.NullString

	err := row.Scan(
		&o.ID, &o.Name, &o.Address, &o.OpenDate, &o.BranchID, &o.ParentID,
		&o.StatusID, &o.CreatedAt, &o.UpdatedAt, &externalID, &sourceSystem,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		r.logger.Error("Failed to scan office row", zap.Error(err))
		return nil, err
	}
	if externalID.Valid {
		o.ExternalID = &externalID.String
	}
	if sourceSystem.Valid {
		o.SourceSystem = &sourceSystem.String
	}
	return &o, nil
}

func (r *OfficeRepository) CreateOffice(ctx context.Context, tx pgx.Tx, office entities.Office) (uint64, error) {
	query := `INSERT INTO offices (name, address, open_date, branch_id, parent_id, status_id, external_id, source_system, created_at, updated_at) 
			  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW()) RETURNING id`
	var newID uint64
	err := tx.QueryRow(ctx, query,
		office.Name, office.Address, office.OpenDate, office.BranchID, office.ParentID, office.StatusID, office.ExternalID, office.SourceSystem).Scan(&newID)
	return newID, err
}

func (r *OfficeRepository) UpdateOffice(ctx context.Context, tx pgx.Tx, id uint64, office entities.Office) error {
	query := `UPDATE offices SET name = $1, address = $2, open_date = $3, branch_id = $4, parent_id = $5, status_id = $6, updated_at = NOW() WHERE id = $7`
	result, err := tx.Exec(ctx, query,
		office.Name, office.Address, office.OpenDate, office.BranchID, office.ParentID, office.StatusID, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *OfficeRepository) findOneOffice(ctx context.Context, querier Querier, where sq.Eq) (*entities.Office, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	query, args, err := psql.Select("id, name, address, open_date, branch_id, parent_id, status_id, created_at, updated_at, external_id, source_system").
		From(officeTable).Where(where).ToSql()
	if err != nil {
		return nil, err
	}
	return r.scanOffice(querier.QueryRow(ctx, query, args...))
}

func (r *OfficeRepository) FindByExternalID(ctx context.Context, tx pgx.Tx, externalID string, sourceSystem string) (*entities.Office, error) {
	var querier Querier = r.storage
	if tx != nil {
		querier = tx
	}
	return r.findOneOffice(ctx, querier, sq.Eq{"external_id": externalID, "source_system": sourceSystem})
}

func (r *OfficeRepository) FindByName(ctx context.Context, tx pgx.Tx, name string) (*entities.Office, error) {
	var querier Querier = r.storage
	if tx != nil {
		querier = tx
	}
	return r.findOneOffice(ctx, querier, sq.Eq{"name": name})
}

func (r *OfficeRepository) DeleteOffice(ctx context.Context, id uint64) error {
	query := `DELETE FROM offices WHERE id = $1`
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// --- ХЕЛПЕРЫ И МЕТОДЫ, ПЕРЕПИСАННЫЕ НА SQUIRREL ---

func (r *OfficeRepository) scanOfficeWithRelations(row pgx.Row) (*entities.Office, error) {
	var o entities.Office
	var b entities.Branch
	var s entities.Status
	var p entities.Office
	var parentName sql.NullString

	err := row.Scan(
		&o.ID, &o.Name, &o.Address, &o.OpenDate, &o.CreatedAt, &o.UpdatedAt,
		&o.BranchID, &b.Name, &b.ShortName,
		&o.StatusID, &s.Name,
		&o.ParentID, &parentName,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		r.logger.Error("Failed to scan office row with relations", zap.Error(err))
		return nil, err
	}
	if o.BranchID != nil {
		b.ID = *o.BranchID
		o.Branch = &b
	}
	s.ID = o.StatusID
	o.Status = &s
	if o.ParentID != nil {
		p.ID = *o.ParentID
		if parentName.Valid {
			p.Name = parentName.String
		}
		o.Parent = &p
	}
	return &o, nil
}

func (r *OfficeRepository) applyFilters(builder sq.SelectBuilder, filter types.Filter) sq.SelectBuilder {
	if filter.Search != "" {
		builder = builder.Where(sq.Or{
			sq.ILike{"o.name": "%" + filter.Search + "%"},
			sq.ILike{"o.address": "%" + filter.Search + "%"},
		})
	}
	for key, value := range filter.Filter {
		if dbColumn, ok := officeAllowedFilterFields[key]; ok {
			if items, ok := value.(string); ok && strings.Contains(items, ",") {
				builder = builder.Where(sq.Eq{dbColumn: strings.Split(items, ",")})
			} else {
				builder = builder.Where(sq.Eq{dbColumn: value})
			}
		}
	}
	return builder
}

func (r *OfficeRepository) GetOffices(ctx context.Context, filter types.Filter) ([]entities.Office, uint64, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	baseBuilder := psql.Select().From(officeTable + " AS o")

	baseBuilder = r.applyFilters(baseBuilder, filter)

	countBuilder := baseBuilder.Columns("COUNT(o.id)")
	countQuery, countArgs, err := countBuilder.ToSql()
	if err != nil {
		return nil, 0, err
	}
	var total uint64
	if err := r.storage.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []entities.Office{}, 0, nil
	}

	selectBuilder := baseBuilder.Columns(
		"o.id", "o.name", "o.address", "o.open_date", "o.created_at", "o.updated_at",
		"o.branch_id", "COALESCE(b.name, '')", "COALESCE(b.short_name, '')",
		"o.status_id", "COALESCE(s.name, '')",
		"o.parent_id", "COALESCE(p_o.name, '') AS parent_name",
	).
		LeftJoin("branches b ON o.branch_id = b.id").
		LeftJoin("statuses s ON o.status_id = s.id").
		LeftJoin(officeTable + " p_o ON o.parent_id = p_o.id")
	if len(filter.Sort) > 0 {
		for field, direction := range filter.Sort {
			if dbColumn, ok := officeAllowedSortFields[field]; ok {
				order := "ASC"
				if strings.ToUpper(direction) == "DESC" {
					order = "DESC"
				}
				selectBuilder = selectBuilder.OrderBy(fmt.Sprintf("%s %s", dbColumn, order))
			}
		}
	} else {
		selectBuilder = selectBuilder.OrderBy("o.id DESC")
	}

	if filter.WithPagination {
		selectBuilder = selectBuilder.Limit(uint64(filter.Limit)).Offset(uint64(filter.Offset))
	}

	query, args, err := selectBuilder.ToSql()
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.storage.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	offices := make([]entities.Office, 0, filter.Limit)
	for rows.Next() {
		office, err := r.scanOfficeWithRelations(rows)
		if err != nil {
			return nil, 0, err
		}
		offices = append(offices, *office)
	}
	return offices, total, rows.Err()
}

func (r *OfficeRepository) FindOffice(ctx context.Context, id uint64) (*entities.Office, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	query, args, err := psql.Select(
		"o.id", "o.name", "o.address", "o.open_date", "o.created_at", "o.updated_at",
		"o.branch_id", "COALESCE(b.name, '')", "COALESCE(b.short_name, '')",
		"o.status_id", "COALESCE(s.name, '')",
		"o.parent_id", "COALESCE(p_o.name, '') AS parent_name",
	).
		From(officeTable + " AS o").
		LeftJoin("branches b ON o.branch_id = b.id").
		LeftJoin("statuses s ON o.status_id = s.id").
		LeftJoin(officeTable + " p_o ON o.parent_id = p_o.id").
		Where(sq.Eq{"o.id": id}).
		ToSql()
	if err != nil {
		return nil, err
	}

	return r.scanOfficeWithRelations(r.storage.QueryRow(ctx, query, args...))
}

func (r *OfficeRepository) UpdateOfficeWithDTO(ctx context.Context, id uint64, dto dto.UpdateOfficeDTO) error {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	updateBuilder := psql.Update(officeTable).Where(sq.Eq{"id": id}).Set("updated_at", sq.Expr("NOW()"))

	hasChanges := false
	if dto.Name != nil {
		updateBuilder = updateBuilder.Set("name", *dto.Name)
		hasChanges = true
	}
	if dto.Address != nil {
		updateBuilder = updateBuilder.Set("address", *dto.Address)
		hasChanges = true
	}
	if dto.BranchID != nil {
		updateBuilder = updateBuilder.Set("branch_id", *dto.BranchID).Set("parent_id", nil)
		hasChanges = true
	}
	if dto.ParentID != nil {
		updateBuilder = updateBuilder.Set("parent_id", *dto.ParentID).Set("branch_id", nil)
		hasChanges = true
	}
	if dto.StatusID != nil {
		updateBuilder = updateBuilder.Set("status_id", *dto.StatusID)
		hasChanges = true
	}
	if dto.OpenDate != nil {
		openDate, err := time.Parse("2006-01-02", *dto.OpenDate)
		if err != nil {
			return apperrors.NewBadRequestError("Неверный формат даты")
		}
		updateBuilder = updateBuilder.Set("open_date", openDate)
		hasChanges = true
	}

	if !hasChanges {
		return nil
	}

	query, args, err := updateBuilder.ToSql()
	if err != nil {
		return err
	}

	result, err := r.storage.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}

	return nil
}
