package repositories

import (
	"context"
	"errors"
	"fmt"
	"request-system/internal/dto"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	departmentTable  = "departments"
	departmentFields = "id, name, status_id, created_at, updated_at"
	userTable        = "users"
	managerTable     = "department_managers"
)

var allowedFilterFields = map[string]bool{"status_id": true}
var allowedSortFields = map[string]bool{"id": true, "name": true, "created_at": true, "updated_at": true}

// Интерфейс со всеми вашими методами + новый метод для статистики
type DepartmentRepositoryInterface interface {
	GetDepartments(ctx context.Context, filter types.Filter) ([]dto.DepartmentDTO, uint64, error)
	GetDepartmentsWithStats(ctx context.Context, filter types.Filter) ([]dto.DepartmentStatsDTO, uint64, error)
	FindDepartment(ctx context.Context, id uint64) (*dto.DepartmentDTO, error)
	CreateDepartment(ctx context.Context, createDTO dto.CreateDepartmentDTO) (*dto.DepartmentDTO, error)
	UpdateDepartment(ctx context.Context, id uint64, updateDTO dto.UpdateDepartmentDTO) (*dto.DepartmentDTO, error)
	DeleteDepartment(ctx context.Context, id uint64) error
	IsManager(ctx context.Context, userID uint64, departmentID uint64) (bool, error)
	FindManagerByID(ctx context.Context, departmentID uint64) (*dto.UserDTO, error)
}

type DepartmentRepository struct {
	storage *pgxpool.Pool
}

func NewDepartmentRepository(storage *pgxpool.Pool) DepartmentRepositoryInterface {
	return &DepartmentRepository{
		storage: storage,
	}
}

// Хелпер для построения WHERE условия, используется всеми GET методами
func buildWhereClause(filter types.Filter, tableAlias string) (string, []interface{}) {
	prefix := ""
	if tableAlias != "" {
		prefix = tableAlias + "."
	}

	whereConditions := make([]string, 0)
	var args []interface{}
	argId := 1

	if filter.Search != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("%sname ILIKE $%d", prefix, argId))
		args = append(args, "%"+filter.Search+"%")
		argId++
	}

	for field, value := range filter.Filter {
		if allowed, ok := allowedFilterFields[field]; ok && allowed {
			whereConditions = append(whereConditions, fmt.Sprintf("%s%s = $%d", prefix, field, argId))
			args = append(args, value)
			argId++
		}
	}

	// Если фильтров нет, возвращаем пустую строку
	if len(whereConditions) == 0 {
		return "", nil
	}

	// Если фильтры есть, добавляем WHERE
	return "WHERE " + strings.Join(whereConditions, " AND "), args
}

func (r *DepartmentRepository) GetDepartments(ctx context.Context, filter types.Filter) ([]dto.DepartmentDTO, uint64, error) {
	whereClause, args := buildWhereClause(filter, "")

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", departmentTable, whereClause)
	var total uint64
	if err := r.storage.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []dto.DepartmentDTO{}, 0, nil
	}

	var orderByClause string
	if len(filter.Sort) > 0 {
		var sortParts []string
		for field, direction := range filter.Sort {
			if allowed, ok := allowedSortFields[field]; ok && allowed {
				sortParts = append(sortParts, fmt.Sprintf("%s %s", field, direction))
			}
		}
		if len(sortParts) > 0 {
			orderByClause = "ORDER BY " + strings.Join(sortParts, ", ")
		}
	} else {
		orderByClause = "ORDER BY id ASC"
	}

	limitClause := ""
	if filter.WithPagination {
		limitClause = fmt.Sprintf("LIMIT %d OFFSET %d", filter.Limit, filter.Offset)
	}

	query := fmt.Sprintf(`SELECT %s FROM %s %s %s %s`, departmentFields, departmentTable, whereClause, orderByClause, limitClause)

	rows, err := r.storage.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	departments := make([]dto.DepartmentDTO, 0, filter.Limit)
	for rows.Next() {
		var d dto.DepartmentDTO
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&d.ID, &d.Name, &d.StatusID, &createdAt, &updatedAt); err != nil {
			return nil, 0, err
		}
		d.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
		d.UpdatedAt = updatedAt.Local().Format("2006-01-02 15:04:05")
		departments = append(departments, d)
	}

	return departments, total, rows.Err()
}

