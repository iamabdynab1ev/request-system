// Файл: internal/repositories/status-repository.go

package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"request-system/internal/dto"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type dbStatus struct {
	ID        uint64
	IconSmall sql.NullString
	IconBig   sql.NullString
	Name      string
	Type      int
	Code      sql.NullString
	CreatedAt time.Time
	UpdatedAt sql.NullTime
}

func (db *dbStatus) ToDTO() dto.StatusDTO {
	return dto.StatusDTO{
		ID:        db.ID,
		IconSmall: utils.NullStringToString(db.IconSmall),
		IconBig:   utils.NullStringToString(db.IconBig),
		Name:      db.Name,
		Type:      db.Type,
		Code:      utils.NullStringToString(db.Code),
		CreatedAt: db.CreatedAt.Local().Format("2006-01-02 15:04:05"),
		UpdatedAt: utils.NullTimeToEmptyString(db.UpdatedAt),
	}
}

func (db *dbStatus) ToEntity() entities.Status {
	var codePtr *string
	if db.Code.Valid {
		code := db.Code.String
		codePtr = &code
	}
	return entities.Status{
		ID:   int(db.ID),
		Name: db.Name,
		Code: codePtr,
		Type: db.Type,
	}
}

const (
	statusTable  = "statuses"
	statusFields = "id, icon_small, icon_big, name, type, code, created_at, updated_at"
)

type StatusRepositoryInterface interface {
	GetStatuses(ctx context.Context, filter types.Filter) ([]dto.StatusDTO, uint64, error)
	FindStatus(ctx context.Context, id uint64) (*entities.Status, error)
	FindStatusAsDTO(ctx context.Context, id uint64) (*dto.StatusDTO, error)
	CreateStatus(ctx context.Context, payload dto.CreateStatusDTO, iconSmallPath string, iconBigPath string) (*dto.StatusDTO, error)
	UpdateStatus(ctx context.Context, id uint64, dto dto.UpdateStatusDTO, iconSmallPath *string, iconBigPath *string) (*dto.StatusDTO, error)
	DeleteStatus(ctx context.Context, id uint64) error
	FindByCodeInTx(ctx context.Context, tx pgx.Tx, code string) (*entities.Status, error)
	FindStatusInTx(ctx context.Context, tx pgx.Tx, id uint64) (*entities.Status, error)
	FindIDByCode(ctx context.Context, code string) (uint64, error)
}

type statusRepository struct{ storage *pgxpool.Pool }

func NewStatusRepository(storage *pgxpool.Pool) StatusRepositoryInterface {
	return &statusRepository{storage: storage}
}

func (r *statusRepository) scanRow(row pgx.Row) (*dbStatus, error) {
	var dbRow dbStatus
	err := row.Scan(&dbRow.ID, &dbRow.IconSmall, &dbRow.IconBig, &dbRow.Name, &dbRow.Type, &dbRow.Code, &dbRow.CreatedAt, &dbRow.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &dbRow, nil
}

func (r *statusRepository) GetStatuses(ctx context.Context, filter types.Filter) ([]dto.StatusDTO, uint64, error) {
	var args []interface{}
	conditions := []string{}
	argIndex := 1

	// Поиск
	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(name ILIKE $%d OR code ILIKE $%d)", argIndex, argIndex))
		args = append(args, "%"+filter.Search+"%")
		argIndex++
	}

	// Фильтры (множественные значения через запятую)
	for key, val := range filter.Filter {
		strVal, ok := val.(string)
		if !ok || strVal == "" {
			continue
		}

		values := strings.Split(strVal, ",")
		placeholders := []string{}
		for _, v := range values {
			placeholders = append(placeholders, fmt.Sprintf("$%d", argIndex))
			args = append(args, strings.TrimSpace(v))
			argIndex++
		}

		if len(placeholders) == 1 {
			conditions = append(conditions, fmt.Sprintf("%s = %s", key, placeholders[0]))
		} else {
			conditions = append(conditions, fmt.Sprintf("%s IN (%s)", key, strings.Join(placeholders, ",")))
		}
	}

	// WHERE
	whereSQL := ""
	if len(conditions) > 0 {
		whereSQL = "WHERE " + strings.Join(conditions, " AND ")
	}

	// ORDER BY
	orderSQL := ""
	if len(filter.Sort) > 0 {
		orderParts := []string{}
		for col, dir := range filter.Sort {
			dir = strings.ToUpper(dir)
			if dir != "ASC" && dir != "DESC" {
				dir = "ASC"
			}
			orderParts = append(orderParts, fmt.Sprintf("%s %s", col, dir))
		}
		orderSQL = "ORDER BY " + strings.Join(orderParts, ", ")
	} else {
		orderSQL = "ORDER BY id ASC"
	}

	// Пагинация
	limit := filter.Limit
	offset := filter.Offset
	if filter.WithPagination && limit > 0 {
		orderSQL += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
	}

	// Основной запрос
	query := fmt.Sprintf(`SELECT %s FROM %s %s %s`, statusFields, statusTable, whereSQL, orderSQL)

	rows, err := r.storage.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	statuses := []dto.StatusDTO{}
	for rows.Next() {
		var dbRow dbStatus
		if err := rows.Scan(&dbRow.ID, &dbRow.IconSmall, &dbRow.IconBig, &dbRow.Name, &dbRow.Type, &dbRow.Code, &dbRow.CreatedAt, &dbRow.UpdatedAt); err != nil {
			return nil, 0, err
		}
		statuses = append(statuses, dbRow.ToDTO())
	}

	// Подсчёт общего количества
	var total uint64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", statusTable, whereSQL)
	if err := r.storage.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	return statuses, total, nil
}

