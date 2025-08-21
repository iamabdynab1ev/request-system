// Файл: internal/repositories/office-repository.go
// СКОПИРУЙТЕ И ПОЛНОСТЬЮ ЗАМЕНИТЕ СОДЕРЖИМОЕ

package repositories

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"request-system/internal/dto"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const officeTable = "offices"

// Белый список полей для фильтрации и сортировки для безопасности
var officeAllowedFilterFields = map[string]string{
	"status_id": "o.status_id",
	"branch_id": "o.branch_id",
}

var officeAllowedSortFields = map[string]string{
	"id":        "o.id",
	"name":      "o.name",
	"open_date": "o.open_date",
}

type OfficeRepositoryInterface interface {
	GetOffices(ctx context.Context, filter types.Filter) ([]dto.OfficeDTO, uint64, error)
	FindOffice(ctx context.Context, id uint64) (*dto.OfficeDTO, error)
	CreateOffice(ctx context.Context, dto dto.CreateOfficeDTO) (uint64, error)
	UpdateOffice(ctx context.Context, id uint64, dto dto.UpdateOfficeDTO) error
	DeleteOffice(ctx context.Context, id uint64) error
}

type OfficeRepository struct {
	storage *pgxpool.Pool
}

func NewOfficeRepository(storage *pgxpool.Pool) OfficeRepositoryInterface {
	return &OfficeRepository{storage: storage}
}

func (r *OfficeRepository) buildSortQuery(filter types.Filter) string {
	if filter.Sort != nil {
		for field, direction := range filter.Sort {
			dbColumn, isAllowed := officeAllowedSortFields[field]
			if !isAllowed {
				continue // Пропускаем неразрешенные поля
			}
			// Приводим направление к верхнему регистру и проверяем
			dir := strings.ToUpper(direction)
			if dir != "ASC" && dir != "DESC" {
				dir = "ASC" // По умолчанию
			}
			return fmt.Sprintf("ORDER BY %s %s", dbColumn, dir)
		}
	}
	// Сортировка по умолчанию
	return "ORDER BY o.id DESC"
}

func (r *OfficeRepository) buildFilterQuery(filter types.Filter) (string, []interface{}) {
	args := make([]interface{}, 0)
	conditions := []string{}
	argCounter := 1

	if filter.Search != "" {
		searchPattern := "%" + strings.ToLower(filter.Search) + "%"
		conditions = append(conditions, fmt.Sprintf("(LOWER(o.name) LIKE $%d OR LOWER(o.address) LIKE $%d)", argCounter, argCounter))
		args = append(args, searchPattern)
		argCounter++
	}

	for key, value := range filter.Filter {
		dbColumn, isAllowed := officeAllowedFilterFields[key]
		if !isAllowed {
			continue
		}
		if itemStr, ok := value.(string); ok && itemStr != "" {
			if strings.Contains(itemStr, ",") {
				items := strings.Split(itemStr, ",")
				placeholders := []string{}
				for _, item := range items {
					val, err := strconv.Atoi(strings.TrimSpace(item))
					if err != nil {
						continue
					}
					placeholders = append(placeholders, fmt.Sprintf("$%d", argCounter))
					args = append(args, val)
					argCounter++
				}
				if len(placeholders) > 0 {
					conditions = append(conditions, fmt.Sprintf("%s IN (%s)", dbColumn, strings.Join(placeholders, ",")))
				}
			} else {
				val, err := strconv.Atoi(strings.TrimSpace(itemStr))
				if err != nil {
					continue
				}
				conditions = append(conditions, fmt.Sprintf("%s = $%d", dbColumn, argCounter))
				args = append(args, val)
				argCounter++
			}
		}
	}

	var whereClause string
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}
	return whereClause, args
}

