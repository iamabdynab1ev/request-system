// repositories/otdel_repository.go
package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
	otdelAllowedSortFields   = map[string]bool{"id": true, "name": true, "created_at": true, "updated_at": true, "status_id": true, "department_id": true, "otdel_id": true}
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

func (r *OtdelRepository) GetOtdels(ctx context.Context, filter types.Filter) ([]entities.Otdel, uint64, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

	baseBuilder := psql.Select().From(otdelTable)

	// --- ИСПОЛЬЗУЕМ otdelAllowedFilterFields ---
	if len(filter.Filter) > 0 {
		for key, value := range filter.Filter {
			// Проверяем, разрешено ли фильтровать по этому полю
			if dbColumn, ok := otdelAllowedFilterFields[key]; ok {
				items := strings.Split(fmt.Sprintf("%v", value), ",")
				if len(items) > 1 {
					baseBuilder = baseBuilder.Where(sq.Eq{dbColumn: items})
				} else {
					baseBuilder = baseBuilder.Where(sq.Eq{dbColumn: value})
				}
			}
		}
	}
	// ----------------------------------------

	if filter.Search != "" {
		baseBuilder = baseBuilder.Where(sq.ILike{"name": "%" + filter.Search + "%"})
	}

	// --- ПОДСЧЕТ ---
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
		return []entities.Otdel{}, 0, nil
	}

	// --- ВЫБОРКА, СОРТИРОВКА И ПАГИНАЦИЯ ---
	selectBuilder := baseBuilder.Columns("id, name, status_id, department_id, created_at, updated_at")

	if len(filter.Sort) > 0 {
		for field, direction := range filter.Sort {
			if _, ok := otdelAllowedSortFields[field]; ok {
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
	// Мы будем работать в транзакции, чтобы гарантировать целостность
	tx, err := r.storage.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) // Откатится, если что-то пойдет не так

	// --- 1. СНАЧАЛА ПРОВЕРЯЕМ, СУЩЕСТВУЕТ ЛИ ЗАПИСЬ ---
	var exists bool
	// Используем `tx` для запроса внутри транзакции
	checkQuery := "SELECT EXISTS(SELECT 1 FROM otdels WHERE id = $1)"
	if err := tx.QueryRow(ctx, checkQuery, id).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		// Если не существует - сразу возвращаем ошибку 404
		return nil, apperrors.ErrNotFound
	}

	// --- 2. ЕСЛИ СУЩЕСТВУЕТ - СТРОИМ ЗАПРОС НА ОБНОВЛЕНИЕ ---
	updateBuilder := sq.Update(otdelTable).
		PlaceholderFormat(sq.Dollar).
		Where(sq.Eq{"id": id}).
		Set("updated_at", time.Now()) // Используем time.Now()

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

	// Если в DTO не было данных для обновления, просто возвращаем текущее состояние
	if !hasChanges {
		tx.Commit(ctx) // Не забываем закрыть транзакцию
		return r.FindOtdel(ctx, id)
	}

	// --- 3. ВЫПОЛНЯЕМ ОБНОВЛЕНИЕ ---
	query, args, err := updateBuilder.ToSql()
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, query, args...); err != nil {
		// Здесь может быть ошибка уникальности имени и т.д.
		return nil, err
	}

	// --- 4. КОММИТИМ ТРАНЗАКЦИЮ И ВОЗВРАЩАЕМ РЕЗУЛЬТАТ ---
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// После успешного обновления, запрашиваем и возвращаем актуальное состояние
	return r.FindOtdel(ctx, id)
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
