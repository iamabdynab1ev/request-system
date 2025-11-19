package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
)

const (
	positionTable  = "positions"
	positionFields = `id, name, department_id, otdel_id, branch_id, office_id, "type", status_id, created_at, updated_at, external_id, source_system`
)

var (
	positionAllowedFilterFields = map[string]string{ /*...*/ }
	positionAllowedSortFields   = map[string]bool{ /*...*/ }
)

// PositionRepositoryInterface - ИНТЕРФЕЙС ОБНОВЛЕН
type PositionRepositoryInterface interface {
	// Старые методы
	FindByID(ctx context.Context, tx pgx.Tx, id uint64) (*entities.Position, error)
	GetAll(ctx context.Context, filter types.Filter) ([]*entities.Position, uint64, error)
	FindByTypeAndOrg(ctx context.Context, tx pgx.Tx, posType string, depID *uint64, otdelID *uint64) ([]*entities.Position, error)
	Delete(ctx context.Context, tx pgx.Tx, id int) error

	// Методы для синхронизации. Имена сохранены, сигнатуры обновлены.
	Create(ctx context.Context, tx pgx.Tx, p entities.Position) (uint64, error)
	Update(ctx context.Context, tx pgx.Tx, id uint64, p entities.Position) error
	FindByExternalID(ctx context.Context, tx pgx.Tx, externalID string, sourceSystem string) (*entities.Position, error)
	FindByName(ctx context.Context, tx pgx.Tx, name string) (*entities.Position, error)
}

type positionRepository struct {
	storage *pgxpool.Pool
	logger  *zap.Logger
}

func NewPositionRepository(storage *pgxpool.Pool, logger *zap.Logger) PositionRepositoryInterface {
	return &positionRepository{storage: storage, logger: logger}
}

// scanRow - обновлена для сканирования всех полей.

func (r *positionRepository) scanRow(row pgx.Row) (*entities.Position, error) {
	var p entities.Position
	var externalID, sourceSystem sql.NullString

	err := row.Scan(
		&p.ID, &p.Name, &p.DepartmentID, &p.OtdelID, &p.BranchID, &p.OfficeID,
		&p.Type, &p.StatusID, &p.CreatedAt, &p.UpdatedAt,
		&externalID, &sourceSystem,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("ошибка сканирования positions: %w", err)
	}

	if externalID.Valid {
		p.ExternalID = &externalID.String
	}
	if sourceSystem.Valid {
		p.SourceSystem = &sourceSystem.String
	}

	return &p, nil
}

func (r *positionRepository) Create(ctx context.Context, tx pgx.Tx, p entities.Position) (uint64, error) {
	query := `
		INSERT INTO positions (name, department_id, otdel_id, branch_id, office_id, "type", status_id, external_id, source_system, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW()) 
		RETURNING id`

	var newID uint64
	err := tx.QueryRow(ctx, query,
		p.Name, p.DepartmentID, p.OtdelID, p.BranchID, p.OfficeID,
		p.Type, p.StatusID, p.ExternalID, p.SourceSystem,
	).Scan(&newID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return 0, fmt.Errorf("должность с таким external_id или именем уже существует: %w", apperrors.ErrConflict)
		}
		return 0, fmt.Errorf("ошибка создания positions: %w", err)
	}
	return newID, nil
}

func (r *positionRepository) Update(ctx context.Context, tx pgx.Tx, id uint64, p entities.Position) error {
	query := `
		UPDATE positions SET 
			name = $1, department_id = $2, otdel_id = $3, branch_id = $4,
			office_id = $5, "type" = $6, status_id = $7, updated_at = NOW() 
		WHERE id = $8`

	result, err := tx.Exec(ctx, query,
		p.Name, p.DepartmentID, p.OtdelID, p.BranchID, p.OfficeID,
		p.Type, p.StatusID, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("должность с таким именем уже существует: %w", apperrors.ErrConflict)
		}
		return fmt.Errorf("ошибка обновления positions: %w", err)
	}

	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *positionRepository) findOnePosition(ctx context.Context, querier Querier, where sq.Eq) (*entities.Position, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	query, args, err := psql.Select(positionFields).From(positionTable).Where(where).ToSql()
	if err != nil {
		return nil, err
	}
	return r.scanRow(querier.QueryRow(ctx, query, args...))
}

func (r *positionRepository) FindByExternalID(ctx context.Context, tx pgx.Tx, externalID string, sourceSystem string) (*entities.Position, error) {
	var querier Querier = r.storage
	if tx != nil {
		querier = tx
	}
	return r.findOnePosition(ctx, querier, sq.Eq{"external_id": externalID, "source_system": sourceSystem})
}