func (r *OfficeRepository) GetOffices(ctx context.Context, filter types.Filter) ([]dto.OfficeDTO, uint64, error) {
	whereClause, args := r.buildFilterQuery(filter)
	sortClause := r.buildSortQuery(filter)

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s o %s", officeTable, whereClause)
	var total uint64
	if err := r.storage.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("ошибка подсчета офисов: %w", err)
	}

	if total == 0 {
		return []dto.OfficeDTO{}, 0, nil
	}

	limitClause := ""
	if filter.WithPagination {
		argCounter := len(args) + 1
		limitClause = fmt.Sprintf("LIMIT $%d OFFSET $%d", argCounter, argCounter+1)
		args = append(args, filter.Limit, filter.Offset)
	}

	query := fmt.Sprintf(`
		SELECT
			o.id, o.name, o.address, o.open_date, o.created_at, o.updated_at,
			COALESCE(b.id, 0), COALESCE(b.name, ''), COALESCE(b.short_name, ''),
			COALESCE(s.id, 0), COALESCE(s.name, '')
		FROM %s o
		LEFT JOIN %s b ON o.branch_id = b.id
		LEFT JOIN %s s ON o.status_id = s.id
		%s %s %s
		`, officeTable, branchTable, statusTable, whereClause, sortClause, limitClause)

	rows, err := r.storage.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var offices []dto.OfficeDTO
	for rows.Next() {
		var office dto.OfficeDTO
		var branch dto.ShortBranchDTO
		var status dto.ShortStatusDTO
		if err := rows.Scan(
			&office.ID, &office.Name, &office.Address, &office.OpenDate, &office.CreatedAt, &office.UpdatedAt,
			&branch.ID, &branch.Name, &branch.ShortName,
			&status.ID, &status.Name,
		); err != nil {
			return nil, 0, err
		}
		if branch.ID > 0 {
			office.Branch = &branch
		}
		if status.ID > 0 {
			office.Status = &status
		}
		offices = append(offices, office)
	}
	return offices, total, nil
}

func (r *OfficeRepository) FindOffice(ctx context.Context, id uint64) (*dto.OfficeDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			o.id, o.name, o.address, o.open_date, o.created_at, o.updated_at,
			COALESCE(b.id, 0), COALESCE(b.name, ''), COALESCE(b.short_name, ''),
			COALESCE(s.id, 0), COALESCE(s.name, '')
		FROM %s o
		LEFT JOIN %s b ON o.branch_id = b.id
		LEFT JOIN %s s ON o.status_id = s.id
		WHERE o.id = $1
	`, officeTable, branchTable, statusTable)

	var office dto.OfficeDTO
	var branch dto.ShortBranchDTO
	var status dto.ShortStatusDTO
	err := r.storage.QueryRow(ctx, query, id).Scan(
		&office.ID, &office.Name, &office.Address, &office.OpenDate, &office.CreatedAt, &office.UpdatedAt,
		&branch.ID, &branch.Name, &branch.ShortName,
		&status.ID, &status.Name,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	if branch.ID > 0 {
		office.Branch = &branch
	}
	if status.ID > 0 {
		office.Status = &status
	}
	return &office, nil
}

func (r *OfficeRepository) CreateOffice(ctx context.Context, dto dto.CreateOfficeDTO) (uint64, error) {
	query := fmt.Sprintf(`
        INSERT INTO %s (name, address, open_date, branch_id, status_id)
        VALUES ($1, $2, $3, $4, $5) RETURNING id
    `, officeTable)

	openDate, err := time.Parse("2006-01-02", dto.OpenDate)
	if err != nil {
		return 0, fmt.Errorf("invalid open_date format: %w", err)
	}

	var newID uint64
	err = r.storage.QueryRow(ctx, query, dto.Name, dto.Address, openDate, dto.BranchID, dto.StatusID).Scan(&newID)
	if err != nil {
		return 0, err
	}
	return newID, nil
}

func (r *OfficeRepository) UpdateOffice(ctx context.Context, id uint64, dto dto.UpdateOfficeDTO) error {
	updates := make([]string, 0)
	args := make([]interface{}, 0)
	argID := 1

	if dto.Name != "" {
		updates = append(updates, fmt.Sprintf("name = $%d", argID))
		args = append(args, dto.Name)
		argID++
	}
	if dto.Address != "" {
		updates = append(updates, fmt.Sprintf("address = $%d", argID))
		args = append(args, dto.Address)
		argID++
	}
	if dto.BranchID != 0 {
		updates = append(updates, fmt.Sprintf("branch_id = $%d", argID))
		args = append(args, dto.BranchID)
		argID++
	}
	if dto.StatusID != 0 {
		updates = append(updates, fmt.Sprintf("status_id = $%d", argID))
		args = append(args, dto.StatusID)
		argID++
	}
	if dto.OpenDate != "" {
		openDate, err := time.Parse("2006-01-02", dto.OpenDate)
		if err != nil {
			return fmt.Errorf("invalid open_date format: %w", err)
		}
		updates = append(updates, fmt.Sprintf("open_date = $%d", argID))
		args = append(args, openDate)
		argID++
	}

	if len(updates) == 0 {
		return nil
	}
	updates = append(updates, fmt.Sprintf("updated_at = $%d", argID))
	args = append(args, time.Now())
	argID++
	args = append(args, id)

	query := fmt.Sprintf(`UPDATE %s SET %s WHERE id = $%d`, officeTable, strings.Join(updates, ", "), argID)
	result, err := r.storage.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *OfficeRepository) DeleteOffice(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", officeTable)
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