func (r *DepartmentRepository) GetDepartmentsWithStats(ctx context.Context, filter types.Filter) ([]dto.DepartmentStatsDTO, uint64, error) {
	whereClause, args := buildWhereClause(filter, "d")

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s AS d %s", departmentTable, whereClause)
	var total uint64
	if err := r.storage.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []dto.DepartmentStatsDTO{}, 0, nil
	}

	orderByClause := "ORDER BY d.id ASC"
	limitClause := ""
	if filter.WithPagination {
		limitClause = fmt.Sprintf("LIMIT %d OFFSET %d", filter.Limit, filter.Offset)
	}

	mainQuery := fmt.Sprintf(`
		SELECT d.id, d.name,
			COUNT(o.id) FILTER (WHERE s.code = 'OPEN') AS open_orders,
			COUNT(o.id) FILTER (WHERE s.code = 'CLOSED') AS closed_orders
		FROM departments AS d
		LEFT JOIN orders AS o ON d.id = o.department_id AND o.deleted_at IS NULL
		LEFT JOIN statuses AS s ON o.status_id = s.id
		%s
		GROUP BY d.id, d.name
		%s %s`, whereClause, orderByClause, limitClause)

	rows, err := r.storage.Query(ctx, mainQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	stats := make([]dto.DepartmentStatsDTO, 0)
	for rows.Next() {
		var s dto.DepartmentStatsDTO
		if err := rows.Scan(&s.ID, &s.Name, &s.OpenOrdersCount, &s.ClosedOrdersCount); err != nil {
			return nil, 0, err
		}
		stats = append(stats, s)
	}

	return stats, total, rows.Err()
}

// ----- ВАШИ СУЩЕСТВУЮЩИЕ МЕТОДЫ ОСТАЛИСЬ НЕИЗМЕННЫМИ -----
func (r *DepartmentRepository) CountDepartments(ctx context.Context, filter types.Filter) (uint64, error) {
	whereClause, args := buildWhereClause(filter, "")
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", departmentTable, whereClause)
	var total uint64
	err := r.storage.QueryRow(ctx, query, args...).Scan(&total)
	return total, err
}

func (r *DepartmentRepository) FindDepartment(ctx context.Context, id uint64) (*dto.DepartmentDTO, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1 AND deleted_at IS NULL", departmentFields, departmentTable)
	row := r.storage.QueryRow(ctx, query, id)
	var dept dto.DepartmentDTO
	var createdAt, updatedAt time.Time
	err := row.Scan(&dept.ID, &dept.Name, &dept.StatusID, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	dept.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
	dept.UpdatedAt = updatedAt.Local().Format("2006-01-02 15:04:05")
	return &dept, err
}

func (r *DepartmentRepository) CreateDepartment(ctx context.Context, createDTO dto.CreateDepartmentDTO) (*dto.DepartmentDTO, error) {
	query := fmt.Sprintf(`INSERT INTO %s (name, status_id) VALUES($1, $2) RETURNING %s`, departmentTable, departmentFields)
	row := r.storage.QueryRow(ctx, query, createDTO.Name, createDTO.StatusID)
	var dept dto.DepartmentDTO
	var createdAt, updatedAt time.Time
	err := row.Scan(&dept.ID, &dept.Name, &dept.StatusID, &createdAt, &updatedAt)
	dept.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
	dept.UpdatedAt = updatedAt.Local().Format("2006-01-02 15:04:05")
	return &dept, err
}

func (r *DepartmentRepository) UpdateDepartment(ctx context.Context, id uint64, updateDTO dto.UpdateDepartmentDTO) (*dto.DepartmentDTO, error) {
	setClauses, args, argID := make([]string, 0), make([]interface{}, 0), 1
	if updateDTO.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argID))
		args = append(args, *updateDTO.Name)
		argID++
	}
	if updateDTO.StatusID != nil {
		setClauses = append(setClauses, fmt.Sprintf("status_id = $%d", argID))
		args = append(args, *updateDTO.StatusID)
		argID++
	}
	if len(setClauses) == 0 {
		return r.FindDepartment(ctx, id)
	}

	setClauses = append(setClauses, "updated_at = NOW()")
	query := fmt.Sprintf(`UPDATE %s SET %s WHERE id = $%d AND deleted_at IS NULL RETURNING %s`, departmentTable, strings.Join(setClauses, ", "), argID, departmentFields)
	args = append(args, id)

	row := r.storage.QueryRow(ctx, query, args...)
	var dept dto.DepartmentDTO
	var createdAt, updatedAt time.Time
	err := row.Scan(&dept.ID, &dept.Name, &dept.StatusID, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	dept.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
	dept.UpdatedAt = updatedAt.Local().Format("2006-01-02 15:04:05")
	return &dept, err
}

func (r *DepartmentRepository) DeleteDepartment(ctx context.Context, id uint64) error {
	query := `UPDATE departments SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *DepartmentRepository) IsManager(ctx context.Context, userID uint64, departmentID uint64) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM department_managers WHERE user_id = $1 AND department_id = $2)`
	var exists bool
	return exists, r.storage.QueryRow(ctx, query, userID, departmentID).Scan(&exists)
}

func (r *DepartmentRepository) FindManagerByID(ctx context.Context, departmentID uint64) (*dto.UserDTO, error) {
	query := `SELECT u.id, u.fio, u.email FROM users u JOIN department_managers dm ON u.id = dm.user_id WHERE dm.department_id = $1 LIMIT 1`
	var user dto.UserDTO
	err := r.storage.QueryRow(ctx, query, departmentID).Scan(&user.ID, &user.Fio, &user.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	return &user, err
}
