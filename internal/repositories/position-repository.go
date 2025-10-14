// Файл: internal/repositories/position-repository.go
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
)

const (
	positionTable  = "positions"
	positionFields = "id, name, code, level, status_id, created_at, updated_at"
)

type PositionRepositoryInterface interface {
	Create(ctx context.Context, tx pgx.Tx, p *entities.Position) (int, error)
	Update(ctx context.Context, tx pgx.Tx, p *entities.Position) error
	Delete(ctx context.Context, tx pgx.Tx, id int) error
	FindByID(ctx context.Context, id int) (*entities.Position, error)
	GetAll(ctx context.Context, limit, offset uint64, search string) ([]*entities.Position, uint64, error)
	FindNextInHierarchy(ctx context.Context, tx pgx.Tx, currentLevel int, departmentID uint64) (*entities.Position, error)
}

type positionRepository struct{ storage *pgxpool.Pool }

func NewPositionRepository(storage *pgxpool.Pool) PositionRepositoryInterface {
	return &positionRepository{storage: storage}
}

func (r *positionRepository) scanRow(row pgx.Row) (*entities.Position, error) {
	var p entities.Position
	var code sql.NullString
	err := row.Scan(&p.Id, &p.Name, &code, &p.Level, &p.StatusID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("ошибка сканирования positions: %w", err)
	}
	if code.Valid {
		p.Code = &code.String
	}
	return &p, nil
}

func (r *positionRepository) Create(ctx context.Context, tx pgx.Tx, p *entities.Position) (int, error) {
	query := `INSERT INTO positions (name, code, level, status_id) VALUES ($1, $2, $3, $4) RETURNING id`
	var id int
	err := tx.QueryRow(ctx, query, p.Name, p.Code, p.Level, p.StatusID).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return 0, fmt.Errorf("должность с таким кодом уже существует: %w", apperrors.ErrConflict)
		}
		return 0, fmt.Errorf("ошибка создания positions: %w", err)
	}
	return id, nil
}

func (r *positionRepository) Update(ctx context.Context, tx pgx.Tx, p *entities.Position) error {
	query := `UPDATE positions SET name = $1, code = $2, level = $3, status_id = $4, updated_at = NOW() WHERE id = $5`
	result, err := tx.Exec(ctx, query, p.Name, p.Code, p.Level, p.StatusID, p.Id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("должность с таким кодом уже существует: %w", apperrors.ErrConflict)
		}
		return fmt.Errorf("ошибка обновления positions: %w", err)
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
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

func (r *positionRepository) FindByID(ctx context.Context, id int) (*entities.Position, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", positionFields, positionTable)
	row := r.storage.QueryRow(ctx, query, id)
	return r.scanRow(row)
}

func (r *positionRepository) GetAll(ctx context.Context, limit, offset uint64, search string) ([]*entities.Position, uint64, error) {
	// ... (эта функция очень похожа на GetAll из order_type_repository, можете скопировать или я могу прислать)
	var total uint64
	var args []interface{}
	whereClause := ""

	if search != "" {
		whereClause = "WHERE name ILIKE $1 OR code ILIKE $1"
		args = append(args, "%"+search+"%")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", positionTable, whereClause)
	if err := r.storage.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*entities.Position{}, 0, nil
	}

	queryArgs := append(args, limit, offset)
	query := fmt.Sprintf("SELECT %s FROM %s %s ORDER BY level DESC, name ASC LIMIT $%d OFFSET $%d",
		positionFields, positionTable, whereClause, len(args)+1, len(args)+2)

	rows, err := r.storage.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	positions := make([]*entities.Position, 0, limit)
	for rows.Next() {
		pos, err := r.scanRow(rows)
		if err != nil {
			return nil, 0, err
		}
		positions = append(positions, pos)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}
	return positions, total, nil
}

func (r *positionRepository) FindNextInHierarchy(ctx context.Context, tx pgx.Tx, currentLevel int, departmentID uint64) (*entities.Position, error) {
	query := `
        SELECT p.id, p.name, p.code, p.level, p.status_id, p.created_at, p.updated_at
        FROM positions p
        WHERE p.level < $1 AND p.status_id = 2 AND EXISTS ( -- Ищем только активные должности
            SELECT 1 FROM users u 
            WHERE u.position_id = p.id AND u.department_id = $2 AND u.deleted_at IS NULL
        )
        ORDER BY p.level DESC
        LIMIT 1
    `
	row := tx.QueryRow(ctx, query, currentLevel, departmentID)
	return r.scanRow(row) // Ваш scanRow уже возвращает *entities.Position, это правильно
}
