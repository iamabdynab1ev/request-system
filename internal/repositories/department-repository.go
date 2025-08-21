package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"request-system/internal/dto"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const departmentTable = "departments"

var departmentAllowedFilterFields = map[string]string{
	"status_id": "d.status_id", // Используем алиас "d", так как он используется в Stats
}

var departmentAllowedSortFields = map[string]string{
	"id":         "d.id",
	"name":       "d.name",
	"created_at": "d.created_at",
}

// Интерфейс соответствует всем методам
type DepartmentRepositoryInterface interface {
	GetDepartments(ctx context.Context, filter types.Filter) ([]entities.Department, uint64, error)
	CountDepartments(ctx context.Context, filter types.Filter, tableAlias string) (uint64, error)
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
	return &DepartmentRepository{storage: storage, logger: logger}
}

// Приватный хелпер для сборки запроса, который будет использоваться всеми
func (r *DepartmentRepository) buildFilterQuery(filter types.Filter, tableAlias string) (string, []interface{}) {
	args := make([]interface{}, 0)
	conditions := []string{}
	argCounter := 1

	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		conditions = append(conditions, fmt.Sprintf("%s.name ILIKE $%d", tableAlias, argCounter))
		args = append(args, searchPattern)
		argCounter++
	}

	for key, value := range filter.Filter {
		// Ключ в мапе должен совпадать с ключом из URL (без алиаса)
		if dbColumnWithAlias, ok := departmentAllowedFilterFields[key]; ok {
			// Но для SQL мы используем значение из мапы (с алиасом)
			if strVal, ok := value.(string); ok && strings.Contains(strVal, ",") {
				items := strings.Split(strVal, ",")
				placeholders := make([]string, len(items))
				for i, item := range items {
					placeholders[i] = fmt.Sprintf("$%d", argCounter)
					args = append(args, item)
					argCounter++
				}
				conditions = append(conditions, fmt.Sprintf("%s IN (%s)", dbColumnWithAlias, strings.Join(placeholders, ",")))
			} else {
				conditions = append(conditions, fmt.Sprintf("%s = $%d", dbColumnWithAlias, argCounter))
				args = append(args, value)
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
	// Для простого запроса используем алиас "d" для единообразия
	whereClause, args := r.buildFilterQuery(filter, "d")

	total, err := r.CountDepartments(ctx, filter, "d")
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []entities.Department{}, 0, nil
	}

	orderByClause := "ORDER BY d.id DESC"
	// Логика сортировки...

	limitClause := ""
	argCounter := len(args) + 1
	if filter.WithPagination {
		limitClause = fmt.Sprintf("LIMIT $%d OFFSET $%d", argCounter, argCounter+1)
		args = append(args, filter.Limit, filter.Offset)
	}

	query := fmt.Sprintf(`
		SELECT d.id, d.name, d.status_id, d.created_at, d.updated_at
		FROM %s d
		%s %s %s
	`, departmentTable, whereClause, orderByClause, limitClause)

	rows, err := r.storage.Query(ctx, query, args...)
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
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}
	return departments, total, nil
}

func (r *DepartmentRepository) CountDepartments(ctx context.Context, filter types.Filter, tableAlias string) (uint64, error) {
	whereClause, args := r.buildFilterQuery(filter, tableAlias)
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s AS %s %s", departmentTable, tableAlias, whereClause)
	var total uint64
	if err := r.storage.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		r.logger.Error("ошибка подсчета департаментов", zap.Error(err), zap.String("query", countQuery))
		return 0, err
	}
	return total, nil
}

func (r *DepartmentRepository) FindDepartment(ctx context.Context, id uint64) (*entities.Department, error) {
	query := `SELECT id, name, status_id, created_at, updated_at FROM departments WHERE id = $1`
	return scanDepartment(r.storage.QueryRow(ctx, query, id))
}

func (r *DepartmentRepository) CreateDepartment(ctx context.Context, department entities.Department) (*entities.Department, error) {
	query := `INSERT INTO departments (name, status_id) VALUES($1, $2) RETURNING id, name, status_id, created_at, updated_at`
	return scanDepartment(r.storage.QueryRow(ctx, query, department.Name, department.StatusID))
}

func (r *DepartmentRepository) UpdateDepartment(ctx context.Context, id uint64, department entities.Department) (*entities.Department, error) {
	query := `UPDATE departments SET name = $1, status_id = $2, updated_at = NOW() WHERE id = $3 RETURNING id, name, status_id, created_at, updated_at`
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
	total, err := r.CountDepartments(ctx, filter, "d")
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []dto.DepartmentStatsDTO{}, 0, nil
	}

	whereClause, args := r.buildFilterQuery(filter, "d")

	mainQuery := `
		SELECT d.id, d.name,
			COUNT(o.id) FILTER (WHERE s.code = 'OPEN') AS open_orders,
			COUNT(o.id) FILTER (WHERE s.code = 'CLOSED') AS closed_orders
		FROM departments AS d
		LEFT JOIN orders AS o ON d.id = o.department_id AND o.deleted_at IS NULL
		LEFT JOIN statuses AS s ON o.status_id = s.id
	`
	orderByClause := " GROUP BY d.id, d.name ORDER BY d.id ASC "

	var finalQuery strings.Builder
	finalQuery.WriteString(mainQuery)
	finalQuery.WriteString(whereClause)
	finalQuery.WriteString(orderByClause)

	argCounter := len(args) + 1
	if filter.WithPagination {
		finalQuery.WriteString(fmt.Sprintf("LIMIT $%d OFFSET $%d", argCounter, argCounter+1))
		args = append(args, filter.Limit, filter.Offset)
	}

	rows, err := r.storage.Query(ctx, finalQuery.String(), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	stats := make([]dto.DepartmentStatsDTO, 0)
	for rows.Next() {
		var s dto.DepartmentStatsDTO
		if err := rows.Scan(&s.ID, &s.Name, &s.OpenOrdersCount, &s.ClosedOrdersCount); err != nil {
			r.logger.Error("ошибка сканирования статистики", zap.Error(err))
			return nil, 0, err
		}
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return stats, total, nil
}
