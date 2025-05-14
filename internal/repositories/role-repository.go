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
	ROLE_TABLE  = "roles"
	ROLE_FIELDS = "id, name, description, status_id, created_at"
)

type RoleRepositoryInterface interface {
	GetRoles(ctx context.Context, limit uint64, offset uint64) ([]dto.RoleDTO, error)
	FindRole(ctx context.Context, id uint64) (*dto.RoleDTO, error)
	CreateRole(ctx context.Context, dto dto.CreateRoleDTO) error
	UpdateRole(ctx context.Context, id uint64, dto dto.UpdateRoleDTO) error
	DeleteRole(ctx context.Context, id uint64) error
}

type RoleRepository struct {
	storage *pgxpool.Pool
}

func NewRoleRepository(storage *pgxpool.Pool) RoleRepositoryInterface {
	return &RoleRepository{
		storage: storage,
	}
}

func (r *RoleRepository) GetRoles(ctx context.Context, limit uint64, offset uint64) ([]dto.RoleDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			%s
		FROM %s r
		`, ROLE_FIELDS, ROLE_TABLE)

	rows, err := r.storage.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := make([]dto.RoleDTO, 0)

	for rows.Next() {
		var role dto.RoleDTO
		var createdAt time.Time

		err := rows.Scan(
			&role.ID,
			&role.Name,
			&role.Description,
			&role.StatusID,
			&createdAt,
		)

		if err != nil {
			return nil, err
		}

		role.CreatedAt = createdAt.Format("2006-01-02, 15:04:05")

		roles = append(roles, role)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return roles, nil
}

func (r *RoleRepository) FindRole(ctx context.Context, id uint64) (*dto.RoleDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			%s
		FROM %s r
		WHERE r.id = $1
	`, ROLE_FIELDS, ROLE_TABLE)

	var role dto.RoleDTO
	var createdAt time.Time

	err := r.storage.QueryRow(ctx, query, id).Scan(
		&role.ID,
		&role.Name,
		&role.Description,
		&role.StatusID,
		&createdAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, utils.ErrorNotFound
		}
		return nil, err
	}

	role.CreatedAt = createdAt.Format("2006-01-02, 15:04:05")

	return &role, nil
}

func (r *RoleRepository) CreateRole(ctx context.Context, dto dto.CreateRoleDTO) error {
	query := fmt.Sprintf("INSERT INTO %s (name, description, status_id) VALUES($1, $2, $3)", ROLE_TABLE)

	_, err := r.storage.Exec(ctx, query, dto.Name, dto.Description, dto.StatusID)
	if err != nil {
		return err
	}

	return nil
}

func (r *RoleRepository) UpdateRole(ctx context.Context, id uint64, dto dto.UpdateRoleDTO) error {
	query := fmt.Sprintf("UPDATE %s SET name = $1, description = $2, status_id = $3 WHERE id = $4", ROLE_TABLE)

	result, err := r.storage.Exec(ctx, query, dto.Name, dto.Description, dto.StatusID, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return utils.ErrorNotFound
	}

	return nil
}

func (r *RoleRepository) DeleteRole(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", ROLE_TABLE)

	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return utils.ErrorNotFound
	}

	return nil
}