func (r *positionRepository) FindByID(ctx context.Context, tx pgx.Tx, id uint64) (*entities.Position, error) {
	var querier Querier = r.storage
	if tx != nil {
		querier = tx
	}
	return r.findOnePosition(ctx, querier, sq.Eq{"id": id})
}

func (r *positionRepository) Delete(ctx context.Context, tx pgx.Tx, id int) error {
	query := `DELETE FROM positions WHERE id = $1`
	result, err := tx.Exec(ctx, query, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return apperrors.NewHttpError(http.StatusBadRequest, "Должность используется и не может быть удалена", err, nil)
		}
		return fmt.Errorf("ошибка удаления positions: %w", err)
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *positionRepository) GetAll(ctx context.Context, filter types.Filter) ([]*entities.Position, uint64, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	baseBuilder := psql.Select().From(positionTable)
	// ФИЛЬТРАЦИЯ
	if len(filter.Filter) > 0 {
		for key, value := range filter.Filter {
			if dbColumn, ok := positionAllowedFilterFields[key]; ok {
				if items, ok := value.(string); ok && strings.Contains(items, ",") {
					baseBuilder = baseBuilder.Where(sq.Eq{dbColumn: strings.Split(items, ",")})
				} else {
					baseBuilder = baseBuilder.Where(sq.Eq{dbColumn: value})
				}
			}
		}
	}

	// ПОИСК
	if filter.Search != "" {
		baseBuilder = baseBuilder.Where(sq.ILike{"name": "%" + filter.Search + "%"})
	}

	// ПОДСЧЕТ
	countBuilder := baseBuilder.Columns("COUNT(id)")
	countQuery, countArgs, err := countBuilder.ToSql()
	if err != nil {
		return nil, 0, err
	}
	var total uint64
	if err := r.storage.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*entities.Position{}, 0, nil
	}

	// ВЫБОРКА + СОРТИРОВКА + ПАГИНАЦИЯ
	selectBuilder := baseBuilder.Columns(positionFields)

	// СОРТИРОВКА
	if len(filter.Sort) > 0 {
		for field, direction := range filter.Sort {
			if _, ok := positionAllowedSortFields[field]; ok {
				safeDirection := "ASC"
				if strings.ToUpper(direction) == "DESC" {
					safeDirection = "DESC"
				}
				selectBuilder = selectBuilder.OrderBy(fmt.Sprintf("%s %s", field, safeDirection))
			}
		}
	} else {
		selectBuilder = selectBuilder.OrderBy("id DESC")
	}

	// ПАГИНАЦИЯ
	if filter.WithPagination {
		selectBuilder = selectBuilder.Limit(uint64(filter.Limit)).Offset(uint64(filter.Offset))
	}

	// ВЫПОЛНЕНИЕ
	query, args, err := selectBuilder.ToSql()
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.storage.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	positions := make([]*entities.Position, 0)
	for rows.Next() {
		pos, err := r.scanRow(rows)
		if err != nil {
			return nil, 0, err
		}
		positions = append(positions, pos)
	}

	return positions, total, rows.Err()
}

func (r *positionRepository) FindByTypeAndOrg(ctx context.Context, tx pgx.Tx, posType string, depID *uint64, otdelID *uint64) ([]*entities.Position, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	builder := psql.Select(positionFields).
		From(positionTable).
		Where(sq.Eq{"\"type\"": posType}).
		OrderBy("id")

	if depID != nil {
		builder = builder.Where(sq.Eq{"department_id": *depID})
	}
	if otdelID != nil {
		builder = builder.Where(sq.Eq{"otdel_id": *otdelID})
	}

	query, args, err := builder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("ошибка сборки запроса FindByTypeAndOrg: %w", err)
	}

	var querier Querier = r.storage
	if tx != nil {
		querier = tx
	}

	rows, err := querier.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var positions []*entities.Position
	for rows.Next() {
		pos, err := r.scanRow(rows)
		if err != nil {
			return nil, err
		}
		positions = append(positions, pos)
	}
	return positions, rows.Err()
}

func (r *positionRepository) FindByName(ctx context.Context, tx pgx.Tx, name string) (*entities.Position, error) {
	var querier Querier = r.storage
	if tx != nil {
		querier = tx
	}
	return r.findOnePosition(ctx, querier, sq.Eq{"name": name})
}
