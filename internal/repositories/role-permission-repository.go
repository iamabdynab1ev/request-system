package repositories

import (
	"context"
	"fmt"
	"request-system/internal/dto"
	"request-system/pkg/utils"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const rolePermissionTable = "role_permission"
const rolePermissionFields = "id, role_id, permission_id"

type RolePermissionRepositoryInterface interface {
	GetRolePermissions(ctx context.Context, limit uint64, offset uint64) ([]dto.RolePermissionDTO, error)
	FindRolePermission(ctx context.Context, id uint64) (*dto.RolePermissionDTO, error)
	CreateRolePermission(ctx context.Context, dto dto.CreateRolePermissionDTO) error
	UpdateRolePermission(ctx context.Context, id uint64, dto dto.UpdateRolePermissionDTO) error
	DeleteRolePermission(ctx context.Context, id uint64) error
}

type RolePermissionRepository struct {
	storage *pgxpool.Pool
}

func NewRolePermissionRepository(storage *pgxpool.Pool) RolePermissionRepositoryInterface {

	return &RolePermissionRepository{
		storage: storage,
	}
}

func (r *RolePermissionRepository) GetRolePermissions(ctx context.Context, limit uint64, offset uint64) ([]dto.RolePermissionDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			%s
		FROM %s r
		`, rolePermissionFields, rolePermissionTable)

	rows, err := r.storage.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	rolePermissions := make([]dto.RolePermissionDTO, 0)

	for rows.Next() {
		var rp dto.RolePermissionDTO

		err := rows.Scan(
			&rp.ID,
			&rp.RoleID,
			&rp.PermissionID,
		)

		if err != nil {
			return nil, err
		}

		rolePermissions = append(rolePermissions, rp)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rolePermissions, nil
}

func (r *RolePermissionRepository) FindRolePermission(ctx context.Context, id uint64) (*dto.RolePermissionDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			%s
		FROM %s r
		WHERE r.id = $1
	`, rolePermissionFields, rolePermissionTable)

	var rp dto.RolePermissionDTO

	err := r.storage.QueryRow(ctx, query, id).Scan(
		&rp.ID,
		&rp.RoleID,
		&rp.PermissionID,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, utils.ErrorNotFound
		}
		return nil, err
	}

	return &rp, nil
}

func (r *RolePermissionRepository) CreateRolePermission(ctx context.Context, dto dto.CreateRolePermissionDTO) error {
	query := fmt.Sprintf(`
        INSERT INTO %s (role_id, permission_id)
        VALUES ($1, $2)
    `, rolePermissionTable)

	_, err := r.storage.Exec(ctx, query,
		dto.RoleID,
		dto.PermissionID,
	)

	if err != nil {
		return err
	}
	return nil
}

func (r *RolePermissionRepository) UpdateRolePermission(ctx context.Context, id uint64, dto dto.UpdateRolePermissionDTO) error {
	query := fmt.Sprintf(`
        UPDATE %s
        SET role_id = $1, permission_id = $2
        WHERE id = $3
    `, rolePermissionTable)

	result, err := r.storage.Exec(ctx, query,
		dto.RoleID,
		dto.PermissionID,
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

func (r *RolePermissionRepository) DeleteRolePermission(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", rolePermissionTable)

	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return utils.ErrorNotFound
	}

	return nil
}
