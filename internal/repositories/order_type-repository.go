// Файл: internal/repositories/order_type_repository.go

package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	orderTypeTable  = "order_types"
	orderTypeFields = "id, name, code, status_id, created_at, updated_at"
)

// OrderTypeRepositoryInterface определяет контракт для работы с типами заявок в БД.
type OrderTypeRepositoryInterface interface {
	Create(ctx context.Context, tx pgx.Tx, orderType *entities.OrderType) (int, error)
	Update(ctx context.Context, tx pgx.Tx, orderType *entities.OrderType) error
	Delete(ctx context.Context, tx pgx.Tx, id int) error
	FindByID(ctx context.Context, id int) (*entities.OrderType, error)
	FindCodeByID(ctx context.Context, id uint64) (string, error)
	GetAll(ctx context.Context, limit, offset uint64, search string) ([]*entities.OrderType, uint64, error)
	FindCodesByIDs(ctx context.Context, ids []uint64) (map[uint64]string, error)
	ExistsByName(ctx context.Context, tx pgx.Tx, name string, excludeID int) (bool, error)
	ExistsByCode(ctx context.Context, tx pgx.Tx, code *string, excludeID int) (bool, error)
}

type orderTypeRepository struct {
	storage *pgxpool.Pool
}

func NewOrderTypeRepository(storage *pgxpool.Pool) OrderTypeRepositoryInterface {
	return &orderTypeRepository{storage: storage}
}

// scanRow - вспомогательная функция для сканирования одной строки из БД.
func (r *orderTypeRepository) scanRow(row pgx.Row) (*entities.OrderType, error) {
	var ot entities.OrderType
	var code sql.NullString

	err := row.Scan(&ot.ID, &ot.Name, &code, &ot.StatusID, &ot.CreatedAt, &ot.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("ошибка сканирования строки order_type: %w", err)
	}

	if code.Valid {
		ot.Code = &code.String
	}

	return &ot, nil
}

// Create создает новый тип заявки в транзакции.
func (r *orderTypeRepository) Create(ctx context.Context, tx pgx.Tx, orderType *entities.OrderType) (int, error) {
	query := fmt.Sprintf(`
		INSERT INTO %s (name, code, status_id) 
		VALUES ($1, $2, $3) 
		RETURNING id`, orderTypeTable)

	var id int
	err := tx.QueryRow(ctx, query, orderType.Name, orderType.Code, orderType.StatusID).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return 0, fmt.Errorf("тип заявки с таким кодом уже существует: %w", apperrors.ErrConflict)
		}
		return 0, fmt.Errorf("ошибка создания order_type: %w", err)
	}

	return id, nil
}

// Update обновляет существующий тип заявки в транзакции.
func (r *orderTypeRepository) Update(ctx context.Context, tx pgx.Tx, orderType *entities.OrderType) error {
	query := fmt.Sprintf(`
		UPDATE %s 
		SET name = $1, code = $2, status_id = $3, updated_at = NOW() 
		WHERE id = $4`, orderTypeTable)

	result, err := tx.Exec(ctx, query, orderType.Name, orderType.Code, orderType.StatusID, orderType.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("тип заявки с таким кодом уже существует: %w", apperrors.ErrConflict)
		}
		return fmt.Errorf("ошибка обновления order_type: %w", err)
	}

	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}

	return nil
}

// Delete удаляет тип заявки по ID в транзакции.
func (r *orderTypeRepository) Delete(ctx context.Context, tx pgx.Tx, id int) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", orderTypeTable)

	result, err := tx.Exec(ctx, query, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
			return fmt.Errorf("невозможно удалить тип заявки, так как он используется: %w", apperrors.NewHttpError(http.StatusBadRequest, "Тип заявки используется и не может быть удалён", err, nil))
		}
		return fmt.Errorf("ошибка удаления order_type: %w", err)
	}

	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}

	return nil
}

