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
	userTable        = "users"
	managerTable     = "department_managers"
	departmentFields = "id, name, status_id, created_at, updated_at"
)

var allowedFilterFields = map[string]bool{"status_id": true}
var allowedSortFields = map[string]bool{"id": true, "name": true, "created_at": true, "updated_at": true}

type DepartmentRepositoryInterface interface {
	GetDepartments(ctx context.Context, filter types.Filter) ([]dto.DepartmentDTO, error)
	CountDepartments(ctx context.Context, filter types.Filter) (uint64, error)
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

func buildWhereClause(filter types.Filter) (string, []interface{}) {
	var whereConditions []string
	var args []interface{}
	argId := 1

	if filter.Search != "" {
		whereConditions = append(whereConditions, fmt.Sprintf("name ILIKE $%d", argId))
		args = append(args, "%"+filter.Search+"%")
		argId++
	}

	for field, value := range filter.Filter {
		if allowed, ok := allowedFilterFields[field]; ok && allowed {
			whereConditions = append(whereConditions, fmt.Sprintf("%s = $%d", field, argId))
			args = append(args, value)
			argId++
		}
	}

	if len(whereConditions) == 0 {
		return "", nil
	}

	return "WHERE " + strings.Join(whereConditions, " AND "), args
}

func (r *DepartmentRepository) GetDepartments(ctx context.Context, filter types.Filter) ([]dto.DepartmentDTO, error) {
	whereClause, args := buildWhereClause(filter)

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

	limitClause := fmt.Sprintf("LIMIT %d OFFSET %d", filter.Limit, filter.Offset)

	query := fmt.Sprintf(`
		SELECT %s FROM %s
		%s
		%s
		%s
	`, departmentFields, departmentTable, whereClause, orderByClause, limitClause)

	rows, err := r.storage.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var departments []dto.DepartmentDTO
	for rows.Next() {
		var department dto.DepartmentDTO
		var createdAt, updatedAt time.Time
		err := rows.Scan(&department.ID, &department.Name, &department.StatusID, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}
		department.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
		department.UpdatedAt = updatedAt.Local().Format("2006-01-02 15:04:05")
		departments = append(departments, department)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return departments, nil
}

func (r *DepartmentRepository) CountDepartments(ctx context.Context, filter types.Filter) (uint64, error) {
	whereClause, args := buildWhereClause(filter)
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", departmentTable, whereClause)
	var total uint64
	err := r.storage.QueryRow(ctx, query, args...).Scan(&total)
	return total, err
}

func (r *DepartmentRepository) FindDepartment(ctx context.Context, id uint64) (*dto.DepartmentDTO, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", departmentFields, departmentTable)
	row := r.storage.QueryRow(ctx, query, id)
	var department dto.DepartmentDTO
	var createdAt, updatedAt time.Time
	err := row.Scan(&department.ID, &department.Name, &department.StatusID, &createdAt, &updatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	department.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
	department.UpdatedAt = updatedAt.Local().Format("2006-01-02 15:04:05")
	return &department, nil
}

func (r *DepartmentRepository) CreateDepartment(ctx context.Context, createDTO dto.CreateDepartmentDTO) (*dto.DepartmentDTO, error) {
	query := fmt.Sprintf(`
		INSERT INTO %s (name, status_id) VALUES($1, $2)
		RETURNING %s
	`, departmentTable, departmentFields)

	row := r.storage.QueryRow(ctx, query, createDTO.Name, createDTO.StatusID)

	// Теперь компилятор корректно распознает `dto` как пакет, а не переменную.
	var department dto.DepartmentDTO
	var createdAt, updatedAt time.Time

	err := row.Scan(&department.ID, &department.Name, &department.StatusID, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	department.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
	department.UpdatedAt = updatedAt.Local().Format("2006-01-02 15:04:05")
	return &department, nil
}

// ИСПРАВЛЕНИЕ 2: Параметр переименован в `updateDTO`.
func (r *DepartmentRepository) UpdateDepartment(ctx context.Context, id uint64, updateDTO dto.UpdateDepartmentDTO) (*dto.DepartmentDTO, error) {
	setClauses := make([]string, 0)
	args := make([]interface{}, 0)
	argID := 1

	// Динамически строим SET часть запроса
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

	// Если не пришло ни одного поля для обновления, не делаем запрос
	if len(setClauses) == 0 {
		return r.FindDepartment(ctx, id)
	}

	setClauses = append(setClauses, "updated_at = NOW()")
	setQuery := strings.Join(setClauses, ", ")

	query := fmt.Sprintf(`UPDATE %s SET %s WHERE id = $%d RETURNING %s`,
		departmentTable, setQuery, argID, departmentFields)

	args = append(args, id) // Добавляем ID в конец списка аргументов

	row := r.storage.QueryRow(ctx, query, args...)
	var department dto.DepartmentDTO
	var createdAt, updatedAt time.Time

	err := row.Scan(&department.ID, &department.Name, &department.StatusID, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}

	department.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
	department.UpdatedAt = updatedAt.Local().Format("2006-01-02 15:04:05")

	return &department, nil
}
func (r *DepartmentRepository) DeleteDepartment(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", departmentTable)
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
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE user_id = $1 AND department_id = $2)", managerTable)
	var exists bool
	err := r.storage.QueryRow(ctx, query, userID, departmentID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *DepartmentRepository) FindManagerByID(ctx context.Context, departmentID uint64) (*dto.UserDTO, error) {
	// Этот код уже был исправлен, чтобы соответствовать вашей dto.UserDTO с полем Fio
	query := fmt.Sprintf(`
		SELECT u.id, u.fio, u.email
		FROM %s u
		INNER JOIN %s dm ON u.id = dm.user_id
		WHERE dm.department_id = $1
		LIMIT 1
	`, userTable, managerTable)
	var user dto.UserDTO
	err := r.storage.QueryRow(ctx, query, departmentID).Scan(&user.ID, &user.Fio, &user.Email)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}
