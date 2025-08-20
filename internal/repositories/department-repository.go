package repositories

import (
	"context"
	"errors"
	"fmt"
	"request-system/internal/dto"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const departmentTable = "departments"

var departmentAllowedFilterFields = map[string]bool{
	"status_id": true,
}
var departmentAllowedSortFields = map[string]bool{
	"id":         true,
	"name":       true,
	"created_at": true,
}

type DepartmentRepositoryInterface interface {
	GetDepartments(ctx context.Context, filter types.Filter) ([]entities.Department, uint64, error)
	GetDepartmentsWithStats(ctx context.Context, filter types.Filter) ([]dto.DepartmentStatsDTO, uint64, error)
	FindDepartment(ctx context.Context, id uint64) (*entities.Department, error)
	CreateDepartment(ctx context.Context, department entities.Department) (*entities.Department, error)
	UpdateDepartment(ctx context.Context, id uint64, department entities.Department) (*entities.Department, error)
	DeleteDepartment(ctx context.Context, id uint64) error
}

type DepartmentRepository struct {
	storage *pgxpool.Pool
	logger  *zap.Logger
}

func NewDepartmentRepository(storage *pgxpool.Pool, logger *zap.Logger) DepartmentRepositoryInterface {
	return &DepartmentRepository{
		storage: storage,
		logger:  logger,
	}
}

func scanDepartment(row pgx.Row) (*entities.Department, error) {
	var d entities.Department
	var createdAt, updatedAt time.Time

	err := row.Scan(&d.ID, &d.Name, &d.StatusID, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ошибка сканирования department: %w", err)
	}

	d.CreatedAt = createdAt
	d.UpdatedAt = updatedAt
	return &d, nil
}

func (r *DepartmentRepository) GetDepartments(ctx context.Context, filter types.Filter) ([]entities.Department, uint64, error) {
	allArgs := make([]interface{}, 0)
	conditions := []string{}
	placeholderNum := 1

	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", placeholderNum))
		allArgs = append(allArgs, searchPattern)
		placeholderNum++
	}

	for key, value := range filter.Filter {
		if !departmentAllowedFilterFields[key] {
			continue
		}
		if strVal, ok := value.(string); ok && strings.Contains(strVal, ",") {
			items := strings.Split(strVal, ",")
			placeholders := make([]string, len(items))
			for i, item := range items {
				placeholders[i] = fmt.Sprintf("$%d", placeholderNum)
				allArgs = append(allArgs, item)
				placeholderNum++
			}
			conditions = append(conditions, fmt.Sprintf("%s IN (%s)", key, strings.Join(placeholders, ",")))
		} else {
			conditions = append(conditions, fmt.Sprintf("%s = $%d", key, placeholderNum))
			allArgs = append(allArgs, value)
			placeholderNum++
		}
	}

	var whereClause string
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}
	countQuery := fmt.Sprintf("SELECT COUNT(id) FROM %s %s", departmentTable, whereClause)
	var total uint64
	if err := r.storage.QueryRow(ctx, countQuery, allArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []entities.Department{}, 0, nil
	}

	orderByClause := "ORDER BY id DESC"
	if len(filter.Sort) > 0 {
		var sortParts []string
		for field, direction := range filter.Sort {
			if departmentAllowedSortFields[field] {
				safeDirection := "ASC"
				if strings.ToLower(direction) == "desc" {
					safeDirection = "DESC"
				}
				sortParts = append(sortParts, fmt.Sprintf("%s %s", field, safeDirection))
			}
		}
		if len(sortParts) > 0 {
			orderByClause = "ORDER BY " + strings.Join(sortParts, ", ")
		}
	}

	limitClause := ""
	if filter.WithPagination {
		limitClause = fmt.Sprintf("LIMIT $%d OFFSET $%d", placeholderNum, placeholderNum+1)
		allArgs = append(allArgs, filter.Limit, filter.Offset)
	}

	selectFields := "id, name, status_id, created_at, updated_at"
	mainQuery := fmt.Sprintf("SELECT %s FROM %s %s %s %s", selectFields, departmentTable, whereClause, orderByClause, limitClause)

	rows, err := r.storage.Query(ctx, mainQuery, allArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	departments := make([]entities.Department, 0)
	for rows.Next() {
		dept, err := scanDepartment(rows)
		if err != nil {
			return nil, 0, err
		}
		departments = append(departments, *dept)
	}
	return departments, total, rows.Err()
}

func (r *DepartmentRepository) FindDepartment(ctx context.Context, id uint64) (*entities.Department, error) {
	query := `SELECT id, name, status_id, created_at, updated_at FROM departments WHERE id = $1 AND deleted_at IS NULL`
	return scanDepartment(r.storage.QueryRow(ctx, query, id))
}

func (r *DepartmentRepository) CreateDepartment(ctx context.Context, department entities.Department) (*entities.Department, error) {
	query := `INSERT INTO departments (name, status_id) VALUES($1, $2) RETURNING id, name, status_id, created_at, updated_at`
	return scanDepartment(r.storage.QueryRow(ctx, query, department.Name, department.StatusID))
}

func (r *DepartmentRepository) UpdateDepartment(ctx context.Context, id uint64, department entities.Department) (*entities.Department, error) {
	query := `UPDATE departments SET name = $1, status_id = $2, updated_at = NOW() WHERE id = $3 AND deleted_at IS NULL RETURNING id, name, status_id, created_at, updated_at`
	return scanDepartment(r.storage.QueryRow(ctx, query, department.Name, department.StatusID, id))
}

func (r *DepartmentRepository) DeleteDepartment(ctx context.Context, id uint64) error {
	query := `DELETE FROM departments WHERE id = $1`
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
func (r *DepartmentRepository) GetDepartmentsWithStats(ctx context.Context, filter types.Filter) ([]dto.DepartmentStatsDTO, uint64, error) {
	// Пока что оставим этот метод как заглушку, чтобы все компилировалось
	return []dto.DepartmentStatsDTO{}, 0, nil
}