// FindByID находит тип заявки по ID.
func (r *orderTypeRepository) FindByID(ctx context.Context, id int) (*entities.OrderType, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", orderTypeFields, orderTypeTable)
	row := r.storage.QueryRow(ctx, query, id)
	return r.scanRow(row)
}

// GetAll получает список типов заявок с пагинацией и поиском.
func (r *orderTypeRepository) GetAll(ctx context.Context, limit, offset uint64, search string) ([]*entities.OrderType, uint64, error) {
	var total uint64
	var args []interface{}
	whereClause := ""

	if search != "" {
		whereClause = "WHERE name ILIKE $1 OR code ILIKE $1"
		args = append(args, "%"+search+"%")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", orderTypeTable, whereClause)
	if err := r.storage.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("ошибка подсчета order_types: %w", err)
	}
	if total == 0 {
		return []*entities.OrderType{}, 0, nil
	}

	queryArgs := append(args, limit, offset)
	query := fmt.Sprintf("SELECT %s FROM %s %s ORDER BY id LIMIT $%d OFFSET $%d",
		orderTypeFields, orderTypeTable, whereClause, len(args)+1, len(args)+2)

	rows, err := r.storage.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка получения списка order_types: %w", err)
	}
	defer rows.Close()

	orderTypes := make([]*entities.OrderType, 0, limit)
	for rows.Next() {
		orderType, err := r.scanRow(rows)
		if err != nil {
			return nil, 0, err
		}
		orderTypes = append(orderTypes, orderType)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("ошибка итерации по списку order_types: %w", err)
	}

	return orderTypes, total, nil
}

func (r *orderTypeRepository) FindCodeByID(ctx context.Context, id uint64) (string, error) {
	query := "SELECT code FROM order_types WHERE id = $1"
	var code sql.NullString

	err := r.storage.QueryRow(ctx, query, id).Scan(&code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperrors.ErrNotFound
		}
		return "", fmt.Errorf("ошибка получения кода order_type: %w", err)
	}

	if !code.Valid {
		return "", errors.New("код для данного типа заявки не установлен (NULL)")
	}

	return code.String, nil
}

func (r *orderTypeRepository) FindCodesByIDs(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	if len(ids) == 0 {
		return make(map[uint64]string), nil
	}

	// Используем `squirrel` для построения `IN` запроса
	queryBuilder := sq.Select("id", "code").
		From(orderTypeTable).
		Where(sq.Eq{"id": ids}).
		PlaceholderFormat(sq.Dollar)

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("ошибка сборки запроса FindCodesByIDs: %w", err)
	}

	rows, err := r.storage.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса FindCodesByIDs: %w", err)
	}
	defer rows.Close()

	codesMap := make(map[uint64]string)
	for rows.Next() {
		var id uint64
		var code sql.NullString
		if err := rows.Scan(&id, &code); err != nil {
			return nil, err
		}
		if code.Valid {
			codesMap[id] = code.String
		}
	}

	return codesMap, rows.Err()
}

func (r *orderTypeRepository) ExistsByName(ctx context.Context, tx pgx.Tx, name string, excludeID int) (bool, error) {
	// Добавляем `AND id != $2` в запрос
	query := "SELECT EXISTS(SELECT 1 FROM order_types WHERE name = $1 AND id != $2)"
	var exists bool
	if tx == nil {
		return false, errors.New("ExistsByName должен вызываться внутри транзакции")
	}
	err := tx.QueryRow(ctx, query, name, excludeID).Scan(&exists)
	return exists, err
}

func (r *orderTypeRepository) ExistsByCode(ctx context.Context, tx pgx.Tx, code *string, excludeID int) (bool, error) {
	if code == nil || *code == "" {
		return false, nil
	}
	// Добавляем `AND id != $2` в запрос
	query := "SELECT EXISTS(SELECT 1 FROM order_types WHERE code = $1 AND id != $2)"
	var exists bool
	if tx == nil {
		return false, errors.New("ExistsByCode должен вызываться внутри транзакции")
	}
	err := tx.QueryRow(ctx, query, *code, excludeID).Scan(&exists)
	return exists, err
}
