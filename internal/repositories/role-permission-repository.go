package repositories

import (
	"context"
	"errors"
	"fmt"

	// <-- ЭТА СТРОКА ДОЛЖНА БЫТЬ ТОЧНО ТУТ
	"request-system/internal/dto"
	apperrors "request-system/pkg/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ИСПРАВЛЕНО на правильное имя таблицы из DDL.
const rolePermissionTable = "role_permissions"

// Добавлены id, created_at, updated_at в список полей для RETURNING и SELECT.
const rolePermissionFields = "id, role_id, permission_id, created_at, updated_at"

type RolePermissionRepositoryInterface interface {
	GetRolePermissions(ctx context.Context, limit uint64, offset uint64) ([]dto.RolePermissionDTO, uint64, error)
	FindRolePermission(ctx context.Context, id uint64) (*dto.RolePermissionDTO, error)
	CreateRolePermission(ctx context.Context, in dto.CreateRolePermissionDTO) (*dto.RolePermissionDTO, error)
	UpdateRolePermission(ctx context.Context, id uint64, in dto.UpdateRolePermissionDTO) (*dto.RolePermissionDTO, error)
	DeleteRolePermission(ctx context.Context, id uint64) error
}

type RolePermissionRepository struct {
	storage *pgxpool.Pool
}

func NewRolePermissionRepository(storage *pgxpool.Pool) RolePermissionRepositoryInterface {
	return &RolePermissionRepository{storage: storage}
}

func (r *RolePermissionRepository) GetRolePermissions(ctx context.Context, limit uint64, offset uint64) ([]dto.RolePermissionDTO, uint64, error) {
	var total uint64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", rolePermissionTable)
	if err := r.storage.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("ошибка подсчета role_permissions: %w", err)
	}

	query := fmt.Sprintf(`SELECT %s FROM %s ORDER BY id LIMIT $1 OFFSET $2`, rolePermissionFields, rolePermissionTable)
	rows, err := r.storage.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка запроса role_permissions: %w", err)
	}
	defer rows.Close()

	var rolePermissions []dto.RolePermissionDTO
	for rows.Next() {
		var rp dto.RolePermissionDTO
		if err := rows.Scan(
			&rp.ID,
			&rp.RoleID,
			&rp.PermissionID,
			&rp.CreatedAt,
			&rp.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("ошибка сканирования role_permission: %w", err)
		}
		rolePermissions = append(rolePermissions, rp)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return rolePermissions, total, nil
}

func (r *RolePermissionRepository) FindRolePermission(ctx context.Context, id uint64) (*dto.RolePermissionDTO, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM %s r
		WHERE r.id = $1
	`, rolePermissionFields, rolePermissionTable)

	var rp dto.RolePermissionDTO
	if err := r.storage.QueryRow(ctx, query, id).Scan(
		&rp.ID,
		&rp.RoleID,
		&rp.PermissionID,
		&rp.CreatedAt,
		&rp.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &rp, nil
}

func (r *RolePermissionRepository) CreateRolePermission(ctx context.Context, in dto.CreateRolePermissionDTO) (*dto.RolePermissionDTO, error) {
	query := fmt.Sprintf(`
        INSERT INTO %s (role_id, permission_id)
        VALUES ($1, $2)
        RETURNING %s
    `, rolePermissionTable, rolePermissionFields)

	var createdRP dto.RolePermissionDTO
	if err := r.storage.QueryRow(ctx, query,
		in.RoleID,
		in.PermissionID,
	).Scan(
		&createdRP.ID,
		&createdRP.RoleID,
		&createdRP.PermissionID,
		&createdRP.CreatedAt,
		&createdRP.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("ошибка при создании role_permission: %w", err)
	}
	return &createdRP, nil
}

func (r *RolePermissionRepository) UpdateRolePermission(ctx context.Context, id uint64, in dto.UpdateRolePermissionDTO) (*dto.RolePermissionDTO, error) {
	query := fmt.Sprintf(`
        UPDATE %s
        SET role_id = $1, permission_id = $2, updated_at = CURRENT_TIMESTAMP
        WHERE id = $3
        RETURNING %s
    `, rolePermissionTable, rolePermissionFields)

	var updatedRP dto.RolePermissionDTO
	row := r.storage.QueryRow(ctx, query,
		in.RoleID,
		in.PermissionID,
		id,
	)
	if err := row.Scan(
		&updatedRP.ID,
		&updatedRP.RoleID,
		&updatedRP.PermissionID,
		&updatedRP.CreatedAt,
		&updatedRP.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("ошибка при обновлении role_permission: %w", err)
	}
	return &updatedRP, nil
}

func (r *RolePermissionRepository) DeleteRolePermission(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", rolePermissionTable)
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
