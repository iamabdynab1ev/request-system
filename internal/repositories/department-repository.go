package repositories

import (
	"context"
	"fmt"
	"request-system/internal/dto"
	"request-system/pkg/utils"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	DEPARTMENT_TABLE  = "departments"
	DEPARTMENT_FIELDS = "id, name, status_id, created_at, updated_at"
)

type DepartmentRepositoryInterface interface {
	GetDepartments(ctx context.Context, limit uint64, offset uint64) ([]dto.DepartmentDTO, error)
	FindDepartment(ctx context.Context, id uint64) (*dto.DepartmentDTO, error)
	CreateDepartment(ctx context.Context, dto dto.CreateDepartmentDTO) error
	UpdateDepartment(ctx context.Context, id uint64, dto dto.UpdateDepartmentDTO) error
	DeleteDepartment(ctx context.Context, id uint64) error
}

type DepartmentRepository struct{
	storage *pgxpool.Pool
}

func NewDepartmentRepository(storage *pgxpool.Pool) DepartmentRepositoryInterface {

	return &DepartmentRepository{
		storage: storage,
	}
}

func (r *DepartmentRepository) GetDepartments(ctx context.Context, limit uint64, offset uint64) ([]dto.DepartmentDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			%s
		FROM %s r
		`, DEPARTMENT_FIELDS, DEPARTMENT_TABLE)

	rows, err := r.storage.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	departments := make([]dto.DepartmentDTO, 0)

	for rows.Next() {
		var department dto.DepartmentDTO
		var createdAt time.Time
		var updatedAt time.Time

		err := rows.Scan(
			&department.ID,
			&department.Name,
			&department.StatusID,
			&createdAt,
			&updatedAt,
		)

		if err != nil {
			return nil, err
		}

		createdAtLocal := createdAt.Local()
        updatedAtLocal := updatedAt.Local()

		department.CreatedAt = createdAtLocal.Format("2006-01-02 15:04:05")
		department.UpdatedAt = updatedAtLocal.Format("2006-01-02 15:04:05")


		departments = append(departments, department)
	}

	if err:= rows.Err(); err != nil {
		return nil, err
	}
	return departments, nil
}

func (r *DepartmentRepository) FindDepartment(ctx context.Context, id uint64) (*dto.DepartmentDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			%s
		FROM %s r
		WHERE r.id = $1
	`, DEPARTMENT_FIELDS, DEPARTMENT_TABLE)

	var department dto.DepartmentDTO
	var createdAt time.Time
	var updatedAt time.Time

	err := r.storage.QueryRow(ctx, query, id).Scan(
		&department.ID,
		&department.Name,
		&department.StatusID,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, utils.ErrorNotFound
		}
		return nil, err
	}

		createdAtLocal := createdAt.Local()
        updatedAtLocal := updatedAt.Local()

		department.CreatedAt = createdAtLocal.Format("2006-01-02 15:04:05")
		department.UpdatedAt = updatedAtLocal.Format("2006-01-02 15:04:05")


	return &department, nil
}

func (r *DepartmentRepository) CreateDepartment(ctx context.Context, dto dto.CreateDepartmentDTO) error {
	query := fmt.Sprintf("INSERT INTO %s (name, status_id) VALUES($1, $2)", DEPARTMENT_TABLE)

	_, err := r.storage.Exec(ctx, query,
		dto.Name,
		dto.StatusID,
	)

	if err != nil {
		return err
	}
	return nil
}

func (r *DepartmentRepository) UpdateDepartment(ctx context.Context, id uint64, dto dto.UpdateDepartmentDTO) error {
	query := fmt.Sprintf("UPDATE %s SET name = $1, status_id = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3", DEPARTMENT_TABLE)

	result, err := r.storage.Exec(ctx, query,
		dto.Name,
		dto.StatusID,
		id,
	)

	if err != nil {

		return err
	}

	if result.RowsAffected() == 0 {
		return utils.ErrorNotFound
	}
	return nil
}

func (r *DepartmentRepository) DeleteDepartment(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", DEPARTMENT_TABLE)

	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return utils.ErrorNotFound
	}

	return nil
}