func (r *statusRepository) FindStatus(ctx context.Context, id uint64) (*entities.Status, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", statusFields, statusTable)
	row := r.storage.QueryRow(ctx, query, id)
	dbRow, err := r.scanRow(row)
	if err != nil {
		return nil, err
	}
	entity := dbRow.ToEntity()
	return &entity, nil
}

func (r *statusRepository) FindStatusInTx(ctx context.Context, tx pgx.Tx, id uint64) (*entities.Status, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", statusFields, statusTable)
	row := tx.QueryRow(ctx, query, id)
	dbRow, err := r.scanRow(row)
	if err != nil {
		return nil, err
	}
	entity := dbRow.ToEntity()
	return &entity, nil
}

func (r *statusRepository) FindByCodeInTx(ctx context.Context, tx pgx.Tx, code string) (*entities.Status, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE code = $1 LIMIT 1", statusFields, statusTable)
	row := tx.QueryRow(ctx, query, code)
	dbRow, err := r.scanRow(row)
	if err != nil {
		return nil, err
	}
	entity := dbRow.ToEntity()
	return &entity, nil
}

// FindIDByCode - НОВЫЙ, КЛЮЧЕВОЙ МЕТОД
func (r *statusRepository) FindIDByCode(ctx context.Context, code string) (uint64, error) {
	var id uint64
	err := r.storage.QueryRow(ctx, `SELECT id FROM statuses WHERE code = $1`, code).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, fmt.Errorf("статус с кодом %s не найден", code)
		}
		return 0, err
	}
	return id, nil
}

func (r *statusRepository) FindStatusAsDTO(ctx context.Context, id uint64) (*dto.StatusDTO, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", statusFields, statusTable)
	row := r.storage.QueryRow(ctx, query, id)
	dbRow, err := r.scanRow(row)
	if err != nil {
		return nil, err
	}
	dto := dbRow.ToDTO()
	return &dto, nil
}

func (r *statusRepository) CreateStatus(ctx context.Context, payload dto.CreateStatusDTO, iconSmallPath, iconBigPath string) (*dto.StatusDTO, error) {
	query := fmt.Sprintf("INSERT INTO %s (name, type, code, icon_small, icon_big) VALUES($1, $2, $3, $4, $5) RETURNING %s", statusTable, statusFields)
	var dbRow dbStatus
	err := r.storage.QueryRow(ctx, query, payload.Name, payload.Type, payload.Code, iconSmallPath, iconBigPath).Scan(&dbRow.ID, &dbRow.IconSmall, &dbRow.IconBig, &dbRow.Name, &dbRow.Type, &dbRow.Code, &dbRow.CreatedAt, &dbRow.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, apperrors.ErrConflict
		}
		return nil, err
	}
	statusDTO := dbRow.ToDTO()
	return &statusDTO, nil
}

func (r *statusRepository) UpdateStatus(ctx context.Context, id uint64, dto dto.UpdateStatusDTO, iconSmallPath, iconBigPath *string) (*dto.StatusDTO, error) {
	var setClauses []string
	var args []interface{}
	argId := 1

	if dto.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argId))
		args = append(args, *dto.Name)
		argId++
	}
	if dto.Type != nil {
		setClauses = append(setClauses, fmt.Sprintf("type = $%d", argId))
		args = append(args, *dto.Type)
		argId++
	}
	if dto.Code != nil {
		setClauses = append(setClauses, fmt.Sprintf("code = $%d", argId))
		args = append(args, *dto.Code)
		argId++
	}
	if iconSmallPath != nil {
		setClauses = append(setClauses, fmt.Sprintf("icon_small = $%d", argId))
		args = append(args, *iconSmallPath)
		argId++
	}
	if iconBigPath != nil {
		setClauses = append(setClauses, fmt.Sprintf("icon_big = $%d", argId))
		args = append(args, *iconBigPath)
		argId++
	}

	if len(setClauses) == 0 {
		return r.FindStatusAsDTO(ctx, id)
	}

	setClauses = append(setClauses, "updated_at = NOW()")
	setQuery := strings.Join(setClauses, ", ")

	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = $%d RETURNING %s", statusTable, setQuery, argId, statusFields)
	args = append(args, id)

	row := r.storage.QueryRow(ctx, query, args...)
	dbRow, err := r.scanRow(row)
	if err != nil {
		return nil, err
	}

	statusDTO := dbRow.ToDTO()
	return &statusDTO, nil
}

func (r *statusRepository) DeleteStatus(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", statusTable)
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return apperrors.ErrStatusInUse
		}
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
